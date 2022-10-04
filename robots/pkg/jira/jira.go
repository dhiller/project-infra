/*
 * This file is part of the KubeVirt project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright 2022 Red Hat, Inc.
 *
 */

package jira

import (
	"context"
	"fmt"
	gojira "github.com/andygrunwald/go-jira"
	"golang.org/x/oauth2"
	"io"
	"k8s.io/test-infra/prow/config/secret"
	"strings"
	"time"
)

type JiraComponentMapper struct {
	mapping []*IssueComponentMapping
}

func (j JiraComponentMapper) MapObservedFailureToComponentMappings(o *ObservedFailure) []*IssueComponentMapping {
	var results []*IssueComponentMapping
	for _, mappingEntry := range j.mapping {
		if mappingEntry.Matcher.MatchString(o.Test.Name) {
			results = append(results, mappingEntry)
		}
	}
	return results
}

type JiraHandlerConfiguration struct {
	ProjectKey          string
	ClientID            string
	ClientTokenPath     string
	Context             context.Context
	DefaultLabels       []string
	Type                string
	JiraComponentMapper *JiraComponentMapper
	BaseURL             string
}

type JiraHandler struct {
	client *gojira.Client
	config *JiraHandlerConfiguration
}

func NewJiraHandler(config *JiraHandlerConfiguration) (*JiraHandler, error) {
	jiraClient, err := newJiraClient(config.ClientTokenPath, config.BaseURL, config.Context)
	if err != nil {
		return nil, err
	}
	return &JiraHandler{
		client: jiraClient,
		config: config,
	}, nil
}

func newJiraClient(clientTokenPath, baseURL string, context context.Context) (*gojira.Client, error) {
	err := secret.Add(clientTokenPath)
	if err != nil {
		return nil, err
	}
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: string(secret.GetSecret(clientTokenPath))},
	)
	httpClient := oauth2.NewClient(context, ts)
	return gojira.NewClient(httpClient, baseURL)
}

func (j *JiraHandler) ConvergeIssueFor(o *ObservedFailure) (*Issue, error) {
	workingIssue := j.newIssue(o)

	var existingIssue *Issue
	existingIssue, err := j.searchForExisting(workingIssue)
	if err != nil {
		return nil, err
	}
	if existingIssue != nil {
		j.reopen(existingIssue)
		workingIssue, err = existingIssue.Merge(workingIssue)
		if err != nil {
			return nil, err
		}
	}

	err = j.persist(workingIssue)
	if err != nil {
		return nil, err
	}
	return workingIssue, nil
}

func (j *JiraHandler) DefaultLabels() []string {
	return j.config.DefaultLabels
}

func (j *JiraHandler) Type() string {
	return j.config.Type
}

func (j *JiraHandler) newIssue(o *ObservedFailure) *Issue {
	var components []string
	for _, mapping := range j.config.JiraComponentMapper.MapObservedFailureToComponentMappings(o) {
		components = append(components, mapping.To)
	}
	return &Issue{
		Summary: o.Test.Name,
		Description: fmt.Sprintf(`%s

%s`, o.Test.Message, o.Test.SystemOut),
		Labels:     j.DefaultLabels(),
		Components: components,
		Links:      []string{o.BuildURL},
		Type:       j.Type(),
	}
}

// searchForExisting finds an existing issue matching the input
func (j *JiraHandler) searchForExisting(issue *Issue) (*Issue, error) {
	// label for ds + test_id or test description
	// first one ordered by updated-desc

	jql := fmt.Sprintf("project = %s and summary ~ %q and labels IN (%q) ORDER BY updated DESC", j.config.ProjectKey, issue.Summary, strings.Join(j.config.DefaultLabels, "\", \""))
	searchOptions := &gojira.SearchOptions{MaxResults: 1}
	foundIssues, response, err := j.client.Issue.SearchWithContext(j.config.Context, jql, searchOptions)
	if err != nil {
		return nil, err
	}
	if response.Total == 0 {
		return nil, nil
	}
	existingIssue, err := j.convertToIssue(foundIssues[0])
	return existingIssue, err
}

func (j *JiraHandler) convertToIssue(issue gojira.Issue) (*Issue, error) {
	remoteLinks, _, err := j.client.Issue.GetRemoteLinks(issue.ID)
	if err != nil {
		return nil, err
	}
	var links []string
	if len(*remoteLinks) > 0 {
		links = j.convertToLinks(remoteLinks)
	}
	result := &Issue{
		Summary:     issue.Fields.Summary,
		Description: issue.Fields.Description,
		Labels:      issue.Fields.Labels,
		Components:  j.convertToComponents(&issue),
		Links:       links,
		Type:        j.convertToType(&issue),
		Identity: &IssueTrackerIdentity{
			ID:  issue.ID,
			Key: issue.Key,
		},
		Status: issue.Fields.Status.Name,
	}
	return result, nil
}

func (j *JiraHandler) convertToLinks(links *[]gojira.RemoteLink) []string {
	var result []string
	for _, link := range *links {
		result = append(result, link.Object.URL)
	}
	return result
}

func (j *JiraHandler) convertToComponents(issue *gojira.Issue) []string {
	var result []string
	if issue.Fields != nil && issue.Fields.Components != nil {
		for _, component := range issue.Fields.Components {
			result = append(result, component.Name)
		}
	}
	return result
}

func (j *JiraHandler) convertToType(issue *gojira.Issue) string {
	if issue == nil || issue.Fields == nil {
		return ""
	}
	return issue.Fields.Type.Name
}

var statusTransitions = map[string]string{
	"To Do":       "To Do",
	"Obsolete":    "To Do",
	"QE Review":   "To Do",
	"Code Review": "To Do",
	"Done":        "To Do",
	"In Progress": "In Progress",
}

func (j *JiraHandler) reopen(issue *Issue) {
	issue.Status = statusTransitions[issue.Status]
}

func (j *JiraHandler) persist(issue *Issue) error {
	project, _, err := j.client.Project.Get(j.config.ProjectKey)
	if err != nil {
		return fmt.Errorf("couldn't fetch jira project by ID %s! %v", j.config.ProjectKey, err)
	}

	var components []*gojira.Component
	for _, component := range issue.Components {
		components = append(components, &gojira.Component{
			Name: component,
		})
	}

	var remoteLinks []*gojira.RemoteLink
	for _, remoteLink := range issue.Links {
		remoteLinks = append(remoteLinks, &gojira.RemoteLink{
			Object: &gojira.RemoteLinkObject{
				URL: remoteLink,
			},
		})
	}

	jiraIssue := &gojira.Issue{
		Fields: &gojira.IssueFields{
			Project:     *project,
			Summary:     issue.Summary,
			Description: issue.Description,
			Labels:      issue.Labels,
			Components:  components,
			Type: gojira.IssueType{
				Name: issue.Type,
			},
		},
	}
	if issue.Identity != nil {
		jiraIssue.ID = issue.Identity.ID
		jiraIssue.Key = issue.Identity.Key
		_, response, err := j.client.Issue.Update(jiraIssue)
		if err != nil {
			return j.newPersistenceErrorFromResponseBody("error updating issue", response, err)
		}
	} else {
		result, response, err := j.client.Issue.Create(jiraIssue)
		if err != nil {
			return j.newPersistenceErrorFromResponseBody("error creating issue", response, err)
		}
		issue.Identity = &IssueTrackerIdentity{
			ID:  result.ID,
			Key: result.Key,
		}
		jiraIssue.ID = result.ID
		jiraIssue.Key = result.Key
	}

	if issue.Status != "" {
		err2 := j.persistStatusTransition(issue, jiraIssue)
		if err2 != nil {
			return err2
		}
	}

	return j.appendNewRemoteLinks(issue, remoteLinks)
}

func (j *JiraHandler) appendNewRemoteLinks(issue *Issue, remoteLinks []*gojira.RemoteLink) error {
	if len(issue.Links) == 0 {
		return nil
	}

	existingLinks, response, err := j.client.Issue.GetRemoteLinks(issue.Identity.ID)
	if err != nil {
		return j.newPersistenceErrorFromResponseBody("error fetching existing remote links", response, err)
	}
	existingLinksDictionary := map[string]struct{}{}
	for _, existingLink := range *existingLinks {
		existingLinksDictionary[existingLink.Object.URL] = struct{}{}
	}
	for _, remoteLink := range remoteLinks {
		if _, found := existingLinksDictionary[remoteLink.Object.URL]; !found {
			remoteLink.Object.Title = fmt.Sprintf("Failed build from %s", time.Now().Format(time.RFC1123))
			_, response, err := j.client.Issue.AddRemoteLink(issue.Identity.ID, remoteLink)
			if err != nil {
				return j.newPersistenceErrorFromResponseBody("error adding remote link", response, err)
			}
			issue.Links = append(issue.Links, remoteLink.Object.URL)
		}
	}
	return nil
}

func (j *JiraHandler) persistStatusTransition(issue *Issue, jiraIssue *gojira.Issue) error {
	errorDescription := "error transitioning issue status"

	// fetch possible status transitions
	transitions, response, err := j.client.Issue.GetTransitions(jiraIssue.ID)
	if err != nil {
		return j.newPersistenceErrorFromResponseBody(errorDescription, response, err)
	}
	var foundTransition *gojira.Transition
	for _, transition := range transitions {
		if transition.To.Name == issue.Status {
			foundTransition = &transition
			break
		}
	}

	// do status transition if possible transition found
	if foundTransition != nil {
		response, err := j.client.Issue.DoTransition(jiraIssue.ID, foundTransition.ID)
		if err != nil {
			return j.newPersistenceErrorFromResponseBody(errorDescription, response, err)
		}
	} else {
		return fmt.Errorf("error transitioning issue status: not found %s, possible values %v", issue.Status, transitions)
	}
	return nil
}

func (j *JiraHandler) newPersistenceErrorFromResponseBody(errorDescription string, response *gojira.Response, err error) error {
	body, _ := io.ReadAll(response.Body)
	defer response.Body.Close()
	return fmt.Errorf("%s: %v (%v)", errorDescription, err, string(body))
}

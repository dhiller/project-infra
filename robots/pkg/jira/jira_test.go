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
	"io"
	"reflect"
	"testing"
)

func TestNewJiraHandler(t *testing.T) {
	type args struct {
		config *JiraHandlerConfiguration
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "no config regarding auth",
			args: args{
				config: &JiraHandlerConfiguration{},
			},
			wantErr: true,
		},
		{
			name: "should create instance",
			args: args{
				config: &JiraHandlerConfiguration{
					ClientID:        jiraClientId,
					ClientTokenPath: jiraTokenPath,
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if skipIfClientIdOrTokenPathUndefined(t) {
				return
			}
			got, err := NewJiraHandler(tt.args.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewJiraHandler() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && got == nil {
				t.Errorf("NewJiraHandler() didn't create an instance")
			}
		})
	}
}

func TestJiraHandler_searchForExisting(t *testing.T) {
	if skipIfClientIdOrTokenPathUndefined(t) {
		return
	}

	testHelper := TestJiraHandler{}
	if !testHelper.setUp(t, defaultKubeVirtProjectKey) {
		return
	}
	defer testHelper.tearDown(t)

	handler, err := NewJiraHandler(NewKubeVirtJiraHandlerConfiguration(jiraURL, jiraClientId, jiraTokenPath))
	if err != nil {
		t.Errorf("couldn't create handler! %v", err)
		return
	}
	type args struct {
		issue *Issue
	}
	tests := []struct {
		name    string
		args    args
		want    *Issue
		wantErr bool
	}{
		{
			name: "finds nothing",
			args: args{
				&Issue{
					Summary:     "non existing issue",
					Description: "blah",
					Labels:      []string{defaultKubeVirtIssueLabel},
					Components:  nil,
					Links:       nil,
					Type:        "",
				},
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "finds an existing issue",
			args: args{
				&Issue{
					Summary: "test CNVQuarantine",
					Labels:  []string{defaultKubeVirtIssueLabel},
					Type:    "Story",
				},
			},
			want: &Issue{
				Summary: "test CNVQuarantine",
				Labels:  []string{defaultKubeVirtIssueLabel},
				Components: []string{
					cnvVirtualization.To,
				},
				Type:   "Story",
				Status: "New",
				Identity: &IssueTrackerIdentity{
					ID:  testHelper.createdIssue.ID,
					Key: testHelper.createdIssue.Key,
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := handler.searchForExisting(tt.args.issue)
			if (err != nil) != tt.wantErr {
				t.Errorf("searchForExisting() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("searchForExisting() got = %v, want %v", got, tt.want)
			}
		})
	}

}

type TestJiraHandler struct {
	client       *gojira.Client
	createdIssue *gojira.Issue
}

// setUp create issue that we want to find later
func (h *TestJiraHandler) setUp(t *testing.T, projectKey string) bool {
	err := h.setUpClientOnly(t)
	if err != nil {
		t.Errorf("couldn't create test jira client! %v", err)
		return false
	}

	project, _, err := h.client.Project.Get(projectKey)
	if err != nil {
		t.Errorf("couldn't fetch jira project by ID %s! %v", projectKey, err)
		return false
	}

	user, response, err := h.client.User.GetSelf()
	if err != nil {
		h.errorWithResponse(t, response, err)
		return false
	}
	t.Logf("user %q", user.Name)

	createdIssue, response, err := h.client.Issue.Create(&gojira.Issue{
		Fields: &gojira.IssueFields{
			Project: *project,
			Summary: "test CNVQuarantine",
			Labels:  []string{defaultKubeVirtIssueLabel},
			Components: []*gojira.Component{
				{
					Name: cnvVirtualization.To,
				},
			},
			Type: gojira.IssueType{
				Name: "Story",
			},
		},
	})
	if err != nil {
		h.errorWithResponse(t, response, err)
		return false
	}
	h.createdIssue = createdIssue

	return true
}

func (h *TestJiraHandler) errorWithResponse(t *testing.T, response *gojira.Response, err error) {
	body := response.Body
	defer body.Close()
	bytes, err2 := io.ReadAll(body)
	if err2 != nil {
		panic(err2)
	}
	t.Errorf("couldn't perform jira acction: %v\n%s", err, string(bytes))
}

func (h *TestJiraHandler) setUpClientOnly(t *testing.T) error {
	client, err := newJiraClient(jiraTokenPath, jiraURL, context.Background())
	if err != nil {
		return err
	}
	h.client = client
	return nil
}

// tearDown remove stuff
func (h *TestJiraHandler) tearDown(t *testing.T) {
	if h.createdIssue == nil {
		return
	}
	response, err := h.client.Issue.Delete(h.createdIssue.ID)
	if err != nil {
		t.Logf("couldn't remove test issue %s: %v \n %v", h.createdIssue.ID, err, response)
	}
}

func (h *TestJiraHandler) getStatusMap() (map[string]*gojira.Status, error) {
	statuses, response, err := h.client.Status.GetAllStatuses()
	if err != nil {
		return nil, fmt.Errorf("failed to get statuses: %v (%v)", err, response)
	}
	statusNameToStatus := make(map[string]*gojira.Status)
	for _, status := range statuses {
		statusNameToStatus[status.Name] = &status
	}
	return statusNameToStatus, nil
}

func skipIfClientIdOrTokenPathUndefined(t *testing.T) bool {
	if jiraURL == "" || jiraClientId == "" || jiraTokenPath == "" {
		t.Skip("No url, client id or token path provided")
		return true
	}
	return false
}

func TestJiraHandler_reopen(t *testing.T) {
	testHelper := TestJiraHandler{}
	if !testHelper.setUp(t, defaultKubeVirtProjectKey) {
		return
	}
	defer testHelper.tearDown(t)

	tests := []struct {
		name        string
		issueStatus string
		wantStatus  string
	}{

		{
			name:        "todo",
			issueStatus: "To Do",
			wantStatus:  "To Do",
		},
		{
			name:        "no change if in progress",
			issueStatus: "In Progress",
			wantStatus:  "In Progress",
		},
		{
			name:        "obsolete",
			issueStatus: "Obsolete",
			wantStatus:  "To Do",
		},
		{
			name:        "QE Review",
			issueStatus: "QE Review",
			wantStatus:  "To Do",
		},
		{
			name:        "Code Review",
			issueStatus: "Code Review",
			wantStatus:  "To Do",
		},
		{
			name:        "Done",
			issueStatus: "Done",
			wantStatus:  "To Do",
		},
	}

	j := &JiraHandler{
		client: testHelper.client,
		config: NewKubeVirtJiraHandlerConfiguration(jiraURL, jiraClientId, jiraTokenPath),
	}
	statusNameToStatus, err := testHelper.getStatusMap()
	if err != nil {
		t.Errorf("reopen(): error %v", err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issue := &Issue{
				Status: tt.issueStatus,
			}
			j.reopen(issue)
			if tt.wantStatus != issue.Status {
				t.Errorf("reopen(): got status %q, want %q", issue.Status, tt.wantStatus)
			}
			if _, exists := statusNameToStatus[issue.Status]; !exists {
				t.Errorf("reopen(): status %q does not exist in %v", issue.Status, statusNameToStatus)
			}
		})
	}
}

func TestJiraHandler_persist(t *testing.T) {
	if skipIfClientIdOrTokenPathUndefined(t) {
		return
	}

	testHelper := TestJiraHandler{}
	if !testHelper.setUp(t, defaultKubeVirtProjectKey) {
		return
	}
	defer testHelper.tearDown(t)

	type args struct {
		issue *Issue
	}
	tests := []struct {
		name    string
		args    args
		want    *Issue
		wantErr bool
	}{
		{
			name: "persist new issue (w/o status)",
			args: args{
				issue: &Issue{
					Summary:     "meh summary",
					Description: "Test description",
					Labels: []string{
						defaultKubeVirtIssueLabel,
						"blah",
					},
					Components: []string{
						cnvVirtualization.To,
					},
					Links:    []string{"http://example.org"},
					Type:     "Story",
					Identity: nil,
				},
			},
			want: &Issue{
				Summary:     "meh summary",
				Description: "Test description",
				Labels: []string{
					defaultKubeVirtIssueLabel,
					"blah",
				},
				Components: []string{
					cnvVirtualization.To,
				},
				Links:  []string{"http://example.org"},
				Type:   "Story",
				Status: "New",
			},
			wantErr: false,
		},
		{
			name: "persist new issue (with status)",
			args: args{
				issue: &Issue{
					Summary:     "meh summary",
					Description: "Test description",
					Labels: []string{
						defaultKubeVirtIssueLabel,
						"blah",
					},
					Components: []string{
						cnvVirtualization.To,
					},
					Links:    []string{"http://example.org"},
					Type:     "Story",
					Status:   "In Progress",
					Identity: nil,
				},
			},
			want: &Issue{
				Summary:     "meh summary",
				Description: "Test description",
				Labels: []string{
					defaultKubeVirtIssueLabel,
					"blah",
				},
				Components: []string{
					cnvVirtualization.To,
				},
				Links:  []string{"http://example.org"},
				Type:   "Story",
				Status: "In Progress",
			},
			wantErr: false,
		},
		{
			name: "update existing issue",
			args: args{
				issue: &Issue{
					Summary:     "meh summary",
					Description: "Test description",
					Labels: []string{
						defaultKubeVirtIssueLabel,
						"blah",
					},
					Components: []string{
						cnvVirtualization.To,
					},
					Links:  []string{"http://example.org"},
					Type:   "Story",
					Status: "In Progress",
					Identity: &IssueTrackerIdentity{
						ID:  testHelper.createdIssue.ID,
						Key: testHelper.createdIssue.Key,
					},
				},
			},
			want: &Issue{
				Summary:     "meh summary",
				Description: "Test description",
				Labels: []string{
					defaultKubeVirtIssueLabel,
					"blah",
				},
				Components: []string{
					cnvVirtualization.To,
				},
				Links:  []string{"http://example.org"},
				Type:   "Story",
				Status: "In Progress",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := &JiraHandler{
				client: testHelper.client,
				config: NewKubeVirtJiraHandlerConfiguration(jiraURL, jiraClientId, jiraTokenPath),
			}
			err := j.persist(tt.args.issue)
			if (err != nil) != tt.wantErr {
				t.Errorf("persist() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				existing, err := j.searchForExisting(tt.args.issue)
				if err != nil {
					t.Errorf("searchForExisting() error = %v, issue %v", err, tt.args.issue)
					return
				}
				if !issuesAreEqual(existing, tt.want) {
					t.Errorf("persist() got = %v, want %v", existing, tt.want)
				}
			}
		})
	}
}

func issuesAreEqual(i1, i2 *Issue) bool {
	return reflect.DeepEqual(i1.Summary, i2.Summary) &&
		reflect.DeepEqual(i1.Description, i2.Description) &&
		reflect.DeepEqual(i1.Labels, i2.Labels) &&
		reflect.DeepEqual(i1.Components, i2.Components) &&
		reflect.DeepEqual(i1.Links, i2.Links) &&
		reflect.DeepEqual(i1.Type, i2.Type) &&
		reflect.DeepEqual(i1.Status, i2.Status)
}

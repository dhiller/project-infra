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
	"fmt"
	"github.com/joshdk/go-junit"
	"sort"
)

type IssueTrackerIdentity struct {
	ID  string
	Key string
}

// Issue represents the current state inside the tracker.
type Issue struct {
	Summary     string
	Description string
	Labels      []string
	Components  []string
	Links       []string
	Type        string
	Status      string
	Identity    *IssueTrackerIdentity
}

// Merge merges the data of the given issue with this issue. The given issue must not have been persisted before.
func (i Issue) Merge(issue *Issue) (*Issue, error) {
	if issue.Identity != nil {
		return nil, fmt.Errorf("Can't merge with another existing issue!")
	}
	result := &Issue{
		Summary:     i.Summary,
		Components:  mergeUniqueValues(issue.Components, i.Components),
		Description: i.Description,
		Labels:      mergeUniqueValues(issue.Labels, i.Labels),
		Links:       append(i.Links, issue.Links...),
		Status:      i.Status,
		Type:        i.Type,
		Identity:    i.Identity,
	}
	if issue.Description != "" {
		result.Description += "\n\n" + issue.Description
	}
	return result, nil
}

func mergeUniqueValues(a, b []string) []string {
	labelsMap := map[string]struct{}{}
	for _, label := range a {
		labelsMap[label] = struct{}{}
	}
	for _, label := range b {
		labelsMap[label] = struct{}{}
	}
	var labels []string
	for label, _ := range labelsMap {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	return labels
}

// ObservedFailure is a test failure seen on the current test run.
type ObservedFailure struct {
	Test                junit.Test
	BuildURL            string
	IssueTrackerHandler IssueTrackerHandler
}

// IssueTrackerHandler is responsible for converging ObservedFailure instances with existing Issue s.
type IssueTrackerHandler interface {

	// ConvergeIssueFor converges the issue in the bugtracker with the data of the observed failure.
	// Issues are either updated (and reopened if necessary) or created using the data that is observed.
	ConvergeIssueFor(o *ObservedFailure) (*Issue, error)

	// DefaultLabels defines the labels that the issues regarded by this handler should have.
	// New issues are created with these, while existing issues are filtered by these.
	DefaultLabels() []string

	// Type refers to the type of issue in the bug tracker
	Type() string
}

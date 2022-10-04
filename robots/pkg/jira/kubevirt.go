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
)

var (
	defaultKubeVirtIssueLabel = "KVQuarantine"
	defaultKubeVirtIssueType  = "Story"
	defaultKubeVirtProjectKey = "CNV"

	defaultKubeVirtJiraHandlerConfiguration = &JiraHandlerConfiguration{
		DefaultLabels:       []string{defaultKubeVirtIssueLabel},
		Type:                defaultKubeVirtIssueType,
		JiraComponentMapper: &CNVJiraComponentMapper,
	}
)

func NewKubeVirtJiraHandlerConfiguration(baseURL, clientId, clientTokenPath string) *JiraHandlerConfiguration {
	return &JiraHandlerConfiguration{
		ProjectKey:          defaultKubeVirtProjectKey,
		ClientID:            clientId,
		ClientTokenPath:     clientTokenPath,
		Context:             context.Background(),
		DefaultLabels:       defaultKubeVirtJiraHandlerConfiguration.DefaultLabels,
		Type:                defaultKubeVirtJiraHandlerConfiguration.Type,
		JiraComponentMapper: defaultKubeVirtJiraHandlerConfiguration.JiraComponentMapper,
		BaseURL:             baseURL,
	}
}

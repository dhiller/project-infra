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

import "regexp"

// IssueComponentMapping defines how a failure maps to a component.
type IssueComponentMapping struct {

	// Matcher has the regular expression that is applied to the test name in order to check whether the test maps to
	// the component To
	Matcher *regexp.Regexp

	// To is the name of the component
	To string
}

var (
	cnvVirtualization = IssueComponentMapping{regexp.MustCompile("sig-compute"), "CNV Virtualization"}
	cnvNetwork        = IssueComponentMapping{regexp.MustCompile("sig-network"), "CNV Network"}
	cnvStorage        = IssueComponentMapping{regexp.MustCompile("sig-storage"), "CNV Storage"}
	cnvMapping        = []*IssueComponentMapping{
		&cnvVirtualization,
		&cnvNetwork,
		&cnvStorage,
	}
	CNVJiraComponentMapper = JiraComponentMapper{
		mapping: cnvMapping,
	}
)

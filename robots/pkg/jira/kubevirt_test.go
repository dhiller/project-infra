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
	"github.com/joshdk/go-junit"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

var (
	jiraClientId  = os.Getenv("JIRA_CLIENT_ID")
	jiraTokenPath = os.Getenv("JIRA_TOKEN_PATH")
	jiraURL       = os.Getenv("JIRA_URL")
)

func TestCNVJiraComponentMapper_MapToComponent(t *testing.T) {
	type args struct {
		o *ObservedFailure
	}
	var tests = []struct {
		name string
		args args
		want []*IssueComponentMapping
	}{
		{
			name: "sig-compute",
			args: args{
				o: &ObservedFailure{
					Test: junit.Test{
						Name: "[Serial][crit:medium][level:component][sig-compute]Config With [test_id:666] whatever",
					},
					BuildURL:            "",
					IssueTrackerHandler: nil,
				},
			},
			want: []*IssueComponentMapping{&cnvVirtualization},
		},
		{
			name: "sig-network",
			args: args{
				o: &ObservedFailure{
					Test: junit.Test{
						Name: "[crap:wise][sig-network]moo meh",
					},
					BuildURL:            "",
					IssueTrackerHandler: nil,
				},
			},
			want: []*IssueComponentMapping{&cnvNetwork},
		},
		{
			name: "sig-storage",
			args: args{
				o: &ObservedFailure{
					Test: junit.Test{
						Name: "[meh:blah][sig-storage]bloop",
					},
					BuildURL:            "",
					IssueTrackerHandler: nil,
				},
			},
			want: []*IssueComponentMapping{&cnvStorage},
		},
		{
			name: "compute and storage",
			args: args{
				o: &ObservedFailure{
					Test: junit.Test{
						Name: "[Serial][crit:medium][level:component][sig-compute]Config With [test_id:666] [sig-storage]whatever",
					},
					BuildURL:            "",
					IssueTrackerHandler: nil,
				},
			},
			want: []*IssueComponentMapping{&cnvVirtualization, &cnvStorage},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := CNVJiraComponentMapper
			if got := j.MapObservedFailureToComponentMappings(tt.args.o); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MapObservedFailureToComponentMappings() = %v, want %v", got, tt.want)
			}
		})
	}
}

const (
	testNameWithTestId = "[Serial][rfe_id:899][crit:medium][vendor:cnv-qe@redhat.com][level:component][sig-compute]Config With a ServiceAccount defined [test_id:998]Should be the namespace and token the same for a pod and vmi"
	testMessage        = `tests/config_test.go:274&#xA;Timed out after 90.001s.&#xA;Timed out waiting for VMI testvmi-qx98s to enter [Running] phase(s)&#xA;Expected&#xA;    &lt;v1.VirtualMachineInstancePhase&gt;: Scheduling&#xA;to be an element of&#xA;    &lt;[]interface {} | len:1, cap:1&gt;: [[&#34;Running&#34;]]&#xA;tests/utils.go:2909`
	testSystemOut      = `On failure, artifacts will be collected in /home/cnv-qe-jenkins/jenkins/workspace/test-kubevirt-cnv-4.10-compute-ocs/artifacts/kubevirt-tests/test-artifacts/k8s-reporter/2_*&#xA;�[1mSTEP�[0m: Running VMI&#xA;�[1mSTEP�[0m: Starting a VirtualMachineInstance&#xA;�[1mSTEP�[0m: Waiting until the VirtualMachineInstance will start&#xA;{&#34;component&#34;:&#34;tests&#34;,&#34;kind&#34;:&#34;VirtualMachineInstance&#34;,&#34;level&#34;:&#34;info&#34;,&#34;msg&#34;:&#34;Event(v1.ObjectReference{Kind:\&#34;VirtualMachineInstance\&#34;, Namespace:\&#34;kubevirt-test-default1\&#34;, Name:\&#34;testvmi-qx98s\&#34;, UID:\&#34;1e064baf-e397-4baf-9ec7-543c2fb5def6\&#34;, APIVersion:\&#34;kubevirt.io/v1\&#34;, ResourceVersion:\&#34;998652\&#34;, FieldPath:\&#34;\&#34;}): type: &#39;Normal&#39; reason: &#39;SuccessfulCreate&#39; Created virtual machine pod virt-launcher-testvmi-qx98s-n7b6j&#34;,&#34;name&#34;:&#34;testvmi-qx98s&#34;,&#34;namespace&#34;:&#34;kubevirt-test-default1&#34;,&#34;pos&#34;:&#34;utils.go:365&#34;,&#34;timestamp&#34;:&#34;2022-02-23T07:46:36.563652Z&#34;,&#34;uid&#34;:&#34;1e064baf-e397-4baf-9ec7-543c2fb5def6&#34;}&#xA;&#xA;{&#xA;    &#34;metadata&#34;: {},&#xA;    &#34;items&#34;: [&#xA;        {&#xA;            &#34;metadata&#34;: {&#xA;                &#34;name&#34;: &#34;disk-alpine-host-path.16d65aa8da381008&#34;,&#xA;                &#34;namespace&#34;: &#34;kubevirt-test-default1&#34;,&#xA;                &#34;uid&#34;: &#34;2b0f04e6-886c-4ae3-ab26-d9892cab771c&#34;,&#xA;                &#34;resourceVersion&#34;: &#34;1000404&#34;,&#xA;                &#34;creationTimestamp&#34;: &#34;2022-02-23T07:46:35Z&#34;,&#xA;                &#34;managedFields&#34;: [&#xA;                    {&#xA;                        &#34;manager&#34;: &#34;kube-controller-manager&#34;,&#xA;                        &#34;operation&#34;: &#34;Update&#34;,&#xA;                        &#34;apiVersion&#34;: &#34;v1&#34;,&#xA;                        &#34;time&#34;: &#34;2022-02-23T07:46:35Z&#34;,&#xA;                        &#34;fieldsType&#34;: &#34;FieldsV1&#34;,&#xA;                        &#34;fieldsV1&#34;: {&#xA;                            &#34;f:count&#34;: {},&#xA;                            &#34;f:firstTimestamp&#34;: {},&#xA;`
)

func TestJiraHandler_newIssue(t *testing.T) {
	observedFailure := &ObservedFailure{
		Test: junit.Test{
			Name:       testNameWithTestId,
			Classname:  "Tests Suite",
			Duration:   time.Second * 42,
			Status:     junit.StatusFailed,
			Message:    testMessage,
			Error:      nil,
			Properties: nil,
			SystemOut:  testSystemOut,
			SystemErr:  "",
		},
		BuildURL:            "",
		IssueTrackerHandler: nil,
	}
	wantedComponents := []string{cnvVirtualization.To}
	newIssue := (&JiraHandler{
		config: defaultKubeVirtJiraHandlerConfiguration,
	}).newIssue(observedFailure)
	if !strings.Contains(newIssue.Summary, testNameWithTestId) {
		t.Errorf("Summary doesn't contain testNameWithTestId")
	}
	if !strings.Contains(newIssue.Description, testMessage) {
		t.Errorf("description doesn't contain message")
	}
	if !strings.Contains(newIssue.Description, testSystemOut) {
		t.Errorf("description doesn't contain system out")
	}
	if newIssue.Links == nil || newIssue.Links[0] != observedFailure.BuildURL {
		t.Errorf("link doesn't contain build url")
	}
	if !reflect.DeepEqual(newIssue.Labels, ([]string{defaultKubeVirtIssueLabel})) {
		t.Errorf("issue labels don't match: got: %v, want: %v", newIssue.Labels, []string{defaultKubeVirtIssueLabel})
	}
	if newIssue.Type != defaultKubeVirtIssueType {
		t.Errorf("issue type doesn't match. Got: %s, want: %s", newIssue.Type, defaultKubeVirtIssueType)
	}
	if !reflect.DeepEqual(newIssue.Components, wantedComponents) {
		t.Errorf("issue components don't match: got: %v, want: %v", newIssue.Components, wantedComponents)
	}
}

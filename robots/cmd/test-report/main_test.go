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

package main

import (
	"github.com/sirupsen/logrus"
	"io"
	"kubevirt.io/project-infra/robots/pkg/test-report"
	"os"
	"testing"
)

func Test_writeHTMLReportToOutput(t *testing.T) {
	type args struct {
		htmlReportOutputWriter       io.Writer
		testNames                    []string
		filteredTestNames            []string
		skippedTests                 map[string]interface{}
		lookedAtJobs                 []string
		testNamesToJobNamesToSkipped map[string]map[string]int
		err                          error
		jLog                         *logrus.Entry
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "test template",
			args: args{
				htmlReportOutputWriter: os.Stdout,
				testNames:              []string{"a", "b", "c"},
				filteredTestNames:      []string{"la", "le", "lu"},
				skippedTests: map[string]interface{}{
					"a": struct{}{}},
				lookedAtJobs: []string{"job1", "job2", "job3"},
				testNamesToJobNamesToSkipped: map[string]map[string]int{
					"a": {
						"job1": test_report.TestExecution_Skipped,
						"job2": test_report.TestExecution_Skipped,
					},
					"b": {
						"job1": test_report.TestExecution_Skipped,
						"job2": test_report.TestExecution_Run,
						"job3": test_report.TestExecution_Run,
					},
					"c": {
						"job1": test_report.TestExecution_Skipped,
						"job2": test_report.TestExecution_Skipped,
						"job3": test_report.TestExecution_Run,
					},
				},
				err:  nil,
				jLog: nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := writeHTMLReportToOutput(test_report.NewData(tt.args.testNames, tt.args.filteredTestNames, tt.args.skippedTests, tt.args.lookedAtJobs, tt.args.testNamesToJobNamesToSkipped), tt.args.htmlReportOutputWriter)
			if err != nil {
				t.Errorf("writeHTMLReportToOutput: %v", err)
			}
		})
	}
}

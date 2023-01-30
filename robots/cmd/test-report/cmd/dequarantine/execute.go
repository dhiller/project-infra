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

package dequarantine

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/bndr/gojenkins"
	"github.com/spf13/cobra"
	test_report "kubevirt.io/project-infra/robots/pkg/test-report"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

var dequarantineExecuteCmd = &cobra.Command{
	Use:   "execute",
	Short: "applies the changes to dequarantine tests to the target quarantined_tests.json",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDequarantineExecution()
	},
}

type dequarantineExecuteOpts struct {
	quarantineFileURL        string
	endpoint                 string
	startFrom                time.Duration
	jobNamePattern           string
	maxConnsPerHost          int
	dryRun                   bool
	outputFile               string
	minimumPassedRunsPerTest int
}

var executeJobNamePattern *regexp.Regexp

var dequarantineExecOptions = dequarantineExecuteOpts{}

func (r *dequarantineExecuteOpts) Validate() error {
	if r.quarantineFileURL == "" {
		return fmt.Errorf("quarantineFileURL must be set")
	}
	if r.jobNamePattern == "" {
		return fmt.Errorf("jobNamePattern must be set")
	}
	_, err := regexp.Compile(r.jobNamePattern)
	if err != nil {
		return fmt.Errorf("executeJobNamePattern %q is not a valid regexp", r.jobNamePattern)
	}
	return nil
}

func init() {
	dequarantineExecuteCmd.PersistentFlags().StringVar(&dequarantineExecOptions.endpoint, "endpoint", test_report.DefaultJenkinsBaseUrl, "jenkins base url")
	dequarantineExecuteCmd.PersistentFlags().DurationVar(&dequarantineExecOptions.startFrom, "start-from", 10*24*time.Hour, "time period for report")
	dequarantineExecuteCmd.PersistentFlags().StringVar(&dequarantineExecOptions.quarantineFileURL, "quarantine-file-url", "", "the url to the quarantine file")
	dequarantineExecuteCmd.PersistentFlags().StringVar(&dequarantineExecOptions.jobNamePattern, "job-name-pattern", "", "the pattern to which all jobs have to match")
	dequarantineExecuteCmd.PersistentFlags().IntVar(&dequarantineExecOptions.maxConnsPerHost, "max-conns-per-host", 3, "the maximum number of connections that are going to be made")
	dequarantineExecuteCmd.PersistentFlags().StringVar(&dequarantineExecOptions.outputFile, "output-file", "", "Path to output file, if not given, a temporary file will be used")
	dequarantineExecuteCmd.PersistentFlags().BoolVar(&dequarantineExecOptions.dryRun, "dry-run", true, "whether to only check what jobs are being considered and then exit")
	dequarantineExecuteCmd.PersistentFlags().IntVar(&dequarantineExecOptions.minimumPassedRunsPerTest, "minimum-passed-runs-per-test", 2, "whether to only check what jobs are being considered and then exit")
}

func runDequarantineExecution() error {

	err := dequarantineExecOptions.Validate()
	if err != nil {
		return fmt.Errorf("failed to validate command line arguments: %v", err)
	}

	executeJobNamePattern = regexp.MustCompile(dequarantineExecOptions.jobNamePattern)

	client := &http.Client{
		Transport: &http.Transport{
			MaxConnsPerHost: dequarantineExecOptions.maxConnsPerHost,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	ctx := context.Background()

	logger.Printf("Creating client for %s", dequarantineExecOptions.endpoint)
	jenkins := gojenkins.CreateJenkins(client, dequarantineExecOptions.endpoint)
	_, err = jenkins.Init(ctx)
	if err != nil {
		logger.Fatalf("failed to contact jenkins %s: %v", dequarantineExecOptions.endpoint, err)
	}

	jobNames, err := jenkins.GetAllJobNames(ctx)
	if err != nil {
		logger.Fatalf("failed to get jobs: %v", err)
	}
	jobs, err := test_report.FilterMatchingJobsByJobNamePattern(ctx, jenkins, jobNames, executeJobNamePattern)
	if err != nil {
		logger.Fatalf("failed to filter matching jobs: %v", err)
	}
	var filteredJobNames []string
	for _, job := range jobs {
		filteredJobNames = append(filteredJobNames, job.GetName())
	}
	logger.Infof("jobs that are being considered: %s", strings.Join(filteredJobNames, ", "))
	if dequarantineExecOptions.dryRun {
		logger.Warn("dry-run mode, exiting")
		return nil
	}
	if len(jobs) == 0 {
		logger.Warn("no jobs left, nothing to do")
		return nil
	}

	quarantinedTestEntriesFromFile, err := test_report.FetchDontRunEntriesFromFile(dequarantineExecOptions.quarantineFileURL, client)
	if err != nil {
		logger.Fatalf("failed to filter matching jobs: %v", err)
	}

	startOfReport := time.Now().Add(-1 * dequarantineExecOptions.startFrom)

	quarantinedTestsRunDataValues := generateDequarantineBaseData(jenkins, ctx, jobs, startOfReport, quarantinedTestEntriesFromFile)

	err, remainingQuarantinedTestRecords := filterUnstableTestRecords(dequarantineExecOptions, quarantinedTestsRunDataValues)
	if err != nil {
		return fmt.Errorf("could not create data for output file %s: %v", dequarantineExecOptions.outputFile, err)
	}

	var buffer bytes.Buffer
	err = json.NewEncoder(&buffer).Encode(&remainingQuarantinedTestRecords)
	if err != nil {
		return fmt.Errorf("could not encode output file %s: %v", dequarantineExecOptions.outputFile, err)
	}

	outputFile, err := createOutputFile(dequarantineExecOptions.outputFile)
	if err != nil {
		return err
	}
	err = os.WriteFile(outputFile.Name(), buffer.Bytes(), 0777)
	if err != nil {
		return fmt.Errorf("could not write output file %s: %v", outputFile.Name(), err)
	}

	logger.Infof("Output file written to %q", outputFile.Name())
	return nil
}

func filterUnstableTestRecords(options dequarantineExecuteOpts, values []*quarantinedTestsRunData) (err error, remainingQuarantinedTests []*test_report.FilterTestRecord) {
	if len(values) == 0 {
		return fmt.Errorf("no input data?"), nil
	}
	for _, value := range values {
		filterLogger := logger.WithField("record_id", value.Id)
		if value.Tests == nil {
			filterLogger.Errorf("no matching test names in runs found, please check test id")
			remainingQuarantinedTests = append(remainingQuarantinedTests, value.FilterTestRecord)
			continue
		}
	tests:
		for _, test := range value.Tests {
			passedRunsPerTests := 0
			for _, result := range test.TestResults {
				if isTestFailing(result) {
					filterLogger.WithField("result", result.Result).WithField("test_name", test.TestName).WithField("build_no", result.BuildNo).Warn("test set stays in quarantine")
					remainingQuarantinedTests = append(remainingQuarantinedTests, value.FilterTestRecord)
					break tests
				}
				if isTestPassing(result) {
					passedRunsPerTests++
				}
			}
			if passedRunsPerTests < options.minimumPassedRunsPerTest {
				filterLogger.WithField("result", "UNSTABLE").WithField("test_name", test.TestName).Warnf("test set stays in quarantine, expected %d passes, only %d seen", options.minimumPassedRunsPerTest, passedRunsPerTests)
				remainingQuarantinedTests = append(remainingQuarantinedTests, value.FilterTestRecord)
				break tests
			}
			filterLogger.WithField("result", "STABLE").WithField("test_name", test.TestName).Info("test is stable")
		}
	}
	return nil, remainingQuarantinedTests
}

func isTestFailing(result *quarantinedTestRunData) bool {
	switch result.Result {
	case "PASSED":
		return false
	case "FIXED":
		return false
	case "SKIPPED":
		return false
	default:
		return true
	}
}

func isTestPassing(result *quarantinedTestRunData) bool {
	switch result.Result {
	case "PASSED":
		return true
	case "FIXED":
		return true
	default:
		return false
	}
}

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
	"context"
	"crypto/tls"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bndr/gojenkins"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"io"
	"kubevirt.io/project-infra/robots/pkg/flakefinder"
	flakejenkins "kubevirt.io/project-infra/robots/pkg/jenkins"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	defaultJenkinsBaseUrl = "https://main-jenkins-csb-cnvqe.apps.ocp-c1.prod.psi.redhat.com/"
	defaultFilterFileUrl  = "https://gitlab.cee.redhat.com/contra/cnv-qe-automation/-/raw/master/tests/tier1/kubevirt/dont_run_tests.json"
)

//go:embed test-report.gohtml
var reportTemplate string

var (
	rootCmd *cobra.Command
	opts    options
	logger  *log.Entry
)

func init() {
	rootCmd = &cobra.Command{
		Use:   "test-report",
		Short: "test-report creates a report about which tests have been run on what lane",
		RunE:  runReport,
	}
	opts = options{}
	flagOptions()
	log.StandardLogger().SetFormatter(&log.JSONFormatter{})
	logger = log.StandardLogger().WithField("robot", "test-report")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
	os.Exit(0)
}

type options struct {
	endpoint       string
	startFrom      time.Duration
	jobNamePattern string
	outputFile     string
	overwrite      bool
	filterFileUrls []string
}

func flagOptions() options {
	rootCmd.PersistentFlags().StringVar(&opts.endpoint, "endpoint", defaultJenkinsBaseUrl, "jenkins base url")
	rootCmd.PersistentFlags().DurationVar(&opts.startFrom, "start-from", 14*24*time.Hour, "time period for report")
	rootCmd.PersistentFlags().StringVar(&opts.jobNamePattern, "job-name-pattern", "^test-(ssp-cnv-4\\.[0-9]+|kubevirt-cnv-4\\.[0-9]+-(compute|network|operator)-[a-z]+)$", "jenkins job name go regex pattern to filter jobs for the report")
	rootCmd.PersistentFlags().StringVar(&opts.outputFile, "outputFile", "", "Path to output file, if not given, a temporary file will be used")
	rootCmd.PersistentFlags().BoolVar(&opts.overwrite, "overwrite", true, "overwrite output file")
	rootCmd.PersistentFlags().StringArrayVar(&opts.filterFileUrls, "filter-file-urls", []string{defaultFilterFileUrl}, "file urls to use as filters for test cases, use quarantined_tests.json and/or dont_run_tests.json")
	return opts
}

func (o *options) Validate() error {
	if o.outputFile == "" {
		outputFile, err := os.CreateTemp("", "test-report-*.html")
		if err != nil {
			return fmt.Errorf("failed to write report: %v", err)
		}
		o.outputFile = outputFile.Name()
	} else {
		stat, err := os.Stat(o.outputFile)
		if err != nil && errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if stat.IsDir() {
			return fmt.Errorf("failed to write report, file %s is a directory", o.outputFile)
		}
		if err == nil && !o.overwrite {
			return fmt.Errorf("failed to write report, file %s exists", o.outputFile)
		}
	}
	return nil
}

const (
	test_execution_no_data = iota
	test_execution_skipped
	test_execution_run
	test_execution_unsupported
)

type FilterTestRecord struct {
	Id     string `json:"id"`
	Reason string `json:"reason"`
}

func runReport(cmd *cobra.Command, args []string) error {

	err := opts.Validate()
	if err != nil {
		return fmt.Errorf("failed to validate command line arguments: %v", err)
	}

	ctx := context.Background()

	client := &http.Client{
		Transport: &http.Transport{
			MaxConnsPerHost: 5,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	logger.Printf("Creating client for %s", opts.endpoint)
	jenkins := gojenkins.CreateJenkins(client, opts.endpoint)
	_, err = jenkins.Init(ctx)
	if err != nil {
		logger.Fatalf("failed to contact jenkins %s: %v", opts.endpoint, err)
	}

	jobNames, err := jenkins.GetAllJobNames(ctx)
	if err != nil {
		logger.Fatalf("failed to get jobs: %v", err)
	}
	jobs, err := filterMatchingJobs(ctx, jenkins, jobNames)
	if err != nil {
		logger.Fatalf("failed to filter matching jobs: %v", err)
	}
	if len(jobs) == 0 {
		logger.Warn("no jobs left, nothing to do")
		return nil
	}

	startOfReport := time.Now().Add(-1 * opts.startFrom)

	testNamesToJobNamesToExecutionStatus := getTestNamesToJobNamesToTestExecutions(jobs, startOfReport, ctx)

	err = writeJsonBaseDataFile(testNamesToJobNamesToExecutionStatus)
	if err != nil {
		logger.Fatalf("failed to write json data file: %v", err)
	}

	completeFilterRegex, err := createTestFilterRegexpFromFilterFiles()
	if err != nil {
		logger.Fatalf("failed to create test filter regexp: %v", err)
	}
	data := createReportData(completeFilterRegex, nil, testNamesToJobNamesToExecutionStatus)

	err = writeHTMLReportToOutputFile(err, data)
	if err != nil {
		logger.Fatalf("failed to write report: %v", err)
	}
	return nil
}

func createReportData(testNameFilterRegexp *regexp.Regexp, jobNamePatternsToTestNameFilterRegexps map[*regexp.Regexp]*regexp.Regexp, testNamesToJobNamesToExecutionStatus map[string]map[string]int) Data {
	testNames := []string{}
	skippedTests := map[string]interface{}{}
	filteredTestNames := []string{}
	lookedAtJobsMap := map[string]interface{}{}

	for testName, jobNamesToSkipped := range testNamesToJobNamesToExecutionStatus {
		if testNameFilterRegexp.MatchString(testName) {
			filteredTestNames = append(filteredTestNames, testName)
			logger.Warnf("filtering %s", testName)
			continue
		}
		testSkipped := true
		filteredOnAllLanes := true
		for jobName, executionStatus := range jobNamesToSkipped {
			if _, exists := lookedAtJobsMap[jobName]; !exists {
				lookedAtJobsMap[jobName] = struct{}{}
			}
			switch executionStatus {
			case test_execution_run:
				testSkipped = false
				filteredOnAllLanes = false
				break
			case test_execution_skipped:
				jobNameMatcherFound := false
				for jobNameMatcher, testNameMatcher := range jobNamePatternsToTestNameFilterRegexps {
					if jobNameMatcher.MatchString(jobName) {
						if testNameMatcher.MatchString(testName) {
							testNamesToJobNamesToExecutionStatus[testName][jobName] = test_execution_unsupported
						} else {
							filteredOnAllLanes = false
						}
						jobNameMatcherFound = true
						break
					}
				}
				if !jobNameMatcherFound {
					filteredOnAllLanes = false
				}
			case test_execution_no_data:
				filteredOnAllLanes = false
			}
		}
		if !filteredOnAllLanes {
			testNames = append(testNames, testName)
		} else {
			filteredTestNames = append(filteredTestNames, testName)
		}
		if testSkipped {
			skippedTests[testName] = struct{}{}
		}
	}
	lookedAtJobs := []string{}
	for jobName, _ := range lookedAtJobsMap {
		lookedAtJobs = append(lookedAtJobs, jobName)
	}

	sort.Strings(testNames)
	sort.Strings(filteredTestNames)
	sort.Strings(lookedAtJobs)
	data := newData(testNames, filteredTestNames, skippedTests, lookedAtJobs, testNamesToJobNamesToExecutionStatus)
	return data
}

func writeHTMLReportToOutputFile(err error, data Data) error {
	htmlReportOutputWriter, err := os.OpenFile(opts.outputFile, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to write report %q: %v", opts.outputFile, err)
	}
	logger.Printf("Writing html to %q", opts.outputFile)
	defer htmlReportOutputWriter.Close()
	err = writeHTMLReportToOutput(data, htmlReportOutputWriter)
	if err != nil {
		return fmt.Errorf("failed to write report: %v", err)
	}
	return nil
}

func writeJsonBaseDataFile(testNamesToJobNamesToExecutionStatus map[string]map[string]int) error {
	bytes, err := json.MarshalIndent(testNamesToJobNamesToExecutionStatus, "", "\t")
	if err != nil {
		return fmt.Errorf("failed to marshall result: %v", err)
	}

	jsonFileName := strings.TrimSuffix(opts.outputFile, ".html") + ".json"
	logger.Printf("Writing json to %q", jsonFileName)
	jsonOutputWriter, err := os.OpenFile(jsonFileName, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil && err != os.ErrNotExist {
		return fmt.Errorf("failed to write report: %v", err)
	}
	defer jsonOutputWriter.Close()

	_, err = jsonOutputWriter.Write(bytes)
	return err
}

func getTestNamesToJobNamesToTestExecutions(jobs []*gojenkins.Job, startOfReport time.Time, ctx context.Context) map[string]map[string]int {
	resultsChan := make(chan map[string]map[string]int)
	go getTestNamesToJobNamesToTestExecutionForAllJobs(resultsChan, jobs, startOfReport, ctx, logger)

	testNamesToJobNamesToExecutionStatus := map[string]map[string]int{}
	for result := range resultsChan {
		for testName, jobNamesToExecutionStatus := range result {
			testNamesToJobNamesToExecutionStatus[testName] = jobNamesToExecutionStatus
		}
	}
	return testNamesToJobNamesToExecutionStatus
}

func createTestFilterRegexpFromFilterFiles() (*regexp.Regexp, error) {
	var filterRegexes []string
	for _, filterFileUrl := range opts.filterFileUrls {
		logger.Infof("fetching filter file %q", filterFileUrl)
		response, err := http.Get(filterFileUrl)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch %q: %v", filterFileUrl, err)
		}
		if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusBadRequest {
			var records []*FilterTestRecord
			err := json.NewDecoder(response.Body).Decode(&records)
			if err != nil {
				return nil, fmt.Errorf("failed to decode %q: %v", filterFileUrl, err)
			}
			for _, record := range records {
				filterRegexes = append(filterRegexes, record.Id)
			}
		} else {
			return nil, fmt.Errorf("when fetching %q received status code: %d", filterFileUrl, response.StatusCode)
		}
	}
	completeFilterRegex := regexp.MustCompile(strings.Join(filterRegexes, "|"))
	logger.Infof("filter expression is '%s'", completeFilterRegex)
	return completeFilterRegex, nil
}

type Data struct {
	JenkinsBaseURL string `json:"jenkinsBaseURL"`
	// TestNames contains the names of all tests that have not been filtered on all lanes
	TestNames []string `json:"testNames"`
	// FilteredTestNames contains the names of all tests that have been filtered on all lanes
	FilteredTestNames []string `json:"filteredTestNames"`
	// SkippedTests contains the test names for all tests that have been skipped on all lanes, aka not having been run on any lane
	SkippedTests map[string]interface{} `json:"skippedTests"`
	// LookedAtJobs contains the names of all test lanes that have been looked at
	LookedAtJobs []string `json:"lookedAtJobs"`

	// TestNamesToJobNamesToSkipped contains a map of test names per test pointing to the jobs where that test has been seen, which points to the state that was seen on that lane (see test_execution_no_data, test_execution_skipped, test_execution_run, test_execution_unsupported)
	TestNamesToJobNamesToSkipped map[string]map[string]int `json:"testNamesToJobNamesToSkipped"`
	TestExecutionMapping         map[string]int
}

func (d Data) String() string {
	return fmt.Sprintf(`{
	JenkinsBaseURL: %s,
	TestNames: %v,
	FilteredTestNames: %v,
	SkippedTests: %v,
	LookedAtJobs: %v,
	TestNamesToJobNamesToSkipped: %v,
	TestExecutionMapping: %v,
}`, d.JenkinsBaseURL, d.TestNames, d.FilteredTestNames, d.SkippedTests, d.LookedAtJobs, d.TestNamesToJobNamesToSkipped, d.TestExecutionMapping)
}

func newData(testNames []string, filteredTestNames []string, skippedTests map[string]interface{}, lookedAtJobs []string, testNamesToJobNamesToSkipped map[string]map[string]int) Data {
	return Data{
		TestNames:                    testNames,
		FilteredTestNames:            filteredTestNames,
		SkippedTests:                 skippedTests,
		LookedAtJobs:                 lookedAtJobs,
		TestNamesToJobNamesToSkipped: testNamesToJobNamesToSkipped,
		JenkinsBaseURL:               defaultJenkinsBaseUrl,
		TestExecutionMapping: map[string]int{
			"test_execution_no_data":     test_execution_no_data,
			"test_execution_skipped":     test_execution_skipped,
			"test_execution_run":         test_execution_run,
			"test_execution_unsupported": test_execution_unsupported,
		},
	}
}

func writeHTMLReportToOutput(data Data, htmlReportOutputWriter io.Writer) error {
	err := flakefinder.WriteTemplateToOutput(reportTemplate, data, htmlReportOutputWriter)
	if err != nil {
		return fmt.Errorf("failed to write report: %v", err)
	}
	return nil
}

func getTestNamesToJobNamesToTestExecutionForAllJobs(resultsChan chan map[string]map[string]int, jobs []*gojenkins.Job, startOfReport time.Time, ctx context.Context, jLog *log.Entry) {

	var wg sync.WaitGroup
	wg.Add(len(jobs))

	defer close(resultsChan)
	for _, job := range jobs {
		fLog := jLog.WithField("job", job.GetName())
		go getTestNamesToJobNamesToTestExecutionForJob(startOfReport, ctx, fLog, job, resultsChan, &wg)
	}

	wg.Wait()
	jLog.Printf("done get all jobs")
}

func getTestNamesToJobNamesToTestExecutionForJob(startOfReport time.Time, ctx context.Context, jLog *log.Entry, job *gojenkins.Job, resultsChan chan map[string]map[string]int, wg *sync.WaitGroup) {
	defer wg.Done()
	testResultsForJob := flakejenkins.GetBuildNumbersToTestResultsForJob(startOfReport, job, ctx, jLog)
	testNamesToJobNamesToSkippedForJobName := map[string]map[string]int{}
	for _, testResultForJob := range testResultsForJob {
		for _, suite := range testResultForJob.Suites {
			for _, suiteCase := range suite.Cases {
				if _, exists := testNamesToJobNamesToSkippedForJobName[suiteCase.Name]; !exists {
					testNamesToJobNamesToSkippedForJobName[suiteCase.Name] = map[string]int{}
				}
				if suiteCase.Skipped {
					testNamesToJobNamesToSkippedForJobName[suiteCase.Name][job.GetName()] = test_execution_skipped
				} else {
					testNamesToJobNamesToSkippedForJobName[suiteCase.Name][job.GetName()] = test_execution_run
				}
			}
		}
	}
	resultsChan <- testNamesToJobNamesToSkippedForJobName
}

func filterMatchingJobs(ctx context.Context, jenkins *gojenkins.Jenkins, innerJobs []gojenkins.InnerJob) ([]*gojenkins.Job, error) {
	filteredJobs := []*gojenkins.Job{}
	jobNameRegex := regexp.MustCompile(opts.jobNamePattern)
	logger.Printf("Filtering for jobs matching %s", jobNameRegex)
	for _, innerJob := range innerJobs {
		if !jobNameRegex.MatchString(innerJob.Name) {
			continue
		}
		job, err := jenkins.GetJob(ctx, innerJob.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to get job %s: %v", innerJob.Name, err)
		}
		filteredJobs = append(filteredJobs, job)
	}
	logger.Printf("%d jobs left after filtering", len(filteredJobs))
	return filteredJobs, nil
}

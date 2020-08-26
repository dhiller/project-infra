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
 * Copyright 2019 Red Hat, Inc.
 *
 */

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/go-github/v28/github"
	"io/ioutil"
	"kubevirt.io/project-infra/robots/pkg/flakefinder"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/joshdk/go-junit"
	"github.com/sirupsen/logrus"
)

const (
	//finishedJSON is the JSON file that stores build success info
	finishedJSON = "finished.json"
	startedJSON  = "started.json"
)

func readGcsObject(ctx context.Context, client *storage.Client, bucket, object string) ([]byte, error) {
	logrus.Infof("Trying to read gcs object '%s' in bucket '%s'\n", object, bucket)
	o := client.Bucket(bucket).Object(object)
	reader, err := o.NewReader(ctx)
	if err == storage.ErrObjectNotExist {
		return nil, err
	} else if err != nil {
		return nil, fmt.Errorf("cannot read object '%s': %v", object, err)
	}
	return ioutil.ReadAll(reader)
}

func FindUnitTestFiles(ctx context.Context, client *storage.Client, bucket, repo string, pr *github.PullRequest, startOfReport time.Time, skipBeforeStartOfReport bool) ([]*Result, error) {

	dirOfPrJobs := path.Join("pr-logs", "pull", strings.ReplaceAll(repo, "/", "_"), strconv.Itoa(*pr.Number))

	prJobsDirs, err := flakefinder.ListGcsObjects(ctx, client, bucket, dirOfPrJobs+"/", "/")
	if err != nil {
		return nil, fmt.Errorf("error listing gcs objects: %v", err)
	}

	junits := []*Result{}
	for _, job := range prJobsDirs {
		junit, err := FindUnitTestFileForJob(ctx, client, bucket, dirOfPrJobs, job, pr, startOfReport, skipBeforeStartOfReport)
		if err != nil {
			return nil, err
		}
		if junit != nil {
			junits = append(junits, junit...)
		}
	}
	return junits, err
}

func FindUnitTestFileForJob(ctx context.Context, client *storage.Client, bucket string, dirOfPrJobs string, job string, pr *github.PullRequest, startOfReport time.Time, skipBeforeStartOfReport bool) ([]*Result, error) {
	dirOfJobs := path.Join(dirOfPrJobs, job)

	prJobs, err := flakefinder.ListGcsObjects(ctx, client, bucket, dirOfJobs+"/", "/")
	if err != nil {
		return nil, fmt.Errorf("error listing gcs objects: %v", err)
	}
	builds := sortBuilds(prJobs)
	profilePath := ""
	buildNumber := 0
	reports := []*Result{}
	for _, build := range builds {
		buildDirPath := path.Join(dirOfJobs, strconv.Itoa(build))
		dirOfFinishedJSON := path.Join(buildDirPath, finishedJSON)
		dirOfStartedJSON := path.Join(buildDirPath, startedJSON)

		// Fetch file attributes to check whether this test result should be included into the report
		attrsOfFinishedJsonFile, err := flakefinder.ReadGcsObjectAttrs(ctx, client, bucket, dirOfFinishedJSON)
		if err == storage.ErrObjectNotExist {
			// build still running?
			continue
		} else if err != nil {
			return nil, err
		}
		isBeforeStartOfReport := attrsOfFinishedJsonFile.Created.Before(startOfReport)
		if skipBeforeStartOfReport && isBeforeStartOfReport {
			logrus.Infof("Skipping test results before %v for %s in bucket '%s'\n", startOfReport, buildDirPath, bucket)
			continue
		}

		_, err = readGcsObject(ctx, client, bucket, dirOfFinishedJSON)
		if err == storage.ErrObjectNotExist {
			// build still running?
			continue
		} else if err != nil {
			return nil, fmt.Errorf("Cannot read finished.json (%s) in bucket '%s'", dirOfFinishedJSON, bucket)
		} else {
			startedJSON, err := readGcsObject(ctx, client, bucket, dirOfStartedJSON)
			if err != nil {
				return nil, fmt.Errorf("Cannot read started.json (%s) in bucket '%s'", dirOfStartedJSON, bucket)
			}
			if !IsLatestCommit(startedJSON, pr) {
				continue
			}
			buildNumber = build
			artifactsDirPath := path.Join(buildDirPath, "artifacts")
			profilePath = path.Join(artifactsDirPath, "junit.functest.xml")
			data, err := readGcsObject(ctx, client, bucket, profilePath)
			if err == storage.ErrObjectNotExist {
				logrus.Infof("Didn't find object '%s' in bucket '%s'\n", profilePath, bucket)
				continue
			}
			if err != nil {
				return nil, err
			}
			report, err := junit.Ingest(data)
			if err != nil {
				return nil, err
			}
			reports = append(reports, &Result{Job: job, JUnit: report, BuildNumber: buildNumber, PR: *pr.Number})
		}
	}

	return reports, nil
}

// sortBuilds converts all build from str to int and sorts all builds in descending order and
// returns the sorted slice
func sortBuilds(strBuilds []string) []int {
	var res []int
	for _, buildStr := range strBuilds {
		num, err := strconv.Atoi(buildStr)
		if err != nil {
			logrus.Infof("Non-int build number found: '%s'", buildStr)
		} else {
			res = append(res, num)
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(res)))
	return res
}

type StartedStatus struct {
	Timestamp int
	Repos     map[string]string
}

type Result struct {
	Job         string
	JUnit       []junit.Suite
	BuildNumber int
	PR          int
}

func IsLatestCommit(jsonText []byte, pr *github.PullRequest) bool {
	var status StartedStatus
	err := json.Unmarshal(jsonText, &status)
	if err != nil {
		return false
	}
	for _, v := range status.Repos {
		if strings.Contains(v, fmt.Sprintf("%d:%s", *pr.Number, *pr.Head.SHA)) {
			return true
		}
	}
	return false
}

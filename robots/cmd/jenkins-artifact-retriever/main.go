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
	"flag"
	"fmt"
	"github.com/bndr/gojenkins"
	"github.com/joshdk/go-junit"
	"kubevirt.io/project-infra/robots/pkg/flakefinder"
	"log"
	"os"
	"regexp"
	"time"
)

const (
	defaultJenkinsBaseUrl        = "https://main-jenkins-csb-cnvqe.apps.ocp-c1.prod.psi.redhat.com/"
	defaultJenkinsJobNamePattern = "^test-kubevirt-cnv-%s-(compute|network|operator|storage)(-[a-z0-9]+)?$"
	ReportTemplate               = `
<html>
<head>
    <title>flakefinder report</title>
    <meta charset="UTF-8">
    <style>
        table, th, td {
            border: 1px solid black;
        }
        .yellow {
            background-color: #ffff80;
        }
        .almostgreen {
            background-color: #dfff80;
        }
        .green {
            background-color: #9fff80;
        }
        .red {
            background-color: #ff8080;
        }
        .orange {
            background-color: #ffbf80;
        }
        .unimportant {
        }
        .tests_passed {
            color: #226c18;
            font-weight: bold;
        }
        .tests_failed {
            color: #8a1717;
            font-weight: bold;
        }
        .tests_skipped {
            color: #535453;
            font-weight: bold;
        }
        .center {
            text-align:center
        }
        .right {
            text-align: right;
			width: 100%;
        }
	</style>
</head>
<body>
<h1>flakefinder report</h1>

<div>
	Data since {{ $.StartOfReport }}<br/>
</div>
<table>
    <tr>
        <td></td>
        <td></td>
        {{ range $header := $.Headers }}
        <td>{{ $header }}</td>
        {{ end }}
    </tr>
    {{ range $row, $test := $.Tests }}
    <tr>
        <td><div id="row{{$row}}"><a href="#row{{$row}}">{{ $row }}</a><div></td>
        <td>{{ $test }}</td>
        {{ range $col, $header := $.Headers }}
        {{if not (index $.Data $test $header) }}
        <td class="center">
            N/A
        </td>
        {{else}}
        <td class="{{ (index $.Data $test $header).Severity }} center">
            <div id="r{{$row}}c{{$col}}">
                <span class="tests_failed" title="failed tests">{{ (index $.Data $test $header).Failed }}</span>/<span class="tests_passed" title="passed tests">{{ (index $.Data $test $header).Succeeded }}</span>/<span class="tests_skipped" title="skipped tests">{{ (index $.Data $test $header).Skipped }}</span>
            </div>
            {{end}}
        </td>
        {{ end }}
    </tr>
    {{ end }}
</table>
</body>
</html>
`
)

var (
	fileNameRegex *regexp.Regexp
	opts          options
)

func init() {
	fileNameRegex = regexp.MustCompile("^(partial\\.)junit\\.functest(\\.1)\\.xml$")
}

func flagOptions() options {
	o := options{}
	flag.StringVar(&o.endpoint, "endpoint", defaultJenkinsBaseUrl, "jenkins base url")
	flag.StringVar(&o.jobNamePattern, "jobNamePattern", defaultJenkinsJobNamePattern, "jenkins job name pattern to filter jobs for for the report")
	flag.DurationVar(&o.startFrom, "startFrom", 14*24*time.Hour, "The duration when the report data should be fetched")
	flag.Parse()
	return o
}

type options struct {
	endpoint       string
	jobNamePattern string
	startFrom      time.Duration
}

type SimpleReportParams struct {
	flakefinder.Params
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetPrefix("jenkins-flake-reporter")
	opts = flagOptions()
	RunJenkinsReport(defaultJenkinsBaseUrl, "4.10")
}

func RunJenkinsReport(jenkinsBaseUrl string, cnvVersions ...string) {
	ctx := context.Background()

	log.Printf("Creating client for %s", jenkinsBaseUrl)
	jenkins := gojenkins.CreateJenkins(nil, jenkinsBaseUrl)
	_, err := jenkins.Init(ctx)
	if err != nil {
		log.Fatalf("failed to contact jenkins %s: %v", jenkinsBaseUrl, err)
	}
	log.Printf("Fetching jobs")
	jobs, err := jenkins.GetAllJobs(ctx)
	if err != nil {
		log.Fatalf("failed to fetch jobs: %v", err)
	}
	log.Printf("Fetched %d jobs", len(jobs))

	startOfReport := time.Now().Add(-1 * opts.startFrom)
	endOfReport := time.Now()

	reports := []*flakefinder.JobResult{}
	for _, cnvVersion := range cnvVersions {
		log.Printf( "Filtering jobs for CNV %s matching %s", cnvVersion, opts.jobNamePattern)
		compile, err := regexp.Compile(fmt.Sprintf(opts.jobNamePattern, cnvVersion))
		if err != nil {
			log.Fatalf("failed to fetch jobs: %v", err)
		}
		for _, job := range jobs {
			if !compile.MatchString(job.GetName()) {
				continue
			}

			log.Printf("Job %s matches", job.GetName())

			// fetch junit files from x last completed builds
			log.Printf("Fetching builds")
			ids, err := job.GetAllBuildIds(ctx)
			if err != nil {
				log.Fatalf("failed to fetch build ids: %v", err)
			}

			log.Printf("Fetching completed builds from %s - %s period", startOfReport, endOfReport)
			var completedBuilds []*gojenkins.Build
			for i := 0; i < len(ids); i++ {
				log.Printf("Fetching build no %d", ids[i].Number)
				build, err := job.GetBuild(ctx, ids[i].Number)
				if err != nil {
					log.Fatalf("failed to fetch build data: %v", err)
				}

				if build.GetResult() != "SUCCESS" &&
					build.GetResult() != "UNSTABLE" {
					log.Printf("Skipping %s builds", build.GetResult())
					continue
				}

				buildTime := msecsToTime(build.Info().Timestamp)
				log.Printf("Build %d ran at %v", build.Info().Number, buildTime)
				if buildTime.Before(startOfReport) {
					log.Printf("Skipping remaining builds for %s", job.GetName())
					break
				}

				completedBuilds = append(completedBuilds, build)
			}
			log.Printf("Fetched %d completed builds from %s - %s period", len(completedBuilds), startOfReport, endOfReport)

			log.Printf("Fetch junit files from artifacts for %d completed builds", len(completedBuilds))
			artifacts := []gojenkins.Artifact{}
			for _, completedBuild := range completedBuilds {
				for _, artifact := range completedBuild.GetArtifacts() {
					if !fileNameRegex.MatchString(artifact.FileName) {
						continue
					}
					artifacts = append(artifacts, artifact)
				}
			}
			log.Printf("Fetched %d junit files from artifacts", len(artifacts))

			// fetch artifacts
			reportsPerJob := []*flakefinder.JobResult{}
			for _, artifact := range artifacts {
				data, err := artifact.GetData(ctx)
				if err != nil {
					log.Fatalf("failed to fetch artifact data: %v", err)
				}
				report, err := junit.Ingest(data)
				if err != nil {
					log.Fatalf("failed to fetch artifact data: %v", err)
				}
				reportsPerJob = append(reportsPerJob, &flakefinder.JobResult{Job: job.GetName(), JUnit: report, BuildNumber: int(artifact.Build.Info().Number)})
			}

			reports = append(reports, reportsPerJob...)
		}
	}

	// create report
	parameters := flakefinder.CreateFlakeReportData(reports, []int{}, endOfReport, "kubevirt", "kubevirt", startOfReport)

	outputFile, err := os.CreateTemp("", "flakefinder-*.html")
	if err != nil {
		log.Fatalf("failed to write report: %v", err)
	}
	log.Printf("writing output to %s", outputFile.Name())

	reportOutputWriter, err := os.OpenFile(outputFile.Name(), os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil && err != os.ErrNotExist {
		log.Fatalf("failed to write report: %v", err)
	}
	defer reportOutputWriter.Close()

	err = flakefinder.WriteTemplateToOutput(ReportTemplate, SimpleReportParams{parameters}, reportOutputWriter)
	if err != nil {
		log.Fatalf("failed to write report: %v", err)
	}

}

func msecsToTime(msecs int64) time.Time {
	return time.Unix(msecs / 1000, msecs % 1000)
}

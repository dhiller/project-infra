/*
 * Copyright 2021 The KubeVirt Authors.
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a get of the License at
 *     http://www.apache.org/licenses/LICENSE-2.0
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package get

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/template"

	"k8s.io/test-infra/prow/config"

	"github.com/spf13/cobra"

	"kubevirt.io/project-infra/robots/pkg/kubevirt/cmd/flags"

	"kubevirt.io/project-infra/robots/pkg/kubevirt/prowjobconfigs"

	"github.com/thediveo/enumflag"
)

const (
	shortUse                      = "kubevirt get jobs returns data about the periodic and presubmit jobs for kubevirt"
	sourceAndTargetReleaseDoExist = 2
)

type output int

const (
	markdown = iota
	json
)

var outputTextMapping = map[output][]string{
	markdown: {"markdown"},
	json:     {"json"},
}

type getJobOptions struct {
	jobConfigPathKubevirtPresubmits string
	jobConfigPathKubevirtPeriodics  string
	output                          output
	outputFile                      string
}

func (o getJobOptions) Validate() error {
	if _, err := os.Stat(o.jobConfigPathKubevirtPresubmits); os.IsNotExist(err) {
		return fmt.Errorf("jobConfigPathKubevirtPresubmits is required: %v", err)
	}
	if _, err := os.Stat(o.jobConfigPathKubevirtPeriodics); os.IsNotExist(err) {
		return fmt.Errorf("jobConfigPathKubevirtPeriodics is required: %v", err)
	}
	return nil
}

var getJobsOpts = getJobOptions{
	output: markdown,
}

var getJobsCommand = &cobra.Command{
	Use:   "jobs",
	Short: shortUse,
	Long: fmt.Sprintf(`%s

For presubmit jobs it returns relevant data about how and when the jobs run in prow terms, i.e. alwaysRun vs runIfChanged, whether they are required for tide to merge, i.e. optional and whether they actually report their status to GitHub.
`, shortUse, strings.Join(prowjobconfigs.SigNames, ", ")),
	RunE: run,
}

func GetJobsCommand() *cobra.Command {
	return getJobsCommand
}

func init() {
	getJobsCommand.PersistentFlags().StringVar(&getJobsOpts.jobConfigPathKubevirtPresubmits, "job-config-path-kubevirt-presubmits", "", "The path to the kubevirt presubmit job definitions")
	getJobsCommand.PersistentFlags().StringVar(&getJobsOpts.jobConfigPathKubevirtPeriodics, "job-config-path-kubevirt-periodics", "", "The path to the kubevirt periodic job definitions")
	getJobsCommand.PersistentFlags().StringVar(&getJobsOpts.outputFile, "output-file", "", "The path to the file where the output is written to")
	getJobsCommand.PersistentFlags().VarP(
		enumflag.New(&getJobsOpts.output, "output", outputTextMapping, enumflag.EnumCaseInsensitive),
		"output", "o",
		"the type of output to generate")
}

func run(cmd *cobra.Command, args []string) error {
	err := flags.ParseFlags(cmd, args, getJobsOpts)
	if err != nil {
		return err
	}

	jobConfigs := map[string]func(io.Writer, *config.JobConfig) error{
		getJobsOpts.jobConfigPathKubevirtPresubmits: printPresubmitJobsForProviders,
		getJobsOpts.jobConfigPathKubevirtPeriodics:  printPeriodicJobsForProviders,
	}

	if getJobsOpts.outputFile == "" {
		return fmt.Errorf("output-file is required")
	}
	file, err := os.OpenFile(getJobsOpts.outputFile, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file %s for writing: %v", getJobsOpts.outputFile, err)
	}
	defer file.Close()

	for jobConfigPath, jobConfigPrintFunc := range jobConfigs {
		jobConfig, err := config.ReadJobConfig(jobConfigPath)
		if err != nil {
			return fmt.Errorf("failed to read jobconfig %s: %v", jobConfigPath, err)
		}

		if err = jobConfigPrintFunc(file, &jobConfig); err != nil {
			return err
		}
	}
	return nil
}

var templatePrintPresubmitJobsForProviders = `
Presubmit Jobs
-------------

| Job Name | always_run | optional | skip_report |
| -------- |: ---------- :|: -------- :|: ----------- :|{{ range $presubmitJob := $.PresubmitJobs }}
| {{ if $presubmitJob.RequiredToMerge }}**{{ end }}{{ $presubmitJob.Name }}{{ if $presubmitJob.RequiredToMerge }}**{{ end }} | {{ if $presubmitJob.Job.AlwaysRun }}✅{{ else }}❎{{ end }} | {{ if $presubmitJob.Job.Optional }}✅{{ else }}❎{{ end }} | {{ if $presubmitJob.Job.SkipReport }}✅{{ else }}❎{{ end }} |{{ end }}

**Note:** jobs required to pass for merging with tide are printed in **bold**
`

type PresubmitJobs struct {
	PresubmitJobs []PresubmitJob
}

type PresubmitJob struct {
	Name            string
	RequiredToMerge bool
	Job             config.Presubmit
}

func printPresubmitJobsForProviders(wr io.Writer, jobConfig *config.JobConfig) error {
	kubevirtE2EPresubmitNames := []string{}
	kubevirtE2EPresubmits := map[string]*config.Presubmit{}
	for index := range jobConfig.PresubmitsStatic[prowjobconfigs.OrgAndRepoForJobConfig] {
		job := jobConfig.PresubmitsStatic[prowjobconfigs.OrgAndRepoForJobConfig][index]
		if !strings.HasPrefix(job.Name, "pull-kubevirt-e2e-k8s-") {
			continue
		}
		kubevirtE2EPresubmitNames = append(kubevirtE2EPresubmitNames, job.Name)
		kubevirtE2EPresubmits[job.Name] = &job
	}

	sort.Sort(sort.Reverse(sort.StringSlice(kubevirtE2EPresubmitNames)))

	parse, err := template.New("presubmits").Parse(templatePrintPresubmitJobsForProviders)
	if err != nil {
		return err
	}

	data := PresubmitJobs{}
	for _, jobName := range kubevirtE2EPresubmitNames {
		presubmitJob := *kubevirtE2EPresubmits[jobName]
		requiredToMerge := !presubmitJob.Optional && presubmitJob.AlwaysRun && !presubmitJob.SkipReport
		data.PresubmitJobs = append(data.PresubmitJobs, PresubmitJob{jobName, requiredToMerge, presubmitJob})
	}
	return parse.Execute(wr, data)
}

func printPeriodicJobsForProviders(wr io.Writer, jobConfig *config.JobConfig) error {
	return nil
}

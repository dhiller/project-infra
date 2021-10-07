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
	"os"
	"strings"

	"github.com/spf13/cobra"

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

var outputTextMapping = map[output]string{
	markdown: "text",
	json:     "json",
}

type getJobOptions struct {
	jobConfigPathKubevirtPresubmits string
	jobConfigPathKubevirtPeriodics  string
	output                          output
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
	getJobsCommand.PersistentFlags().VarP(
		enumflag.New(&getJobsOpts.output, "output", outputTextMapping, enumflag.EnumCaseInsensitive),
		"output", "o",
		"the type of output to generate")
}

func run(cmd *cobra.Command, args []string) error {
	return nil
}

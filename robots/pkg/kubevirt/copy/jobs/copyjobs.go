/*
Copyright 2021 The KubeVirt Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package jobs

import (
	"context"
	"fmt"
	"github.com/spf13/cobra"
	github2 "kubevirt.io/project-infra/robots/pkg/kubevirt/github"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/go-github/github"
	"k8s.io/test-infra/prow/config"
	"sigs.k8s.io/yaml"

	"kubevirt.io/project-infra/robots/pkg/kubevirt/flags"
	"kubevirt.io/project-infra/robots/pkg/kubevirt/log"
	"kubevirt.io/project-infra/robots/pkg/querier"
)

const orgAndRepoForJobConfig = "kubevirt/kubevirt"

type options struct {
	jobConfigPathKubevirtPresubmits string
	jobConfigPathKubevirtPeriodics  string
}

func (o *options) validate() error {
	log.Log().Infof("options: %+v", o)
	if _, err := os.Stat(o.jobConfigPathKubevirtPresubmits); os.IsNotExist(err) {
		return fmt.Errorf("jobConfigPathKubevirtPresubmits is required: %v", err)
	}
	if _, err := os.Stat(o.jobConfigPathKubevirtPeriodics); os.IsNotExist(err) {
		return fmt.Errorf("jobConfigPathKubevirtPeriodics is required: %v", err)
	}
	return nil
}

var cronRegex *regexp.Regexp

var o = options{}

var copyJobsCommand = &cobra.Command{
	Use: "jobs",
	Short: "kubevirt copy jobs copies presubmit job definitions in project-infra for kubevirt/kubevirt repo",
	Run: func(cmd *cobra.Command, args []string) {
		err := cmd.InheritedFlags().Parse(args)
		if err != nil {
			fmt.Println(fmt.Errorf("failed to parse args: %v", err))
			os.Exit(1)
		}

		if err := flags.Options.Validate(); err != nil {
			log.Log().WithError(err).Fatal("Invalid arguments provided.")
		}

		if err := o.validate(); err != nil {
			log.Log().WithError(err).Fatal("Invalid arguments provided.")
		}

		run()
	},
}

func NewCopyJobsCommand() *cobra.Command {
	return copyJobsCommand
}

func init() {
	var err error
	cronRegex, err = regexp.Compile("[0-9] [0-9]+,[0-9]+,[0-9]+ \\* \\* \\*")
	if err != nil {
		panic(err)
	}
	copyJobsCommand.PersistentFlags().StringVar(&o.jobConfigPathKubevirtPresubmits, "job-config-path-kubevirt-presubmits", "", "The path to the kubevirt presubmit job definitions")
	copyJobsCommand.PersistentFlags().StringVar(&o.jobConfigPathKubevirtPeriodics, "job-config-path-kubevirt-periodics", "", "The path to the kubevirt periodic job definitions")
}

func run() {

	ctx := context.Background()
	client := github2.NewGitHubClient(ctx)

	releases, _, err := client.Repositories.ListReleases(ctx, "kubernetes", "kubernetes", nil)
	if err != nil {
		log.Log().Panicln(err)
	}
	releases = querier.ValidReleases(releases)
	if len(releases) < 2 {
		log.Log().Info("No two releases found, nothing to do.")
		os.Exit(0)
	}

	targetRelease, sourceRelease, err := getSourceAndTargetRelease(releases)
	if err != nil {
		log.Log().WithError(err).Info("Cannot determine source and target release.")
		os.Exit(0)
	}

	jobConfigs := map[string]func(*config.JobConfig, *querier.SemVer, *querier.SemVer) bool{
		o.jobConfigPathKubevirtPresubmits: func(jobConfig *config.JobConfig, latestReleaseSemver *querier.SemVer, secondLatestReleaseSemver *querier.SemVer) bool { return CopyPresubmitJobsForNewProvider(jobConfig, latestReleaseSemver, secondLatestReleaseSemver) },
		o.jobConfigPathKubevirtPeriodics:  func(jobConfig *config.JobConfig, latestReleaseSemver *querier.SemVer, secondLatestReleaseSemver *querier.SemVer) bool { return CopyPeriodicJobsForNewProvider(jobConfig, latestReleaseSemver, secondLatestReleaseSemver) },
	}
	for jobConfigPath, jobConfigCopyFunc := range jobConfigs {
		jobConfig, err := config.ReadJobConfig(jobConfigPath)
		if err != nil {
			log.Log().WithField("jobConfigPath", jobConfigPath).WithError(err).Fatal("Failed to read jobconfig")
		}

		updated := jobConfigCopyFunc(&jobConfig, targetRelease, sourceRelease)
		if !updated && !flags.Options.DryRun {
			log.Log().WithField("jobConfigPath", jobConfigPath).Info(fmt.Sprintf("presubmit jobs for %v weren't modified, nothing to do.", targetRelease))
			continue
		}

		marshalledConfig, err := yaml.Marshal(&jobConfig)
		if err != nil {
			log.Log().WithField("jobConfigPath", jobConfigPath).WithError(err).Error("Failed to marshall jobconfig")
		}

		if flags.Options.DryRun {
			_, err = os.Stdout.Write(marshalledConfig)
			if err != nil {
				log.Log().WithField("jobConfigPath", jobConfigPath).WithError(err).Error("Failed to write jobconfig")
			}
			continue
		}

		err = os.WriteFile(jobConfigPath, marshalledConfig, os.ModePerm)
		if err != nil {
			log.Log().WithField("jobConfigPath", jobConfigPath).WithError(err).Error("Failed to write jobconfig")
		}
	}
}

func getSourceAndTargetRelease(releases []*github.RepositoryRelease) (targetRelease *querier.SemVer, sourceRelease *querier.SemVer, err error) {
	if len(releases) < 2 {
		err = fmt.Errorf("less than two releases")
		return
	}
	targetRelease = querier.ParseRelease(releases[0])
	for _, release := range releases[1:] {
		nextRelease := querier.ParseRelease(release)
		if nextRelease.Minor < targetRelease.Minor {
			sourceRelease = nextRelease
			break
		}
	}
	if sourceRelease == nil {
		err = fmt.Errorf("no source release found")
	}
	return
}

var sigNames = []string{
	"sig-network",
	"sig-storage",
	"sig-compute",
	"operator",
}

func CopyPresubmitJobsForNewProvider(jobConfig *config.JobConfig, targetProviderReleaseSemver *querier.SemVer, sourceProviderReleaseSemver *querier.SemVer) (updated bool) {
	allPresubmitJobs := map[string]config.Presubmit{}
	for index := range jobConfig.PresubmitsStatic[orgAndRepoForJobConfig] {
		job := jobConfig.PresubmitsStatic[orgAndRepoForJobConfig][index]
		allPresubmitJobs[job.Name] = job
	}

	for _, sigName := range sigNames {
		targetJobName := createPresubmitJobName(targetProviderReleaseSemver, sigName)
		sourceJobName := createPresubmitJobName(sourceProviderReleaseSemver, sigName)

		if _, exists := allPresubmitJobs[targetJobName]; exists {
			log.Log().WithField("targetJobName", targetJobName).WithField("sourceJobName", sourceJobName).Info("Target job exists, nothing to do")
			continue
		}

		if _, exists := allPresubmitJobs[sourceJobName]; !exists {
			log.Log().WithField("targetJobName", targetJobName).WithField("sourceJobName", sourceJobName).Warn("Source job does not exist, can't copy job definition!")
			continue
		}

		log.Log().WithField("targetJobName", targetJobName).WithField("sourceJobName", sourceJobName).Info("Copying source to target job")

		newJob := config.Presubmit{}
		newJob.Annotations = make(map[string]string)
		for k, v := range allPresubmitJobs[sourceJobName].Annotations {
			newJob.Annotations[k] = v
		}
		newJob.Cluster = allPresubmitJobs[sourceJobName].Cluster
		newJob.Decorate = allPresubmitJobs[sourceJobName].Decorate
		newJob.DecorationConfig = allPresubmitJobs[sourceJobName].DecorationConfig.DeepCopy()
		copy(newJob.ExtraRefs, allPresubmitJobs[sourceJobName].ExtraRefs)
		newJob.Labels = make(map[string]string)
		for k, v := range allPresubmitJobs[sourceJobName].Labels {
			newJob.Labels[k] = v
		}
		newJob.MaxConcurrency = allPresubmitJobs[sourceJobName].MaxConcurrency
		newJob.Spec = allPresubmitJobs[sourceJobName].Spec.DeepCopy()

		newJob.AlwaysRun = false
		for index, envVar := range newJob.Spec.Containers[0].Env {
			if envVar.Name != "TARGET" {
				continue
			}
			newEnvVar := *envVar.DeepCopy()
			newEnvVar.Value = createTargetValue(targetProviderReleaseSemver, sigName)
			newJob.Spec.Containers[0].Env[index] = newEnvVar
			break
		}
		newJob.Name = targetJobName
		newJob.Optional = true
		jobConfig.PresubmitsStatic[orgAndRepoForJobConfig] = append(jobConfig.PresubmitsStatic[orgAndRepoForJobConfig], newJob)

		updated = true
	}

	return
}

func CopyPeriodicJobsForNewProvider(jobConfig *config.JobConfig, targetProviderReleaseSemver *querier.SemVer, sourceProviderReleaseSemver *querier.SemVer) (updated bool) {
	allPeriodicJobs := map[string]config.Periodic{}
	for index := range jobConfig.Periodics {
		job := jobConfig.Periodics[index]
		allPeriodicJobs[job.Name] = job
	}

	for _, sigName := range sigNames {
		targetJobName := createPeriodicJobName(targetProviderReleaseSemver, sigName)
		sourceJobName := createPeriodicJobName(sourceProviderReleaseSemver, sigName)

		if _, exists := allPeriodicJobs[targetJobName]; exists {
			log.Log().WithField("targetJobName", targetJobName).WithField("sourceJobName", sourceJobName).Info("Target job exists, nothing to do")
			continue
		}

		if _, exists := allPeriodicJobs[sourceJobName]; !exists {
			log.Log().WithField("targetJobName", targetJobName).WithField("sourceJobName", sourceJobName).Warn("Source job does not exist, can't copy job definition!")
			continue
		}

		log.Log().WithField("targetJobName", targetJobName).WithField("sourceJobName", sourceJobName).Info("Copying source to target job")

		newJob := config.Periodic{}
		newJob.Annotations = make(map[string]string)
		for k, v := range allPeriodicJobs[sourceJobName].Annotations {
			newJob.Annotations[k] = v
		}
		newJob.Cluster = allPeriodicJobs[sourceJobName].Cluster
		newJob.Cron = advanceCronExpression(allPeriodicJobs[sourceJobName].Cron)
		newJob.Decorate = allPeriodicJobs[sourceJobName].Decorate
		newJob.DecorationConfig = allPeriodicJobs[sourceJobName].DecorationConfig.DeepCopy()
		copy(newJob.ExtraRefs, allPeriodicJobs[sourceJobName].ExtraRefs)
		newJob.Labels = make(map[string]string)
		for k, v := range allPeriodicJobs[sourceJobName].Labels {
			newJob.Labels[k] = v
		}
		newJob.MaxConcurrency = allPeriodicJobs[sourceJobName].MaxConcurrency
		newJob.ReporterConfig = allPeriodicJobs[sourceJobName].ReporterConfig.DeepCopy()
		newJob.Spec = allPeriodicJobs[sourceJobName].Spec.DeepCopy()

		for _, extraRef := range allPeriodicJobs[sourceJobName].UtilityConfig.ExtraRefs {
			newJob.UtilityConfig.ExtraRefs = append(newJob.UtilityConfig.ExtraRefs, extraRef)
		}

		for index, envVar := range newJob.Spec.Containers[0].Env {
			if envVar.Name != "TARGET" {
				continue
			}
			newEnvVar := *envVar.DeepCopy()
			newEnvVar.Value = createTargetValue(targetProviderReleaseSemver, sigName)
			newJob.Spec.Containers[0].Env[index] = newEnvVar
			break
		}
		newJob.Name = targetJobName
		jobConfig.Periodics = append(jobConfig.Periodics, newJob)

		updated = true
	}

	return
}

func createPresubmitJobName(latestReleaseSemver *querier.SemVer, sigName string) string {
	return fmt.Sprintf("pull-kubevirt-e2e-k8s-%s.%s-%s", latestReleaseSemver.Major, latestReleaseSemver.Minor, sigName)
}

func createPeriodicJobName(latestReleaseSemver *querier.SemVer, sigName string) string {
	return fmt.Sprintf("periodic-kubevirt-e2e-k8s-%s.%s-%s", latestReleaseSemver.Major, latestReleaseSemver.Minor, sigName)
}

func createTargetValue(latestReleaseSemver *querier.SemVer, sigName string) string {
	return fmt.Sprintf("k8s-%s.%s-%s", latestReleaseSemver.Major, latestReleaseSemver.Minor, sigName)
}

// advanceCronExpression advances source cron expression to +1h10m
// cron expression must have format of i.e. "0 1,9,17 * * *" or it will panic
func advanceCronExpression (sourceCronExpr string) string {
	if !cronRegex.MatchString(sourceCronExpr) {
		log.Log().WithField("cronRegex", cronRegex).WithField("sourceCronExpr", sourceCronExpr).Fatal("cronRegex doesn't match")
	}
	parts := strings.Split(sourceCronExpr, " ")
	mins, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		panic(err)
	}
	mins = ( mins + 10 ) % 60
	firstHour, err := strconv.ParseInt(strings.Split(parts[1], ",")[0], 10, 64)
	firstHour = ( firstHour + 1 ) % 8
	return fmt.Sprintf("%d %d,%d,%d * * *", mins, firstHour, firstHour + 8, firstHour + 16)
}

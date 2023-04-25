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
 * Copyright 2023 Red Hat, Inc.
 *
 */

package filter

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	test_report "kubevirt.io/project-infra/robots/pkg/test-report"
	"os"
	"regexp"
	"sigs.k8s.io/yaml"
	"sort"
)

var filterCmd = &cobra.Command{
	Use:   "filter",
	Short: "filter takes the first argument as a json in format map[string]map[string]int and returns the test names of tests that have not run",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("no file as argument given")
		}
		_, err := os.Stat(args[0])
		if os.IsNotExist(err) {
			return err
		}
		file, err := os.ReadFile(args[0])
		if err != nil {
			return err
		}
		var input *map[string]map[string]int
		err = json.Unmarshal(file, &input)
		if err != nil {
			return err
		}
		filtered := runFilter(input, nil)
		marshal, err := yaml.Marshal(filtered)
		if err != nil {
			return err
		}
		fmt.Printf(string(marshal))
		return nil
	},
}

func FilterCmd() *cobra.Command {
	return filterCmd
}

type groupConfig struct {
	name    string
	sig     *regexp.Regexp
	lanes   *regexp.Regexp
	version *regexp.Regexp
}

type groupConfigs []groupConfig

const versionPattern = ".*-(4\\.[0-9]{2}).*"

var defaultGroupConfigs = groupConfigs{
	groupConfig{
		name:    "storage",
		version: regexp.MustCompile(versionPattern),
		sig:     regexp.MustCompile(".*\\[sig-storage].*"),
		lanes:   regexp.MustCompile(".*(storage|quarantined).*"),
	},
	groupConfig{
		name:    "virtualization",
		version: regexp.MustCompile(versionPattern),
		sig:     regexp.MustCompile(".*\\[sig-(compute|operator)].*"),
		lanes:   regexp.MustCompile(".*(compute|operator|quarantined).*"),
	},
	groupConfig{
		name:    "network",
		version: regexp.MustCompile(versionPattern),
		sig:     regexp.MustCompile(".*\\[sig-network].*"),
		lanes:   regexp.MustCompile(".*(network|quarantined).*"),
	},
	groupConfig{
		name:    "ssp",
		version: regexp.MustCompile(versionPattern),
		sig:     regexp.MustCompile(".*"),
		lanes:   regexp.MustCompile(".*ssp.*"),
	},
}

func runFilter(input *map[string]map[string]int, groupConfigs groupConfigs) map[string]map[string][]string {
	if groupConfigs == nil {
		groupConfigs = defaultGroupConfigs
	}
	result := map[string]map[string][]string{}
	for testName, testLanesToExecutions := range *input {
		groupsToVersions := map[string]map[string]struct{}{}
		for _, currentGroupConfig := range groupConfigs {
			if !currentGroupConfig.sig.MatchString(testName) {
				continue
			}
			testRunPerVersion := map[string]bool{}
			for testLane, testExecution := range testLanesToExecutions {
				if !currentGroupConfig.lanes.MatchString(testLane) || !currentGroupConfig.version.MatchString(testLane) {
					continue
				}
				version := currentGroupConfig.version.FindStringSubmatch(testLane)[1]
				if _, exists := testRunPerVersion[version]; !exists {
					testRunPerVersion[version] = false
				}
				if testExecution == test_report.TestExecution_Run {
					testRunPerVersion[version] = true
				}
			}
			for version, testRun := range testRunPerVersion {
				if !testRun {
					if _, exists := groupsToVersions[currentGroupConfig.name]; !exists {
						groupsToVersions[currentGroupConfig.name] = map[string]struct{}{}
					}
					if _, exists := groupsToVersions[currentGroupConfig.name][version]; !exists {
						groupsToVersions[currentGroupConfig.name][version] = struct{}{}
					}
				}
			}
		}
		if len(groupsToVersions) == 0 {
			continue
		}
		for group, versions := range groupsToVersions {
			if len(versions) == 0 {
				continue
			}
			if _, exists := result[group]; !exists {
				result[group] = map[string][]string{}
			}
			for version := range versions {
				if _, exists := result[group][version]; !exists {
					result[group][version] = []string{testName}
				} else {
					result[group][version] = append(result[group][version], testName)
				}
			}
		}
	}
	for _, versions := range result {
		for _, testNames := range versions {
			sort.Strings(testNames)
		}
	}
	return result
}

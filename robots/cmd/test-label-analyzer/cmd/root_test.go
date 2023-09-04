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
 */

package cmd

import (
	"fmt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"kubevirt.io/project-infra/robots/pkg/git"
	test_label_analyzer "kubevirt.io/project-infra/robots/pkg/test-label-analyzer"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"time"
)

var _ = Describe("root tests", func() {

	Context("getConfig", func() {

		DescribeTable("returns a config",
			func(options *ConfigOptions, expectedConfig *test_label_analyzer.Config, expectedErr error) {
				config, err := options.getConfig()
				if err != nil {
					Expect(err).To(BeEquivalentTo(expectedErr))
				} else {
					Expect(config).To(BeEquivalentTo(expectedConfig))
					Expect(err).ToNot(HaveOccurred())
				}
			},
			Entry("returns err if no config selected",
				&ConfigOptions{
					ConfigFile:         "",
					ConfigName:         "",
					ginkgoOutlinePaths: nil,
					testFilePath:       "",
					remoteURL:          "",
					testNameLabelRE:    "",
					outputHTML:         false,
				},
				nil,
				fmt.Errorf("no configuration found!"),
			),
			Entry("for simple RE",
				&ConfigOptions{
					ConfigFile:         "",
					ConfigName:         "",
					ginkgoOutlinePaths: nil,
					testFilePath:       "",
					remoteURL:          "",
					testNameLabelRE:    "test regex",
					outputHTML:         false,
				},
				test_label_analyzer.NewTestNameDefaultConfig("test regex"),
				nil,
			),
			Entry("for quarantine config",
				&ConfigOptions{
					ConfigFile:         "",
					ConfigName:         "quarantine",
					ginkgoOutlinePaths: nil,
					testFilePath:       "",
					remoteURL:          "",
					testNameLabelRE:    "",
					outputHTML:         false,
				},
				test_label_analyzer.NewQuarantineDefaultConfig(),
				nil,
			),
		)

		// FIXME
		/*
			[FAILED] Unexpected error:
			    <*errors.errorString | 0xc0001ca330>:
			    exec /usr/bin/git blame filter-test-names.json failed: fatal: no such ref: HEAD

			    {
			        s: "exec /usr/bin/git blame filter-test-names.json failed: fatal: no such ref: HEAD\n",
			    }
			occurred
		*/
		PIt("loads for file with test names", func() {
			var tempDir string
			var err error

			tempDir, err = os.MkdirTemp("", "")
			Expect(err).ToNot(HaveOccurred())
			command := exec.Command("git", "init")
			command.Dir = tempDir
			err = command.Run()
			Expect(err).ToNot(HaveOccurred())

			const gitTestFileName = "testdata/filter-test-names.json"
			var file []byte
			file, err = os.ReadFile(gitTestFileName)
			Expect(err).ToNot(HaveOccurred())
			targetFile := filepath.Join(tempDir, path.Base(gitTestFileName))
			err = os.WriteFile(targetFile, file, 0666)
			Expect(err).ToNot(HaveOccurred())

			command = exec.Command("git", "add", path.Base(gitTestFileName))
			command.Dir = tempDir
			err = command.Run()
			Expect(err).ToNot(HaveOccurred())

			command = exec.Command("git", "commit", "-m", "test commit")
			command.Dir = tempDir
			err = command.Run()

			var gitBlameLines []*git.BlameLine
			gitBlameLines, err = git.GetBlameLinesForFile(targetFile)
			Expect(err).ToNot(HaveOccurred())

			options := &ConfigOptions{
				ConfigFile:          "",
				FilterTestNamesFile: targetFile,
				ConfigName:          "",
				ginkgoOutlinePaths:  nil,
				testFilePath:        "",
				remoteURL:           "",
				testNameLabelRE:     "",
				outputHTML:          false,
			}
			expectedConfig := &test_label_analyzer.Config{
				Categories: []*test_label_analyzer.LabelCategory{
					{
						Name:            "flaky",
						TestNameLabelRE: test_label_analyzer.NewRegexp("test name 1"),
						GinkgoLabelRE:   nil,
						BlameLine:       gitBlameLines[2],
					},
					{
						Name:            "also flaky",
						TestNameLabelRE: test_label_analyzer.NewRegexp("test name 2"),
						GinkgoLabelRE:   nil,
						BlameLine:       gitBlameLines[6],
					},
					{
						Name:            "also flaky",
						TestNameLabelRE: test_label_analyzer.NewRegexp(regexp.QuoteMeta("[sig-compute]test name 3")),
						GinkgoLabelRE:   nil,
						BlameLine:       gitBlameLines[10],
					},
				},
			}

			config, err := options.getConfig()
			Expect(config).To(BeEquivalentTo(expectedConfig))
			Expect(err).ToNot(HaveOccurred())
		})
	})
})

func mustParseDate(date string) time.Time {
	parse, err := time.Parse(time.RFC3339, date)
	Expect(err).ToNot(HaveOccurred())
	return parse
}

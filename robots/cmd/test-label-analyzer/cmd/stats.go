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

	"github.com/spf13/cobra"
)

// statsCmd represents the stats command
var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Generates stats over test categories",
	Long:  `TODO`,
	RunE:  runStatsCommand,
}

func init() {
	rootCmd.AddCommand(statsCmd)
}

func runStatsCommand(cmd *cobra.Command, args []string) error {
	err := configOpts.verify()
	if err != nil {
		return err
	}

	ginkgo.internal.

	return fmt.Errorf("stats called")
}

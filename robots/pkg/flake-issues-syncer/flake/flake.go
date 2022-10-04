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

// Package flake contains code to transform FlakeFinder data into actual flake entities and transform those into
// modifications to the target issue tracker system.
//
// A flake (aka flaky test or non-deterministic test) in general describes a test that has not always
// had a succeeding run.
//
// "... Left uncontrolled, non-deterministic tests can completely destroy the value of an automated regression suite. ..."
// [Eradicating Non-Determinism in Tests]
//
// We can have two types of flakes:
//  * those that are always failing, and
//  * those that have a ratio of failed vs. succeeded runs > 0, i.e. a test that has failed at least
// once within the observed interval
//
// While the former indicate a problem either in the test code, or a defect in the code under test, the
// latter either indicate a problem with the setup of the test environment or an incomplete observation of
// the behaviour that is tested, both resulting in a problematic test that needs to get fixed.
//
// A flake can have multiple data sources, i.e. it may have been encountered in several reports.
// It is
//  * new, if it has not been encountered in an earlier report with the same interval, or
//  * existing, if it has been encountered in an earlier report with the same interval, or
//  * old, if it has not been encountered in the latest report with the shortest interval.
//
// A new issue must be created with a description containing advice for each of the involved parties on what to do with
// it, links to where to find the information required to analyze the failures, i.e. links to reports, builds, etc.
//
// An existing issue must be updated with the latest information in form of a comment.
//
// An old issue must be updated with a comment that it has not been seen in the latest report.
//
// [Eradicating Non-Determinism in Tests]: https://www.martinfowler.com/articles/nonDeterminism.html
package flake

import (
	"encoding/json"
	"fmt"
	"io"
	"kubevirt.io/project-infra/robots/pkg/flakefinder"
)

// A Flake contains the observed state of a flake collected from all the report data involved, eventually
// spanning several intervals with differing sizes.
type Flake struct {

	// TestName is the test name as extracted from the JUnit file
	TestName string `json:"testName"`

	// ReportSourcesToDetails has collected every occurrence of the flakes in any of the exported data files,
	// where the URL to the data file acts as the first key and the name of the test lane as the second key
	ReportSourcesToDetails map[string]map[string]*flakefinder.Details `json:"reportSourcesToDetails"`
}

func (f Flake) String() string {
	marshal, err := json.Marshal(f)
	if err != nil {
		panic(err)
	}
	return string(marshal)
}

// ToFlakes converts flakefinder.Params json format files to a set of Flake s that contain the data aggregated from a
// single Flake perspective, i.e. this Flake with this Flake.TestName has been seen in these
// Flake.ReportSourcesToDetails
func ToFlakes(readers map[string]io.Reader) ([]Flake, error) {
	var result []Flake
	for key, r := range readers {
		var p *flakefinder.Params
		err := json.NewDecoder(r).Decode(&p)
		if err != nil {
			return nil, fmt.Errorf("could not read %q: %v", r, err)
		}
		for _, testName := range p.Tests {
			m := p.Data[testName]
			flake := Flake{
				TestName: testName,
				ReportSourcesToDetails: map[string]map[string]*flakefinder.Details{
					key: m,
				},
			}
			result = append(result, flake)
		}
	}
	return result, nil
}

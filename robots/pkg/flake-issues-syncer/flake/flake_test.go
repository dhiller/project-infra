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

package flake

import (
	"io"
	"kubevirt.io/project-infra/robots/pkg/flakefinder"
	"os"
	"reflect"
	"testing"
)

func TestToFlakes(t *testing.T) {
	type args struct {
		readers map[string]io.Reader
	}
	tests := []struct {
		name         string
		args         args
		wantedError  error
		wantedFlakes []Flake
	}{
		{
			name: "empty",
			args: args{
				readers: map[string]io.Reader{
					"testdata/flakefinder-reduced-data-empty.json": toReader("testdata/flakefinder-reduced-data-empty.json"),
				},
			},
			wantedError:  nil,
			wantedFlakes: nil,
		},
		{
			name: "simple",
			args: args{
				readers: map[string]io.Reader{
					"testdata/flakefinder-reduced-data-one-entry.json": toReader("testdata/flakefinder-reduced-data-one-entry.json"),
				},
			},
			wantedError: nil,
			wantedFlakes: []Flake{
				{
					TestName: "[Serial][sig-compute]MediatedDevices with mediated devices configuration Should override default mdev configuration on a specific node",
					ReportSourcesToDetails: map[string]map[string]*flakefinder.Details{
						"testdata/flakefinder-reduced-data-one-entry.json": {
							"pull-kubevirt-e2e-kind-1.23-vgpu": {
								Succeeded: 0,
								Skipped:   0,
								Failed:    4,
								Severity:  "red",
								Jobs: []*flakefinder.Job{
									{
										BuildNumber: 1575059363163279360,
										Severity:    "red",
										PR:          8528,
										Job:         "pull-kubevirt-e2e-kind-1.23-vgpu",
									},
								},
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFlakes, gotErrors := ToFlakes(tt.args.readers)
			if !reflect.DeepEqual(gotFlakes, tt.wantedFlakes) {
				t.Errorf("ToFlakes() gotFlakes = %v, want %v", gotFlakes, tt.wantedFlakes)
			}
			if !reflect.DeepEqual(gotErrors, tt.wantedError) {
				t.Errorf("ToFlakes() gotErrors = %v, want %v", gotErrors, tt.wantedError)
			}
		})
	}
}

func toReader(fileName string) io.Reader {
	file, err := os.Open(fileName)
	if err != nil {
		panic(err)
	}
	return file
}

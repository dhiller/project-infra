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

package jira

import (
	"reflect"
	"testing"
)

func TestIssue_Merge(t *testing.T) {
	type fields struct {
		Summary     string
		Description string
		Labels      []string
		Components  []string
		Links       []string
		Type        string
		Status      string
		Identity    *IssueTrackerIdentity
	}
	type args struct {
		issue *Issue
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *Issue
		wantErr bool
	}{
		{
			name: "summary: keeps the old one",
			fields: fields{
				Summary: "meh",
			},
			args: args{
				issue: &Issue{
					Summary: "moo",
				},
			},
			want: &Issue{
				Summary: "meh",
			},
		},
		{
			name: "description: separates by new line",
			fields: fields{
				Description: "this is a test",
			},
			args: args{
				issue: &Issue{
					Description: "this is a test",
				},
			},
			want: &Issue{
				Description: `this is a test

this is a test`,
			},
		},
		{
			name: "labels: no duplicates",
			fields: fields{
				Labels: []string{"CNVQuarantine", "A"},
			},
			args: args{
				issue: &Issue{
					Labels: []string{"CNVQuarantine", "B"},
				},
			},
			want: &Issue{
				Labels: []string{"A", "B", "CNVQuarantine"},
			},
		},
		{
			name: "status: keeps existing",
			fields: fields{
				Status: "To Do",
			},
			args: args{
				issue: &Issue{
					Status: "In Progress",
				},
			},
			want: &Issue{
				Status: "To Do",
			},
		},
		{
			name: "components: merges w/o duplicates",
			fields: fields{
				Components: []string{"CNV Virtualization", "CNV Network"},
			},
			args: args{
				issue: &Issue{
					Components: []string{"CNV Virtualization"},
				},
			},
			want: &Issue{
				Components: []string{"CNV Network", "CNV Virtualization"},
			},
		},
		{
			name: "links: merges",
			fields: fields{
				Links: []string{"https://a", "https://b"},
			},
			args: args{
				issue: &Issue{
					Links: []string{"https://a"},
				},
			},
			want: &Issue{
				Links: []string{"https://a", "https://b", "https://a"},
			},
		},
		{
			name: "Type: keeps existing",
			fields: fields{
				Type: "Story",
			},
			args: args{
				issue: &Issue{
					Type: "Task",
				},
			},
			want: &Issue{
				Type: "Story",
			},
		},
		{
			name: "merge existing with another issue",
			fields: fields{
				Identity: &IssueTrackerIdentity{
					Key: "CNV-1742",
				},
			},
			args: args{
				issue: &Issue{},
			},
			want: &Issue{
				Identity: &IssueTrackerIdentity{
					Key: "CNV-1742",
				},
			},
		},
		{
			name: "fail: can't merge two already persisted issues",
			fields: fields{
				Identity: &IssueTrackerIdentity{
					Key: "CNV-1742",
				},
			},
			args: args{
				issue: &Issue{
					Identity: &IssueTrackerIdentity{
						Key: "CNV-1742",
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := Issue{
				Summary:     tt.fields.Summary,
				Description: tt.fields.Description,
				Labels:      tt.fields.Labels,
				Components:  tt.fields.Components,
				Links:       tt.fields.Links,
				Type:        tt.fields.Type,
				Status:      tt.fields.Status,
				Identity:    tt.fields.Identity,
			}
			got, err := i.Merge(tt.args.issue)
			if tt.wantErr && err == nil {
				t.Errorf("Merge() = %v, want %v", false, tt.wantErr)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Merge() = %v, want %v", err, false)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Merge() = %v, want %v", got, tt.want)
			}
		})
	}
}

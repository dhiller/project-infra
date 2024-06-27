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
 * Copyright the KubeVirt Authors.
 *
 */

package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/google/go-github/github"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"net/http"
	"os"
	"sigs.k8s.io/yaml"
	"strings"
)

const robotName = "org-access-checker"

type options struct {
	debugLogging bool

	tokenPath string
	endpoint  string

	org string
}

type AccessPermissionsToRepositories map[string]RepositoriesToCollaborators
type RepositoriesToCollaborators map[string]Collaborators
type Collaborators []string

func (o *options) Validate() error {
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	fs.StringVar(&o.tokenPath, "github-token-path", "/etc/github/oauth", "Path to the file containing the GitHub OAuth secret.")
	fs.StringVar(&o.endpoint, "github-endpoint", "https://api.github.com/", "GitHub's API endpoint (may differ for enterprise).")
	fs.StringVar(&o.org, "org", "kubevirt", "The GitHub org")
	fs.BoolVar(&o.debugLogging, "v", false, "verbose aka debug logging")
	err := fs.Parse(os.Args[1:])
	if err != nil {
		return err
	}
	if o.tokenPath == "" {
		return fmt.Errorf("github-token-path is required")
	}
	if o.org == "" {
		return fmt.Errorf("org is required")
	}
	return nil
}

var (
	o                = options{}
	checkPermissions = []string{"admin", "maintain", "push"}
)

func init() {
	logrus.SetFormatter(&logrus.JSONFormatter{})
}
func main() {
	err := o.Validate()
	if err != nil {
		log().WithError(err).Fatal("failed to validate options")
	}

	if o.debugLogging {
		logrus.SetLevel(logrus.DebugLevel)
	}

	rawToken, err := os.ReadFile(o.tokenPath)
	if err != nil {
		log().WithError(err).Fatalf("failed to read token %q", o.tokenPath)
	}
	token := strings.TrimSpace(string(rawToken))
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	githubClient, err := github.NewEnterpriseClient(o.endpoint, o.endpoint, oauth2.NewClient(ctx, ts))
	if err != nil {
		log().WithError(err).Fatal("failed to create github client")
	}

	admins, r, err := githubClient.Organizations.ListMembers(ctx, o.org, &github.ListMembersOptions{
		Role: "admin",
	})
	githubOrgAdmins := make(map[string]struct{})
	switch r.StatusCode {
	case http.StatusOK:
		var adminHandles []string
		for _, admin := range admins {
			adminHandles = append(adminHandles, admin.GetLogin())
			githubOrgAdmins[admin.GetLogin()] = struct{}{}
		}
		log().Infof("org admins are %v", adminHandles)
		break
	default:
		log().WithError(err).Fatalf("failed to get github repositories for org %q", o.org)
	}

	accessPermissionsToRepos := AccessPermissionsToRepositories{
		"admin": RepositoriesToCollaborators{},
	}

	defaultListOptions := github.ListOptions{
		PerPage: 99999,
	}
	repos, r, err := githubClient.Repositories.ListByOrg(ctx, o.org, &github.RepositoryListByOrgOptions{
		Type:        "public",
		ListOptions: defaultListOptions,
	})
	switch r.StatusCode {
	case http.StatusOK:
		log().Infof("%d repositories found", len(repos))
		for _, repo := range repos {
			if repo.GetArchived() || repo.GetPrivate() {
				logForRepo(repo).Info("skipping repo since archived or private")
				continue
			}
			accessPermissionsToRepos["admin"][repo.GetName()] = Collaborators{}
			logForRepo(repo).Info("checking permissions")
			collaborators, r, err := githubClient.Repositories.ListCollaborators(ctx, o.org, repo.GetName(), &github.ListCollaboratorsOptions{
				Affiliation: "all",
				ListOptions: defaultListOptions,
			})
			if err != nil {
				logForRepo(repo).WithError(err).Fatal("failed to get github collaborators")
			}
			switch r.StatusCode {
			case http.StatusOK:
				for _, collaborator := range collaborators {
					if _, isAdmin := githubOrgAdmins[collaborator.GetLogin()]; isAdmin {
						logForRepo(repo).WithField("collaborator", collaborator.GetLogin()).Debug("skipping permission check for org admin")
						continue
					}
					for _, perm := range checkPermissions {
						permissions := fetchImportantPermissions(collaborator)
						if permissions[perm] {
							if _, ok := accessPermissionsToRepos[perm]; !ok {
								accessPermissionsToRepos[perm] = RepositoriesToCollaborators{}
							}
							logForRepo(repo).WithField("collaborator", collaborator.GetLogin()).Debugf("permission %q seen", perm)
							accessPermissionsToRepos[perm][repo.GetName()] = append(accessPermissionsToRepos[perm][repo.GetName()], collaborator.GetLogin())
						}
					}
				}
				break
			default:
				logForRepo(repo).WithError(err).Fatal("failed to get github collaborators")
			}
		}
	default:
		log().WithError(err).Fatal("failed to get github repositories")
	}

	marshal, err := yaml.Marshal(accessPermissionsToRepos)
	if err != nil {
		log().WithError(err).Fatal("failed to marshall file")
	}
	temp, err := os.CreateTemp("", "org-access-*.yaml")
	err = os.WriteFile(temp.Name(), marshal, 0666)
	if err != nil {
		log().WithError(err).Fatalf("failed to write to file %q", temp.Name())
	}
	log().Infof("File written to: %s", temp.Name())
}

func fetchImportantPermissions(collaborator *github.User) map[string]bool {
	permissions := collaborator.GetPermissions()
	delete(permissions, "pull")
	delete(permissions, "triage")
	return permissions
}

func logForRepo(repo *github.Repository) *logrus.Entry {
	return log().WithField("repo", repo.GetName())
}

func log() *logrus.Entry {
	return logrus.StandardLogger().WithFields(logrus.Fields{
		"robot": robotName,
		"org":   o.org,
	})
}

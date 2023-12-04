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
 * Copyright the KubeVirt authors.
 *
 */

package review

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/sourcegraph/go-diff/diff"
	"k8s.io/test-infra/prow/git"
	"k8s.io/test-infra/prow/github"
	"os/exec"
	"strings"
)

type KindOfChange interface {
	AddIfRelevant(fileDiff *diff.FileDiff)
	Review() BotReviewResult
	IsRelevant() bool
}

type BotReviewResult interface {
	String() string

	// IsApproved states if the review has only expected changes
	IsApproved() bool

	// CanMerge states if the pull request can get merged without any further action
	CanMerge() bool

	// AddReviewFailure stores the data of a hunk of code that failed review
	AddReviewFailure(fileName string, hunks ...*diff.Hunk)

	// ShortString provides a short description of the review result
	ShortString() string
}

func newPossibleReviewTypes() []KindOfChange {
	return []KindOfChange{
		&ProwJobImageUpdate{},
		&BumpKubevirtCI{},
		&ProwAutobump{},
	}
}

func GuessReviewTypes(fileDiffs []*diff.FileDiff) []KindOfChange {
	possibleReviewTypes := newPossibleReviewTypes()
	for _, fileDiff := range fileDiffs {
		for _, kindOfChange := range possibleReviewTypes {
			kindOfChange.AddIfRelevant(fileDiff)
		}
	}
	result := []KindOfChange{}
	for _, t := range possibleReviewTypes {
		if t.IsRelevant() {
			result = append(result, t)
		}
	}
	return result
}

type Reviewer struct {
	l       *logrus.Entry
	org     string
	repo    string
	num     int
	user    string
	action  github.PullRequestEventAction
	dryRun  bool
	BaseSHA string
}

func NewReviewer(l *logrus.Entry, action github.PullRequestEventAction, org string, repo string, num int, user string, dryRun bool) *Reviewer {
	return &Reviewer{
		l:      l,
		org:    org,
		repo:   repo,
		num:    num,
		user:   user,
		action: action,
		dryRun: dryRun,
	}
}

func (r *Reviewer) withFields() *logrus.Entry {
	return r.l.WithField("dryRun", r.dryRun).WithField("org", r.org).WithField("repo", r.repo).WithField("pr", r.num).WithField("user", r.user)
}
func (r *Reviewer) info(message string) {
	r.withFields().Info(message)
}
func (r *Reviewer) infoF(message string, args ...interface{}) {
	r.withFields().Infof(message, args...)
}
func (r *Reviewer) fatalF(message string, args ...interface{}) {
	r.withFields().Fatalf(message, args...)
}
func (r *Reviewer) debugF(message string, args ...interface{}) {
	r.withFields().Debugf(message, args...)
}

func (r *Reviewer) ReviewLocalCode() ([]BotReviewResult, error) {

	r.info("preparing review")

	diffCommand := exec.Command("git", "diff", "..main")
	if r.BaseSHA != "" {
		diffCommand = exec.Command("git", "diff", fmt.Sprintf("%s..%s", r.BaseSHA, "HEAD"))
	}
	output, err := diffCommand.Output()
	if err != nil {
		r.fatalF("could not fetch diff output: %v", err)
	}

	multiFileDiffReader := diff.NewMultiFileDiffReader(strings.NewReader(string(output)))
	files, err := multiFileDiffReader.ReadAllFiles()
	if err != nil {
		r.fatalF("could not create diffs from output: %v", err)
	}

	types := GuessReviewTypes(files)
	if len(types) == 0 {
		r.info("this PR didn't match any review type")
		return nil, nil
	}

	results := []BotReviewResult{}
	for _, reviewType := range types {
		result := reviewType.Review()
		results = append(results, result)
	}

	return results, nil
}

const botReviewCommentPattern = `@%s's review-bot says:

%s

%s

%s

**Note: botreview (kubevirt/project-infra#2448) is a Work In Progress!**
`
const holdPRComment = `Holding this PR for further manual action to occur.

/hold`
const unholdPRComment = "This PR does not require further manual action."

const approvePRComment = `This PR satisfies all automated review criteria.

/lgtm
/approve`
const unapprovePRComment = "This PR does not satisfy at least one automated review criteria."

func (r *Reviewer) AttachReviewComments(botReviewResults []BotReviewResult, githubClient github.Client) error {
	botUser, err := githubClient.BotUser()
	if err != nil {
		return fmt.Errorf("error while fetching user data: %v", err)
	}
	isApproved, canMerge := true, true
	botReviewComments := make([]string, 0, len(botReviewResults))
	shortBotReviewComments := make([]string, 0, len(botReviewResults))
	for _, reviewResult := range botReviewResults {
		isApproved, canMerge = isApproved && reviewResult.IsApproved(), canMerge && reviewResult.CanMerge()
		botReviewComments = append(botReviewComments, fmt.Sprintf("%s", reviewResult))
		shortBotReviewComments = append(shortBotReviewComments, fmt.Sprintf(reviewResult.ShortString()))
	}
	approveLabels := unapprovePRComment
	if isApproved {
		approveLabels = approvePRComment
	}
	holdComment := holdPRComment
	if canMerge {
		holdComment = unholdPRComment
	}
	botReviewComment := fmt.Sprintf(
		botReviewCommentPattern,
		botUser.Login,
		"* "+strings.Join(botReviewComments, "\n* "),
		approveLabels,
		holdComment,
	)
	if len(botReviewComment) > 2<<15 {
		botReviewComment = fmt.Sprintf(
			botReviewCommentPattern,
			botUser.Login,
			"* "+strings.Join(shortBotReviewComments, "\n* "),
			approveLabels,
			holdComment,
		)
	}
	if !r.dryRun {
		err = githubClient.CreateComment(r.org, r.repo, r.num, botReviewComment)
		if err != nil {
			return fmt.Errorf("error while creating review comment: %v", err)
		}
	} else {
		r.l.Info(fmt.Sprintf("dry-run: %s/%s#%d <- %s", r.org, r.repo, r.num, botReviewComment))
	}
	return nil
}

type PRReviewOptions struct {
	PullRequestNumber int
	Org               string
	Repo              string
}

func PreparePullRequestReview(gitClient *git.Client, prReviewOptions PRReviewOptions, githubClient github.Client) (*github.PullRequest, string, error) {
	// checkout repo to a temporary directory to have it reviewed
	clone, err := gitClient.Clone(prReviewOptions.Org, prReviewOptions.Repo)
	if err != nil {
		logrus.WithError(err).Fatal("error cloning repo")
	}

	// checkout PR head commit, change dir
	pullRequest, err := githubClient.GetPullRequest(prReviewOptions.Org, prReviewOptions.Repo, prReviewOptions.PullRequestNumber)
	if err != nil {
		logrus.WithError(err).Fatal("error fetching PR")
	}
	err = clone.Checkout(pullRequest.Head.SHA)
	if err != nil {
		logrus.WithError(err).Fatal("error checking out PR head commit")
	}
	return pullRequest, clone.Directory(), err
}

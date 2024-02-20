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

package ghgraphql

import (
	"context"
	"fmt"
	"github.com/shurcooL/githubv4"
	"github.com/sirupsen/logrus"
	"regexp"
	"strings"
	"time"
)

type GitHubGraphQLClient interface {
	// FetchPRTimeLineForLastCommit returns specific events a PR has received
	// after the last commit or force push.
	FetchPRTimeLineForLastCommit(org string, repo string, prNumber int) (PRTimelineForLastCommit, error)
}

type gitHubGraphQLClient struct {
	gitHubClient *githubv4.Client
}

func NewClient(gitHubClient *githubv4.Client) GitHubGraphQLClient {
	return gitHubGraphQLClient{gitHubClient: gitHubClient}
}

// PRTimelineForLastCommit represents the specific events a PR has received
type PRTimelineForLastCommit struct {

	// NumberOfRetestComments is the number of `/(re)test` comments that triggered a testing on the PR
	NumberOfRetestComments int

	// WasHeld determines whether the PR did receive a `/hold` comment
	WasHeld bool

	// WasHoldCanceled determines whether the PR did receive an `/unhold` or `/hold cancel` comment
	WasHoldCanceled bool
}

var (
	cmdHoldRegex   = regexp.MustCompile(`(?mi)^/hold(\s.*)?$`)
	cmdUnholdRegex = regexp.MustCompile(`(?mi)^/(remove-hold|hold\s+cancel|unhold)\s*$`)
)

func (g gitHubGraphQLClient) FetchPRTimeLineForLastCommit(org string, repo string, prNumber int) (PRTimelineForLastCommit, error) {
	timelineItems, err := g.fetchTimelineItemsFromPR(org, repo, prNumber)
	if err != nil {
		return PRTimelineForLastCommit{}, err
	}
	return fetchPRTimeLineItemsFromGraphQuery(timelineItems), nil
}

func (g gitHubGraphQLClient) fetchTimelineItemsFromPR(org string, repo string, prNumber int) (TimelineItems, error) {
	var query struct {
		Repository struct {
			PullRequest struct {
				TimelineItems TimelineItems `graphql:"timelineItems(first:100, itemTypes:[PULL_REQUEST_COMMIT, BASE_REF_FORCE_PUSHED_EVENT, HEAD_REF_FORCE_PUSHED_EVENT, ISSUE_COMMENT])"`
			} `graphql:"pullRequest(number: $prNumber)"`
		} `graphql:"repository(owner: $org, name: $repo)"`
	}
	variables := map[string]interface{}{
		"prNumber": githubv4.Int(prNumber),
		"org":      githubv4.String(org),
		"repo":     githubv4.String(repo),
	}

	err := g.gitHubClient.Query(context.Background(), &query, variables)
	if err != nil {
		return TimelineItems{}, fmt.Errorf("failed to use github query %+v with variables %v: %w", query, variables, err)
	}
	return query.Repository.PullRequest.TimelineItems, nil
}

func fetchPRTimeLineItemsFromGraphQuery(timelineItems TimelineItems) PRTimelineForLastCommit {
	const phase2Intro = "Required labels detected, running phase 2 presubmits:"

	lastPush := determineLastPush(timelineItems)

	result := PRTimelineForLastCommit{}
	for _, timelineItem := range timelineItems.Nodes {
		if strings.Contains(timelineItem.BodyText, phase2Intro) {
			continue
		}
		if isRetestCommentAfterLastPush(timelineItem, lastPush) {
			result.NumberOfRetestComments += 1
		}
		if isHoldCommentAfterLastPush(timelineItem, lastPush) {
			result.WasHeld = true
		}
		if isUnholdCommentAfterLastPush(timelineItem, lastPush) {
			result.WasHoldCanceled = true
		}
	}
	return result
}

func determineLastPush(timelineItems TimelineItems) time.Time {
	lastPush := time.Time{}

	var itemDate time.Time
	for _, timelineItem := range timelineItems.Nodes {
		if isCommit(timelineItem) {
			itemDate = timelineItem.PullRequestCommitFragment.Commit.CommittedDate
			logrus.Infof("commit found: %+v", timelineItem.PullRequestCommitFragment)
		} else if isHeadRefForcePush(timelineItem) {
			itemDate = timelineItem.HeadRefForcePushFragment.CreatedAt
			logrus.Infof("head ref force push found: %+v", timelineItem.HeadRefForcePushFragment)
		} else if isBaseRefForcePush(timelineItem) {
			itemDate = timelineItem.BaseRefForcePushFragment.CreatedAt
			logrus.Infof("base ref force push found: %+v", timelineItem.BaseRefForcePushFragment)
		}
		if itemDate.After(lastPush) {
			logrus.Infof("updating last push: %+v", lastPush)
			lastPush = itemDate
		}
	}
	return lastPush
}

func isCommit(timelineItem TimelineItem) bool {
	return timelineItem.PullRequestCommitFragment != PullRequestCommitFragment{}
}

func isHeadRefForcePush(timelineItem TimelineItem) bool {
	return timelineItem.HeadRefForcePushFragment.Actor.Login != ""
}

func isBaseRefForcePush(timelineItem TimelineItem) bool {
	return timelineItem.BaseRefForcePushFragment.Actor.Login != ""

}

func isRetestCommentAfterLastPush(timelineItem TimelineItem, lastPush time.Time) bool {
	return isIssueCommentAfterLastPush(timelineItem, lastPush) &&
		(strings.HasPrefix(timelineItem.IssueCommentFragment.BodyText, "/retest") ||
			strings.HasPrefix(timelineItem.IssueCommentFragment.BodyText, "/test"))
}

func isHoldCommentAfterLastPush(timelineItem TimelineItem, lastPush time.Time) bool {
	return isIssueCommentAfterLastPush(timelineItem, lastPush) &&
		cmdHoldRegex.MatchString(timelineItem.IssueCommentFragment.BodyText)
}

func isUnholdCommentAfterLastPush(timelineItem TimelineItem, lastPush time.Time) bool {
	return isIssueCommentAfterLastPush(timelineItem, lastPush) &&
		cmdUnholdRegex.MatchString(timelineItem.IssueCommentFragment.BodyText)
}

func isIssueCommentAfterLastPush(timelineItem TimelineItem, lastPush time.Time) bool {
	return timelineItem.IssueCommentFragment != IssueCommentFragment{} &&
		timelineItem.IssueCommentFragment.CreatedAt.After(lastPush)
}

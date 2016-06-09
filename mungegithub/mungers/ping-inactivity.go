/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package mungers

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
	githubapi "github.com/google/go-github/github"
	"github.com/spf13/cobra"
	"k8s.io/contrib/mungegithub/features"
	"k8s.io/contrib/mungegithub/github"
)

const (
	maxReviewCommentInactivity = 2 * 7 * 24 * time.Hour
	maxPrCommentInactivity     = 2 * 7 * 24 * time.Hour
	highlight                  = "@" + botName
	ignoreTrigger              = "@" + botName + " ignore inactivity"
	postponeTrigger            = "@" + botName + " postpone by"

	reviewCommentInactivityMessage = `The comment above looks stale. Could you please take a look?

	You may update the code, reply with "` + ignoreTrigger + `" or with "` + postponeTrigger + ` X day(s)/week(s)/month(s)".`

	prInactivityMessage = `The PR looks stale. Could you please take a look?

	You may reply with "` + ignoreTrigger + `" or with "` + postponeTrigger + ` X day(s)/week(s)/month(s)".`
)

// InactivityPinger identifies inactive PRs and review comments and pings the appropriate person.
type InactivityPinger struct{}

func init() {
	s := InactivityPinger{}
	RegisterMungerOrDie(s)
}

// Name is the name usable in --pr-mungers
func (InactivityPinger) Name() string { return "inactivity-pinger" }

// RequiredFeatures is a slice of 'features' that must be provided
func (InactivityPinger) RequiredFeatures() []string { return []string{} }

// AddFlags will add any request flags to the cobra `cmd`
func (InactivityPinger) AddFlags(cmd *cobra.Command, config *github.Config) {}

// Initialize will initialize the munger
func (InactivityPinger) Initialize(*github.Config, *features.Features) error {
	return nil
}

// Munge is the workhorse the will actually make updates to the PR
func (InactivityPinger) Munge(obj *github.MungeObject) {
	if !obj.IsPR() {
		return
	}

	if !handleReviewComments(obj) {
		handlePR(obj)
	}
}

// EachLoop is called at the start of every munge loop
func (InactivityPinger) EachLoop() error {
	return nil
}

func handleReviewComments(obj *github.MungeObject) (notified bool) {
	pr, err := obj.GetPR()
	if err != nil {
		glog.Errorf("unexpected error getting PR %d: %s", *obj.Issue.Number, err)
	}

	// Collect review comments, sort them by thread and by date.
	reviewCommentsByThread, err := getSortedReviewCommentsByThread(obj, false)
	if err != nil {
		glog.Errorf("unexpected error getting sorted review comments by thread: %s", err)
		return
	}

	for thread := range reviewCommentsByThread {
		threadComments := reviewCommentsByThread[thread]

		// Verify if the thread is active or inactive.
		// Note that the last comment is used for that, and thus, it also considers the bot
		// may has written.
		lastComment := threadComments[len(threadComments)-1]
		if lastComment.CreatedAt.After(time.Now().Add(-maxReviewCommentInactivity)) {
			// Thread is active.
			continue
		}

		// Verify if the thread should be ignored (explicitely ignored or postponed).
		ignored := false
		for _, comment := range threadComments {
			ignored = strings.Contains(*comment.Body, ignoreTrigger) || isPostponed(*comment.Body, *comment.CreatedAt)
		}
		if ignored {
			glog.V(4).Infof("ignoring PR %d's comment thread %s", *pr.Number, thread)
			continue
		}

		// Send a notification.
		var message string
		if *lastComment.User.Login != *pr.User.Login {
			// The last commenter isn't the PR's owner, notify him/her.
			glog.V(4).Infof("notifying PR %d's owner: %s about comment thread %s due to inactivity", *pr.Number, *pr.User.Login, thread)
			message = "@" + *pr.User.Login
		} else {
			if pr.Assignee == nil {
				glog.V(4).Infof("desired to notify PR %d's assignee but there isn't", *pr.Number)
				return
			}

			// The last commenter is the PR's owner, notify the PR's assignee.
			glog.V(4).Infof("notifying PR %d's assignee: %s about comment thread %s due to inactivity", *pr.Number, *pr.Assignee.Login, thread)
			message = "@" + *pr.Assignee.Login
		}
		message = message + " " + reviewCommentInactivityMessage

		if err := obj.ReplyToReviewComment(*lastComment.ID, message); err != nil {
			glog.Errorf("unexpected error replying to review comment: %v", err)
		}

		notified = true
	}

	return
}

func getSortedReviewCommentsByThread(obj *github.MungeObject, keepOutdated bool) (map[string][]githubapi.PullRequestComment, error) {
	// Get all review comments.
	prComments, err := obj.ListReviewComments()
	if err != nil {
		return nil, err
	}

	// Assign each comment to a thread.
	prCommentsByThread := make(map[string][]githubapi.PullRequestComment)
	for _, prComment := range prComments {
		if !keepOutdated && prComment.Position == nil {
			// Ignore outdated diffs.
			continue
		}

		thread := *prComment.Path + ":" + (*prComment.DiffHunk)[3:strings.Index(*prComment.DiffHunk, " @@")]
		prCommentsByThread[thread] = append(prCommentsByThread[thread], prComment)
	}

	// Sort each thread.
	for _, threadComments := range prCommentsByThread {
		sort.Sort(prcByCreatedAt(threadComments))
	}

	return prCommentsByThread, nil
}

func handlePR(obj *github.MungeObject) {
	pr, err := obj.GetPR()
	if err != nil {
		glog.Errorf("unexpected error getting PR %d: %s", *obj.Issue.Number, err)
	}

	// Collect comments, sort them by date.
	prComments, err := getSortedComments(obj)
	if err != nil {
		glog.Errorf("unexpect error getting sorted comments: %s", err)
		return
	}

	// Verify if the PR is active or inactive.
	// Note that the last comment is used for that, and thus, it also considers the bot
	// may has written.
	lastComment := prComments[len(prComments)-1]
	if lastComment.CreatedAt.After(time.Now().Add(-maxPrCommentInactivity)) {
		// Thread is active.
		return
	}

	// Verify if the PR should be ignored (explicitely ignored or postponed).
	ignored := false
	for _, comment := range prComments {
		ignored = strings.Contains(*comment.Body, ignoreTrigger) || isPostponed(*comment.Body, *comment.CreatedAt)
	}
	if ignored {
		glog.V(4).Infof("ignoring PR %d", *pr.Number)
		return
	}

	// Send a notification.
	var message string
	if *lastComment.User.Login != *pr.User.Login {
		// The last commenter isn't the PR's owner, notify him/her.
		glog.V(4).Infof("notifying PR %d's owner: %s due to inactivity", *pr.Number, *pr.User.Login)
		message = "@" + *pr.User.Login
	} else {
		if pr.Assignee == nil {
			glog.V(4).Infof("desired to notify PR %d's assignee but there isn't", *pr.Number)
			return
		}

		// The last commenter is the PR's owner, notify the PR's assignee.
		glog.V(4).Infof("notifying PR %d's assignee: %s due to inactivity", *pr.Number, *pr.Assignee.Login)
		message = "@" + *pr.Assignee.Login
	}
	message = message + " " + prInactivityMessage

	if err := obj.WriteComment(message); err != nil {
		glog.Errorf("unexpected error adding comment: %v", err)
	}
}

func getSortedComments(obj *github.MungeObject) ([]githubapi.IssueComment, error) {
	// Get all review comments.
	prComments, err := obj.ListComments()
	if err != nil {
		return nil, err
	}

	// Sort by date.
	sort.Sort(icByCreatedAt(prComments))

	return prComments, nil
}

type prcByCreatedAt []githubapi.PullRequestComment

func (s prcByCreatedAt) Len() int           { return len(s) }
func (s prcByCreatedAt) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s prcByCreatedAt) Less(i, j int) bool { return s[i].CreatedAt.Before(*(s[j].CreatedAt)) }

type icByCreatedAt []githubapi.IssueComment

func (s icByCreatedAt) Len() int           { return len(s) }
func (s icByCreatedAt) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s icByCreatedAt) Less(i, j int) bool { return s[i].CreatedAt.Before(*(s[j].CreatedAt)) }

func isPostponed(comment string, at time.Time) bool {
	if !strings.Contains(comment, postponeTrigger) {
		return false
	}

	// Tokenize comment.
	comment = strings.TrimPrefix(comment, postponeTrigger+" ")
	commentTokens := strings.Split(comment, " ")
	if len(commentTokens) < 2 {
		glog.Errorf("could not parse postpone duration in comment: %s: invalid token count", comment)
		return false
	}

	// Extract multipler.
	x, err := strconv.Atoi(commentTokens[0])
	if err != nil {
		glog.Errorf("could not parse postpone duration in comment: %s: %s", comment, err)
		return false
	}

	// Roughly calculate the postpone duration, given the unit.
	commentTokens[1] = strings.TrimSuffix(commentTokens[1], ".")
	commentTokens[1] = strings.TrimSuffix(commentTokens[1], "s")
	var duration time.Duration
	switch commentTokens[1] {
	case "day":
		duration = time.Duration(x) * 24 * time.Hour
	case "week":
		duration = time.Duration(x) * 7 * 24 * time.Hour
	case "month":
		duration = time.Duration(x) * 30 * 24 * time.Hour
	default:
		glog.Errorf("could not parse postpone duration in comment: %s: unknown unit", comment)
		return false
	}

	return time.Now().Before(at.Add(duration))
}

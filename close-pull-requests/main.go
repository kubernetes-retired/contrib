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

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/golang/glog"
	githubapi "github.com/google/go-github/github"
	"github.com/spf13/cobra"
	"k8s.io/contrib/mungegithub/github"
)

const (
	day            = 24 * time.Hour
	keepOpenLabel  = "keep-open"
	closingComment = `This PR hasn't been active in %s. Feel free to reopen.

You can add 'keep-open' label to prevent this from happening again.`
	warningComment = `This PR hasn't been active in %s. Will be closed in %s.

You can add 'keep-open' label to prevent this from happening.`
)

var (
	closingCommentRE = regexp.MustCompile(`This PR hasn't been active in \d+ days?\. Feel free to reopen.

You can add 'keep-open' label to prevent this from happening again\.`)
	warningCommentRE = regexp.MustCompile(`This PR hasn't been active in \d+ days?\. Will be closed in \d+ days?\.

You can add 'keep-open' label to prevent this from happening\.`)
)

type closerConfig struct {
	github.Config

	warnBefore   time.Duration
	warnReminder time.Duration
	closeAfter   time.Duration
	dryRun       bool
}

func addRootFlags(cmd *cobra.Command, config *closerConfig) {
	cmd.Flags().DurationVar(&config.warnBefore, "warn-before", 60*day, "Number of hours before we start warning")
	cmd.Flags().DurationVar(&config.warnReminder, "warn-reminder", 30*day, "Number of hours before we warn again")
	cmd.Flags().DurationVar(&config.closeAfter, "close-after", 90*day, "Number of hours before we actually close pull-request")
}

func isBotName(name string) bool {
	return name == "k8s-merge-robot" || name == "k8s-bot"
}

func validIssueComment(comment githubapi.IssueComment) bool {
	if comment.User == nil || comment.User.Login == nil {
		return false
	}
	if comment.CreatedAt == nil {
		return false
	}
	if comment.Body == nil {
		return false
	}
	return true
}

func validPRComment(comment githubapi.PullRequestComment) bool {
	if comment.User == nil || comment.User.Login == nil {
		return false
	}
	if comment.CreatedAt == nil {
		return false
	}
	if comment.Body == nil {
		return false
	}
	return true
}

func findLastHumanIssueUpdate(obj *github.MungeObject) (*time.Time, error) {
	lastHuman := obj.Issue.CreatedAt

	comments, err := obj.ListComments()
	if err != nil {
		return nil, err
	}

	for i := range comments {
		comment := comments[i]
		if !validIssueComment(comment) {
			continue
		}
		if isBotName(*comment.User.Login) {
			continue
		}
		if lastHuman.Before(*comment.UpdatedAt) {
			lastHuman = comment.UpdatedAt
		}
	}

	return lastHuman, nil
}

func findLastInterestingEventUpdate(obj *github.MungeObject) (*time.Time, error) {
	lastInteresting := obj.Issue.CreatedAt

	events, err := obj.GetEvents()
	if err != nil {
		return nil, err
	}

	for i := range events {
		event := events[i]
		if event.Event == nil || *event.Event != "reopened" {
			continue
		}

		if lastInteresting.Before(*event.CreatedAt) {
			lastInteresting = event.CreatedAt
		}
	}

	return lastInteresting, nil
}

func findLastModificationTime(obj *github.MungeObject) (*time.Time, error) {
	lastHumanIssue, err := findLastHumanIssueUpdate(obj)
	if err != nil {
		return nil, err
	}
	lastInterestingEvent, err := findLastInterestingEventUpdate(obj)
	if err != nil {
		return nil, err
	}

	lastModif := lastHumanIssue
	if lastInterestingEvent.After(*lastModif) {
		lastModif = lastInterestingEvent
	}

	return lastModif, nil
}

func findLatestWarningComment(obj *github.MungeObject) *githubapi.IssueComment {
	var lastFoundComment *githubapi.IssueComment

	comments, err := obj.ListComments()
	if err != nil {
		return nil
	}

	for i := range comments {
		comment := comments[i]
		if !validIssueComment(comment) {
			continue
		}
		if !isBotName(*comment.User.Login) {
			continue
		}

		if !warningCommentRE.MatchString(*comment.Body) {
			continue
		}

		if lastFoundComment == nil || lastFoundComment.CreatedAt.Before(*comment.UpdatedAt) {
			if lastFoundComment != nil {
				obj.DeleteComment(lastFoundComment)
			}
			lastFoundComment = &comment
		}
	}

	return lastFoundComment
}

func durationToDays(duration time.Duration) string {
	days := duration / day
	dayString := "days"
	if days == 1 || days == -1 {
		dayString = "day"
	}
	return fmt.Sprintf("%d %s", days, dayString)
}

func closePullRequest(obj *github.MungeObject, inactiveFor time.Duration) {
	comment := findLatestWarningComment(obj)
	if comment != nil {
		obj.DeleteComment(comment)
	}

	obj.WriteComment(fmt.Sprintf(closingComment, durationToDays(inactiveFor)))
	obj.ClosePR()
}

func postWarningComment(obj *github.MungeObject, inactiveFor time.Duration, closeIn time.Duration) {
	obj.WriteComment(fmt.Sprintf(
		warningComment,
		durationToDays(inactiveFor),
		durationToDays(closeIn)))
}

func checkAndWarn(obj *github.MungeObject, reminder time.Duration, inactiveFor time.Duration, closeIn time.Duration) {
	if closeIn < day {
		// We are going to close the PR in less than a day. Too late to warn
		return
	}
	comment := findLatestWarningComment(obj)
	if comment == nil {
		// We don't already have the comment. Post it
		postWarningComment(obj, inactiveFor, closeIn)
	} else if time.Since(*comment.UpdatedAt) > reminder {
		// It's time to warn again
		obj.DeleteComment(comment)
		postWarningComment(obj, inactiveFor, closeIn)
	} else {
		// We already have a warning, and it's not expired. Do nothing
	}
}

func processPullRequest(config *closerConfig, obj *github.MungeObject) error {
	if !obj.IsPR() {
		return nil
	}

	if obj.HasLabel(keepOpenLabel) {
		return nil
	}

	lastModif, err := findLastModificationTime(obj)
	if err != nil {
		return err
	}

	closeIn := -time.Since(lastModif.Add(config.closeAfter))
	inactiveFor := time.Since(*lastModif)
	if closeIn <= 0 {
		closePullRequest(obj, inactiveFor)
	} else if closeIn <= config.warnBefore {
		checkAndWarn(obj, config.warnReminder, inactiveFor, closeIn)
	} else {
		// Pull-request is active. Do nothing
	}
	return nil
}

func runProgram(config *closerConfig) error {
	if err := config.PreExecute(); err != nil {
		return err
	}

	return config.ForEachIssueDo(func(obj *github.MungeObject) error {
		return processPullRequest(config, obj)
	})
}

func main() {
	config := &closerConfig{}
	cmd := &cobra.Command{
		Use:   filepath.Base(os.Args[0]),
		Short: "Close inactive github pull-request",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runProgram(config)
		},
	}

	addRootFlags(cmd, config)
	config.AddRootFlags(cmd)

	if err := cmd.Execute(); err != nil {
		glog.Fatalf("%v\n", err)
	}
}

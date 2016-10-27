/*
Copyright 2016 The Kubernetes Authors.

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
	"strings"

	"k8s.io/contrib/mungegithub/features"
	"k8s.io/contrib/mungegithub/github"
	e "k8s.io/contrib/mungegithub/mungers/matchers/event"
	"k8s.io/contrib/mungegithub/mungers/mungerutil"

	"github.com/golang/glog"
	githubapi "github.com/google/go-github/github"
	"github.com/spf13/cobra"
	"k8s.io/contrib/mungegithub/mungers/matchers/comment"
)

// DibsNackHandler will
// - apply LGTM label if reviewer has commented "/lgtm", or
// - remove LGTM label if reviewer has commented "/lgtm cancel"
type DibsNackHandler struct {
	features *features.Features
}

func init() {
	l := DibsNackHandler{}
	RegisterMungerOrDie(l)
}

// Name is the name usable in --pr-mungers
func (DibsNackHandler) Name() string { return "dibs-nack-handler" }

// RequiredFeatures is a slice of 'features' that must be provided
func (DibsNackHandler) RequiredFeatures() []string { return []string{} }

// Initialize will initialize the munger
func (DibsNackHandler) Initialize(config *github.Config, features *features.Features) error { return nil }

// EachLoop is called at the start of every munge loop
func (DibsNackHandler) EachLoop() error { return nil }

// AddFlags will add any request flags to the cobra `cmd`
func (DibsNackHandler) AddFlags(cmd *cobra.Command, config *github.Config) {}

// Munge is the workhorse the will actually make updates to the PR
func (h DibsNackHandler) Munge(obj *github.MungeObject) {
	if !obj.IsPR() {
		return
	}
	reviewers := getReviewers(obj)
	if len(reviewers) == 0 {
		return
	}

	comments, err := obj.ListComments(obj)
	if err != nil {
		glog.Errorf("unexpected error getting comments: %v", err)
		return
	}

	events, err := obj.GetEvents()
	if err != nil {
		glog.Errorf("unexpected error getting events: %v", err)
		return
	}

	h.assignIfDibs(obj, comments, events, reviewers)
	h.removeIfNack(obj, comments, events, reviewers)
}

func (h *DibsNackHandler) assignIfDibs(obj *github.MungeObject, comments []*githubapi.IssueComment, events []*githubapi.IssueEvent, reviewers mungerutil.UserSet) {
	// Get the last time when the someone applied lgtm manually.
	removeLGTMTime := e.LastEvent(events, e.And{e.RemoveLabel{}, e.LabelName(lgtmLabel), e.HumanActor()}, nil)

	// Assumption: The comments should be sorted (by default from github api) from oldest to latest
	// If they, dibs then nack. Remove them.

	for i := len(comments) - 1; i >= 0; i-- {
		comment := comments[i]
		if !mungerutil.IsValidUser(comment.User) {
			continue
		}

		fields := getFields(*comment.Body)
		potential_owners, _ := getPotentialOwners(obj, h.features, obj.ListFiles())
		if isDibsComment(fields) {
			//check if they are a valid reviewer if so, assign the user. if not, explain why
			if _, ok := potential_owners[comment.User.String()]; ok {
				glog.Infof("Assigning %v to review PR#%v", *comment.User.Login, obj.Issue.Number)
				obj.AssignPR(comment.User.String())
				return
			} else {
				// add comment explaining to them they can't be reviewer
				glog.Infof("%v requested to be added as a reviewer but is not a potential reviewer")
			}
		}

		if !isNackComment(fields) {
			//check if they are already an assigned reviewer. if so, remove them.  if not, do nothing.
			glog.Infof("Removing %v as an reviewer for PR#%v", *comment.User.Login, obj.Issue.Number)
			for assignee := range obj.Issue.Assignees {
				if comment.User == assignee {
					//remove the assignee
				}
			}
			continue
		}

		// check if someone manually removed the lgtm label after the `/lgtm` comment
		// and honor it.
		if removeLGTMTime != nil && removeLGTMTime.After(*comment.CreatedAt) {
			return
		}

		// TODO: support more complex policies for multiple reviewers.
		// See https://github.com/kubernetes/contrib/issues/1389#issuecomment-235161164
		obj.AddLabel(lgtmLabel)
		return
	}
}


func (h *DibsNackHandler) removeIfNack(obj *github.MungeObject, comments []*githubapi.IssueComment, events []*githubapi.IssueEvent, reviewers mungerutil.UserSet) {
	// Get the last time when the someone applied lgtm manually.
	removeLGTMTime := e.LastEvent(events, e.And{e.RemoveLabel{}, e.LabelName(lgtmLabel), e.HumanActor()}, nil)

	// Assumption: The comments should be sorted (by default from github api) from oldest to latest
	for i := len(comments) - 1; i >= 0; i-- {
		comment := comments[i]
		if !mungerutil.IsValidUser(comment.User) {
			continue
		}

		// TODO: An approver should be acceptable.
		// See https://github.com/kubernetes/contrib/pull/1428#discussion_r72563935
		if !mungerutil.IsMungeBot(comment.User) && !isReviewer(comment.User, reviewers) {
			continue
		}

		fields := getFields(*comment.Body)
		if isCancelComment(fields) {
			// "/lgtm cancel" if commented more recently than "/lgtm"
			return
		}

		if !isLGTMComment(fields) {
			continue
		}

		// check if someone manually removed the lgtm label after the `/lgtm` comment
		// and honor it.
		if removeLGTMTime != nil && removeLGTMTime.After(*comment.CreatedAt) {
			return
		}

		// TODO: support more complex policies for multiple reviewers.
		// See https://github.com/kubernetes/contrib/issues/1389#issuecomment-235161164
		glog.Infof("Adding lgtm label. Reviewer (%s) LGTM", *comment.User.Login)
		obj.AddLabel(lgtmLabel)
		return
	}
}
func isDibsComment(fields []string) bool {
	// Note: later we'd probably move all the bot-command parsing code to its own package.
	return len(fields) == 1 && strings.ToLower(fields[0]) == "/dibs"
}


func isNackComment(fields []string) bool {
	// Note: later we'd probably move all the bot-command parsing code to its own package.
	return len(fields) == 1 && strings.ToLower(fields[0]) == "/nack"
}


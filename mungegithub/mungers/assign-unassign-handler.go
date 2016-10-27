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
	"k8s.io/contrib/mungegithub/mungers/mungerutil"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/golang/glog"
	goGithub "github.com/google/go-github/github"
	"github.com/spf13/cobra"
	commentMatcher "k8s.io/contrib/mungegithub/mungers/matchers/comment"
)

const (
	repoOwner       = "kubernetes"
	assignCommand   = "/assign"
	unassignCommand = "/unassign"
	invalidReviewer = "ASSIGN_NOTIFIER"
)

// AssignUnassignHandler will
// - will assign a github user to a PR if they comment "/assign"
// - will unassign a github user to a PR if they comment "/unassign"
type AssignUnassignHandler struct {
	features *features.Features
}

func init() {
	dh := AssignUnassignHandler{}
	RegisterMungerOrDie(dh)
}

// Name is the name usable in --pr-mungers
func (AssignUnassignHandler) Name() string { return "assign-unassign-handler" }

// RequiredFeatures is a slice of 'features' that must be provided
func (AssignUnassignHandler) RequiredFeatures() []string { return []string{} }

// Initialize will initialize the munger
func (AssignUnassignHandler) Initialize(config *github.Config, features *features.Features) error {
	return nil
}

// EachLoop is called at the start of every munge loop
func (AssignUnassignHandler) EachLoop() error { return nil }

// AddFlags will add any request flags to the cobra `cmd`
func (h AssignUnassignHandler) AddFlags(cmd *cobra.Command, config *github.Config) {
}

// Munge is the workhorse the will actually make updates to the PR
func (h AssignUnassignHandler) Munge(obj *github.MungeObject) {
	if !obj.IsPR() {
		return
	}

	comments, err := obj.ListComments()
	if err != nil {
		glog.Errorf("unexpected error getting comments: %v", err)
		return
	}

	fileList, err := obj.ListFiles()
	if err != nil {
		glog.Errorf("Could not list the files for PR %v: %v", obj.Issue.Number, err)
		return
	}

	//get all the people that could potentially own the file based on the blunderbuss.go implementation
	potentialOwners, _ := getPotentialOwners(obj, h.features, fileList)

	toAssign, toUnassign := h.assignOrRemove(obj, comments, fileList, potentialOwners)

	//assign and unassign reviewers as necessary
	for _, username := range toAssign.List() {
		obj.AssignPR(username)
	}
	obj.Unassign(toUnassign.List()...)
}

//assignOrRemove checks to see when someone comments "/assign" or "/unassign"
// returns two sets.String
// 1. github handles to be assigned
// 2. github handles to be unassigned
// [TODO] "/assign <github handle>" assigns to this person
func (h *AssignUnassignHandler) assignOrRemove(obj *github.MungeObject, comments []*goGithub.IssueComment, fileList []*goGithub.CommitFile, potentialOwners weightMap) (toAssign, toUnassign sets.String) {
	toAssign = sets.String{}
	toUnassign = sets.String{}

	assignComments := commentMatcher.FilterComments(comments, commentMatcher.CommandName(assignCommand))
	unassignComments := commentMatcher.FilterComments(comments, commentMatcher.CommandName(unassignCommand))
	invalidUsers := sets.String{}

	//collect all the people that should be assigned
	for _, cmt := range assignComments {
		if !mungerutil.IsValidUser(cmt.User) {
			continue
		}
		if isValidReviewer(potentialOwners, cmt.User) {
			glog.Infof("Assigning %v to review PR#%v", *cmt.User.Login, obj.Issue.Number)
			toAssign.Insert(*cmt.User.Login)
		} else {
			// build the set of people who asked to be assigned but aren't in reviewers
			// use the @ as a prefix so github notifies them in advance
			invalidUsers.Insert("@" + *cmt.User.Login)
		}

	}

	//collect all the people that should be unassigned
	for _, cmt := range unassignComments {
		if !mungerutil.IsValidUser(cmt.User) {
			continue
		}
		if isAssignee(obj.Issue.Assignees, cmt.User) {
			glog.Infof("Removing %v as an reviewer for PR#%v", *cmt.User.Login, obj.Issue.Number)
			toUnassign.Insert(*cmt.User.Login)
		}
	}

	//if some people tried to get assigned that are not valid reviewers, notify them (if they haven't already been notified)
	if invalidUsers.Len() != 0 {
		previousNotifications := commentMatcher.FilterComments(comments, commentMatcher.MungerNotificationName(invalidReviewer))
		if previousNotifications != nil && previousNotifications.GetLast().CreatedAt.After(*assignComments.GetLast().CreatedAt) {
			// no need to create a new comment
			return toAssign, toUnassign
		}
		if previousNotifications != nil {
			for _, c := range previousNotifications {
				obj.DeleteComment(c)
			}
		}
		context := "The following people cannot be assigned because they are not in the OWNERS files\n" + strings.Join(invalidUsers.List(), "\n")
		//post the notification
		commentMatcher.Notification{Name: invalidReviewer, Arguments: "", Context: context}.Post(obj)

	}
	return toAssign, toUnassign
}

func isValidReviewer(potentialOwners weightMap, commenter *goGithub.User) bool {
	if commenter == nil || commenter.Login == nil {
		return false
	}
	if _, ok := potentialOwners[*commenter.Login]; ok {
		return true
	}
	return false
}

func isAssignee(assignees []*goGithub.User, someUser *goGithub.User) bool {
	for _, assignee := range assignees {
		//remove the assignee
		if assignee.Login == nil || someUser.Login == nil {
			continue
		}
		if *assignee.Login == *someUser.Login && someUser.ID == assignee.ID {
			return true
		}
	}
	return false
}

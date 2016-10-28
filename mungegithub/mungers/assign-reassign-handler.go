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

	"github.com/golang/glog"
	goGithub "github.com/google/go-github/github"
	"github.com/spf13/cobra"
	"k8s.io/contrib/mungegithub/mungers/matchers/comment"
	"fmt"
)

const (
	OWNER = "kubernetes"
	assign_keyword = "assign"
	reassign_keyword = "reassign"

	notReviewerInTree = "%v commented /assign on a PR but it looks you are not list in the OWNERs file as a reviewer for the files in this PR"
)

// AssignReassignHandler will
// - will assign a github user to a PR if they comment "/assign"
// - will unassign a github user to a PR if they comment "/reassign"
type AssignReassignHandler struct {
	features *features.Features
}
func init() {
	dh := AssignReassignHandler{}
	RegisterMungerOrDie(dh)
}

// Name is the name usable in --pr-mungers
func (AssignReassignHandler) Name() string { return "assign-reassign-handler" }

// RequiredFeatures is a slice of 'features' that must be provided
func (AssignReassignHandler) RequiredFeatures() []string { return []string{} }

// Initialize will initialize the munger
func (AssignReassignHandler) Initialize(config *github.Config, features *features.Features) error { return nil }

// EachLoop is called at the start of every munge loop
func (AssignReassignHandler) EachLoop() error { return nil }

// AddFlags will add any request flags to the cobra `cmd`
func (AssignReassignHandler) AddFlags(cmd *cobra.Command, config *github.Config) {}

// Munge is the workhorse the will actually make updates to the PR
func (h AssignReassignHandler) Munge(obj *github.MungeObject) {
	if !obj.IsPR() {
		return
	}
	reviewers := getReviewers(obj)
	if len(reviewers) == 0 {
		return
	}

	comments, err := obj.ListComments()
	if err != nil {
		glog.Errorf("unexpected error getting comments: %v", err)
		return
	}

	h.assignOrRemove(obj, comments, reviewers)
}

//assignOrRemove checks to see when someone comments "/assign" or "/reassign"
// "/assign" self assigns the PR
// "/reassign" unassignes the commenter and reassigns to someone else
// [TODO] "/reassign <github handle>" reassign to this person
func (h *AssignReassignHandler) assignOrRemove(obj *github.MungeObject, comments []*goGithub.IssueComment, reviewers mungerutil.UserSet) {

	fileList, err := obj.ListFiles()
	if err != nil {
		glog.Error("Could not list the files for PR %v", obj.Issue.Number)
		return
	}
	//get all the people that could potentially own the file based on the blunderbuss.go implementation
	potential_owners, _ := getPotentialOwners(obj, h.features, fileList)
	for i := len(comments) - 1; i >= 0; i-- {
		comment := comments[i]
		if !mungerutil.IsValidUser(comment.User) {
			continue
		}

		fields := getFields(*comment.Body)
		if isDibsComment(fields) {
			//check if they are a valid reviewer if so, assign the user. if not, explain why
			if isValidReviewer(potential_owners, comment.User){
				glog.Infof("Assigning %v to review PR#%v", *comment.User.Login, obj.Issue.Number)
				obj.AssignPR(comment.User.String())
				return
			} else {
				//inform user that they are not a valid reviewer
				obj.WriteComment(fmt.Sprintf(notReviewerInTree, comment.User.String()))
			}
		}

		if isReassignComment(fields) && isAssignee(obj.Issue.Assignees, comment.User) {
			//check if they are already an assigned reviewer. if so, remove them.  if not, do nothing.
			glog.Infof("Removing %v as an reviewer for PR#%v", *comment.User.Login, obj.Issue.Number)
			is := goGithub.IssuesService{}
			is.RemoveAssignees(OWNER, *obj.Issue.Repository.Name, *obj.Issue.Number, []string{*comment.User.Name})
		}


	}
}

func isValidReviewer(potential_owners weightMap, commenter *goGithub.User) bool{
	if _, ok := potential_owners[commenter.String()]; ok {
		return true
	}
	return false
}

func isAssignee(assignees []*goGithub.User, someUser *goGithub.User) bool{
	for _, assignee := range assignees {
		//remove the assignee
		if someUser.ID == assignee.ID {
			return true
		}
	}
	return false
}

func isDibsComment(fields []string) bool {
	// Note: later we'd probably move all the bot-command parsing code to its own package.
	return len(fields) == 1 && strings.ToLower(fields[0]) == "/" + assign_keyword
}


func isReassignComment(fields []string) bool {
	// Note: later we'd probably move all the bot-command parsing code to its own package.
	return len(fields) == 1 && strings.ToLower(fields[0]) == "/" + reassign_keyword
}


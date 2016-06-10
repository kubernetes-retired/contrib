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
	"fmt"
	"regexp"

	"k8s.io/contrib/mungegithub/features"
	"k8s.io/contrib/mungegithub/github"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
)

// AssignFixesMunger will assign issues to users based on the config file
// provided by --assignfixes-config.
type AssignFixesMunger struct {
	config        *github.Config
	features      *features.Features
	collaborators sets.String
}

const (
	hasPRPosted = "has-pr-posted"
)

var (
	claimMatcher = regexp.MustCompile(`(?:Assigned to|Claimed by) @(\w+)`)
)

func init() {
	assignfixes := &AssignFixesMunger{}
	RegisterMungerOrDie(assignfixes)
}

// Name is the name usable in --pr-mungers
func (a *AssignFixesMunger) Name() string { return "assign-fixes" }

// RequiredFeatures is a slice of 'features' that must be provided
func (a *AssignFixesMunger) RequiredFeatures() []string { return []string{} }

// Initialize will initialize the munger
func (a *AssignFixesMunger) Initialize(config *github.Config, features *features.Features) error {
	a.features = features
	a.config = config
	a.collaborators = sets.String{}
	users, err := config.FetchAllCollaborators()
	if err != nil {
		return err
	}
	for _, user := range users {
		a.collaborators.Insert(github.DescribeUser(&user))
	}

	return nil
}

// EachLoop is called at the start of every munge loop
func (a *AssignFixesMunger) EachLoop() error { return nil }

// AddFlags will add any request flags to the cobra `cmd`
func (a *AssignFixesMunger) AddFlags(cmd *cobra.Command, config *github.Config) {
}

func issueHasMarkFrom(issueObj *github.MungeObject, isCollaborator bool, prOwner string) (bool, error) {
	comments, err := issueObj.ListComments()
	if err != nil {
		glog.Errorf("unexpected error getting comments: %v", err)
		return true, nil
	}

	// This allows for the case of someone adding a PR to an old issue that previously
	// had a different claimee/assignee.
	lastClaimedBy := ""
	for ix := range comments {
		comment := &comments[ix]
		matches := claimMatcher.FindAllStringSubmatch(comment.String(), -1)
		if matches == nil {
			continue
		}
		for _, match := range matches {
			if match[1] != lastClaimedBy {
				lastClaimedBy = match[1]
			}
		}
	}
	return (lastClaimedBy == prOwner), nil
}

func markIssueWith(issueObj *github.MungeObject, isCollaborator bool, prOwner string) {
	hasMark, err := issueHasMarkFrom(issueObj, isCollaborator, prOwner)
	if err != nil {
		return
	}
	if !hasMark {
		var text string
		if isCollaborator {
			text = fmt.Sprintf("Assigned to @%v (by mungegithub:assign-fixes)\n", prOwner)
		} else {
			text = fmt.Sprintf("Claimed by @%v (by mungegithub:assign-fixes)\n", prOwner)
		}
		err := issueObj.WriteComment(text)
		if err != nil {
			glog.Errorf("unexpected error adding comment: %v", err)
		}
	}
}

func (a *AssignFixesMunger) markIssue(issueObj *github.MungeObject, prOwner string) {
	if !issueObj.HasLabel(hasPRPosted) {
		issueObj.AddLabel(hasPRPosted)
	}

	if !a.collaborators.Has(prOwner) {
		markIssueWith(issueObj, false, prOwner)
		return
	}

	if github.DescribeUser(issueObj.Issue.Assignee) == prOwner {
		return
	}

	markIssueWith(issueObj, true, prOwner)
	issueObj.AssignPR(prOwner)
}

// Munge is the workhorse the will actually make updates to the PR
func (a *AssignFixesMunger) Munge(obj *github.MungeObject) {
	if !obj.IsPR() {
		return
	}
	// we need the PR for the "User" (creator of the PR not the assignee)
	pr, err := obj.GetPR()
	if err != nil {
		glog.Infof("Couldn't get PR %v", obj.Issue.Number)
		return
	}
	prOwner := github.DescribeUser(pr.User)

	issuesFixed := obj.GetPRFixesList()
	if issuesFixed == nil {
		return
	}
	for _, fixesNum := range issuesFixed {
		// "issue" is the issue referenced by the "fixes #<num>"
		issueObj, err := a.config.GetObject(fixesNum)
		if err != nil {
			glog.Infof("Couldn't get issue %v", fixesNum)
			continue
		}
		a.markIssue(issueObj, prOwner)
	}
}

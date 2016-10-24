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
	"github.com/spf13/cobra"
)

// ApprovalHandler will try to add "approved" label once
// all files of change has been approved by approvers.
type ApprovalHandler struct {
	features *features.Features
}

func init() {
	h := &ApprovalHandler{}
	RegisterMungerOrDie(h)
}

// Name is the name usable in --pr-mungers
func (*ApprovalHandler) Name() string { return "approval-handler" }

// RequiredFeatures is a slice of 'features' that must be provided
func (*ApprovalHandler) RequiredFeatures() []string {
	return []string{features.RepoFeatureName, features.AliasesFeature}
}

// Initialize will initialize the munger
func (h *ApprovalHandler) Initialize(config *github.Config, features *features.Features) error {
	h.features = features
	return nil
}

// EachLoop is called at the start of every munge loop
func (*ApprovalHandler) EachLoop() error { return nil }

// AddFlags will add any request flags to the cobra `cmd`
func (*ApprovalHandler) AddFlags(cmd *cobra.Command, config *github.Config) {}

// Munge is the workhorse the will actually make updates to the PR
// The algorithm goes as:
// - Initially, we build an approverSet as follows
//   - Go through all comments after latest commit. If anyone said "/approve", add him to approverSet.
// - For each file, we see if any approver of this file is in approverSet.
//   - An approver of a file is defined as:
//     - It's known that each dir has a list of approvers. (This might not hold true. For usability, current situation is enough.)
//     - Approver of a dir is also the approver of child dirs.
// - Iff all files have been approved, the bot will add the "approved" label.
// - Iff a valid approver comments /approve cancel, we remove the approve label and the process must be completed
// again
func (h *ApprovalHandler) Munge(obj *github.MungeObject) {
	if !obj.IsPR() {
		return
	}
	files, err := obj.ListFiles()
	if err != nil {
		glog.Errorf("failed to list files in this PR: %v", err)
		return
	}

	comments, err := getCommentsAfterLastModified(obj)
	if err != nil {
		glog.Errorf("failed to get comments in this PR: %v", err)
		return
	}

	approverSet := sets.String{}

	cancelSet := sets.String{}
	// from oldest to latest
	for i := len(comments) - 1; i >= 0; i-- {
		c := comments[i]

		if !mungerutil.IsValidUser(c.User) {
			continue
		}

		fields := strings.Fields(strings.TrimSpace(*c.Body))

		if len(fields) == 1 && strings.ToLower(fields[0]) == "/approve" {
			approverSet.Insert(*c.User.Login)
			continue
		}

		if len(fields) == 2 && strings.ToLower(fields[0]) == "/approve" && strings.ToLower(fields[1]) == "cancel" {
			approverSet.Delete(*c.User.Login)
			cancelSet.Insert(*c.User.Login)
		}
	}
	glog.Infof("This is the cancel set %v", cancelSet)

	//Checks that no valid approver has commented to cancel the approve label since last commit
	//Checks that all files have an approver in the approverSet
	for _, file := range files {
		fileOwners := h.features.Repos.Assignees(*file.Filename)
		if fileOwners.Intersection(cancelSet).Len() > 0 {
			// a valid approver applied a cancel command since the last commit
			glog.Infof("Canceling the approve label because %v canceled", fileOwners.Intersection(cancelSet).List()[0])
			if obj.HasLabel(approvedLabel) {
				obj.RemoveLabel(approvedLabel)
			}
			return
		} else if fileOwners.Intersection(approverSet).Len() == 0 {
			glog.Infof("File %v does not have approval, and thus PR %v cannot be approved", *file.Filename, obj.Number())
			return
		}
	}

	//every file has been approved by a valid approver
	obj.AddLabel(approvedLabel)
}

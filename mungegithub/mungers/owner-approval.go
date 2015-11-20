/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
)

const (
	ownerApproval = "has-owner-approval"
)

// OwnerApproval it a github munger which labels PRs based on 'owner' approval.
// Owners are listed in files (called OWNERS) in the git tree. An owner owns
// everything beneath the directory. So a person who 'owns' the root directory
// can approve everything
type OwnerApproval struct {
	features *features.Features
}

func init() {
	RegisterMungerOrDie(&OwnerApproval{})
}

// Name is the name usable in --pr-mungers
func (o *OwnerApproval) Name() string { return "owner-approval" }

// RequiredFeatures specifies that we need the features which loads assignees and owners
func (o *OwnerApproval) RequiredFeatures() []string { return []string{features.RepoFeatureName} }

// Initialize does just that
func (o *OwnerApproval) Initialize(config *github.Config, features *features.Features) error {
	o.features = features
	return nil
}

// EachLoop is called at the start of every munge loop
func (o *OwnerApproval) EachLoop() error { return nil }

// AddFlags will add any request flags to the cobra `cmd`
func (o *OwnerApproval) AddFlags(cmd *cobra.Command, config *github.Config) {}

// Munge is the workhorse the will actually make updates to the PR
func (o *OwnerApproval) Munge(obj *github.MungeObject) {
	if !obj.IsPR() {
		return
	}
	commits, err := obj.GetCommits()
	if err != nil {
		return
	}

	neededApprovals := map[string]sets.String{}
	for _, c := range commits {
		for _, f := range c.Files {
			file := *f.Filename
			neededApprovals[file] = o.features.Repos.Owners(file)
		}
	}

	lastModified := obj.LastModifiedTime()
	if lastModified == nil {
		return
	}

	comments, err := obj.ListComments()
	if err != nil {
		return
	}

	approvalsGiven := sets.String{}
	for _, comment := range comments {
		if lastModified.After(*comment.UpdatedAt) {
			continue
		}
		lines := strings.Split(*comment.Body, "\n")
		for _, line := range lines {
			line = strings.TrimPrefix(line, "@k8s-merge-bot")
			line = strings.TrimSpace(line)
			line = strings.ToLower(line)
			switch line {
			case "i approve":
				approvalsGiven.Insert(*comment.User.Login)
			case "approved":
				approvalsGiven.Insert(*comment.User.Login)
			}
		}
	}

	missingApprovals := sets.NewString()
	for _, needed := range neededApprovals {
		intersection := needed.Intersection(approvalsGiven)
		if intersection.Len() != 0 {
			// Someone who approved covered this area
			continue
		}
		missingApprovals = missingApprovals.Union(needed)
	}

	if missingApprovals.Len() == 0 && !obj.HasLabel(ownerApproval) {
		obj.AddLabel(ownerApproval)
	} else if missingApprovals.Len() != 0 && obj.HasLabel(ownerApproval) {
		obj.RemoveLabel(ownerApproval)
	}
}

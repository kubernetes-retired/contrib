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
	"time"

	"k8s.io/contrib/mungegithub/features"
	"k8s.io/contrib/mungegithub/github"
	"k8s.io/contrib/mungegithub/mungers/mungerutil"

	"github.com/golang/glog"
	githubapi "github.com/google/go-github/github"
	"github.com/spf13/cobra"
)

const (
	lgtmRemovedBody = "PR changed after LGTM, removing LGTM."
)

// LGTMHandler will
// - apply the LGTM label if reviewer has said so, or
// - remove the LGTM label from an PR which has been updated since the reviewer added LGTM
type LGTMHandler struct{}

func init() {
	l := LGTMHandler{}
	RegisterMungerOrDie(l)
	RegisterStaleComments(l)
}

// Name is the name usable in --pr-mungers
func (LGTMHandler) Name() string { return "lgtm-after-commit" }

// RequiredFeatures is a slice of 'features' that must be provided
func (LGTMHandler) RequiredFeatures() []string { return []string{} }

// Initialize will initialize the munger
func (LGTMHandler) Initialize(config *github.Config, features *features.Features) error {
	return nil
}

// EachLoop is called at the start of every munge loop
func (LGTMHandler) EachLoop() error { return nil }

// AddFlags will add any request flags to the cobra `cmd`
func (LGTMHandler) AddFlags(cmd *cobra.Command, config *github.Config) {}

// Munge is the workhorse the will actually make updates to the PR
func (h LGTMHandler) Munge(obj *github.MungeObject) {
	if !obj.IsPR() {
		return
	}

	lastModified := obj.LastModifiedTime()
	if !obj.HasLabel(lgtmLabel) {
		reviewers := append(obj.Issue.Assignees, obj.Issue.Assignee)
		if !mungerutil.HasValidReviwer(reviewers) {
			return
		}
		comments, err := obj.ListComments()
		if err != nil {
			glog.Errorf("unexpected error getting comments: %v", err)
		}
		if foundLGTMFromReviewer(comments, reviewers, lastModified) {
			obj.AddLabel(lgtmLabel)
		}
		return
	}

	lgtmTime := obj.LabelTime(lgtmLabel)

	if lastModified == nil || lgtmTime == nil {
		glog.Errorf("PR %d unable to determine lastModified or lgtmTime", *obj.Issue.Number)
		return
	}

	if lastModified.After(*lgtmTime) {
		glog.Infof("PR: %d lgtm:%s  lastModified:%s", *obj.Issue.Number, lgtmTime.String(), lastModified.String())
		if err := obj.WriteComment(lgtmRemovedBody); err != nil {
			return
		}
		obj.RemoveLabel(lgtmLabel)
	}
}

func foundLGTMFromReviewer(comments []*githubapi.IssueComment, reviewers []*githubapi.User, lastModified *time.Time) bool {
	for _, c := range comments {
		if lastModified == nil || c.CreatedAt == nil || (*lastModified).After(*c.CreatedAt) {
			continue
		}
		if !isReviewer(c.User, reviewers) {
			continue
		}
		if strings.ToLower(strings.TrimSpace(*c.Body)) == "lgtm" {
			return true
		}
	}
	return false
}

func isReviewer(user *githubapi.User, reviewers []*githubapi.User) bool {
	if user == nil || user.Login == nil {
		return false
	}
	for _, r := range reviewers {
		if r == nil || r.Login == nil {
			continue
		}
		if *user.Login == *r.Login {
			return true
		}
	}
	return false
}

func (LGTMHandler) isStaleComment(obj *github.MungeObject, comment *githubapi.IssueComment) bool {
	if !mergeBotComment(comment) {
		return false
	}
	if *comment.Body != lgtmRemovedBody {
		return false
	}
	if !obj.HasLabel("lgtm") {
		return false
	}
	lgtmTime := obj.LabelTime("lgtm")
	if lgtmTime == nil {
		return false
	}
	stale := lgtmTime.After(*comment.CreatedAt)
	if stale {
		glog.V(6).Infof("Found stale LGTMHandler comment")
	}
	return stale
}

// StaleComments returns a list of comments which are stale
func (h LGTMHandler) StaleComments(obj *github.MungeObject, comments []*githubapi.IssueComment) []*githubapi.IssueComment {
	return forEachCommentTest(obj, comments, h.isStaleComment)
}

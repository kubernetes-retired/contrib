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
	"regexp"

	"k8s.io/contrib/mungegithub/features"
	"k8s.io/contrib/mungegithub/github"

	"github.com/golang/glog"
	githubapi "github.com/google/go-github/github"
	"github.com/spf13/cobra"
)

// RetestMunger will label a PR with retest-not-required if it only affects
// .md files.
// This may eventually be rewritten using a better heuristic for deciding
// which filetypes require retesting.
type RetestMunger struct{}

const (
	retestNotRequiredLabel = "retest-not-required"
)

var (
	markdownRE      = regexp.MustCompile(`.+\.md$`)
	retestCommentRE = regexp.MustCompile("Retest not required for this PR because of changed filetypes.")
)

func init() {
	r := &RetestMunger{}
	RegisterMungerOrDie(r)
	RegisterStaleComments(r)
}

// Initialize will initialize the munger
func (r *RetestMunger) Initialize(config *github.Config, features *features.Features) error {
	return nil
}

// Name is the name usable in --pr-mungers
func (r *RetestMunger) Name() string { return retestNotRequiredLabel }

// AddFlags will add any request flags to the cobra `cmd`
func (r *RetestMunger) AddFlags(cmd *cobra.Command, config *github.Config) {}

// EachLoop is called at the start of every munge loop
func (r *RetestMunger) EachLoop() error { return nil }

// RequiredFeatures is a slice of 'features' that must be provided
func (r *RetestMunger) RequiredFeatures() []string { return []string{} }

// Munge is the workhorse the will actually make updates to the PR
func (r *RetestMunger) Munge(obj *github.MungeObject) {
	if !obj.IsPR() {
		return
	}

	pr, err := obj.GetPR()
	if err != nil {
		return
	}

	commits, err := obj.GetCommits()
	if err != nil {
		return
	}

	for _, c := range commits {
		for _, f := range c.Files {
			if !retestCommentRE.MatchString(*f.Filename) {
				return
			}
		}
	}

	obj.AddLabel(retestNotRequiredLabel)
}

func (r *RetestMunger) isStaleComment(obj *github.MungeObject, comment githubapi.IssueComment) bool {
	if !mergeBotComment(comment) {
		return false
	}
	stale := retestCommentRE.MatchString(*comment.Body)
	if stale {
		glog.V(6).Infof("Found stale RetestMunger comment")
	}
	return stale
}

// StaleComments returns a slice of stale comments
func (r *RetestMunger) StaleComments(obj *github.MungeObject, comments []githubapi.IssueComment) []githubapi.IssueComment {
	return forEachCommentTest(obj, comments, r.isStaleComment)
}

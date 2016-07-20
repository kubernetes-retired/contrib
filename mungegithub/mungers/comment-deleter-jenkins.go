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
	"regexp"

	"k8s.io/contrib/mungegithub/github"

	"github.com/golang/glog"
	githubapi "github.com/google/go-github/github"
)

const (
	commentDeleterJenkinsName = "comment-deleter-jenkins"
	e2eResultStr              = `GCE e2e( test)? build/test \*\*(passed|failed)\*\* for commit [[:xdigit:]]+\.
\* \[Build Log\]\([^)]+\)
\* \[Test Artifacts\]\([^)]+\)
\* \[Internal Jenkins Results\]\([^)]+\)`
	e2eUpdatedResultStr = `GCE e2e( test)? build/test \*\*(passed|failed)\*\* for commit [[:xdigit:]]+\.
\* \[Test Results\]\([^)]+\)
\* \[Build Log\]\([^)]+\)
\* \[Test Artifacts\]\([^)]+\)
\* \[Internal Jenkins Results\]\([^)]+\)`
	okToTestStr = `Can one of the admins verify that this patch is reasonable to test\? If so, please reply "ok to test"\.`
)

var (
	_ = glog.Infof
	//Changed so that this variable is true if it compiles old or updated
	regs         []*regexp.Regexp
	e2eResultReg *regexp.Regexp
)

// CommentDeleterJenkins looks for jenkins comments which are no longer useful
// and deletes them
type CommentDeleterJenkins struct{}

func init() {
	regs = []*regexp.Regexp{
		regexp.MustCompile(e2eResultStr),
		regexp.MustCompile(e2eUpdatedResultStr),
		regexp.MustCompile(okToTestStr),
	}
	e2eResultReg = regexp.MustCompile(e2eResultStr)
	c := CommentDeleterJenkins{}
	RegisterStaleComments(c)
}

// StaleComments returns a slice of comments which are stale
func (CommentDeleterJenkins) StaleComments(obj *github.MungeObject, comments []githubapi.IssueComment) []githubapi.IssueComment {
	out := []githubapi.IssueComment{}
	last := make([]*githubapi.IssueComment, len(regs), len(regs))

	for i := range comments {
		comment := comments[i]
		if !jenkinsBotComment(comment) {
			continue
		}

		body := *comment.Body

		for j, reg := range regs {
			if reg.MatchString(body) {
				if last[j] != nil {
					out = append(out, *last[j])
				}
				last[j] = &comment
				break
			}
		}
	}
	return out
}

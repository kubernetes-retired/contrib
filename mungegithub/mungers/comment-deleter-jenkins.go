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
	//These two regular expressions for testing commits
	e2eTestStr = `GCE e2e( test)? build/test \*\*(passed|failed)\*\* for commit [[:xdigit:]]+\.
\* \[Build Log\]\([^)]+\)
\* \[Test Artifacts\]\([^)]+\)
\* \[Internal Jenkins Results\]\([^)]+\)`
	e2eUpdatedTestStr = `GCE e2e( test)? build/test \*\*(passed|failed)\*\* for commit [[:xdigit:]]+\.
\* \[Test Results\]\([^)]+\)
\* \[Build Log\]\([^)]+\)
\* \[Test Artifacts\]\([^)]+\)
\* \[Internal Jenkins Results\]\([^)]+\)`

	//This regular expression for testing patches, is spammy
	okToTestStr = `Can one of the admins verify that this patch is reasonable to test\? If so, please reply "ok to test"\.`
)

var (
	_ = glog.Infof
	//For testing commits
	e2eRegexp        = regexp.MustCompile(e2eTestStr)
	e2eUpdatedRegexp = regexp.MustCompile(e2eUpdatedTestStr)
	//For testing patches
	okToTestRegexp = regexp.MustCompile(okToTestStr)
)

// CommentDeleterJenkins looks for jenkins comments which are no longer useful
// and deletes them
type CommentDeleterJenkins struct{}

func init() {
	c := CommentDeleterJenkins{}
	RegisterStaleComments(c)
}

func isOKToTestComment(body string) bool {
	return okToTestRegexp.MatchString(body)
}

func isE2EComment(body string) bool {
	return e2eUpdatedRegexp.MatchString(body) || e2eRegexp.MatchString(body)
}

// StaleComments returns a slice of comments which are stale
func (CommentDeleterJenkins) StaleComments(obj *github.MungeObject, comments []githubapi.IssueComment) []githubapi.IssueComment {
	out := []githubapi.IssueComment{}
	var lastE2E *githubapi.IssueComment
	var lastOKToTest *githubapi.IssueComment

	for i := range comments {
		comment := comments[i]
		//Tests if jenkins bot authored comment
		if !jenkinsBotComment(comment) {
			continue
		}
		//Tests if comment is either about commit or patch
		if !isE2EComment(*comment.Body) && !isOKToTestComment(*comment.Body) {
			continue
		}
		if isE2EComment(*comment.Body) {
			if lastE2E != nil {
				out = append(out, *lastE2E)
			}
			lastE2E = &comment
		} else if isOKToTestComment(*comment.Body) {
			if lastOKToTest != nil {
				out = append(out, *lastOKToTest)
			}
			lastOKToTest = &comment
		}
	}
	return out
}

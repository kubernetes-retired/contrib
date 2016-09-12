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

package states

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"runtime"
	"testing"

	github_util "k8s.io/contrib/mungegithub/github"
	github_test "k8s.io/contrib/mungegithub/github/testing"

	"github.com/google/go-github/github"
)

func TestPreReview(t *testing.T) {
	runtime.GOMAXPROCS(runtime.NumCPU())

	tests := []struct {
		name          string
		assignees     []*github.User
		pr            *github.PullRequest
		prIssue       *github.Issue
		issueComments []*github.IssueComment
		expected      State
	}{
		{
			name:      "Should not proceed if cla: no.",
			assignees: []*github.User{github_test.User("dev45")},
			pr:        github_test.PullRequest("dev45", false, true, true),
			prIssue:   github_test.Issue("dev45", 10, []string{claNo}, true),
			expected:  &End{},
		},
		{
			name:      "Should not proceed if no assignee.",
			assignees: []*github.User{},
			pr:        github_test.PullRequest("dev45", false, true, true),
			prIssue:   github_test.Issue("dev45", 10, []string{claYes, releaseNoteActionRequired}, true),
			expected:  &End{},
		},
		{
			name:      "Should not proceed if no release-note-label.",
			assignees: []*github.User{github_test.User("dev45")},
			pr:        github_test.PullRequest("dev45", false, true, true),
			prIssue:   github_test.Issue("dev45", 10, []string{claYes}, true),
			expected:  &End{},
		},
		{
			name:      "Should proceed if cla:yes, release note set and assignee set.",
			assignees: []*github.User{github_test.User("dev45")},
			pr:        github_test.PullRequest("dev45", false, true, true),
			prIssue:   github_test.Issue("dev45", 10, []string{claYes, releaseNoteActionRequired}, true),
			expected:  &NeedsReview{},
		},
	}
	for _, test := range tests {
		test.prIssue.Assignees = test.assignees
		client, server, mux := github_test.InitServer(t, test.prIssue, test.pr, nil, nil, nil, nil, nil)
		path := fmt.Sprintf("/repos/o/r/issue/%d/labels", *test.prIssue.Number)
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			out := []github.Label{{}}
			w.WriteHeader(http.StatusOK)
			data, err := json.Marshal(out)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			w.Write(data)
		})

		path = fmt.Sprintf("/repos/o/r/issues/%d/labels", *test.prIssue.Number)
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			out := []github.Label{{}}
			w.WriteHeader(http.StatusOK)
			data, err := json.Marshal(out)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			w.Write(data)
		})

		config := &github_util.Config{}
		config.Org = "o"
		config.Project = "r"
		config.SetClient(client)

		obj, err := config.GetObject(*test.prIssue.Number)
		if err != nil {
			t.Fatalf("%v", err)
		}
		p := PreReview{}
		nextState, err := p.Process(obj)
		if err != nil {
			t.Fatalf("%v", err)
		}

		if !reflect.DeepEqual(nextState, test.expected) {
			t.Errorf("%s: expected next state: %#v, got: %#v", test.name, test.expected, nextState)
		}
		server.Close()
	}
}

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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"runtime"
	"testing"

	github_util "k8s.io/contrib/mungegithub/github"
	github_test "k8s.io/contrib/mungegithub/github/testing"

	"github.com/golang/glog"
	"github.com/google/go-github/github"
)

var (
	_ = fmt.Printf
	_ = glog.Errorf
)

func TestAssignFixes(t *testing.T) {
	runtime.GOMAXPROCS(runtime.NumCPU())

	tests := []struct {
		name           string
		assignee       string
		expectAssignee string
		pr             *github.PullRequest
		prIssue        *github.Issue
		prBody         string
		fixesIssue     *github.Issue
		comments       []github.IssueComment
		expectClaim    bool
	}{
		{
			name:           "fixes an issue, with comment and contributor change",
			assignee:       "eparis",
			expectAssignee: "eparis",
			pr:             github_test.PullRequest("eparis", false, true, true),
			prIssue:        github_test.Issue("eparis", 7779, []string{}, true),
			prBody:         "does stuff and fixes #8889.",
			fixesIssue:     github_test.Issue("jill", 8889, []string{}, true),
			comments: []github.IssueComment{
				comment(8889, "bla \nbla"),
				comment(8889, "food for all")},
			expectClaim: true,
		},
		{
			name:           "fixes an issue, no comment",
			assignee:       "eparis",
			expectAssignee: "eparis",
			pr:             github_test.PullRequest("eparis", false, true, true),
			prIssue:        github_test.Issue("eparis", 7779, []string{}, true),
			prBody:         "does stuff and fixes #8889.",
			fixesIssue:     github_test.Issue("eparis", 8889, []string{}, true),
			comments: []github.IssueComment{
				comment(8889, "bla \nbla"),
				comment(8889, "Assigned to @eparis")},
			expectClaim: false,
		},
		{
			name:           "fixes an issue, no comment",
			assignee:       "asalkeld",
			expectAssignee: "k8s-almost-a-contributor",
			pr:             github_test.PullRequest("asalkeld", false, true, true),
			prIssue:        github_test.Issue("eparis", 7779, []string{}, true),
			prBody:         "does stuff and fixes #8889.",
			fixesIssue:     github_test.Issue("jill", 8889, []string{}, true),
			comments: []github.IssueComment{
				comment(8889, "bla \nbla"),
				comment(8889, "food for all\nClaimed by @asalkeld")},
			expectClaim: false,
		},
		{
			name:           "fixes an issue, with comment and contributor",
			assignee:       "asalkeld",
			expectAssignee: "k8s-almost-a-contributor",
			pr:             github_test.PullRequest("asalkeld", false, true, true),
			prIssue:        github_test.Issue("eparis", 7779, []string{}, true),
			prBody:         "does stuff and fixes #8889.",
			fixesIssue:     github_test.Issue("jill", 8889, []string{}, true),
			comments: []github.IssueComment{
				comment(8889, "Claimed by @jonny"),
				comment(8889, "food for all\nClaimed by @asalkeld"),
				comment(8889, "Claimed by @lazy.dude")},
			expectClaim: true,
		},
	}
	collaborators := []github.User{*github_test.User("eparis")}
	for _, test := range tests {
		test.prIssue.Body = &test.prBody
		client, server, mux := github_test.InitServerWithCollaborators(t, test.prIssue, test.pr, nil, nil, nil, nil, collaborators)
		path := fmt.Sprintf("/repos/o/r/issues/%d", *test.fixesIssue.Number)
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			data, err := json.Marshal(test.fixesIssue)
			if err != nil {
				t.Errorf("%v", err)
			}
			if r.Method != "PATCH" && r.Method != "GET" {
				t.Errorf("Unexpected method: expected: GET/PATCH got: %s", r.Method)
			}
			if r.Method == "PATCH" {
				body, _ := ioutil.ReadAll(r.Body)

				type IssuePatch struct {
					Assignee string
				}
				var ip IssuePatch
				err := json.Unmarshal(body, &ip)
				if err != nil {
					fmt.Println("error:", err)
				}
				if ip.Assignee != test.expectAssignee {
					t.Errorf("Patching the incorrect Assignee %v instead of %v", ip.Assignee, test.expectAssignee)
				}
			}
			w.WriteHeader(http.StatusOK)
			w.Write(data)
		})
		mux.HandleFunc(fmt.Sprintf("/repos/o/r/issues/8889/comments"), func(w http.ResponseWriter, r *http.Request) {
			if !(r.Method == "GET" || (r.Method == "POST" && test.expectClaim)) {
				t.Errorf("Unexpected method: expected: GET/POST got: %s", r.Method)
			}
			w.WriteHeader(http.StatusOK)
			if r.Method == "GET" {
				data, _ := json.Marshal(test.comments)
				w.Write(data)
			}
		})

		config := &github_util.Config{}
		config.Org = "o"
		config.Project = "r"
		config.SetClient(client)

		c := AssignFixesMunger{}
		err := c.Initialize(config, nil)
		if err != nil {
			t.Fatalf("%v", err)
		}

		err = c.EachLoop()
		if err != nil {
			t.Fatalf("%v", err)
		}

		obj, err := config.GetObject(*test.prIssue.Number)
		if err != nil {
			t.Fatalf("%v", err)
		}

		c.Munge(obj)
		server.Close()
	}
}

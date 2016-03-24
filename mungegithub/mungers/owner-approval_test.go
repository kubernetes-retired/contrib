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
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"testing"
	"time"

	github_features "k8s.io/contrib/mungegithub/features"
	github_util "k8s.io/contrib/mungegithub/github"
	github_test "k8s.io/contrib/mungegithub/github/testing"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/golang/glog"
	"github.com/google/go-github/github"
)

var (
	_ = fmt.Printf
	_ = glog.Errorf
)

func comment(body string, user string, t int64) github.IssueComment {
	return github.IssueComment{
		Body: &body,
		User: &github.User{
			Login: &user,
		},
		CreatedAt: timePtr(time.Unix(t, 0)),
		UpdatedAt: timePtr(time.Unix(t, 0)),
	}
}

func TestOwnerMunge(t *testing.T) {
	runtime.GOMAXPROCS(runtime.NumCPU())

	tests := []struct {
		name     string // because when the fail, counting is hard
		commits  []github.RepositoryCommit
		comments []github.IssueComment
		pass     bool
	}{
		{
			name:    "Approved by lowest level",
			commits: pathsCommit([]string{"hack/file.sh", "hack/after-build/file.sh", "pkg/util/file.go"}), // Modified at time.Unix(7), 8, and 9
			comments: []github.IssueComment{
				comment("hello", "root2", 11),
				comment("i ApproVe", "hack1", 11),
				comment("ApproVed", "afterbuild1", 11),
				comment("ApproVed", "pkg2", 11),
			},
			pass: true,
		},
		{
			name:    "Approved by root",
			commits: pathsCommit([]string{"hack/file.sh", "hack/after-build/file.sh", "pkg/util/file.go"}), // Modified at time.Unix(7), 8, and 9
			comments: []github.IssueComment{
				comment("ApproVed", "root2", 11),
			},
			pass: true,
		},
		{
			name:    "Missing pkg approval",
			commits: pathsCommit([]string{"hack/file.sh", "hack/after-build/file.sh", "pkg/util/file.go"}), // Modified at time.Unix(7), 8, and 9
			comments: []github.IssueComment{
				comment("i ApproVe", "hack1", 11),
				comment("ApproVed", "afterbuild1", 11),
			},
			pass: false,
		},
		{
			name:    "Early approval",
			commits: pathsCommit([]string{"hack/file.sh", "hack/after-build/file.sh", "pkg/util/file.go"}), // Modified at time.Unix(7), 8, and 9
			comments: []github.IssueComment{
				comment("i ApproVe", "hack1", 6),
				comment("ApproVed", "afterbuild1", 11),
				comment("approved", "pkg1", 12),
			},
			pass: false,
		},
	}
	owners := map[string]sets.String{
		"":                 sets.NewString("root1", "root2"),
		"hack":             sets.NewString("hack1", "hack2"),
		"hack/after-build": sets.NewString("afterbuild1", "afterbuild2"),
		"pkg":              sets.NewString("pkg1", "pkg2"),
	}
	assignees := map[string]sets.String{}

	for testNum, test := range tests {
		pr := ValidPR()
		issue := NoOKToMergeIssue()
		issueNum := testNum + 1
		issue.Number = &issueNum
		events := NewLGTMEvents()

		client, server, mux := github_test.InitServer(t, issue, pr, events, test.commits, nil)

		config := &github_util.Config{}
		config.Org = "o"
		config.Project = "r"
		config.SetClient(client)

		path := fmt.Sprintf("/repos/o/r/issues/%d/labels", issueNum)
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			if r.Method != "POST" {
				t.Errorf("Unexpected method: %s", r.Method)
			}
			data, err := json.Marshal([]github.Label{})
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			w.Write(data)
		})
		path = fmt.Sprintf("/repos/o/r/issues/%d/comments", issueNum)
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			if r.Method == "GET" {
				data, err := json.Marshal(test.comments)
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				w.Write(data)
				return
			}
			if r.Method != "POST" {
				t.Errorf("Unexpected method: %s", r.Method)
			}

			type comment struct {
				Body string `json:"body"`
			}
			c := new(comment)
			json.NewDecoder(r.Body).Decode(c)
			msg := c.Body
			if strings.HasPrefix(msg, "@k8s-bot test this") {
				glog.Errorf("WTF")
			}
			data, err := json.Marshal(github.IssueComment{})
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			w.Write(data)
		})

		o := OwnerApproval{}
		f := &github_features.Features{
			Repos: github_features.TestFakeRepo(assignees, owners),
		}
		o.features = f
		obj := github_util.TestObject(config, issue, pr, test.commits, events)
		o.Munge(obj)
		if test.pass && !obj.HasLabel(ownerApproval) {
			t.Errorf("%d:%q should have label but doesn't", testNum, test.name)
		} else if !test.pass && obj.HasLabel(ownerApproval) {
			t.Errorf("%d:%q should not have label but does", testNum, test.name)
		}
		server.Close()
	}
}

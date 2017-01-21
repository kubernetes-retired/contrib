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
	"testing"

	github_util "k8s.io/contrib/mungegithub/github"
	github_test "k8s.io/contrib/mungegithub/github/testing"
)

func TestUpdates(t *testing.T) {
	munger := &FileToPRMap{}

	client, server, _ := github_test.InitServer(t, nil, ValidPR(), nil, nil, nil, nil, nil)
	defer server.Close()

	config := &github_util.Config{}
	config.Org = "o"
	config.Project = "r"
	config.SetClient(client)

	munger.Initialize(config, nil)
	munger.updateFiles(commitFiles([]string{
		"a/b",
		"c",
	}), 1)

	munger.updateFiles(commitFiles([]string{
		"a/b",
		"d",
	}), 2)

	munger.updateFiles(commitFiles([]string{
		"d",
		"e",
	}), 3)

	munger.EachLoop()

	prs, found := munger.fileToPR["a/b"]
	if !found {
		t.Errorf("failed to find")
	}
	if len(prs) != 2 || prs[0] != 1 || prs[1] != 2 {
		t.Errorf("unexpected: %v", prs)
	}

	prs, found = munger.fileToPR["c"]
	if !found {
		t.Errorf("failed to find")
	}
	if len(prs) != 1 || prs[0] != 1 {
		t.Errorf("unexpected: %v", prs)
	}

	prs, found = munger.fileToPR["d"]
	if !found {
		t.Errorf("failed to find")
	}
	if len(prs) != 2 || prs[0] != 2 || prs[1] != 3 {
		t.Errorf("unexpected: %v", prs)
	}

	prs, found = munger.fileToPR["e"]
	if !found {
		t.Errorf("failed to find")
	}
	if len(prs) != 1 || prs[0] != 3 {
		t.Errorf("unexpected: %v", prs)
	}
}

func TestFileToPR(t *testing.T) {
	issue := github_test.Issue("k8s-bot", 1, []string{}, true)
	fileStr := []string{
		"docs/proposals",
		"foo/bar.go",
	}
	files := commitFiles(fileStr)
	client, server, _ := github_test.InitServer(t, issue, ValidPR(), nil, nil, nil, nil, files)
	defer server.Close()

	config := &github_util.Config{}
	config.Org = "o"
	config.Project = "r"
	config.SetClient(client)

	munger := &FileToPRMap{}
	munger.Initialize(config, nil)

	obj, err := config.GetObject(1)
	if err != nil {
		t.Fatalf("%v", err)
	}

	munger.Munge(obj)

	munger.EachLoop()

	for _, file := range fileStr {
		prs, found := munger.fileToPR[file]
		if !found {
			t.Errorf("Couldn't find: %s\n%v", file, munger.nextFileToPR)
		}
		if len(prs) != 1 || prs[0] != *issue.Number {
			t.Errorf("unexpected pr list: %v, expected [%d]", prs, *issue.Number)
		}
	}

	// No munging this time, should clear the variables
	munger.EachLoop()
	for _, file := range fileStr {
		_, found := munger.fileToPR[file]
		if found {
			t.Errorf("Didn't expect to find: %s\n%v", file, munger.nextFileToPR)
		}
	}
}

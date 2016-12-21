/*
Copyright 2015 The Kubernetes Authors.

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

package features

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/golang/glog"
	"github.com/google/go-github/github"
	"k8s.io/kubernetes/pkg/util/sets"
	"path/filepath"
)

var (
	_ = fmt.Printf
	_ = glog.Errorf
)

const (
	baseDir        = ""
	leafDir        = "a/b/c"
	nonExistentDir = "DELETED_DIR"
)

// Commit returns a filled out github.Commit which happened at time.Unix(t, 0)
func getTestRepo() *RepoInfo {
	testRepo := RepoInfo{BaseDir: baseDir, EnableMdYaml: false, UseReviewers: false}
	approvers := map[string]sets.String{}
	baseApprovers := sets.NewString("Alice", "Bob")
	leafApprovers := sets.NewString("Carl", "Dave")
	approvers[baseDir] = baseApprovers
	approvers[leafDir] = leafApprovers
	testRepo.approvers = approvers

	return &testRepo
}

func TestRepoUpdates(t *testing.T) {
	runtime.GOMAXPROCS(runtime.NumCPU())

	testFile0 := filepath.Join(baseDir, "testFile.md")
	testFile1 := filepath.Join(leafDir, "testFile.md")
	testFile2 := filepath.Join(nonExistentDir, "testFile.md")
	TestRepo := getTestRepo()
	tests := []struct {
		testName           string
		testFile           *github.CommitFile
		expectedOwnersPath string
		expectedLeafOwners sets.String
		expectedAllOwners  sets.String
	}{
		{
			testName:           "Modified Base Dir Only",
			testFile:           &github.CommitFile{Filename: &testFile0},
			expectedOwnersPath: baseDir,
			expectedLeafOwners: TestRepo.approvers[baseDir],
			expectedAllOwners:  TestRepo.approvers[baseDir],
		},
		{
			testName:           "Modified Leaf Dir Only",
			testFile:           &github.CommitFile{Filename: &testFile1},
			expectedOwnersPath: leafDir,
			expectedLeafOwners: TestRepo.approvers[leafDir],
			expectedAllOwners:  TestRepo.approvers[leafDir].Union(TestRepo.approvers[baseDir]),
		},
		{
			testName:           "Modified Nonexistent Dir (Default to Base)",
			testFile:           &github.CommitFile{Filename: &testFile2},
			expectedOwnersPath: baseDir,
			expectedLeafOwners: TestRepo.approvers[baseDir],
			expectedAllOwners:  TestRepo.approvers[baseDir],
		},
	}
	for testNum, test := range tests {
		foundLeafApprovers := TestRepo.LeafApprovers(*test.testFile.Filename)
		foundApprovers := TestRepo.Approvers(*test.testFile.Filename)
		foundOwnersPath := TestRepo.FindOwnersForPath(*test.testFile.Filename)
		if !foundLeafApprovers.Equal(test.expectedLeafOwners) {
			t.Errorf("The Leaf Approvers Found Do Not Match Expected For Test %d: %s", testNum, test.testName)
			t.Errorf("\tExpected Owners: %v\tFound Owners: %v ", test.expectedLeafOwners, foundLeafApprovers)
		}
		if !foundApprovers.Equal(test.expectedAllOwners) {
			t.Errorf("The Approvers Found Do Not Match Expected For Test %d: %s", testNum, test.testName)
			t.Errorf("\tExpected Owners: %v\tFound Owners: %v ", test.expectedAllOwners, foundApprovers)
		}
		if foundOwnersPath != test.expectedOwnersPath {
			t.Errorf("The Owners Path Found Does Not Match Expected For Test %d: %s", testNum, test.testName)
			t.Errorf("\tExpected Owners: %v\tFound Owners: %v ", test.expectedOwnersPath, foundOwnersPath)
		}
	}
}

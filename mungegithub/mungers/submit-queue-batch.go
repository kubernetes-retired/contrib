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
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
	githubapi "github.com/google/go-github/github"
	"k8s.io/contrib/mungegithub/github"
)

// xref k8s.io/test-infra/prow/cmd/deck/jobs.go
type ProwJob struct {
	Type    string `json:"type"`
	Repo    string `json:"repo"`
	Refs    string `json:"refs"`
	State   string `json:"state"`
	Context string `json:"context"`
}

// getSuccessfulBatchJobs reads test results from Prow and returns
// all batch jobs that succeeded for the current repo.
func getSuccessfulBatchJobs(repo, url string) ([]ProwJob, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	allJobs := []ProwJob{}
	err = json.Unmarshal(body, &allJobs)
	if err != nil {
		return nil, err
	}

	jobs := []ProwJob{}
	for _, job := range allJobs {
		if job.Repo == repo && job.Type == "batch" && job.State == "success" {
			jobs = append(jobs, job)
		}
	}
	return jobs, nil
}

type BatchPull struct {
	Number int
	Sha    string
}

type Batch struct {
	BaseName string
	BaseSha  string
	Pulls    []BatchPull
}

// batchRefToBatch parses a string into a Batch
func batchRefToBatch(batchRef string) (Batch, error) {
	batch := Batch{}
	for i, ref := range strings.Split(batchRef, ",") {
		parts := strings.Split(ref, ":")
		if len(parts) != 2 {
			return Batch{}, errors.New("bad batchref")
		}
		if i == 0 {
			batch.BaseName = parts[0]
			batch.BaseSha = parts[1]
		} else {
			num, err := strconv.ParseInt(parts[0], 10, 32)
			if err != nil {
				return Batch{}, err
			}
			batch.Pulls = append(batch.Pulls, BatchPull{int(num), parts[1]})
		}
	}
	return batch, nil
}

// getCompleteBatches returns a list of Batches that passed all
// required tests.
func (sq *SubmitQueue) getCompleteBatches(jobs []ProwJob) []Batch {
	batchContexts := make(map[string]map[string]interface{})
	for _, job := range jobs {
		if _, ok := batchContexts[job.Refs]; !ok {
			batchContexts[job.Refs] = make(map[string]interface{})
		}
		batchContexts[job.Refs][job.Context] = nil
	}
	batches := []Batch{}
	for batchRef, contexts := range batchContexts {
		match := true
		// Did this succeed in all the contexts we want?
		for _, ctx := range sq.RequiredStatusContexts {
			if _, ok := contexts[ctx]; !ok {
				match = false
			}
		}
		for _, ctx := range sq.RequiredRetestContexts {
			if _, ok := contexts[ctx]; !ok {
				match = false
			}
		}
		if match {
			batch, err := batchRefToBatch(batchRef)
			if err != nil {
				continue
			}
			batches = append(batches, batch)
		}
	}
	return batches
}

// batchIntersectsQueue returns whether at least one PR in the batch is queued.
func (sq *SubmitQueue) batchIntersectsQueue(batch Batch) bool {
	sq.Lock()
	defer sq.Unlock()
	for _, pull := range batch.Pulls {
		if _, ok := sq.githubE2EQueue[pull.Number]; ok {
			return true
		}
	}
	return false
}

// matchesCommit determines if the batch can be merged given some commits.
// That is, does it contain exactly:
// 1) the batch's BaseSha
// 2) (optional) merge commits for PRs in the batch
// 3) any merged PRs in the batch are sequential from the beginning
// The return value is -1 on error, else the number of PRs already merged.
func (batch *Batch) matchesCommits(commits []*githubapi.RepositoryCommit) int {
	if len(commits) == 0 {
		return -1
	}

	shaToPR := make(map[string]int)

	for _, pull := range batch.Pulls {
		shaToPR[pull.Sha] = pull.Number
	}

	matchedPRs := []int{}

	// convert the list of commits into a DAG for easy following
	dag := make(map[string]*githubapi.RepositoryCommit)
	for _, commit := range commits {
		dag[*commit.SHA] = commit
	}

	ref := *commits[0].SHA
	for {
		if ref == batch.BaseSha {
			break // found the base ref (condition #1)
		}
		commit, ok := dag[ref]
		if !ok {
			return -1 // ref not in partial list of commits
		}
		if len(commit.Parents) == 2 && commit.Message != nil && (*commit.Message)[:5] == "Merge" {
			// looks like a merge commit!

			// first parent is the normal branch
			ref = *commit.Parents[0].SHA
			// second parent is the PR
			pr, ok := shaToPR[*commit.Parents[1].SHA]
			if !ok {
				return -1 // merge of a PR not in batch
			}
			matchedPRs = append(matchedPRs, pr)
		} else {
			return -1 // unhandled non-merge commit
		}
	}

	// Now, ensure that the merged PRs are ordered correctly.
	for i, pr := range matchedPRs {
		if batch.Pulls[len(matchedPRs)-1-i].Number != pr {
			return -1
		}
	}
	return len(matchedPRs)
}

// batchIsApplicable returns whether a successful batch result can be used--
// 1) some of the batch is still unmerged and in the queue.
// 2) the recent commits are the batch head ref or merges of batch PRs.
// 3) all unmerged PRs in the batch are still in the queue.
// The return value is -1 on error, else the number of PRs already merged.
func (sq *SubmitQueue) batchIsApplicable(batch Batch) int {
	// batch must intersect the queue
	if !sq.batchIntersectsQueue(batch) {
		return -1
	}
	commits, err := sq.githubConfig.GetBranchCommits(batch.BaseName)
	if err != nil {
		glog.Errorf("Error getting commits for batchIsApplicable: %v", err)
		return -1
	}
	return batch.matchesCommits(commits)
}

func (sq *SubmitQueue) handleGithubE2EBatchMerge() {
	repo := sq.githubConfig.Org + "/" + sq.githubConfig.Project
	for range time.Tick(1 * time.Minute) {
		jobs, err := getSuccessfulBatchJobs(repo, sq.BatchURL)
		if err != nil {
			glog.Errorf("Error reading batch jobs from Prow URL %v", sq.BatchURL)
			continue
		}
		batches := sq.getCompleteBatches(jobs)
		for _, batch := range batches {
			if sq.batchIsApplicable(batch) < 0 {
				continue
			}
			sq.doBatchMerge(batch)
		}
	}
}

// doBatchMerge iteratively merges PRs in the batch if possible.
// If you modify this, consider modifying doGithubE2EAndMerge too.
func (sq *SubmitQueue) doBatchMerge(batch Batch) {
	sq.mergeLock.Lock()
	defer sq.mergeLock.Unlock()
	match := sq.batchIsApplicable(batch)
	if match < 0 {
		return
	}
	if !sq.e2eStable(true) {
		return
	}
	prs := []*github.MungeObject{}
	// check batch preconditions first
	for _, pull := range batch.Pulls[match:] {
		obj, err := sq.githubConfig.GetObject(pull.Number)
		if err != nil {
			glog.Errorf("error getting object for pr #%d: %v", pull.Number, err)
			return
		}
		if sha, _, ok := obj.GetHeadAndBase(); !ok {
			glog.Errorf("error getting pr #%d sha", pull.Number, err)
			return
		} else if sha != pull.Sha {
			glog.Errorf("error: batch PR #%d HEAD changed: %s instead of %s",
				sha, pull.Sha)
			return
		}
		if !sq.validForMergeExt(obj, false) {
			return
		}
		prs = append(prs, obj)
	}
	// then merge each
	for _, pr := range prs {
		err := sq.mergePullRequest(pr, mergedBatch)
		if err != nil {
			return
		}
	}
}

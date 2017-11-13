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
    "net/http"

    "k8s.io/contrib/mungegithub/github"
    "k8s.io/contrib/mungegithub/features"

    "github.com/golang/glog"
	githubapi "github.com/google/go-github/github"
	"github.com/spf13/cobra"
)

var _ Munger = &BulkLGTM{}

// Bulk LGTM knows how to aggregate a large number of small PRs into a single page for
// easy bulk review.
type BulkLGTM struct {
    config *github.Config
    // The list of PRs under construction
    nextPRList []*githubapi.PullRequest
    currentPRList []*githubapi.PullRequest
}

func (b *BulkLGTM) Munge(obj *github.MungeObject) {
    if !obj.IsPR() {
        return
    }
    pr, err := obj.GetPR()
    if err != nil {
        glog.Errorf("Unexpected error getting PR: %v", err)
        return
    }
    if *pr.Commits != 1 {
        return
    }
    if *pr.Additions + *pr.Deletions > 20 {
        return
    }
    b.nextPRList = append(b.nextPRList, pr)
}

func (b *BulkLGTM) AddFlags(cmd *cobra.Command, config *github.Config) {
}

func (b *BulkLGTM) Name() string {
    return "bulk-lgtm"
}

func (b *BulkLGTM) RequiredFeatures() []string {
    return nil
}

func (b *BulkLGTM) Initialize(config *github.Config, features *features.Features) error {
    b.config = config

    http.HandleFunc("/bulkprs/prs", b.ServePRs)

    return nil
}

func (b *BulkLGTM) EachLoop() error {
    b.currentPRList = b.nextPRList
    b.nextPRList = nil
    return nil
}

func (b *BulkLGTM) ServePRs(res http.ResponseWriter, req *http.Request) {
    var data []byte
    var err error
    if b.currentPRList == nil {
        data = []byte("[]");
    }
    data, err = json.Marshal(b.currentPRList)
    if err != nil {
        res.Header().Set("Content-type", "text/plain")
        res.WriteHeader(http.StatusInternalServerError)
        return
    }
    
	res.WriteHeader(http.StatusOK)
    res.Write(data)
}
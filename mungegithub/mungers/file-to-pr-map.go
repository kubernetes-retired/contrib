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
	"net/http"
	"strings"
	"sync"

	"k8s.io/contrib/mungegithub/features"
	"k8s.io/contrib/mungegithub/github"

	"github.com/golang/glog"
	github_api "github.com/google/go-github/github"
	"github.com/spf13/cobra"
)

var _ Munger = &FileToPRMap{}

func init() {
	RegisterMungerOrDie(&FileToPRMap{})
}

// FileToPRMap maintains a map from files (and directories) to a list of PRs
type FileToPRMap struct {
	config *github.Config
	// protects the map
	lock         sync.Mutex
	fileToPR     map[string][]int
	nextFileToPR map[string][]int
}

// Munge implements the Munger interface
func (f *FileToPRMap) Munge(obj *github.MungeObject) {
	if !obj.IsPR() {
		return
	}
	pr, err := obj.GetPR()
	if err != nil {
		glog.Errorf("Unexpected error getting PR: %v", err)
		return
	}
	glog.V(4).Infof("Found a PR: %#v", *pr)

	files, err := f.config.GetFilesForPR(*pr.Number)
	if err != nil {
		glog.Errorf("unexpected error getting files: %v", err)
		return
	}
	f.updateFiles(files, *pr.Number)
}

func (f *FileToPRMap) updateFiles(files []*github_api.CommitFile, prNumber int) {
	f.lock.Lock()
	defer f.lock.Unlock()
	for _, file := range files {
		if file.Filename != nil {
			f.nextFileToPR[*file.Filename] = append(f.nextFileToPR[*file.Filename], prNumber)
		}
	}
}

// AddFlags implements the Munger interface
func (f *FileToPRMap) AddFlags(cmd *cobra.Command, config *github.Config) {
}

// Name implements the Munger interface
func (f *FileToPRMap) Name() string {
	return "bulk-lgtm"
}

// RequiredFeatures implements the Munger interface
func (f *FileToPRMap) RequiredFeatures() []string {
	return nil
}

// Initialize implements the Munger interface
func (f *FileToPRMap) Initialize(config *github.Config, features *features.Features) error {
	f.config = config
	f.nextFileToPR = map[string][]int{}

	if len(config.Address) > 0 {
		http.HandleFunc("/file2pr/prs", f.ServePRs)

		go http.ListenAndServe(config.Address, nil)
	}

	return nil
}

// EachLoop implements the Munger interface
func (f *FileToPRMap) EachLoop() error {
	f.lock.Lock()
	defer f.lock.Unlock()

	f.fileToPR = f.nextFileToPR
	f.nextFileToPR = map[string][]int{}

	return nil
}

// ServePRs serves the current PR list over HTTP
func (f *FileToPRMap) ServePRs(res http.ResponseWriter, req *http.Request) {
	prefix := req.URL.Query().Get("prefix")
	f.lock.Lock()
	defer f.lock.Unlock()
	output := map[string][]int{}
	for file, prNum := range f.fileToPR {
		if strings.HasPrefix(file, prefix) {
			output[file] = append(output[file], prNum...)
		}
	}
	data, err := json.Marshal(output)
	if err != nil {
		res.Header().Set("Content-type", "text/plain")
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	res.WriteHeader(http.StatusOK)
	res.Write(data)
}

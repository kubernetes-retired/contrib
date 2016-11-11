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
	"sync"

	"k8s.io/contrib/mungegithub/features"
	"k8s.io/contrib/mungegithub/github"

	"github.com/golang/glog"
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
	lock     sync.Mutex
	fileToPR map[string]int
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
	f.lock.Lock()
	defer f.lock.Unlock()
	for _, file := range files {
		if file.Filename != nil {
			f.fileToPR[*file.Filename] = *pr.Number
		}
	}
}

// AddFlags implements the Munger interface
func (b *FileToPRMap) AddFlags(cmd *cobra.Command, config *github.Config) {
}

// Name implements the Munger interface
func (b *FileToPRMap) Name() string {
	return "bulk-lgtm"
}

// RequiredFeatures implements the Munger interface
func (b *FileToPRMap) RequiredFeatures() []string {
	return nil
}

// Initialize implements the Munger interface
func (b *FileToPRMap) Initialize(config *github.Config, features *features.Features) error {
	b.config = config

	if len(config.Address) > 0 {
		http.HandleFunc("/file2pr/prs", b.ServePRs)

		go http.ListenAndServe(config.Address, nil)
	}

	return nil
}

// EachLoop implements the Munger interface
func (b *FileToPRMap) EachLoop() error {
	return nil
}

// ServePRs serves the current PR list over HTTP
func (b *FileToPRMap) ServePRs(res http.ResponseWriter, req *http.Request) {
	b.lock.Lock()
	defer b.lock.Unlock()
	data, err := json.Marshal(b.fileToPR)
	if err != nil {
		res.Header().Set("Content-type", "text/plain")
		res.WriteHeader(http.StatusInternalServerError)
		return
	}

	res.WriteHeader(http.StatusOK)
	res.Write(data)
}

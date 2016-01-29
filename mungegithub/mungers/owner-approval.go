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
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"k8s.io/contrib/mungegithub/github"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/yaml"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
)

const (
	ownerApproval = "has-owner-approval"
)

type assignmentConfig struct {
	Owners []string
}

// OwnerApproval it a github munger which labels PRs based on 'owner' approval.
// Owners are listed in files (called OWNERS) in the git tree. An owner owns
// everything beneath the directory. So a person who 'owns' the root directory
// can approve everything
type OwnerApproval struct {
	kubernetesDir string
	owners        map[string]sets.String
}

func init() {
	RegisterMungerOrDie(&OwnerApproval{})
}

// Name is the name usable in --pr-mungers
func (o *OwnerApproval) Name() string { return "owner-approval" }

func (o *OwnerApproval) walkFunc(path string, info os.FileInfo, err error) error {
	if err != nil {
		glog.Errorf("%v", err)
		return nil
	}
	filename := filepath.Base(path)
	if info.Mode().IsDir() {
		switch filename {
		case ".git":
			return filepath.SkipDir
		case "_output":
			return filepath.SkipDir
		}
	}
	if !info.Mode().IsRegular() {
		return nil
	}
	if filename != "OWNERS" {
		return nil
	}

	file, err := os.Open(path)
	if err != nil {
		glog.Errorf("%v", err)
		return nil
	}
	defer file.Close()

	c := &assignmentConfig{}
	if err := yaml.NewYAMLToJSONDecoder(file).Decode(c); err != nil {
		glog.Errorf("%v", err)
		return nil
	}

	path = strings.TrimPrefix(path, o.kubernetesDir)
	path = strings.TrimSuffix(path, "OWNERS")
	o.owners[path] = sets.NewString(c.Owners...)
	return nil
}

func (o *OwnerApproval) updateOwnerMap() error {
	cmd := exec.Command("git", "pull")
	cmd.Dir = o.kubernetesDir
	err := cmd.Run()
	if err != nil {
		glog.Errorf("%v", err)
		return err
	}

	o.owners = map[string]sets.String{}
	err = filepath.Walk(o.kubernetesDir, o.walkFunc)
	if err != nil {
		glog.Errorf("Got error %v", err)
	}
	glog.Infof("Loaded config from %s", o.kubernetesDir)
	glog.Infof("owners=%v", o.owners)
	return nil
}

// Initialize will initialize the munger
func (o *OwnerApproval) Initialize(config *github.Config) error {
	if len(o.kubernetesDir) == 0 {
		glog.Fatalf("--kubernetes-dir is required with the owner approval munger")
	}

	finfo, err := os.Stat(o.kubernetesDir)
	if err != nil {
		glog.Fatalf("Unable to stat --kubernetes-dir: %v", err)
	}
	if !finfo.IsDir() {
		glog.Fatalf("--kubernetes-dir is not a git directory")
	}
	return o.updateOwnerMap()
}

// EachLoop is called at the start of every munge loop
func (o *OwnerApproval) EachLoop() error {
	return o.updateOwnerMap()
}

// AddFlags will add any request flags to the cobra `cmd`
func (o *OwnerApproval) AddFlags(cmd *cobra.Command, config *github.Config) {
	cmd.Flags().StringVar(&o.kubernetesDir, "kubernetes-dir", "./kubernetes", "Path to git checkout of kubernetes tree")
	//o.addOwnerApprovalCommand(cmd)
}

func (o OwnerApproval) approvers(path string) (string, sets.String) {
	d := filepath.Dir(path)
	outDir := d
	out := sets.NewString()
	for {
		s, ok := o.owners[d]
		if ok {
			out = out.Union(s)
		}
		if d == "" {
			break
		}
		d, _ = filepath.Split(d)
		d = strings.TrimSuffix(d, "/")
	}
	return outDir, out
}

// Munge is the workhorse the will actually make updates to the PR
func (o *OwnerApproval) Munge(obj *github.MungeObject) {
	neededApprovals := map[string]sets.String{}

	if !obj.IsPR() {
		return
	}
	commits, err := obj.GetCommits()
	if err != nil {
		return
	}

	for _, c := range commits {
		for _, f := range c.Files {
			filename := *f.Filename
			d, s := o.approvers(filename)
			neededApprovals[d] = s
		}
	}

	lastModified := obj.LastModifiedTime()
	if lastModified == nil {
		glog.Errorf("%d: Unable to determine last modified time", *obj.Issue.Number)
	}

	comments, err := obj.GetComments()
	if err != nil {
		return
	}

	approvalsGiven := sets.String{}
	for _, comment := range comments {
		if lastModified.After(*comment.UpdatedAt) {
			continue
		}
		lines := strings.Split(*comment.Body, "\n")
		for _, line := range lines {
			line = strings.TrimPrefix(line, "@k8s-merge-bot")
			line = strings.TrimSpace(line)
			line = strings.ToLower(line)
			switch line {
			case "i approve":
				approvalsGiven.Insert(*comment.User.Login)
			case "approved":
				approvalsGiven.Insert(*comment.User.Login)
			}
		}
	}

	missingApprovals := sets.NewString()
	for _, needed := range neededApprovals {
		intersection := needed.Intersection(approvalsGiven)
		if intersection.Len() != 0 {
			// Someone who approved covered this area
			continue
		}
		missingApprovals = missingApprovals.Union(needed)
	}

	if missingApprovals.Len() == 0 && !obj.HasLabel(ownerApproval) {
		obj.AddLabel(ownerApproval)
	} else if missingApprovals.Len() != 0 && obj.HasLabel(ownerApproval) {
		obj.RemoveLabel(ownerApproval)
	}
}

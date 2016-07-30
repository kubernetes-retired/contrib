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
	"strings"

	"github.com/golang/glog"
	githubapi "github.com/google/go-github/github"
	"github.com/spf13/cobra"
	"k8s.io/contrib/mungegithub/features"
	"k8s.io/contrib/mungegithub/github"
)

// ApprovalHandler will check if approval is given to a RP and apply approval label if so.
type ApprovalHandler struct{}

func init() {
	l := ApprovalHandler{}
	RegisterMungerOrDie(l)
}

// Name is the name usable in --pr-mungers
func (ApprovalHandler) Name() string { return "lgtm-after-commit" }

// RequiredFeatures is a slice of 'features' that must be provided
func (ApprovalHandler) RequiredFeatures() []string { return []string{} }

// Initialize will initialize the munger
func (ApprovalHandler) Initialize(config *github.Config, features *features.Features) error {
	return nil
}

// EachLoop is called at the start of every munge loop
func (ApprovalHandler) EachLoop() error { return nil }

// AddFlags will add any request flags to the cobra `cmd`
func (ApprovalHandler) AddFlags(cmd *cobra.Command, config *github.Config) {}

// Munge is the workhorse the will actually make updates to the PR
func (ApprovalHandler) Munge(obj *github.MungeObject) {
	if !obj.IsPR() {
		return
	}
	lastModified := obj.LastModifiedTime()
	if lastModified == nil {
		return
	}
	if !(obj.HasLabel(lgtmLabel) && !obj.HasLabel(approvalLabel)) {
		return
	}
	comments, err := obj.ListComments()
	if err != nil {
		glog.Errorf("unexpected error getting comments: %v", err)
	}

	for _, c := range comments {
		if c.Body == nil {
			continue
		}
		if !isApprover(c.User) {
			return
		}

		line := strings.ToLower(strings.TrimSpace(*c.Body))
		switch line {
		case "i approve":
			fallthrough
		case "approved":
			obj.AddLabel(approvalLabel)
		}
	}
}

func isApprover(*githubapi.User) bool {
	panic("TODO")
}

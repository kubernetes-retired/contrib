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

package mungers

import (
	"k8s.io/contrib/mungegithub/features"
	"k8s.io/contrib/mungegithub/github"
	"k8s.io/contrib/mungegithub/mungers/matchers/event"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
)

// ApproveFollowsLGTM will try to follow the LGTM label closely. If it doesn't have LGTM, then removes approve.
// If LGTM is there, then print a warning to let people know that this is not going to work forever.
// No warning yet.
type ApproveFollowsLGTM struct {
}

func init() {
	a := &ApproveFollowsLGTM{}
	RegisterMungerOrDie(a)
}

// Name is the name usable in --pr-mungers
func (ApproveFollowsLGTM) Name() string { return "approve-lgtm" }

// RequiredFeatures is a slice of 'features' that must be provided
func (ApproveFollowsLGTM) RequiredFeatures() []string { return []string{} }

// Initialize will initialize the munger
func (ApproveFollowsLGTM) Initialize(config *github.Config, features *features.Features) error {
	return nil
}

// EachLoop is called at the start of every munge loop
func (ApproveFollowsLGTM) EachLoop() error { return nil }

// AddFlags will add any request flags to the cobra `cmd`
func (ApproveFollowsLGTM) AddFlags(cmd *cobra.Command, config *github.Config) {
}

// Munge is the workhorse the will actually make updates to the PR
func (ApproveFollowsLGTM) Munge(obj *github.MungeObject) {
	if !obj.IsPR() {
		return
	}

	events, err := obj.GetEvents()
	if err != nil {
		return
	}

	// Find last lgtm or approved label add/remove
	lgtmOrApproved := event.Or{
		event.LabelName("lgtm"),
		event.LabelName("approved"),
	}

	lastLabel := event.FilterEvents(events, lgtmOrApproved).GetLast()
	// There is no such event, or the Event field is missing
	if lastLabel == nil || lastLabel.Event == nil {
		return
	}

	if *lastLabel.Event == "labeled" {
		obj.AddLabel("approved")
		obj.AddLabel("lgtm")
	} else if *lastLabel.Event == "unlabeled" {
		obj.RemoveLabel("approved")
		obj.RemoveLabel("lgtm")
	} else {
		glog.Fatal("Received unexpected event: ", *lastLabel.Event)
	}
}

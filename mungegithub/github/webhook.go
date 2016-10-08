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

package github

import (
	"net/http"

	"github.com/golang/glog"
	"github.com/google/go-github/github"
)

// WebHook listen for events and list changed issues asynchronously
type WebHook struct {
	GithubKey string
	ListenURL string
	States    *StateChange
}

func (webhook WebHook) findCommitFromEvent(r *http.Request) *string {
	payload, err := github.ValidatePayload(r, []byte(webhook.GithubKey))
	if err != nil {
		glog.Error(err)
	}
	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		glog.Error(err)
	}

	switch event := event.(type) {
	case github.StatusEvent:
		if event.Commit != nil {
			return event.Commit.SHA
		}
	}
	return nil
}

// ServeHTTP receives the webhook, and process it
func (webhook WebHook) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	commit := webhook.findCommitFromEvent(r)
	if commit == nil {
		return
	}
	webhook.States.Change(*commit)
}

// Initialize the webhook. Returns false if it's missing parameters/shouldn't run.
func (webhook *WebHook) Initialize() {
	webhook.States = NewStateChange()
}

// Listen receives webhooks.
func (webhook *WebHook) Listen() {
	http.Handle("/", webhook)

	glog.Fatal(http.ListenAndServe(webhook.ListenURL, nil))
}

// PopIssues returns the list of issues that changed since last time it was called
func (webhook *WebHook) PopIssues() []int {
	if webhook.States == nil {
		return []int{}
	}
	return webhook.States.PopChanged()
}

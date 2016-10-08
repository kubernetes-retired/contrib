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
	Status    *StatusChange
}

// ServeHTTP receives the webhook, and process it
func (webhook WebHook) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	payload, err := github.ValidatePayload(r, []byte(webhook.GithubKey))
	if err != nil {
		glog.Error(err)
		http.Error(w, "Failed to validate payload", 400)
		return
	}
	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		glog.Error(err)
		http.Error(w, "Failed to parse event", 400)
		return

	}

	switch event := event.(type) {
	case *github.StatusEvent:
		if event.Commit != nil && event.Commit.SHA != nil {
			webhook.Status.CommitStatusChanged(*event.Commit.SHA)
		}
	case *github.PushEvent:
		if event.Ref != nil && event.Before != nil && event.Head != nil {
			webhook.Status.UpdateRefHead(*event.Ref, *event.Before, *event.Head)
		}
	}
}

// Initialize the webhook. Returns false if it's missing parameters/shouldn't run.
func (webhook *WebHook) Initialize() {
	webhook.Status = NewStatusChange()

	go webhook.Listen()
}

// Listen receives webhooks.
func (webhook *WebHook) Listen() {
	mux := http.NewServeMux()
	mux.Handle("/", webhook)

	glog.Fatal(http.ListenAndServe(webhook.ListenURL, mux))
}

// CreateRefIfNeeded will add the pull-request ref and update the latest commit
func (webhook *WebHook) CreateRefIfNeeded(ref string, head string, id int) {
	if webhook.Status.SetPullRequestRef(id, ref) {
		// Update the ref with existing commit.
		webhook.Status.UpdateRefHead(ref, "", head)
	}
}

// PopIssues returns the list of issues that changed since last time it was called
func (webhook *WebHook) PopIssues() []int {
	if webhook.Status == nil {
		return []int{}
	}
	return webhook.Status.PopChangedPullRequests()
}

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

package main

import (
	"testing"
	"time"

	"github.com/google/go-github/github"
)

type FakeClient struct {
	Issues      []github.Issue
	IssueEvents []github.IssueEvent
}

func (client FakeClient) FetchIssues(latest time.Time, c chan github.Issue) error {
	for _, issue := range client.Issues {
		c <- issue
	}
	close(c)
	return nil
}

func (client FakeClient) FetchIssueEvents(latest *int, c chan github.IssueEvent) error {
	for _, event := range client.IssueEvents {
		c <- event
	}
	close(c)
	return nil
}

func createIssueEvent(id int) github.IssueEvent {
	return github.IssueEvent{ID: &id}
}

func TestWasIdFound(t *testing.T) {
	tests := []struct {
		events []github.IssueEvent
		id     int
		isIn   bool
	}{
		{
			[]github.IssueEvent{},
			1,
			false,
		},
		{
			[]github.IssueEvent{
				createIssueEvent(1),
			},
			1,
			true,
		},
		{
			[]github.IssueEvent{
				createIssueEvent(0),
				createIssueEvent(2),
			},
			1,
			false,
		},
		{
			[]github.IssueEvent{
				createIssueEvent(2),
				createIssueEvent(3),
				createIssueEvent(1),
			},
			1,
			true,
		},
	}

	for _, test := range tests {
		found := wasIdFound(test.events, test.id)
		if found != test.isIn {
			if found {
				t.Error(test.id, "was found in", test.events, "but shouldn't")
			} else {
				t.Error(test.id, "wasn't found in", test.events)
			}
		}
	}
}

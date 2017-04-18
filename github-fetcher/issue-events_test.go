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
	"reflect"
	"testing"
	"time"

	"github.com/google/go-github/github"
)

func TestFindLatestUpdate(t *testing.T) {
	config := SQLiteConfig{":memory:"}
	tests := []struct {
		events         []IssueEvent
		expectedLatest int
	}{
		// If we don't have any issue, return 1900/1/1 0:0:0 UTC
		{
			[]IssueEvent{},
			0,
		},
		{
			[]IssueEvent{
				{ID: 2, EventCreatedAt: time.Date(1999, 1, 1, 0, 0, 0, 0, time.UTC)},
				{ID: 7, EventCreatedAt: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)},
				{ID: 1, EventCreatedAt: time.Date(1998, 1, 1, 0, 0, 0, 0, time.UTC)},
			},
			7,
		},
	}

	for _, test := range tests {
		db, err := config.CreateDatabase()
		if err != nil {
			t.Fatal("Failed to create database:", err)
		}

		tx := db.Begin()
		for _, event := range test.events {
			tx.Create(&event)
		}
		tx.Commit()

		actualLatest := findLatestEvent(db)
		if actualLatest == nil {
			if test.expectedLatest != 0 {
				t.Error("Didn't found event, expected:", test.expectedLatest)
			}
		} else if *actualLatest != test.expectedLatest {
			t.Error("Actual:", actualLatest,
				"doesn't match expected:", test.expectedLatest)
		}
	}
}

func TestUpdateEvents(t *testing.T) {
	config := SQLiteConfig{":memory:"}

	tests := []struct {
		before []IssueEvent
		new    []github.IssueEvent
		after  []IssueEvent
	}{
		// No new issues
		{
			before: []IssueEvent{
				*makeIssueEvent(1, 2, "", "Event", "", "",
					time.Date(2000, time.January, 1, 19, 30, 0, 0, time.UTC)),
			},
			new: []github.IssueEvent{},
			after: []IssueEvent{
				*makeIssueEvent(1, 2, "", "Event", "", "",
					time.Date(2000, time.January, 1, 19, 30, 0, 0, time.UTC)),
			},
		},
		// New issues
		{
			before: []IssueEvent{
				*makeIssueEvent(1, 2, "", "Event", "", "",
					time.Date(2000, time.January, 1, 19, 30, 0, 0, time.UTC)),
			},
			new: []github.IssueEvent{
				*makeGithubIssueEvent(2, 2, "Label", "Event", "Assignee", "Actor",
					time.Date(2001, time.January, 1, 19, 30, 0, 0, time.UTC)),
			},
			after: []IssueEvent{
				*makeIssueEvent(1, 2, "", "Event", "", "",
					time.Date(2000, time.January, 1, 19, 30, 0, 0, time.UTC)),
				*makeIssueEvent(2, 2, "Label", "Event", "Assignee", "Actor",
					time.Date(2001, time.January, 1, 19, 30, 0, 0, time.UTC)),
			},
		},
		// New issues + already existing (doesn't update)
		{
			before: []IssueEvent{
				*makeIssueEvent(1, 2, "", "Event", "", "",
					time.Date(2000, time.January, 1, 19, 30, 0, 0, time.UTC)),
				*makeIssueEvent(2, 2, "Label", "Event", "Assignee", "Actor",
					time.Date(2001, time.January, 1, 19, 30, 0, 0, time.UTC)),
			},
			new: []github.IssueEvent{
				*makeGithubIssueEvent(1, 2, "", "EventNameChanged", "", "",
					time.Date(2000, time.January, 1, 19, 30, 0, 0, time.UTC)),
				*makeGithubIssueEvent(3, 2, "Label", "Event", "Assignee", "",
					time.Date(2002, time.January, 1, 19, 30, 0, 0, time.UTC)),
			},
			after: []IssueEvent{
				*makeIssueEvent(1, 2, "", "Event", "", "",
					time.Date(2000, time.January, 1, 19, 30, 0, 0, time.UTC)),
				*makeIssueEvent(2, 2, "Label", "Event", "Assignee", "Actor",
					time.Date(2001, time.January, 1, 19, 30, 0, 0, time.UTC)),
				*makeIssueEvent(3, 2, "Label", "Event", "Assignee", "",
					time.Date(2002, time.January, 1, 19, 30, 0, 0, time.UTC)),
			},
		},
		// Return invalid event
		{
			before: []IssueEvent{
				*makeIssueEvent(1, 2, "", "Event", "", "",
					time.Date(2000, time.January, 1, 19, 30, 0, 0, time.UTC)),
			},
			new: []github.IssueEvent{{}},
			after: []IssueEvent{
				*makeIssueEvent(1, 2, "", "Event", "", "",
					time.Date(2000, time.January, 1, 19, 30, 0, 0, time.UTC)),
			},
		},
	}

	for _, test := range tests {
		db, err := config.CreateDatabase()
		if err != nil {
			t.Fatal("Failed to create database:", err)
		}

		for _, event := range test.before {
			db.Create(&event)
		}

		if err := UpdateIssueEvents(db, FakeClient{IssueEvents: test.new}); err != nil {
			t.Error("UpdateIssues failed:", err)
			continue
		}
		var issues []IssueEvent
		if err := db.Order("ID").Find(&issues).Error; err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(issues, test.after) {
			t.Error("Actual:", issues,
				"doesn't match expected:", test.after)
		}
	}
}

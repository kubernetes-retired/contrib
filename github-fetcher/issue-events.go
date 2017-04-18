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
	"github.com/google/go-github/github"
	"github.com/jinzhu/gorm"
)

func findLatestEvent(db *gorm.DB) *int {
	var latestEvent IssueEvent

	db.Select("id, event_created_at").Order("event_created_at desc").First(&latestEvent)
	if latestEvent.EventCreatedAt.IsZero() {
		return nil
	}

	return &latestEvent.ID
}

// UpdateIssueEvents fetches all events until we find the most recent we
// have in db, and saves everything in database
func UpdateIssueEvents(db *gorm.DB, client ClientInterface) error {
	latest := findLatestEvent(db)
	c := make(chan github.IssueEvent, 500)

	go client.FetchIssueEvents(latest, c)

	for event := range c {
		eventOrm := NewIssueEvent(&event)
		if eventOrm == nil {
			continue
		}
		db.Create(eventOrm)
	}

	return nil
}

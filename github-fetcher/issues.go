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
	"time"

	"github.com/google/go-github/github"
	"github.com/jinzhu/gorm"
)

func findLatestIssueUpdate(db *gorm.DB) time.Time {
	var issue Issue
	issue.IssueUpdatedAt = time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)

	db.Select("issue_updated_at").Order("issue_updated_at desc").First(&issue)

	return issue.IssueUpdatedAt
}

// UpdateIssues downloads new issues and saves in database
func UpdateIssues(db *gorm.DB, client ClientInterface) error {
	latest := findLatestIssueUpdate(db)
	c := make(chan github.Issue, 200)

	go client.FetchIssues(latest, c)

	for issue := range c {
		issueOrm := NewIssue(&issue)
		if issueOrm == nil {
			continue
		}
		if db.Create(issueOrm).Error != nil {
			// If we can't create, let's try update
			db.Save(issueOrm)
		}
		// Issue is updated, find if we have new comments
		UpdateComments(issueOrm.ID, issueOrm.IsPR, db, client)
	}

	return nil
}

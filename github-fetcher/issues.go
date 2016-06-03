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

	newIssues, err := client.FetchIssues(latest)
	if err != nil {
		return err
	}
	tx := db.Begin()
	for _, issue := range newIssues {
		issueOrm := NewIssue(&issue)
		if tx.Create(issueOrm).Error != nil {
			// If we can't create, let's try update
			tx.Save(issueOrm)
		}
	}
	tx.Commit()

	return nil
}

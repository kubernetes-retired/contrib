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

func findLatestCommentUpdate(issueId int, db *gorm.DB) time.Time {
	var comment Comment
	comment.CommentUpdatedAt = time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)

	db.Select("comment_updated_at").Where(&Comment{IssueID: issueId}).Order("comment_updated_at desc").First(&comment)

	return comment.CommentUpdatedAt
}

func updateIssueComments(issueId int, latest time.Time, db *gorm.DB, client ClientInterface) {
	c := make(chan github.IssueComment, 200)

	go client.FetchIssueComments(issueId, latest, c)

	for comment := range c {
		commentOrm := NewIssueComment(issueId, &comment)
		if db.Create(commentOrm).Error != nil {
			// If we can't create, let's try update
			db.Save(commentOrm)
		}
	}
}

func updatePullComments(issueId int, latest time.Time, db *gorm.DB, client ClientInterface) {
	c := make(chan github.PullRequestComment, 200)

	go client.FetchPullComments(issueId, latest, c)

	for comment := range c {
		commentOrm := NewPullComment(issueId, &comment)
		if db.Create(commentOrm).Error != nil {
			// If we can't create, let's try update
			db.Save(commentOrm)
		}
	}
}

// UpdateComments downloads issue and pull-request comments and save in DB
func UpdateComments(issueId int, pullRequest bool, db *gorm.DB, client ClientInterface) error {
	latest := findLatestCommentUpdate(issueId, db)

	updateIssueComments(issueId, latest, db, client)
	if pullRequest {
		updatePullComments(issueId, latest, db, client)
	}

	return nil
}

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

func TestFindLatestCommentUpdate(t *testing.T) {
	config := SQLiteConfig{":memory:"}
	tests := []struct {
		comments       []Comment
		issueId        int
		expectedLatest time.Time
	}{
		// If we don't have any comment, return 1900/1/1 0:0:0 UTC
		{
			[]Comment{},
			1,
			time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		// There are no comment for this issue, return the min date
		{
			[]Comment{
				{IssueID: 1, CommentUpdatedAt: time.Date(1999, 1, 1, 0, 0, 0, 0, time.UTC)},
				{IssueID: 1, CommentUpdatedAt: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)},
			},
			2,
			time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		// Only pick selected issue
		{
			[]Comment{
				{IssueID: 1, CommentUpdatedAt: time.Date(1999, 1, 1, 0, 0, 0, 0, time.UTC)},
				{IssueID: 1, CommentUpdatedAt: time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)},
				{IssueID: 1, CommentUpdatedAt: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)},
				{IssueID: 2, CommentUpdatedAt: time.Date(2002, 1, 1, 0, 0, 0, 0, time.UTC)},
			},
			1,
			time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		// Can pick pull-request comments
		{
			[]Comment{
				{IssueID: 1, PullRequest: true, CommentUpdatedAt: time.Date(1999, 1, 1, 0, 0, 0, 0, time.UTC)},
				{IssueID: 1, PullRequest: false, CommentUpdatedAt: time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)},
				{IssueID: 1, PullRequest: true, CommentUpdatedAt: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)},
			},
			1,
			time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		// Can pick issue comments
		{
			[]Comment{
				{IssueID: 1, PullRequest: false, CommentUpdatedAt: time.Date(1999, 1, 1, 0, 0, 0, 0, time.UTC)},
				{IssueID: 1, PullRequest: true, CommentUpdatedAt: time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)},
				{IssueID: 1, PullRequest: false, CommentUpdatedAt: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)},
			},
			1,
			time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, test := range tests {
		db, err := config.CreateDatabase()
		if err != nil {
			t.Fatal("Failed to create database:", err)
		}

		for _, comment := range test.comments {
			db.Create(&comment)
		}

		actualLatest := findLatestCommentUpdate(test.issueId, db)
		if actualLatest != test.expectedLatest {
			t.Error("Actual:", actualLatest,
				"doesn't match expected:", test.expectedLatest)
		}
	}
}

func TestUpdateComments(t *testing.T) {
	config := SQLiteConfig{":memory:"}

	tests := []struct {
		before           []Comment
		newIssueComments map[int][]github.IssueComment
		newPullComments  map[int][]github.PullRequestComment
		after            []Comment
		updateId         int
		isPullRequest    bool
	}{
		// No new comments
		{
			before: []Comment{
				*makeComment(12, 1, "Body", "Login",
					time.Date(2000, time.January, 1, 19, 30, 0, 0, time.UTC),
					time.Date(2001, time.January, 1, 19, 30, 0, 0, time.UTC), true),
			},
			newIssueComments: map[int][]github.IssueComment{},
			newPullComments:  map[int][]github.PullRequestComment{},
			after: []Comment{
				*makeComment(12, 1, "Body", "Login",
					time.Date(2000, time.January, 1, 19, 30, 0, 0, time.UTC),
					time.Date(2001, time.January, 1, 19, 30, 0, 0, time.UTC), true),
			},
			updateId:      1,
			isPullRequest: true,
		},
		// New comments, include PR
		{
			before: []Comment{
				*makeComment(12, 1, "Body", "Login",
					time.Date(2000, time.January, 1, 19, 30, 0, 0, time.UTC),
					time.Date(2001, time.January, 1, 19, 30, 0, 0, time.UTC), true),
			},
			newIssueComments: map[int][]github.IssueComment{
				3: {
					*makeGithubIssueComment(2, "IssueBody", "SomeLogin",
						time.Date(2000, time.January, 1, 19, 30, 0, 0, time.UTC),
						time.Date(2001, time.January, 1, 19, 30, 0, 0, time.UTC)),
					*makeGithubIssueComment(3, "AnotherBody", "AnotherLogin",
						time.Date(2000, time.January, 1, 19, 30, 0, 0, time.UTC),
						time.Date(2001, time.January, 1, 19, 30, 0, 0, time.UTC)),
				},
			},
			newPullComments: map[int][]github.PullRequestComment{
				2: {
					*makeGithubPullComment(4, "Body", "Login",
						time.Date(2000, time.January, 1, 19, 30, 0, 0, time.UTC),
						time.Date(2001, time.February, 1, 19, 30, 0, 0, time.UTC)),
				},
				3: {
					*makeGithubPullComment(5, "SecondBody", "OtherLogin",
						time.Date(2000, time.December, 1, 19, 30, 0, 0, time.UTC),
						time.Date(2001, time.November, 1, 19, 30, 0, 0, time.UTC)),
				},
			},
			after: []Comment{
				*makeComment(12, 1, "Body", "Login",
					time.Date(2000, time.January, 1, 19, 30, 0, 0, time.UTC),
					time.Date(2001, time.January, 1, 19, 30, 0, 0, time.UTC), true),
				*makeComment(3, 2, "IssueBody", "SomeLogin",
					time.Date(2000, time.January, 1, 19, 30, 0, 0, time.UTC),
					time.Date(2001, time.January, 1, 19, 30, 0, 0, time.UTC), false),
				*makeComment(3, 3, "AnotherBody", "AnotherLogin",
					time.Date(2000, time.January, 1, 19, 30, 0, 0, time.UTC),
					time.Date(2001, time.January, 1, 19, 30, 0, 0, time.UTC), false),
				*makeComment(3, 5, "SecondBody", "OtherLogin",
					time.Date(2000, time.December, 1, 19, 30, 0, 0, time.UTC),
					time.Date(2001, time.November, 1, 19, 30, 0, 0, time.UTC), true),
			},
			updateId:      3,
			isPullRequest: true,
		},
		// Only interesting new comment is in PR, and we don't take PR
		{
			before: []Comment{
				*makeComment(12, 1, "Body", "Login",
					time.Date(2000, time.January, 1, 19, 30, 0, 0, time.UTC),
					time.Date(2001, time.January, 1, 19, 30, 0, 0, time.UTC), true),
			},
			newIssueComments: map[int][]github.IssueComment{
				3: {
					*makeGithubIssueComment(2, "IssueBody", "SomeLogin",
						time.Date(2000, time.January, 1, 19, 30, 0, 0, time.UTC),
						time.Date(2001, time.January, 1, 19, 30, 0, 0, time.UTC)),
					*makeGithubIssueComment(3, "AnotherBody", "AnotherLogin",
						time.Date(2000, time.January, 1, 19, 30, 0, 0, time.UTC),
						time.Date(2001, time.January, 1, 19, 30, 0, 0, time.UTC)),
				},
			},
			newPullComments: map[int][]github.PullRequestComment{
				2: {
					*makeGithubPullComment(4, "Body", "Login",
						time.Date(2000, time.January, 1, 19, 30, 0, 0, time.UTC),
						time.Date(2001, time.February, 1, 19, 30, 0, 0, time.UTC)),
				},
				3: {
					*makeGithubPullComment(5, "SecondBody", "OtherLogin",
						time.Date(2000, time.December, 1, 19, 30, 0, 0, time.UTC),
						time.Date(2001, time.November, 1, 19, 30, 0, 0, time.UTC)),
				},
			},
			after: []Comment{
				*makeComment(12, 1, "Body", "Login",
					time.Date(2000, time.January, 1, 19, 30, 0, 0, time.UTC),
					time.Date(2001, time.January, 1, 19, 30, 0, 0, time.UTC), true),
			},
			updateId:      2,
			isPullRequest: false,
		},
		// New modified comment
		{
			before: []Comment{
				*makeComment(12, 1, "Body", "Login",
					time.Date(2000, time.January, 1, 19, 30, 0, 0, time.UTC),
					time.Date(2001, time.January, 1, 19, 30, 0, 0, time.UTC), true),
			},
			newIssueComments: map[int][]github.IssueComment{},
			newPullComments: map[int][]github.PullRequestComment{
				12: {
					*makeGithubPullComment(1, "IssueBody", "SomeLogin",
						time.Date(2000, time.January, 1, 19, 30, 0, 0, time.UTC),
						time.Date(2001, time.January, 1, 19, 30, 0, 0, time.UTC)),
				},
			},
			after: []Comment{
				*makeComment(12, 1, "IssueBody", "SomeLogin",
					time.Date(2000, time.January, 1, 19, 30, 0, 0, time.UTC),
					time.Date(2001, time.January, 1, 19, 30, 0, 0, time.UTC), true),
			},
			updateId:      12,
			isPullRequest: true,
		},
		// Invalid new comments
		{
			before: []Comment{
				*makeComment(1, 1, "Body", "Login",
					time.Date(2000, time.January, 1, 19, 30, 0, 0, time.UTC),
					time.Date(2001, time.January, 1, 19, 30, 0, 0, time.UTC), true),
			},
			newIssueComments: map[int][]github.IssueComment{1: {github.IssueComment{}}},
			newPullComments:  map[int][]github.PullRequestComment{1: {github.PullRequestComment{}}},
			after: []Comment{
				*makeComment(1, 1, "Body", "Login",
					time.Date(2000, time.January, 1, 19, 30, 0, 0, time.UTC),
					time.Date(2001, time.January, 1, 19, 30, 0, 0, time.UTC), true),
			},
			updateId:      1,
			isPullRequest: true,
		},
	}

	for _, test := range tests {
		db, err := config.CreateDatabase()
		if err != nil {
			t.Fatal("Failed to create database:", err)
		}

		for _, comment := range test.before {
			db.Create(&comment)
		}

		client := FakeClient{PullComments: test.newPullComments, IssueComments: test.newIssueComments}
		if err := UpdateComments(test.updateId, test.isPullRequest, db, client); err != nil {
			t.Error("UpdateComments failed:", err)
			continue
		}
		var comments []Comment
		if err := db.Order("ID").Find(&comments).Error; err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(comments, test.after) {
			t.Error("Actual:", comments,
				"doesn't match expected:", test.after)
		}
	}
}

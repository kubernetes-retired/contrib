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

	"github.com/golang/glog"
	"github.com/google/go-github/github"
)

// Issue is a pull-request or issue. It's format fits into the ORM
type Issue struct {
	ID             int
	Labels         []Label
	Title          string
	Body           string
	User           string
	Assignee       *string
	State          string
	Comments       int
	IsPR           bool
	IssueClosedAt  *time.Time
	IssueCreatedAt time.Time
	IssueUpdatedAt time.Time
}

// NewIssue creates a new (orm) Issue from a github Issue
func NewIssue(gIssue *github.Issue) *Issue {
	if gIssue.Number == nil ||
		gIssue.Title == nil ||
		gIssue.User == nil ||
		gIssue.User.Login == nil ||
		gIssue.State == nil ||
		gIssue.Comments == nil ||
		gIssue.CreatedAt == nil ||
		gIssue.UpdatedAt == nil {
		glog.Error("Issue is missing mandatory field:", gIssue)
		return nil
	}
	var closedAt *time.Time
	if gIssue.ClosedAt != nil {
		closedAt = gIssue.ClosedAt
	}
	var assignee *string
	if gIssue.Assignee != nil {
		assignee = gIssue.Assignee.Login
	}
	var body string
	if gIssue.Body != nil {
		body = *gIssue.Body
	}
	isPR := (gIssue.PullRequestLinks != nil && gIssue.PullRequestLinks.URL != nil)

	return &Issue{
		*gIssue.Number,
		newLabels(*gIssue.Number, gIssue.Labels),
		*gIssue.Title,
		body,
		*gIssue.User.Login,
		assignee,
		*gIssue.State,
		*gIssue.Comments,
		isPR,
		closedAt,
		*gIssue.CreatedAt,
		*gIssue.UpdatedAt,
	}
}

// IssueEvent is an event associated to a specific issued.
// It's format fits into the ORM
type IssueEvent struct {
	ID             int
	Label          *string
	Event          string
	EventCreatedAt time.Time
	IssueId        int
	Assignee       *string
	Actor          *string
}

// NewIssueEvent creates a new (orm) Issue from a github Issue
func NewIssueEvent(gIssueEvent *github.IssueEvent) *IssueEvent {
	if gIssueEvent.ID == nil ||
		gIssueEvent.Event == nil ||
		gIssueEvent.CreatedAt == nil ||
		gIssueEvent.Issue == nil ||
		gIssueEvent.Issue.Number == nil {
		glog.Error("IssueEvent is missing mandatory field:", gIssueEvent)
		return nil
	}

	var label *string
	if gIssueEvent.Label != nil {
		label = gIssueEvent.Label.Name
	}
	var assignee *string
	if gIssueEvent.Assignee != nil {
		assignee = gIssueEvent.Assignee.Login
	}
	var actor *string
	if gIssueEvent.Actor != nil {
		actor = gIssueEvent.Actor.Login
	}

	return &IssueEvent{
		*gIssueEvent.ID,
		label,
		*gIssueEvent.Event,
		*gIssueEvent.CreatedAt,
		*gIssueEvent.Issue.Number,
		assignee,
		actor,
	}
}

// Label is a tag on an Issue. It's format fits into the ORM.
type Label struct {
	IssueID int
	Name    string
}

// newLabels creates a new Label for each label in the issue
func newLabels(issueId int, gLabels []github.Label) []Label {
	labels := []Label{}

	for _, label := range gLabels {
		if label.Name == nil {
			glog.Error("Label is missing name field")
			continue
		}
		labels = append(labels, Label{issueId, *label.Name})
	}

	return labels
}

// Comment is either a pull-request comment or an issue comment.
type Comment struct {
	ID               int
	IssueID          int
	Body             string
	User             string
	CommentCreatedAt time.Time
	CommentUpdatedAt time.Time
	PullRequest      bool
}

// NewIssueComment creates a Comment from a github.IssueComment
func NewIssueComment(issueId int, gComment *github.IssueComment) *Comment {
	if gComment.ID == nil ||
		gComment.Body == nil ||
		gComment.CreatedAt == nil ||
		gComment.UpdatedAt == nil {
		glog.Error("IssueComment is missing mandatory field:", gComment)
		return nil
	}
	var login string
	if gComment.User != nil && gComment.User.Login != nil {
		login = *gComment.User.Login
	}

	return &Comment{
		*gComment.ID,
		issueId,
		*gComment.Body,
		login,
		*gComment.CreatedAt,
		*gComment.UpdatedAt,
		false,
	}
}

// NewPullComment creates a Comment from a github.PullRequestComment
func NewPullComment(issueId int, gComment *github.PullRequestComment) *Comment {
	if gComment.ID == nil ||
		gComment.Body == nil ||
		gComment.CreatedAt == nil ||
		gComment.UpdatedAt == nil {
		glog.Error("PullComment is missing mandatory field:", gComment)
		return nil
	}
	var login string
	if gComment.User != nil && gComment.User.Login != nil {
		login = *gComment.User.Login
	}
	return &Comment{
		*gComment.ID,
		issueId,
		*gComment.Body,
		login,
		*gComment.CreatedAt,
		*gComment.UpdatedAt,
		true,
	}
}

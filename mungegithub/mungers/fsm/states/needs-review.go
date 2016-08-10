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

package states

import (
	"k8s.io/contrib/mungegithub/github"
	"k8s.io/contrib/mungegithub/mungers/matchers/comment"
)

// NeedsReview is the state when the ball is in the reviewer's court.
type NeedsReview struct{}

const (
	lgtmLabel = "lgtm"
)

// Initialize can be used to set up initialization.
func (nr *NeedsReview) Initialize(obj *github.MungeObject) error {
	return nil
}

// Process does the necessary processing to compute whether to stay in
// this state, or proceed to the next.
func (nr *NeedsReview) Process(obj *github.MungeObject) (State, error) {
	var result bool
	var err error
	if nr.checkLGTM(obj) {
		return &End{}, nil
	}

	result, err = nr.assigneeActionNeeded(obj)
	if err != nil {
		return &End{}, err
	}

	if !result {
		obj.RemoveLabelIfPresent(labelNeedsReview)
		return &ChangesNeeded{}, nil
	}

	obj.AddLabelIfAbsent(labelNeedsReview)
	return &End{}, nil
}

func (nr *NeedsReview) checkLGTM(obj *github.MungeObject) bool {
	return obj.HasLabel(lgtmLabel)
}

func (nr *NeedsReview) assigneeActionNeeded(obj *github.MungeObject) (bool, error) {
	comments, err := obj.ListComments()
	if err != nil {
		return false, err
	}

	lastAuthorComment := comment.FilterComments(comments, comment.AuthorLogin(*obj.Issue.User.Login)).GetLast()

	// TODO: fetch for each assignee.
	lastReviewerComment := comment.FilterComments(comments, comment.AuthorLogin(*obj.Issue.Assignee.Login)).GetLast()
	return lastAuthorComment.CreatedAt.After(*lastReviewerComment.CreatedAt), nil
}

// Name is the name of the state machine's state.
func (nr *NeedsReview) Name() string {
	return "NeedsReview"
}

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

package matchers

import "github.com/google/go-github/github"

// True is a matcher that is always true
type True struct{}

var _ Matcher = True{}

// MatchEvent returns true no matter what
func (t True) MatchEvent(event *github.IssueEvent) bool {
	return true
}

// MatchComment returns true no matter what
func (t True) MatchComment(comment *github.IssueComment) bool {
	return true
}

// MatchReviewComment returns true no matter what
func (t True) MatchReviewComment(review *github.PullRequestComment) bool {
	return true
}

// False is a matcher that is always false
type False struct{}

var _ Matcher = False{}

// MatchEvent returns false no matter what
func (t False) MatchEvent(event *github.IssueEvent) bool {
	return false
}

// MatchComment returns false no matter what
func (t False) MatchComment(comment *github.IssueComment) bool {
	return false
}

// MatchReviewComment returns false no matter what
func (t False) MatchReviewComment(review *github.PullRequestComment) bool {
	return false
}

// And makes sure that each match in the list matches (true if empty)
type AndMatcher []Matcher

var _ Matcher = AndMatcher{}

// MatchEvent returns true if all the matchers in the list matche
func (a AndMatcher) MatchEvent(event *github.IssueEvent) bool {
	for _, matcher := range a {
		if !matcher.MatchEvent(event) {
			return false
		}
	}
	return true
}

// MatchComment returns true if all the matchers in the list matche
func (a AndMatcher) MatchComment(comment *github.IssueComment) bool {
	for _, matcher := range a {
		if !matcher.MatchComment(comment) {
			return false
		}
	}
	return true
}

// MatchReviewComment returns true if all the matchers in the list matche
func (a AndMatcher) MatchReviewComment(review *github.PullRequestComment) bool {
	for _, matcher := range a {
		if !matcher.MatchReviewComment(review) {
			return false
		}
	}
	return true
}

func And(matchers ...Matcher) AndMatcher {
	and := AndMatcher{}
	for _, matcher := range matchers {
		and = append(and, matcher)
	}
	return and
}

// OrMatcher makes sure that at least one element in the list matches (false if empty)
type OrMatcher []Matcher

var _ Matcher = OrMatcher{}

// MatchEvent returns true if one of the matcher in the list matches
func (o OrMatcher) MatchEvent(event *github.IssueEvent) bool {
	for _, matcher := range o {
		if matcher.MatchEvent(event) {
			return true
		}
	}
	return false
}

// MatchComment returns true if one of the matcher in the list matches
func (o OrMatcher) MatchComment(comment *github.IssueComment) bool {
	for _, matcher := range o {
		if matcher.MatchComment(comment) {
			return true
		}
	}
	return false
}

// MatchReviewComment returns true no matter what
func (o OrMatcher) MatchReviewComment(review *github.PullRequestComment) bool {
	for _, matcher := range o {
		if matcher.MatchReviewComment(review) {
			return true
		}
	}
	return false
}

func Or(matchers ...Matcher) OrMatcher {
	or := OrMatcher{}
	for _, matcher := range matchers {
		or = append(or, matcher)
	}
	return or
}

// Not reverses the effect of the matcher
type Not struct {
	Matcher Matcher
}

var _ Matcher = Not{}

// MatchEvent returns true if the matcher would return false, and vice-versa
func (n Not) MatchEvent(event *github.IssueEvent) bool {
	return !n.Matcher.MatchEvent(event)
}

// MatchComment returns true if the matcher would return false, and vice-versa
func (n Not) MatchComment(comment *github.IssueComment) bool {
	return !n.Matcher.MatchComment(comment)
}

// MatchReviewComment returns true if the matcher would return false, and vice-versa
func (n Not) MatchReviewComment(review *github.PullRequestComment) bool {
	return !n.Matcher.MatchReviewComment(review)
}

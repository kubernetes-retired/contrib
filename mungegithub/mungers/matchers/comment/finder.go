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

package comment

import "github.com/google/go-github/github"

// Filter let's you retrieve your filtered comments
type Filter struct {
	comments []*github.IssueComment
	matcher  Matcher
}

// Iterator over the list of filtered comments
type Iterator struct {
	ascending bool
	current   int
	filter    *Filter
}

func (it *Iterator) increment() int {
	if it.ascending {
		return 1
	}
	return -1
}

// Next looks for the next item. Returns true if there is one, false otherwise.
func (it *Iterator) Next() bool {
	for it.current += it.increment(); it.current >= 0 && it.current < len(it.filter.comments); it.current += it.increment() {
		if it.filter.filterComment(it.current) {
			return true
		}
	}
	return false
}

// Current returns the current item. You must call Next (find the item) before calling this.
func (it *Iterator) Current() *github.IssueComment {
	return it.filter.comments[it.current]
}

// It returns an iterator over the filter
func (f *Filter) It() *Iterator {
	return &Iterator{
		ascending: true,
		current:   -1,
		filter:    f,
	}
}

// ReverseIt returns a reversed iterator
func (f *Filter) ReverseIt() *Iterator {
	return &Iterator{
		ascending: false,
		current:   len(f.comments),
		filter:    f,
	}
}

// List returns the entire list of filtered comments
func (f *Filter) List() []*github.IssueComment {
	return all(f.It())
}

// ReverseList returns the entire list (reversed) of filtered comments
func (f *Filter) ReverseList() []*github.IssueComment {
	return all(f.ReverseIt())
}

func (f *Filter) filterComment(current int) bool {
	return f.matcher.Match(f.comments[current])
}

func all(it *Iterator) []*github.IssueComment {
	comments := []*github.IssueComment{}

	for it.Next() {
		comments = append(comments, it.Current())
	}

	return comments
}

// FilterComments will return a filter to get filtered items
func FilterComments(comments []*github.IssueComment, matcher Matcher) *Filter {
	return &Filter{
		comments: comments,
		matcher:  matcher,
	}
}

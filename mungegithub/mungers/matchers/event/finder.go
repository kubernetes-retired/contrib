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

package event

import "github.com/google/go-github/github"

// Filter let's you retrieve your filtered events
type Filter struct {
	events  []*github.IssueEvent
	matcher Matcher
}

// Iterator over the list of filtered events
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
	for it.current += it.increment(); it.current >= 0 && it.current < len(it.filter.events); it.current += it.increment() {
		if it.filter.filterEvent(it.current) {
			return true
		}
	}
	return false
}

// Current returns the current item. You must call Next (find the item) before calling this.
func (it *Iterator) Current() *github.IssueEvent {
	return it.filter.events[it.current]
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
		current:   len(f.events),
		filter:    f,
	}
}

// List returns the entire list of filtered events
func (f *Filter) List() []*github.IssueEvent {
	return all(f.It())
}

// ReverseList returns the entire list (reversed) of filtered events
func (f *Filter) ReverseList() []*github.IssueEvent {
	return all(f.ReverseIt())
}

func (f *Filter) filterEvent(current int) bool {
	return f.matcher.Match(f.events[current])
}

func all(it *Iterator) []*github.IssueEvent {
	events := []*github.IssueEvent{}

	for it.Next() {
		events = append(events, it.Current())
	}

	return events
}

// FilterEvents will return a filter to get filtered items
func FilterEvents(events []*github.IssueEvent, matcher Matcher) *Filter {
	return &Filter{
		events:  events,
		matcher: matcher,
	}
}

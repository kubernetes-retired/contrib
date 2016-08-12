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

import (
	"reflect"
	"testing"

	"github.com/google/go-github/github"
)

func makeEvent(event string) *github.IssueEvent {
	return &github.IssueEvent{
		Event: &event,
	}
}

func TestFilterEvents(t *testing.T) {
	events := []*github.IssueEvent{
		makeEvent("1"),
		makeEvent("2"),
		makeEvent("3"),
		makeEvent("4"),
	}
	reversedEvents := []*github.IssueEvent{
		makeEvent("4"),
		makeEvent("3"),
		makeEvent("2"),
		makeEvent("1"),
	}

	falseFilter := FilterEvents(events, False{})
	if len(falseFilter.List()) != 0 {
		t.Error("False filter shouldn't match any element")
	}

	if len(falseFilter.ReverseList()) != 0 {
		t.Error("False filter shouldn't match any element")
	}

	trueFilter := FilterEvents(events, True{})
	if !reflect.DeepEqual(trueFilter.List(), events) {
		t.Error("True filter should have kept every element")
	}
	if !reflect.DeepEqual(trueFilter.ReverseList(), reversedEvents) {
		t.Error("True filter should have kept every element")
	}

}

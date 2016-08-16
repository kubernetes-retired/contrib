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

import (
	"reflect"
	"testing"

	"github.com/google/go-github/github"
)

func TestFilterComments(t *testing.T) {
	comments := []*github.IssueComment{
		makeCommentWithBody("1"),
		makeCommentWithBody("2"),
		makeCommentWithBody("3"),
		makeCommentWithBody("4"),
	}
	reversedComments := []*github.IssueComment{
		makeCommentWithBody("4"),
		makeCommentWithBody("3"),
		makeCommentWithBody("2"),
		makeCommentWithBody("1"),
	}

	falseFilter := FilterComments(comments, False{})
	if len(falseFilter.List()) != 0 {
		t.Error("False filter shouldn't match any element")
	}

	if len(falseFilter.ReverseList()) != 0 {
		t.Error("False filter shouldn't match any element")
	}

	trueFilter := FilterComments(comments, True{})
	if !reflect.DeepEqual(trueFilter.List(), comments) {
		t.Error("True filter should have kept every element")
	}
	if !reflect.DeepEqual(trueFilter.ReverseList(), reversedComments) {
		t.Error("True filter should have kept every element")
	}

}

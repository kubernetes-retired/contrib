/*
Copyright 2016 The Kubernetes Authors.

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

package github

import (
	"reflect"
	"testing"
)

func TestStatusChange(t *testing.T) {
	sc := NewStatusChange()

	want := []int{}
	if got := sc.PopChangedPullRequests(); !reflect.DeepEqual(got, want) {
		t.Errorf("Changed() = %+v, %+v", got, want)
	}

	// Commit 123456 has changed, but we don't know what PR it is, just discard
	sc.CommitStatusChanged("123456")
	want = []int{}
	if got := sc.PopChangedPullRequests(); !reflect.DeepEqual(got, want) {
		t.Errorf("Changed() = %+v, %+v", got, want)
	}

	// Let's add a ref for this
	sc.UpdateRefHead("branch1", "", "123456")
	want = []int{}
	if got := sc.PopChangedPullRequests(); !reflect.DeepEqual(got, want) {
		t.Errorf("Changed() = %+v, %+v", got, want)
	}

	// Attach a pull-request to the ref
	// But the changes have been consumed already anyway
	sc.SetPullRequestRef(1, "branch1")
	want = []int{}
	if got := sc.PopChangedPullRequests(); !reflect.DeepEqual(got, want) {
		t.Errorf("Changed() = %+v, %+v", got, want)
	}

	// Now we have an actual change
	sc.CommitStatusChanged("123456")

	want = []int{1}
	if got := sc.PopChangedPullRequests(); !reflect.DeepEqual(got, want) {
		t.Errorf("Changed() = %+v, %+v", got, want)
	}

	// We only get it once
	want = []int{}
	if got := sc.PopChangedPullRequests(); !reflect.DeepEqual(got, want) {
		t.Errorf("Changed() = %+v, %+v", got, want)
	}

	// Let's have another change
	sc.CommitStatusChanged("123456")

	// And then the ref changes
	sc.UpdateRefHead("branch1", "123456", "654321")

	// Let's have another change to the old commit
	sc.CommitStatusChanged("123456")

	// That change is not considered
	want = []int{}
	if got := sc.PopChangedPullRequests(); !reflect.DeepEqual(got, want) {
		t.Errorf("Changed() = %+v, %+v", got, want)
	}
}

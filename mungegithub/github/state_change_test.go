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

func TestStateChange(t *testing.T) {
	sc := NewStateChange()

	want := []int{}
	if got := sc.PopChanged(); !reflect.DeepEqual(got, want) {
		t.Errorf("Changed() = %+v, %+v", got, want)
	}

	// We know that issue 1 points to commit 123456
	// It doesn't mean we had a status change
	sc.UpdateCommit(1, "123456")
	want = []int{}
	if got := sc.PopChanged(); !reflect.DeepEqual(got, want) {
		t.Errorf("Changed() = %+v, %+v", got, want)
	}

	// And it's updated to another value
	sc.UpdateCommit(1, "654321")
	want = []int{}
	if got := sc.PopChanged(); !reflect.DeepEqual(got, want) {
		t.Errorf("Changed() = %+v, %+v", got, want)
	}

	// And another pull-request/commit
	sc.UpdateCommit(2, "456789")

	// Now we have a change of status on the previous commit, should
	// be ignored.
	sc.Change("123456")

	want = []int{}
	if got := sc.PopChanged(); !reflect.DeepEqual(got, want) {
		t.Errorf("Changed() = %+v, %+v", got, want)
	}

	// Let's change an actual commit
	sc.Change("456789")
	want = []int{2}
	if got := sc.PopChanged(); !reflect.DeepEqual(got, want) {
		t.Errorf("Changed() = %+v, %+v", got, want)
	}

	// Item is removed once retrieved
	want = []int{}
	if got := sc.PopChanged(); !reflect.DeepEqual(got, want) {
		t.Errorf("Changed() = %+v, %+v", got, want)
	}

	// Add another issue with the same commit as issue 1
	sc.UpdateCommit(3, "654321")

	// Make it changed
	sc.Change("654321")

	// One commit change can change multiple issues
	want = []int{1, 3}
	if got := sc.PopChanged(); !reflect.DeepEqual(got, want) {
		t.Errorf("Changed() = %+v, %+v", got, want)
	}
}

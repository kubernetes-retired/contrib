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
	"testing"
	"time"

	"github.com/google/go-github/github"
)

func getDate(year int, month time.Month, day, hour, min, sec int) *time.Time {
	date := time.Date(year, month, day, hour, min, sec, 0, time.UTC)
	return &date
}

func TestCreationBefore(t *testing.T) {
	if CreatedBefore(
		time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC),
	).Match(nil) {
		t.Error("CreatedBefore shouldn't match nil CreatedAt")
	}
	if CreatedBefore(
		time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC),
	).Match(&github.IssueEvent{}) {
		t.Error("CreatedBefore shouldn't match nil CreatedAt")
	}
	if CreatedBefore(
		time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC),
	).Match(&github.IssueEvent{
		CreatedAt: getDate(2000, 1, 1, 12, 0, 1),
	}) {
		t.Error("CreatedBefore matches later event")
	}
	if !CreatedBefore(
		time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC),
	).Match(&github.IssueEvent{
		CreatedAt: getDate(2000, 1, 1, 11, 0, 0),
	}) {
		t.Error("CreatedBefore doesn't match earlier event")
	}
}

func TestCreationAfter(t *testing.T) {
	if CreatedAfter(
		time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC),
	).Match(nil) {
		t.Error("CreatedAfter shouldn't match nil CreatedAt")
	}
	if CreatedAfter(
		time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC),
	).Match(&github.IssueEvent{}) {
		t.Error("CreatedAfter shouldn't match nil CreatedAt")
	}
	if !CreatedAfter(
		time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC),
	).Match(&github.IssueEvent{
		CreatedAt: getDate(2000, 1, 1, 12, 0, 1),
	}) {
		t.Error("CreatedAfter matches later event")
	}
	if !CreatedAfter(
		time.Date(2000, 1, 1, 11, 0, 0, 0, time.UTC),
	).Match(&github.IssueEvent{
		CreatedAt: getDate(2000, 1, 1, 12, 0, 0),
	}) {
		t.Error("CreatedAfter doesn't match earlier event")
	}
}

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
	"sync"

	"k8s.io/kubernetes/pkg/util/sets"
)

// StateChange keeps track of issue/commit for state changes
type StateChange struct {
	issues  map[string]sets.Int // Issues for each commit
	commits map[int]string      // Commit for each issue
	changed sets.Int            // Changed issues since last call
	mutex   sync.Mutex
}

// NewStateChange creates a new state change tracker
func NewStateChange() *StateChange {
	return &StateChange{
		issues:  map[string]sets.Int{},
		commits: map[int]string{},
		changed: sets.NewInt(),
	}
}

// UpdateCommit must be called when we know how commits and issues are associated
func (s *StateChange) UpdateCommit(issue int, commit string) {
	s.mutex.Lock()
	if prevCommit, has := s.commits[issue]; has {
		s.issues[prevCommit].Delete(issue)
	}
	if _, has := s.issues[commit]; !has {
		s.issues[commit] = sets.NewInt()
	}
	s.issues[commit].Insert(issue)
	s.commits[issue] = commit
	s.mutex.Unlock()
}

// Change should be called when the status for this commit has changed
func (s *StateChange) Change(commit string) {
	s.mutex.Lock()
	s.changed.Insert(s.issues[commit].List()...)
	s.mutex.Unlock()
}

// PopChanged returns the list of issues changed since last call
func (s *StateChange) PopChanged() []int {
	s.mutex.Lock()
	tmp := s.changed.List()
	s.changed = sets.NewInt()
	s.mutex.Unlock()

	return tmp
}

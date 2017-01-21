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

// StatusChange keeps track of issue/commit for status changes
type StatusChange struct {
	refs    map[string]int         // ref-name -> Pull-Request ID
	heads   map[string]sets.String // head-sha -> ref-name
	changed sets.String            // SHA of commits whose status changed
	mutex   sync.Mutex
}

// NewStatusChange creates a new status change tracker
func NewStatusChange() *StatusChange {
	return &StatusChange{
		refs:    map[string]int{},
		heads:   map[string]sets.String{},
		changed: sets.NewString(),
	}
}

// UpdateRefHead updates the head commit for a ref
func (s *StatusChange) UpdateRefHead(ref, previous, commit string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, has := s.heads[previous]; has {
		s.heads[previous].Delete(ref)
	}
	if s.heads[commit] == nil {
		s.heads[commit] = sets.NewString()
	}
	s.heads[commit].Insert(ref)
}

// SetPullRequestRef sets the refname for a pull-request. Returns true
// if we didn't know about the ref.
func (s *StatusChange) SetPullRequestRef(id int, ref string) bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	_, has := s.refs[ref]
	s.refs[ref] = id

	return !has
}

// CommitStatusChanged must be called when the status for this commit has changed
func (s *StatusChange) CommitStatusChanged(commit string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.changed.Insert(commit)
}

// PopChangedPullRequests returns the list of issues changed since last call
func (s *StatusChange) PopChangedPullRequests() []int {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	tmp := []int{}
	for _, commit := range s.changed.List() {
		if refs, has := s.heads[commit]; has {
			for _, ref := range refs.List() {
				if id, has := s.refs[ref]; has {
					tmp = append(tmp, id)
				}
			}
		}
	}
	s.changed = sets.NewString()

	return tmp
}

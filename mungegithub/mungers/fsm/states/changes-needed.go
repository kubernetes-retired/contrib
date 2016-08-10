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

package states

import (
	"k8s.io/contrib/mungegithub/github"
)

// ChangesNeeded is the state when the ball is in the author's court.
type ChangesNeeded struct{}

var _ State = &ChangesNeeded{}

// Initialize can be used to set up initialization.
func (c *ChangesNeeded) Initialize(obj *github.MungeObject) error {
	return nil
}

// Process does the necessary processing to compute whether to stay in
// this state, or proceed to the next.
func (c *ChangesNeeded) Process(obj *github.MungeObject) (State, error) {
	// TODO: process and ping people when changes are needed.
	// There is no proceeding from this state.
	obj.AddLabelIfAbsent(labelChangesNeeded)
	return &End{}, nil
}

// Name is the name of the state machine's state.
func (c *ChangesNeeded) Name() string {
	return "ChangesNeeded"
}

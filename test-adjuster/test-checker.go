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

package main

import (
	"github.com/golang/glog"
)

// Checker is an struct that stores trigger and corresponding action.
type Checker struct {
	trigger Trigger
	action  Action
}

// NewChecker is a constructor for Checker
func NewChecker(trigger Trigger, action Action) *Checker {
	checker := &Checker{}
	checker.trigger = trigger
	checker.action = action
	return checker
}

// Run checks the condition of the trigger and runs and action if it returned true.
func (c *Checker) Run() {
	if c.trigger == nil || c.action == nil {
		glog.Error("Trying to run TestChecker without registered trigger or action")
		return
	}
	if c.trigger.CheckCondition() {
		if err := c.action.Do(); err != nil {
			glog.Errorf("Got error while trying to invoke an action: %v", err)
		}
	}
}

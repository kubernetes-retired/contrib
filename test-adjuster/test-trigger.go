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
	"k8s.io/contrib/test-utils/utils"
)

// Trigger is an interface for checking a condition.
type Trigger interface {
	CheckCondition() bool
}

// BasicUnstableTestTrigger is an trigger that fires (returns false) if given test job is unstable.
type BasicUnstableTestTrigger struct {
	jobName string
}

// NewBasicUnstableTestTrigger is a construtor for BasicUnstableTestTrigger.
func NewBasicUnstableTestTrigger(jobName string) *BasicUnstableTestTrigger {
	trigger := &BasicUnstableTestTrigger{}
	trigger.jobName = jobName
	return trigger
}

// CheckCondition checks the status of a job in GCS and returns false if any of last five runs failed.
func (t *BasicUnstableTestTrigger) CheckCondition() bool {
	glog.V(2).Infof("Checking if %v is unstable...", t.jobName)
	lastBuildNumber, err := utils.GetLastestBuildNumberFromJenkinsGoogleBucket(t.jobName)
	if err != nil {
		glog.Errorf("Failed to get last build number for %v: %v", t.jobName, err)
		return false
	}
	for i := 0; i < 5; i++ {
		stable, err := utils.CheckFinishedStatus(t.jobName, lastBuildNumber-i)
		if err != nil {
			glog.Errorf("Failed to check status for %v: %v", t.jobName, err)
			return false
		}
		if !stable {
			glog.V(2).Info("It is")
			return true
		}
	}
	glog.V(2).Info("It is not")
	return false
}

// BasicStableTestTrigger is an trigger that fires (returns false) if given test job is stable.
type BasicStableTestTrigger struct {
	jobName string
}

// NewBasicStableTestTrigger is a construtor for BasicStableTestTrigger.
func NewBasicStableTestTrigger(jobName string) *BasicStableTestTrigger {
	trigger := &BasicStableTestTrigger{}
	trigger.jobName = jobName
	return trigger
}

// CheckCondition checks the status of a job in GCS and returns false if all last five runs succeeded.
func (t *BasicStableTestTrigger) CheckCondition() bool {
	glog.V(2).Infof("Checking if %v is stable...", t.jobName)
	lastBuildNumber, err := utils.GetLastestBuildNumberFromJenkinsGoogleBucket(t.jobName)
	if err != nil {
		glog.Errorf("Failed to get last build number for %v: %v", t.jobName, err)
		return false
	}
	for i := 0; i < 5; i++ {
		stable, err := utils.CheckFinishedStatus(t.jobName, lastBuildNumber-i)
		if err != nil {
			glog.Errorf("Failed to check status for %v: %v", t.jobName, err)
			return false
		}
		if !stable {
			glog.V(2).Info("It is not")
			return false
		}
	}
	glog.V(2).Info("It is")
	return true
}

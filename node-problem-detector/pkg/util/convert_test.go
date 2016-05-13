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

package util

import (
	"testing"
	"time"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"

	"k8s.io/contrib/node-problem-detector/pkg/types"
)

func TestConvertToAPICondition(t *testing.T) {
	now := time.Now()
	condition := types.Condition{
		Type:       "TestCondition",
		Status:     true,
		Transition: now,
		Reason:     "test reason",
		Message:    "test message",
	}
	expected := api.NodeCondition{
		Type:               "TestCondition",
		Status:             api.ConditionTrue,
		LastTransitionTime: unversioned.NewTime(now),
		Reason:             "test reason",
		Message:            "test message",
	}
	apiCondition := ConvertToAPICondition(condition)
	if apiCondition != expected {
		t.Errorf("expected %+v, got %+v", expected, apiCondition)
	}
}

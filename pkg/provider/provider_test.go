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

package provider

import (
	"testing"
	"time"

	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/client/clientset_generated/release_1_4/fake"
)

func TestClusterHealthy(t *testing.T) {
	healthyCond := []v1.ComponentCondition{
		{
			Type:   "Healthy",
			Status: "True",
		},
	}
	components := &v1.ComponentStatusList{
		Items: []v1.ComponentStatus{
			{
				ObjectMeta: v1.ObjectMeta{
					Name: "c1",
				},
				Conditions: healthyCond,
			},
		},
	}
	fakeClientset := fake.NewSimpleClientset(components)
	if !clusterHealthy(fakeClientset, 1*time.Second, 10*time.Second) {
		t.Errorf("Expected cluster to be healthy, got failure")
	}
}

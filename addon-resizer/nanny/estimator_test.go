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

package nanny

import (
	"reflect"
	"testing"

	resource "k8s.io/kubernetes/pkg/api/resource"
	api "k8s.io/kubernetes/pkg/api/v1"
)

var (
	fullEstimator = LinearEstimator{
		Resources: []Resource{
			{
				Base:         resource.MustParse("0.3"),
				ExtraPerNode: resource.MustParse("1"),
				Name:         "cpu",
			},
			{
				Base:         resource.MustParse("30Mi"),
				ExtraPerNode: resource.MustParse("1Mi"),
				Name:         "memory",
			},
			{
				Base:         resource.MustParse("30Gi"),
				ExtraPerNode: resource.MustParse("1Gi"),
				Name:         "storage",
			},
		},
	}
	noCPUEstimator = LinearEstimator{
		Resources: []Resource{
			{
				Base:         resource.MustParse("30Mi"),
				ExtraPerNode: resource.MustParse("1Mi"),
				Name:         "memory",
			},
			{
				Base:         resource.MustParse("30Gi"),
				ExtraPerNode: resource.MustParse("1Gi"),
				Name:         "storage",
			},
		},
	}
	noMemoryEstimator = LinearEstimator{
		Resources: []Resource{
			{
				Base:         resource.MustParse("0.3"),
				ExtraPerNode: resource.MustParse("1"),
				Name:         "cpu",
			},
			{
				Base:         resource.MustParse("30Gi"),
				ExtraPerNode: resource.MustParse("1Gi"),
				Name:         "storage",
			},
		},
	}
	noStorageEstimator = LinearEstimator{
		Resources: []Resource{
			{
				Base:         resource.MustParse("0.3"),
				ExtraPerNode: resource.MustParse("1"),
				Name:         "cpu",
			},
			{
				Base:         resource.MustParse("30Mi"),
				ExtraPerNode: resource.MustParse("1Mi"),
				Name:         "memory",
			},
		},
	}
	emptyEstimator = LinearEstimator{
		Resources: []Resource{},
	}

	baseResources = api.ResourceList{
		"cpu":     resource.MustParse("0.3"),
		"memory":  resource.MustParse("30Mi"),
		"storage": resource.MustParse("30Gi"),
	}

	noCPUBaseResources = api.ResourceList{
		"memory":  resource.MustParse("30Mi"),
		"storage": resource.MustParse("30Gi"),
	}
	noMemoryBaseResources = api.ResourceList{
		"cpu":     resource.MustParse("0.3"),
		"storage": resource.MustParse("30Gi"),
	}
	noStorageBaseResources = api.ResourceList{
		"cpu":    resource.MustParse("0.3"),
		"memory": resource.MustParse("30Mi"),
	}
	threeNodeResources = api.ResourceList{
		"cpu":     resource.MustParse("3.3"),
		"memory":  resource.MustParse("33Mi"),
		"storage": resource.MustParse("33Gi"),
	}
	threeNodeNoCPUResources = api.ResourceList{
		"memory":  resource.MustParse("33Mi"),
		"storage": resource.MustParse("33Gi"),
	}
	threeNodeNoMemoryResources = api.ResourceList{
		"cpu":     resource.MustParse("3.3"),
		"storage": resource.MustParse("33Gi"),
	}
	threeNodeNoStorageResources = api.ResourceList{
		"cpu":    resource.MustParse("3.3"),
		"memory": resource.MustParse("33Mi"),
	}
	noResources = api.ResourceList{}
)

func TestEstimateResources(t *testing.T) {
	testCases := []struct {
		e        ResourceEstimator
		numNodes uint64
		limits   api.ResourceList
		requests api.ResourceList
	}{
		{fullEstimator, 0, baseResources, baseResources},
		{fullEstimator, 3, threeNodeResources, threeNodeResources},
		{noCPUEstimator, 0, noCPUBaseResources, noCPUBaseResources},
		{noCPUEstimator, 3, threeNodeNoCPUResources, threeNodeNoCPUResources},
		{noMemoryEstimator, 0, noMemoryBaseResources, noMemoryBaseResources},
		{noMemoryEstimator, 3, threeNodeNoMemoryResources, threeNodeNoMemoryResources},
		{noStorageEstimator, 0, noStorageBaseResources, noStorageBaseResources},
		{noStorageEstimator, 3, threeNodeNoStorageResources, threeNodeNoStorageResources},
		{emptyEstimator, 0, noResources, noResources},
		{emptyEstimator, 3, noResources, noResources},
	}

	for i, tc := range testCases {
		got := tc.e.scaleWithNodes(tc.numNodes)
		want := &api.ResourceRequirements{
			Limits:   tc.limits,
			Requests: tc.requests,
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("scaleWithNodes got %v, want %v in test case %d", got, want, i)
		}
	}
}

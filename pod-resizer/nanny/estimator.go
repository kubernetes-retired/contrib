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
	resource "k8s.io/kubernetes/pkg/api/resource"
	api "k8s.io/kubernetes/pkg/api/v1"

	inf "speter.net/go/exp/math/dec/inf"
)

type ResourceEstimator interface {
	scaleWithNodes(numNodes uint64) *api.ResourceRequirements
}

type Resource struct {
	Base, ExtraPerNode resource.Quantity
	Name               api.ResourceName
}

type LinearEstimator struct {
	Resources []Resource
}

func (e LinearEstimator) scaleWithNodes(numNodes uint64) *api.ResourceRequirements {
	limits := make(api.ResourceList)
	requests := make(api.ResourceList)
	for _, r := range e.Resources {
		num := inf.NewDec(int64(numNodes), 0)
		num.Mul(num, r.ExtraPerNode.Amount)
		num.Add(num, r.Base.Amount)
		limits[r.Name] = resource.Quantity{
			Amount: num,
			Format: r.Base.Format,
		}
		requests[r.Name] = resource.Quantity{
			Amount: num,
			Format: r.Base.Format,
		}
	}
	return &api.ResourceRequirements{
		Limits:   limits,
		Requests: requests,
	}
}

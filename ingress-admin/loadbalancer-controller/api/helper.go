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
package api


import (
	"k8s.io/client-go/1.5/pkg/runtime"
	"k8s.io/client-go/1.5/pkg/util/json"
)

func ToLoadbalancerClaim(unstructed *runtime.Unstructured) (*LoadBalancerClaim, error) {
	data, err := unstructed.MarshalJSON()
	if err != nil {
		return nil, err
	}
	claim := &LoadBalancerClaim{}
	if err := json.Unmarshal(data, claim); err != nil {
		return nil, err
	}
	return claim, nil
}

func (claim LoadBalancerClaim) ToUnstructured() (*runtime.Unstructured, error) {
	data, err := json.Marshal(claim)
	if err != nil {
		return nil, err
	}
	unstructed := &runtime.Unstructured{}
	if err := unstructed.UnmarshalJSON(data); err != nil {
		return nil, err
	}
	return unstructed, nil
}

func (lb LoadBalancer) ToUnstructured() (*runtime.Unstructured, error) {
	data, err := json.Marshal(lb)
	if err != nil {
		return nil, err
	}
	unstructed := &runtime.Unstructured{}
	if err := unstructed.UnmarshalJSON(data); err != nil {
		return nil, err
	}
	return unstructed, nil
}
/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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

package schedulercache

import (
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/plugin/pkg/scheduler/schedulercache"
)

// PodsToCache is used for testing
type PodsToCache []*api.Pod

func (p PodsToCache) AssumePod(pod *api.Pod) error {
	return nil
}

func (p PodsToCache) AddPod(pod *api.Pod) error { return nil }

func (p PodsToCache) UpdatePod(oldPod, newPod *api.Pod) error { return nil }

func (p PodsToCache) RemovePod(pod *api.Pod) error { return nil }

func (p PodsToCache) AddNode(node *api.Node) error { return nil }

func (p PodsToCache) UpdateNode(oldNode, newNode *api.Node) error { return nil }

func (p PodsToCache) RemoveNode(node *api.Node) error { return nil }

func (p PodsToCache) GetNodeNameToInfoMap() (map[string]*schedulercache.NodeInfo, error) {
	return schedulercache.CreateNodeNameToInfoMap(p), nil
}

func (p PodsToCache) List(s labels.Selector) (selected []*api.Pod, err error) {
	for _, pod := range p {
		if s.Matches(labels.Set(pod.Labels)) {
			selected = append(selected, pod)
		}
	}
	return selected, nil
}

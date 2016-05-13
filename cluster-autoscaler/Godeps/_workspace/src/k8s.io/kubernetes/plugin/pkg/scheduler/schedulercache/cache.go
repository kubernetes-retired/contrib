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
	"fmt"
	"sync"
	"time"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/util/wait"
)

var (
	cleanAssumedPeriod = 1 * time.Second
)

// New returns a Cache implementation.
// It automatically starts a go routine that manages expiration of assumed pods.
// "ttl" is how long the assumed pod will get expired.
// "stop" is the channel that would close the background goroutine.
func New(ttl time.Duration, stop <-chan struct{}) Cache {
	cache := newSchedulerCache(ttl, cleanAssumedPeriod, stop)
	cache.run()
	return cache
}

type schedulerCache struct {
	stop   <-chan struct{}
	ttl    time.Duration
	period time.Duration

	// This mutex guards all fields within this cache struct.
	mu sync.Mutex
	// a set of assumed pod keys.
	// The key could further be used to get an entry in podStates.
	assumedPods map[string]bool
	// a map from pod key to podState.
	podStates map[string]*podState
	nodes     map[string]*NodeInfo
}

type podState struct {
	pod *api.Pod
	// Used by assumedPod to determinate expiration.
	deadline *time.Time
}

func newSchedulerCache(ttl, period time.Duration, stop <-chan struct{}) *schedulerCache {
	return &schedulerCache{
		ttl:    ttl,
		period: period,
		stop:   stop,

		nodes:       make(map[string]*NodeInfo),
		assumedPods: make(map[string]bool),
		podStates:   make(map[string]*podState),
	}
}

func (cache *schedulerCache) GetNodeNameToInfoMap() (map[string]*NodeInfo, error) {
	nodeNameToInfo := make(map[string]*NodeInfo)
	cache.mu.Lock()
	defer cache.mu.Unlock()
	for name, info := range cache.nodes {
		nodeNameToInfo[name] = info.Clone()
	}
	return nodeNameToInfo, nil
}

func (cache *schedulerCache) List(selector labels.Selector) ([]*api.Pod, error) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	var pods []*api.Pod
	for _, info := range cache.nodes {
		for _, pod := range info.pods {
			if selector.Matches(labels.Set(pod.Labels)) {
				pods = append(pods, pod)
			}
		}
	}
	return pods, nil
}

func (cache *schedulerCache) AssumePod(pod *api.Pod) error {
	return cache.assumePod(pod, time.Now())
}

// assumePod exists for making test deterministic by taking time as input argument.
func (cache *schedulerCache) assumePod(pod *api.Pod, now time.Time) error {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	key, err := getPodKey(pod)
	if err != nil {
		return err
	}
	if _, ok := cache.podStates[key]; ok {
		return fmt.Errorf("pod state wasn't initial but get assumed. Pod key: %v", key)
	}

	cache.addPod(pod)
	dl := now.Add(cache.ttl)
	ps := &podState{
		pod:      pod,
		deadline: &dl,
	}
	cache.podStates[key] = ps
	cache.assumedPods[key] = true
	return nil
}

func (cache *schedulerCache) AddPod(pod *api.Pod) error {
	key, err := getPodKey(pod)
	if err != nil {
		return err
	}

	cache.mu.Lock()
	defer cache.mu.Unlock()

	_, ok := cache.podStates[key]
	switch {
	case ok && cache.assumedPods[key]:
		delete(cache.assumedPods, key)
		cache.podStates[key].deadline = nil
	case !ok:
		// Pod was expired. We should add it back.
		cache.addPod(pod)
		ps := &podState{
			pod: pod,
		}
		cache.podStates[key] = ps
	default:
		return fmt.Errorf("pod was already in added state. Pod key: %v", key)
	}
	return nil
}

func (cache *schedulerCache) UpdatePod(oldPod, newPod *api.Pod) error {
	key, err := getPodKey(oldPod)
	if err != nil {
		return err
	}

	cache.mu.Lock()
	defer cache.mu.Unlock()

	_, ok := cache.podStates[key]
	switch {
	// An assumed pod won't have Update/Remove event. It needs to have Add event
	// before Update event, in which case the state would change from Assumed to Added.
	case ok && !cache.assumedPods[key]:
		if err := cache.updatePod(oldPod, newPod); err != nil {
			return err
		}
	default:
		return fmt.Errorf("pod state wasn't added but get updated. Pod key: %v", key)
	}
	return nil
}

func (cache *schedulerCache) updatePod(oldPod, newPod *api.Pod) error {
	if err := cache.removePod(oldPod); err != nil {
		return err
	}
	cache.addPod(newPod)
	return nil
}

func (cache *schedulerCache) addPod(pod *api.Pod) {
	n, ok := cache.nodes[pod.Spec.NodeName]
	if !ok {
		n = NewNodeInfo()
		cache.nodes[pod.Spec.NodeName] = n
	}
	n.addPod(pod)
}

func (cache *schedulerCache) removePod(pod *api.Pod) error {
	n := cache.nodes[pod.Spec.NodeName]
	if err := n.removePod(pod); err != nil {
		return err
	}
	if len(n.pods) == 0 && n.node == nil {
		delete(cache.nodes, pod.Spec.NodeName)
	}
	return nil
}

func (cache *schedulerCache) RemovePod(pod *api.Pod) error {
	key, err := getPodKey(pod)
	if err != nil {
		return err
	}

	cache.mu.Lock()
	defer cache.mu.Unlock()

	_, ok := cache.podStates[key]
	switch {
	// An assumed pod won't have Delete/Remove event. It needs to have Add event
	// before Remove event, in which case the state would change from Assumed to Added.
	case ok && !cache.assumedPods[key]:
		err := cache.removePod(pod)
		if err != nil {
			return err
		}
		delete(cache.podStates, key)
	default:
		return fmt.Errorf("pod state wasn't added but get removed. Pod key: %v", key)
	}
	return nil
}

func (cache *schedulerCache) AddNode(node *api.Node) error {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	n, ok := cache.nodes[node.Name]
	if !ok {
		n = NewNodeInfo()
		cache.nodes[node.Name] = n
	}
	return n.SetNode(node)
}

func (cache *schedulerCache) UpdateNode(oldNode, newNode *api.Node) error {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	n, ok := cache.nodes[newNode.Name]
	if !ok {
		n = NewNodeInfo()
		cache.nodes[newNode.Name] = n
	}
	return n.SetNode(newNode)
}

func (cache *schedulerCache) RemoveNode(node *api.Node) error {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	n := cache.nodes[node.Name]
	if err := n.RemoveNode(node); err != nil {
		return err
	}
	// We remove NodeInfo for this node only if there aren't any pods on this node.
	// We can't do it unconditionally, because notifications about pods are delivered
	// in a different watch, and thus can potentially be observed later, even though
	// they happened before node removal.
	if len(n.pods) == 0 && n.node == nil {
		delete(cache.nodes, node.Name)
	}
	return nil
}

func (cache *schedulerCache) run() {
	go wait.Until(cache.cleanupExpiredAssumedPods, cache.period, cache.stop)
}

func (cache *schedulerCache) cleanupExpiredAssumedPods() {
	cache.cleanupAssumedPods(time.Now())
}

// cleanupAssumedPods exists for making test deterministic by taking time as input argument.
func (cache *schedulerCache) cleanupAssumedPods(now time.Time) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	// The size of assumedPods should be small
	for key := range cache.assumedPods {
		ps, ok := cache.podStates[key]
		if !ok {
			panic("Key found in assumed set but not in podStates. Potentially a logical error.")
		}
		if now.After(*ps.deadline) {
			if err := cache.expirePod(key, ps); err != nil {
				glog.Errorf(" expirePod failed for %s: %v", key, err)
			}
		}
	}
}

func (cache *schedulerCache) expirePod(key string, ps *podState) error {
	if err := cache.removePod(ps.pod); err != nil {
		return err
	}
	delete(cache.assumedPods, key)
	delete(cache.podStates, key)
	return nil
}

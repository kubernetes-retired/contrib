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

package scheduler

import (
	"fmt"
	"testing"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/plugin/pkg/scheduler/algorithm"
	schedulerapi "k8s.io/kubernetes/plugin/pkg/scheduler/api"
	"k8s.io/kubernetes/plugin/pkg/scheduler/schedulercache"
	schedulertesting "k8s.io/kubernetes/plugin/pkg/scheduler/testing"
)

type fitPredicate func(pod *api.Pod, node *api.Node) (bool, error)
type priorityFunc func(pod *api.Pod, nodes *api.NodeList) (*schedulerapi.HostPriorityList, error)

type priorityConfig struct {
	function priorityFunc
	weight   int
}

func errorPredicateExtender(pod *api.Pod, node *api.Node) (bool, error) {
	return false, fmt.Errorf("Some error")
}

func falsePredicateExtender(pod *api.Pod, node *api.Node) (bool, error) {
	return false, nil
}

func truePredicateExtender(pod *api.Pod, node *api.Node) (bool, error) {
	return true, nil
}

func machine1PredicateExtender(pod *api.Pod, node *api.Node) (bool, error) {
	if node.Name == "machine1" {
		return true, nil
	}
	return false, nil
}

func machine2PredicateExtender(pod *api.Pod, node *api.Node) (bool, error) {
	if node.Name == "machine2" {
		return true, nil
	}
	return false, nil
}

func errorPrioritizerExtender(pod *api.Pod, nodes *api.NodeList) (*schedulerapi.HostPriorityList, error) {
	return &schedulerapi.HostPriorityList{}, fmt.Errorf("Some error")
}

func machine1PrioritizerExtender(pod *api.Pod, nodes *api.NodeList) (*schedulerapi.HostPriorityList, error) {
	result := schedulerapi.HostPriorityList{}
	for _, node := range nodes.Items {
		score := 1
		if node.Name == "machine1" {
			score = 10
		}
		result = append(result, schedulerapi.HostPriority{Host: node.Name, Score: score})
	}
	return &result, nil
}

func machine2PrioritizerExtender(pod *api.Pod, nodes *api.NodeList) (*schedulerapi.HostPriorityList, error) {
	result := schedulerapi.HostPriorityList{}
	for _, node := range nodes.Items {
		score := 1
		if node.Name == "machine2" {
			score = 10
		}
		result = append(result, schedulerapi.HostPriority{Host: node.Name, Score: score})
	}
	return &result, nil
}

func machine2Prioritizer(_ *api.Pod, nodeNameToInfo map[string]*schedulercache.NodeInfo, nodeLister algorithm.NodeLister) (schedulerapi.HostPriorityList, error) {
	nodes, err := nodeLister.List()
	if err != nil {
		return []schedulerapi.HostPriority{}, err
	}

	result := []schedulerapi.HostPriority{}
	for _, node := range nodes.Items {
		score := 1
		if node.Name == "machine2" {
			score = 10
		}
		result = append(result, schedulerapi.HostPriority{Host: node.Name, Score: score})
	}
	return result, nil
}

type FakeExtender struct {
	predicates   []fitPredicate
	prioritizers []priorityConfig
	weight       int
}

func (f *FakeExtender) Filter(pod *api.Pod, nodes *api.NodeList) (*api.NodeList, error) {
	filtered := []api.Node{}
	for _, node := range nodes.Items {
		fits := true
		for _, predicate := range f.predicates {
			fit, err := predicate(pod, &node)
			if err != nil {
				return &api.NodeList{}, err
			}
			if !fit {
				fits = false
				break
			}
		}
		if fits {
			filtered = append(filtered, node)
		}
	}
	return &api.NodeList{Items: filtered}, nil
}

func (f *FakeExtender) Prioritize(pod *api.Pod, nodes *api.NodeList) (*schedulerapi.HostPriorityList, int, error) {
	result := schedulerapi.HostPriorityList{}
	combinedScores := map[string]int{}
	for _, prioritizer := range f.prioritizers {
		weight := prioritizer.weight
		if weight == 0 {
			continue
		}
		priorityFunc := prioritizer.function
		prioritizedList, err := priorityFunc(pod, nodes)
		if err != nil {
			return &schedulerapi.HostPriorityList{}, 0, err
		}
		for _, hostEntry := range *prioritizedList {
			combinedScores[hostEntry.Host] += hostEntry.Score * weight
		}
	}
	for host, score := range combinedScores {
		result = append(result, schedulerapi.HostPriority{Host: host, Score: score})
	}
	return &result, f.weight, nil
}

func TestGenericSchedulerWithExtenders(t *testing.T) {
	tests := []struct {
		name                 string
		predicates           map[string]algorithm.FitPredicate
		prioritizers         []algorithm.PriorityConfig
		extenders            []FakeExtender
		extenderPredicates   []fitPredicate
		extenderPrioritizers []priorityConfig
		nodes                []string
		pod                  *api.Pod
		pods                 []*api.Pod
		expectedHost         string
		expectsErr           bool
	}{
		{
			predicates:   map[string]algorithm.FitPredicate{"true": truePredicate},
			prioritizers: []algorithm.PriorityConfig{{EqualPriority, 1}},
			extenders: []FakeExtender{
				{
					predicates: []fitPredicate{truePredicateExtender},
				},
				{
					predicates: []fitPredicate{errorPredicateExtender},
				},
			},
			nodes:      []string{"machine1", "machine2"},
			expectsErr: true,
			name:       "test 1",
		},
		{
			predicates:   map[string]algorithm.FitPredicate{"true": truePredicate},
			prioritizers: []algorithm.PriorityConfig{{EqualPriority, 1}},
			extenders: []FakeExtender{
				{
					predicates: []fitPredicate{truePredicateExtender},
				},
				{
					predicates: []fitPredicate{falsePredicateExtender},
				},
			},
			nodes:      []string{"machine1", "machine2"},
			expectsErr: true,
			name:       "test 2",
		},
		{
			predicates:   map[string]algorithm.FitPredicate{"true": truePredicate},
			prioritizers: []algorithm.PriorityConfig{{EqualPriority, 1}},
			extenders: []FakeExtender{
				{
					predicates: []fitPredicate{truePredicateExtender},
				},
				{
					predicates: []fitPredicate{machine1PredicateExtender},
				},
			},
			nodes:        []string{"machine1", "machine2"},
			expectedHost: "machine1",
			name:         "test 3",
		},
		{
			predicates:   map[string]algorithm.FitPredicate{"true": truePredicate},
			prioritizers: []algorithm.PriorityConfig{{EqualPriority, 1}},
			extenders: []FakeExtender{
				{
					predicates: []fitPredicate{machine2PredicateExtender},
				},
				{
					predicates: []fitPredicate{machine1PredicateExtender},
				},
			},
			nodes:      []string{"machine1", "machine2"},
			expectsErr: true,
			name:       "test 4",
		},
		{
			predicates:   map[string]algorithm.FitPredicate{"true": truePredicate},
			prioritizers: []algorithm.PriorityConfig{{EqualPriority, 1}},
			extenders: []FakeExtender{
				{
					predicates:   []fitPredicate{truePredicateExtender},
					prioritizers: []priorityConfig{{errorPrioritizerExtender, 10}},
					weight:       1,
				},
			},
			nodes:        []string{"machine1"},
			expectedHost: "machine1",
			name:         "test 5",
		},
		{
			predicates:   map[string]algorithm.FitPredicate{"true": truePredicate},
			prioritizers: []algorithm.PriorityConfig{{EqualPriority, 1}},
			extenders: []FakeExtender{
				{
					predicates:   []fitPredicate{truePredicateExtender},
					prioritizers: []priorityConfig{{machine1PrioritizerExtender, 10}},
					weight:       1,
				},
				{
					predicates:   []fitPredicate{truePredicateExtender},
					prioritizers: []priorityConfig{{machine2PrioritizerExtender, 10}},
					weight:       5,
				},
			},
			nodes:        []string{"machine1", "machine2"},
			expectedHost: "machine2",
			name:         "test 6",
		},
		{
			predicates:   map[string]algorithm.FitPredicate{"true": truePredicate},
			prioritizers: []algorithm.PriorityConfig{{machine2Prioritizer, 20}},
			extenders: []FakeExtender{
				{
					predicates:   []fitPredicate{truePredicateExtender},
					prioritizers: []priorityConfig{{machine1PrioritizerExtender, 10}},
					weight:       1,
				},
			},
			nodes:        []string{"machine1", "machine2"},
			expectedHost: "machine2", // machine2 has higher score
			name:         "test 7",
		},
	}

	for _, test := range tests {
		extenders := []algorithm.SchedulerExtender{}
		for ii := range test.extenders {
			extenders = append(extenders, &test.extenders[ii])
		}
		scheduler := NewGenericScheduler(schedulertesting.PodsToCache(test.pods), test.predicates, test.prioritizers, extenders)
		machine, err := scheduler.Schedule(test.pod, algorithm.FakeNodeLister(makeNodeList(test.nodes)))
		if test.expectsErr {
			if err == nil {
				t.Errorf("Unexpected non-error for %s, machine %s", test.name, machine)
			}
		} else {
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if test.expectedHost != machine {
				t.Errorf("Failed : %s, Expected: %s, Saw: %s", test.name, test.expectedHost, machine)
			}
		}
	}
}

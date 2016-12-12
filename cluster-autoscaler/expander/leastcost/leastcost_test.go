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

package leastcost

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/contrib/cluster-autoscaler/expander"
	kube_api "k8s.io/kubernetes/pkg/api"
)

type FakeNodeGroup struct {
	id string
}

func (f *FakeNodeGroup) MaxSize() int { return 2 }
func (f *FakeNodeGroup) MinSize() int { return 1 }
func (f *FakeNodeGroup) NodeCost() (float64, error) {
	switch f.Id() {
	case "AnotherOnDemand":
		fallthrough
	case "OnDemand":
		return 1.675, nil
	case "CheapSpot":
		return 0.5, nil
	case "ExpensiveSpot":
		return 16.75, nil
	default:
		return 0, nil
	}
}
func (f *FakeNodeGroup) TargetSize() (int, error)           { return 2, nil }
func (f *FakeNodeGroup) IncreaseSize(delta int) error       { return nil }
func (f *FakeNodeGroup) DeleteNodes([]*kube_api.Node) error { return nil }
func (f *FakeNodeGroup) Id() string                         { return f.id }
func (f *FakeNodeGroup) Debug() string                      { return f.id }

func TestLeastCost(t *testing.T) {
	od1 := expander.Option{NodeGroup: &FakeNodeGroup{"OnDemand"}}
	od2 := expander.Option{NodeGroup: &FakeNodeGroup{"AnotherOnDemand"}}
	cs := expander.Option{NodeGroup: &FakeNodeGroup{"CheapSpot"}}
	es := expander.Option{NodeGroup: &FakeNodeGroup{"ExpensiveSpot"}}
	e := NewStrategy()

	// Select the cheapest Spot node group over the more expensive groups
	ret := e.BestOption([]expander.Option{od1, cs, es}, nil)
	assert.Equal(t, cs, *ret)

	// Select the On-Demand node group over both spot groups once the cheapest Spot node group is full
	cs.NodeCount = 2
	ret = e.BestOption([]expander.Option{od1, cs, es}, nil)
	assert.Equal(t, od1, *ret)

	// Select the OnDemand node group over the expensive Spot
	ret = e.BestOption([]expander.Option{od1, es}, nil)
	assert.Equal(t, od1, *ret)

	// Select an OnDemand node group at random
	ret = e.BestOption([]expander.Option{od1, od2}, nil)
	assert.True(t, assert.ObjectsAreEqual(*ret, od1) || assert.ObjectsAreEqual(*ret, od2))
}

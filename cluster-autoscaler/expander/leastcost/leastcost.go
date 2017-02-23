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
	"github.com/golang/glog"
	"k8s.io/contrib/cluster-autoscaler/expander"
	"k8s.io/contrib/cluster-autoscaler/expander/random"
	"k8s.io/kubernetes/plugin/pkg/scheduler/schedulercache"
)

type leastcost struct {
	next expander.Strategy
}

// NewStrategy returns a scale up strategy (expander) that picks the node group that will cost the least to scale up
func NewStrategy() expander.Strategy {
	return &leastcost{random.NewStrategy()}
}

// BestOption Finds the option with the least cost per node, selecting a random node group if costs are equal.
func (l *leastcost) BestOption(expansionOptions []expander.Option, nodeInfo map[string]*schedulercache.NodeInfo) *expander.Option {
	var leastCost float64
	var leastCostOptions []expander.Option

	for _, option := range expansionOptions {
		if option.NodeCount == option.NodeGroup.MaxSize() {
			glog.V(1).Infof("Skipping full Node Group: %s", option.NodeGroup.Id())
			continue
		}

		nodeCost, err := option.NodeGroup.NodeCost()

		if err != nil {
			glog.Errorf("Error calculating NodeCost: %v", err)
			return nil
		}

		glog.V(1).Infof("Expanding Node Group %s would cost $%f per node", option.NodeGroup.Id(), nodeCost)

		if nodeCost == leastCost {
			leastCostOptions = append(leastCostOptions, option)
		}

		if leastCostOptions == nil || nodeCost < leastCost {
			leastCost = nodeCost
			leastCostOptions = []expander.Option{option}
		}
	}

	if len(leastCostOptions) == 0 {
		glog.V(1).Info("Unable to determine NodeGroup with lowest cost per node")
		return nil
	}

	return l.next.BestOption(leastCostOptions, nodeInfo)
}

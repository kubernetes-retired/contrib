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

package nanny

import (
	"fmt"
	"math"

	log "github.com/golang/glog"
	api "k8s.io/kubernetes/pkg/api/v1"

	"k8s.io/kubernetes/pkg/api/resource"
)

// Resource defines the name of a resource, the quantity, and the marginal value.
type Resource struct {
	Base, ExtraPerNode resource.Quantity
	Name               api.ResourceName
}

// ResourceListPair is a pair of ResourceLists, denoting a range.
type ResourceListPair struct {
	lower, upper api.ResourceList
}

// EstimatorResult is the result of the resource Estimation, used by Estimator struct.
type EstimatorResult struct {
	RecommendedRange, AcceptableRange ResourceListPair
}

// Estimator is a struct used for estimating accepted and recommended resource requirements.
type Estimator struct {
	Resources            []Resource
	AcceptanceOffset     int64
	RecommendationOffset int64
}

func decWithPercentageOffset(value uint64, offset int64, rounder func(float64) float64) uint64 {
	return uint64(int64(value) + int64(rounder(float64(offset)*float64(value)/100)))
}

func nodesAndOffsetToRange(numNodes uint64, offset int64, res []Resource) ResourceListPair {
	numNodesMin := decWithPercentageOffset(numNodes, -offset, math.Floor)
	numNodesMax := decWithPercentageOffset(numNodes, offset, math.Ceil)
	return ResourceListPair{
		lower: calculateResources(numNodesMin, res),
		upper: calculateResources(numNodesMax, res),
	}
}

func (e Estimator) scaleWithNodes(numNodes uint64) *EstimatorResult {
	return &EstimatorResult{
		RecommendedRange: nodesAndOffsetToRange(numNodes, e.RecommendationOffset, e.Resources),
		AcceptableRange:  nodesAndOffsetToRange(numNodes, e.AcceptanceOffset, e.Resources),
	}
}

func calculateResources(numNodes uint64, resources []Resource) api.ResourceList {
	resourceList := make(api.ResourceList)
	for _, r := range resources {
		// Since we want to enable passing values smaller than e.g. 1 millicore per node,
		// we need to have some more hacky solution here than operating on MilliValues.
		perNodeString := r.ExtraPerNode.String()
		var perNode float64
		read, _ := fmt.Sscanf(perNodeString, "%f", &perNode)
		overhead := resource.MustParse(fmt.Sprintf("%f%s", perNode*float64(numNodes), perNodeString[read:]))

		newRes := r.Base
		newRes.Add(overhead)

		log.V(4).Infof("New requirement for resource %s with %d nodes is %s", r.Name, numNodes, newRes.String())

		resourceList[r.Name] = newRes
	}
	return resourceList
}

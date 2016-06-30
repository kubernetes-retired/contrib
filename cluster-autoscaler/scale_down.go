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

package main

import (
	"fmt"
	"time"

	"k8s.io/contrib/cluster-autoscaler/config"
	"k8s.io/contrib/cluster-autoscaler/simulator"
	"k8s.io/contrib/cluster-autoscaler/utils/cloud"
	kube_api "k8s.io/kubernetes/pkg/api"
	kube_client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/plugin/pkg/scheduler/schedulercache"

	"github.com/golang/glog"
)

// ScaleDownResult represents the state of scale down.
type ScaleDownResult int

const (
	// ScaleDownError - scale down finished with error.
	ScaleDownError ScaleDownResult = iota
	// ScaleDownNoUnneeded - no unneeded nodes and no errors.
	ScaleDownNoUnneeded ScaleDownResult = iota
	// ScaleDownNoNodeDeleted - unneeded nodes present but not available for deletion.
	ScaleDownNoNodeDeleted ScaleDownResult = iota
	// ScaleDownNodeDeleted - a node was deleted.
	ScaleDownNodeDeleted ScaleDownResult = iota
)

// FindUnneededNodes calculates which nodes are not needed, i.e. all pods can be scheduled somewhere else,
// and updates unneededNodes map accordingly.
func FindUnneededNodes(nodes []*kube_api.Node,
	unneededNodes map[string]time.Time,
	utilizationThreshold float64,
	pods []*kube_api.Pod,
	predicateChecker *simulator.PredicateChecker) map[string]time.Time {

	currentlyUnneededNodes := make([]*kube_api.Node, 0)
	nodeNameToNodeInfo := schedulercache.CreateNodeNameToInfoMap(pods)

	// Phase1 - look at the nodes utilization.
	for _, node := range nodes {
		nodeInfo, found := nodeNameToNodeInfo[node.Name]
		if !found {
			glog.Errorf("Node info for %s not found", node.Name)
			continue
		}
		utilization, err := simulator.CalculateUtilization(node, nodeInfo)

		if err != nil {
			glog.Warningf("Failed to calculate utilization for %s: %v", node.Name, err)
		}
		glog.V(4).Infof("Node %s - utilization %f", node.Name, utilization)

		if utilization >= utilizationThreshold {
			glog.V(4).Infof("Node %s is not suitable for removal - utilization to big (%f)", node.Name, utilization)
			continue
		}
		currentlyUnneededNodes = append(currentlyUnneededNodes, node)
	}

	// Phase2 - check which nodes can be probably removed using fast drain.
	nodesToRemove, err := simulator.FindNodesToRemove(currentlyUnneededNodes, nodes, pods,
		nil, predicateChecker,
		len(currentlyUnneededNodes), true)
	if err != nil {
		glog.Errorf("Error while simulating node drains: %v", err)
		return map[string]time.Time{}
	}

	// Update the timestamp map.
	now := time.Now()
	result := make(map[string]time.Time)
	for _, node := range nodesToRemove {
		name := node.Name
		if val, found := unneededNodes[name]; !found {
			result[name] = now
		} else {
			result[name] = val
		}
	}
	return result
}

// ScaleDown tries to scale down the cluster. It returns ScaleDownResult indicating if any node was
// removed and error if such occured.
func ScaleDown(
	nodes []*kube_api.Node,
	unneededNodes map[string]time.Time,
	unneededTime time.Duration,
	pods []*kube_api.Pod,
	cloudManager cloud.Manager,
	client *kube_client.Client,
	predicateChecker *simulator.PredicateChecker) (ScaleDownResult, error) {

	now := time.Now()
	candidates := make([]*kube_api.Node, 0)
	for _, node := range nodes {
		if val, found := unneededNodes[node.Name]; found {

			glog.V(2).Infof("%s was unneeded for %s", node.Name, now.Sub(val).String())

			// Check how long the node was underutilized.
			if !val.Add(unneededTime).Before(now) {
				continue
			}

			// Check mig size.
			instance, err := config.InstanceConfigFromProviderId(node.Spec.ProviderID)
			if err != nil {
				glog.Errorf("Error while parsing providerid of %s: %v", node.Name, err)
				continue
			}
			migConfig, err := cloudManager.GetScalingGroupForInstance(instance)
			if err != nil {
				glog.Errorf("Error while checking mig config for instance %v: %v", instance, err)
				continue
			}
			size, err := cloudManager.GetScalingGroupSize(migConfig)
			if err != nil {
				glog.Errorf("Error while checking mig size for instance %v: %v", instance, err)
				continue
			}

			if size <= int64(migConfig.MinSize) {
				glog.V(1).Infof("Skipping %s - mig min size reached", node.Name)
				continue
			}

			candidates = append(candidates, node)
		}
	}
	if len(candidates) == 0 {
		glog.Infof("No candidates for scale down")
		return ScaleDownNoUnneeded, nil
	}

	nodesToRemove, err := simulator.FindNodesToRemove(candidates, nodes, pods, client, predicateChecker, 1, false)
	if err != nil {
		return ScaleDownError, fmt.Errorf("Find node to remove failed: %v", err)
	}
	if len(nodesToRemove) == 0 {
		glog.V(1).Infof("No node to remove")
		return ScaleDownNoNodeDeleted, nil
	}
	nodeToRemove := nodesToRemove[0]
	glog.Infof("Removing %s", nodeToRemove.Name)
	instanceConfig, err := config.InstanceConfigFromProviderId(nodeToRemove.Spec.ProviderID)
	if err != nil {
		return ScaleDownError, fmt.Errorf("Failed to get instance config for %s: %v", nodeToRemove.Name, err)
	}

	err = cloudManager.DeleteInstances([]*config.InstanceConfig{instanceConfig})
	if err != nil {
		return ScaleDownError, fmt.Errorf("Failed to delete %v: %v", instanceConfig, err)
	}

	return ScaleDownNodeDeleted, nil
}

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

package core

import (
	"fmt"
	"reflect"
	"time"

	"k8s.io/contrib/cluster-autoscaler/cloudprovider"
	"k8s.io/contrib/cluster-autoscaler/clusterstate"
	"k8s.io/contrib/cluster-autoscaler/simulator"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiv1 "k8s.io/kubernetes/pkg/api/v1"
	kube_client "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	"k8s.io/kubernetes/plugin/pkg/scheduler/schedulercache"

	"github.com/golang/glog"
)

// GetAllNodesAvailableTime returns time when the newest node became available for scheduler.
// TODO: This function should use LastTransitionTime from NodeReady condition.
func GetAllNodesAvailableTime(nodes []*apiv1.Node) time.Time {
	var result time.Time
	for _, node := range nodes {
		if node.CreationTimestamp.After(result) {
			result = node.CreationTimestamp.Time
		}
	}
	return result.Add(1 * time.Minute)
}

// SlicePodsByPodScheduledTime slices given pod array into those where PodScheduled condition
// have been updated after the thresold and others.
// Each pod must be in condition "Scheduled: False; Reason: Unschedulable"
func SlicePodsByPodScheduledTime(pods []*apiv1.Pod, threshold time.Time) (oldPods []*apiv1.Pod, newPods []*apiv1.Pod) {
	for _, pod := range pods {
		_, condition := apiv1.GetPodCondition(&pod.Status, apiv1.PodScheduled)
		if condition != nil {
			if condition.LastTransitionTime.After(threshold) {
				newPods = append(newPods, pod)
			} else {
				oldPods = append(oldPods, pod)
			}
		}
	}
	return
}

// ResetPodScheduledCondition resets pod condition PodScheduled to "unknown" for all the pods with LastTransitionTime
// not after the threshold time.
func ResetPodScheduledCondition(kubeClient kube_client.Interface, pods []*apiv1.Pod) {
	for _, pod := range pods {
		if err := resetPodScheduledConditionForPod(kubeClient, pod); err != nil {
			glog.Errorf("Error during reseting pod condition for %s/%s: %v", pod.Namespace, pod.Name, err)
		}
	}
}

func resetPodScheduledConditionForPod(kubeClient kube_client.Interface, pod *apiv1.Pod) error {
	_, condition := apiv1.GetPodCondition(&pod.Status, apiv1.PodScheduled)
	if condition != nil {
		glog.V(4).Infof("Reseting pod condition for %s/%s, last transition: %s",
			pod.Namespace, pod.Name, condition.LastTransitionTime.Time.String())
		condition.Status = apiv1.ConditionUnknown
		condition.LastTransitionTime = metav1.Time{
			Time: time.Now(),
		}
		_, err := kubeClient.Core().Pods(pod.Namespace).UpdateStatus(pod)
		return err
	}
	return fmt.Errorf("Expected condition PodScheduled")
}

// FilterOutSchedulable checks whether pods from <unschedulableCandidates> marked as unschedulable
// by Scheduler actually can't be scheduled on any node and filter out the ones that can.
func FilterOutSchedulable(unschedulableCandidates []*apiv1.Pod, nodes []*apiv1.Node, allPods []*apiv1.Pod, predicateChecker *simulator.PredicateChecker) []*apiv1.Pod {
	unschedulablePods := []*apiv1.Pod{}
	nodeNameToNodeInfo := createNodeNameToInfoMap(allPods, nodes)

	for _, pod := range unschedulableCandidates {
		if nodeName, err := predicateChecker.FitsAny(pod, nodeNameToNodeInfo); err == nil {
			glog.Warningf("Pod %s marked as unschedulable can be scheduled on %s. Ignoring in scale up.", pod.Name, nodeName)
		} else {
			unschedulablePods = append(unschedulablePods, pod)
		}
	}

	return unschedulablePods
}

// TODO: move this function to scheduler utils.
func createNodeNameToInfoMap(pods []*apiv1.Pod, nodes []*apiv1.Node) map[string]*schedulercache.NodeInfo {
	nodeNameToNodeInfo := schedulercache.CreateNodeNameToInfoMap(pods, nodes)
	for _, node := range nodes {
		if nodeInfo, found := nodeNameToNodeInfo[node.Name]; found {
			nodeInfo.SetNode(node)
		}
	}

	// Some pods may be out of sync with node lists. Removing incomplete node infos.
	keysToRemove := make([]string, 0)
	for key, nodeInfo := range nodeNameToNodeInfo {
		if nodeInfo.Node() == nil {
			keysToRemove = append(keysToRemove, key)
		}
	}
	for _, key := range keysToRemove {
		delete(nodeNameToNodeInfo, key)
	}

	return nodeNameToNodeInfo
}

// GetNodeInfosForGroups finds NodeInfos for all node groups used to manage the given nodes. It also returns a node group to sample node mapping.
// TODO(mwielgus): This returns map keyed by url, while most code (including scheduler) uses node.Name for a key.
func GetNodeInfosForGroups(nodes []*apiv1.Node, cloudProvider cloudprovider.CloudProvider, kubeClient kube_client.Interface) (map[string]*schedulercache.NodeInfo, error) {
	result := make(map[string]*schedulercache.NodeInfo)
	for _, node := range nodes {

		nodeGroup, err := cloudProvider.NodeGroupForNode(node)
		if err != nil {
			return map[string]*schedulercache.NodeInfo{}, err
		}
		if nodeGroup == nil || reflect.ValueOf(nodeGroup).IsNil() {
			continue
		}
		id := nodeGroup.Id()
		if _, found := result[id]; !found {
			nodeInfo, err := simulator.BuildNodeInfoForNode(node, kubeClient)
			if err != nil {
				return map[string]*schedulercache.NodeInfo{}, err
			}
			result[id] = nodeInfo
		}
	}
	return result, nil
}

// Removes unregisterd nodes if needed. Returns true if anything was removed and error if such occurred.
func removeOldUnregisteredNodes(unregisteredNodes []clusterstate.UnregisteredNode, context *AutoscalingContext,
	currentTime time.Time) (bool, error) {
	removedAny := false
	for _, unregisteredNode := range unregisteredNodes {
		if unregisteredNode.UnregisteredSince.Add(context.UnregisteredNodeRemovalTime).Before(currentTime) {
			glog.V(0).Infof("Removing unregistered node %v", unregisteredNode.Node.Name)
			nodeGroup, err := context.CloudProvider.NodeGroupForNode(unregisteredNode.Node)
			if err != nil {
				glog.Warningf("Failed to get node group for %s: %v", unregisteredNode.Node.Name, err)
				return removedAny, err
			}
			err = nodeGroup.DeleteNodes([]*apiv1.Node{unregisteredNode.Node})
			if err != nil {
				glog.Warningf("Failed to remove node %s: %v", unregisteredNode.Node.Name, err)
				return removedAny, err
			}
			removedAny = true
		}
	}
	return removedAny, nil
}

// Sets the target size of node groups to the current number of nodes in them
// if the difference was constant for a prolonged time. Returns true if managed
// to fix something.
func fixNodeGroupSize(context *AutoscalingContext, currentTime time.Time) (bool, error) {
	fixed := false
	for _, nodeGroup := range context.CloudProvider.NodeGroups() {
		incorrectSize := context.ClusterStateRegistry.GetIncorrectNodeGroupSize(nodeGroup.Id())
		if incorrectSize == nil {
			continue
		}
		if incorrectSize.FirstObserved.Add(context.UnregisteredNodeRemovalTime).Before(currentTime) {
			delta := incorrectSize.CurrentSize - incorrectSize.ExpectedSize
			if delta < 0 {
				glog.V(0).Infof("Decreasing size of %s, expected=%d current=%d delta=%d", nodeGroup.Id(),
					incorrectSize.ExpectedSize,
					incorrectSize.CurrentSize,
					delta)
				if err := nodeGroup.DecreaseTargetSize(delta); err != nil {
					return fixed, fmt.Errorf("Failed to decrease %s: %v", nodeGroup.Id(), err)
				}
				fixed = true
			}
		}
	}
	return fixed, nil
}

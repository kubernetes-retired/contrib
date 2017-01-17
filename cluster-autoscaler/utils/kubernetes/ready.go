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

package kubernetes

import (
	apiv1 "k8s.io/kubernetes/pkg/api/v1"
)

// IsNodeReadyAndSchedulable returns true if the node is ready and schedulable.
func IsNodeReadyAndSchedulable(node *apiv1.Node) bool {
	for i := range node.Status.Conditions {
		cond := &node.Status.Conditions[i]
		// We consider the node for scheduling only when its:
		// - NodeReady condition status is ConditionTrue,
		// - NodeOutOfDisk condition status is ConditionFalse,
		// - NodeNetworkUnavailable condition status is ConditionFalse.
		if cond.Type == apiv1.NodeReady && cond.Status != apiv1.ConditionTrue {
			return false
		} else if cond.Type == apiv1.NodeOutOfDisk && cond.Status != apiv1.ConditionFalse {
			return false
		} else if cond.Type == apiv1.NodeNetworkUnavailable && cond.Status != apiv1.ConditionFalse {
			return false
		}
	}
	// Ignore nodes that are marked unschedulable
	if node.Spec.Unschedulable {
		return false
	}
	return true
}

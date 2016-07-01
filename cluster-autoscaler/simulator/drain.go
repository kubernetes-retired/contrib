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

package simulator

import (
	"fmt"

	"k8s.io/kubernetes/pkg/api"
	kube_client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/kubelet/types"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/plugin/pkg/scheduler/schedulercache"
)

// FastGetPodsToMove returns a list of pods that should be moved elsewhere if the node
// is drained. Raises error if there is an unreplicated pod and force option was not specified.
// Based on kubectl drain code. It makes an assumption that RC, DS, Jobs and RS were deleted
// along with their pods (no abandoned pods with dangling created-by annotation). Usefull for fast
// checks.
func FastGetPodsToMove(nodeInfo *schedulercache.NodeInfo, force bool,
	skipNodesWithSystemPods bool, skipNodesWithLocalStorage bool, decoder runtime.Decoder) ([]*api.Pod, error) {
	pods := make([]*api.Pod, 0)
	unreplicatedPodNames := []string{}
	for _, pod := range nodeInfo.Pods() {
		_, found := pod.ObjectMeta.Annotations[types.ConfigMirrorAnnotationKey]
		if found {
			// Skip mirror pod
			continue
		}
		replicated := false
		daemonsetPod := false

		creatorRef, found := pod.ObjectMeta.Annotations[controller.CreatedByAnnotation]
		if found {
			var sr api.SerializedReference
			if err := runtime.DecodeInto(decoder, []byte(creatorRef), &sr); err != nil {
				return []*api.Pod{}, err
			}
			if sr.Reference.Kind == "ReplicationController" {
				replicated = true
			} else if sr.Reference.Kind == "DaemonSet" {
				daemonsetPod = true
			} else if sr.Reference.Kind == "Job" {
				replicated = true
			} else if sr.Reference.Kind == "ReplicaSet" {
				replicated = true
			}
		}

		if !daemonsetPod && pod.Namespace == "kube-system" && skipNodesWithSystemPods {
			return []*api.Pod{}, fmt.Errorf("non-deamons set, non-mirrored, kube-system pod present: %s", pod.Name)
		}

		if !daemonsetPod && hasLocalStorage(pod) && skipNodesWithLocalStorage {
			return []*api.Pod{}, fmt.Errorf("pod with local storage present: %s", pod.Name)
		}

		switch {
		case daemonsetPod:
			break
		case !replicated:
			unreplicatedPodNames = append(unreplicatedPodNames, pod.Name)
			if force {
				pods = append(pods, pod)
			}
		default:
			pods = append(pods, pod)
		}
	}
	if !force && len(unreplicatedPodNames) > 0 {
		return []*api.Pod{}, fmt.Errorf("unreplicated pods present")
	}
	return pods, nil
}

func hasLocalStorage(pod *api.Pod) bool {
	for _, volume := range pod.Spec.Volumes {
		if isLocalVolume(&volume) {
			return true
		}
	}
	return false
}

func isLocalVolume(volume *api.Volume) bool {
	return volume.HostPath != nil || volume.EmptyDir != nil
}

// GetPodsForDeletionOnNodeDrain returns pods that should be deleted on node drain as well as some extra information
// about possibly problematic pods (unreplicated and deamon sets).
func GetPodsForDeletionOnNodeDrain(client *kube_client.Client, nodename string, decoder runtime.Decoder, force bool,
	ignoreDeamonSet bool) (pods []api.Pod, unreplicatedPodNames []string, daemonSetPodNames []string, finalError error) {

	pods = []api.Pod{}
	unreplicatedPodNames = []string{}
	daemonSetPodNames = []string{}
	podList, err := client.Pods(api.NamespaceAll).List(api.ListOptions{FieldSelector: fields.SelectorFromSet(fields.Set{"spec.nodeName": nodename})})
	if err != nil {
		return []api.Pod{}, []string{}, []string{}, err
	}

	for _, pod := range podList.Items {
		_, found := pod.ObjectMeta.Annotations[types.ConfigMirrorAnnotationKey]
		if found {
			// Skip mirror pod
			continue
		}
		replicated := false
		daemonsetPod := false

		creatorRef, found := pod.ObjectMeta.Annotations[controller.CreatedByAnnotation]
		if found {
			// Now verify that the specified creator actually exists.
			var sr api.SerializedReference
			if err := runtime.DecodeInto(decoder, []byte(creatorRef), &sr); err != nil {
				return []api.Pod{}, []string{}, []string{}, err
			}
			if sr.Reference.Kind == "ReplicationController" {
				rc, err := client.ReplicationControllers(sr.Reference.Namespace).Get(sr.Reference.Name)
				// Assume the only reason for an error is because the RC is
				// gone/missing, not for any other cause.  TODO(mml): something more
				// sophisticated than this
				if err == nil && rc != nil {
					replicated = true
				}
			} else if sr.Reference.Kind == "DaemonSet" {
				ds, err := client.DaemonSets(sr.Reference.Namespace).Get(sr.Reference.Name)

				// Assume the only reason for an error is because the DaemonSet is
				// gone/missing, not for any other cause.  TODO(mml): something more
				// sophisticated than this
				if err == nil && ds != nil {
					// Otherwise, treat daemonset-managed pods as unmanaged since
					// DaemonSet Controller currently ignores the unschedulable bit.
					// FIXME(mml): Add link to the issue concerning a proper way to drain
					// daemonset pods, probably using taints.
					daemonsetPod = true
				}
			} else if sr.Reference.Kind == "Job" {
				job, err := client.ExtensionsClient.Jobs(sr.Reference.Namespace).Get(sr.Reference.Name)

				// Assume the only reason for an error is because the Job is
				// gone/missing, not for any other cause.  TODO(mml): something more
				// sophisticated than this
				if err == nil && job != nil {
					replicated = true
				}
			} else if sr.Reference.Kind == "ReplicaSet" {
				rs, err := client.ExtensionsClient.ReplicaSets(sr.Reference.Namespace).Get(sr.Reference.Name)

				// Assume the only reason for an error is because the RS is
				// gone/missing, not for any other cause.  TODO(mml): something more
				// sophisticated than this
				if err == nil && rs != nil {
					replicated = true
				}
			}
		}

		switch {
		case daemonsetPod:
			daemonSetPodNames = append(daemonSetPodNames, pod.Name)
		case !replicated:
			unreplicatedPodNames = append(unreplicatedPodNames, pod.Name)
			if force {
				pods = append(pods, pod)
			}
		default:
			pods = append(pods, pod)
		}
	}
	return pods, unreplicatedPodNames, daemonSetPodNames, nil
}

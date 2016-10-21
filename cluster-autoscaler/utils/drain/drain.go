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

package drain

import (
	"fmt"

	"k8s.io/kubernetes/pkg/api"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/kubelet/types"
	"k8s.io/kubernetes/pkg/runtime"
)

// GetPodsForDeletionOnNodeDrain returns pods that should be deleted on node drain as well as some extra information
// about possibly problematic pods (unreplicated and deamon sets).
func GetPodsForDeletionOnNodeDrain(
	podList []*api.Pod,
	decoder runtime.Decoder,
	skipNodesWithSystemPods bool,
	skipNodesWithLocalStorage bool,
	checkReferences bool,
	client *client.Client,
	minReplica int32) ([]*api.Pod, error) {

	pods := make([]*api.Pod, 0)

	for _, pod := range podList {
		if IsMirrorPod(pod) {
			continue
		}

		daemonsetPod := false
		replicated := false

		sr, err := CreatorRef(pod)
		if err != nil {
			return []*api.Pod{}, fmt.Errorf("failed to obtain refkind: %v", err)
		}
		refKind := ""
		if sr != nil {
			refKind = sr.Reference.Kind
		}

		if refKind == "ReplicationController" {
			if checkReferences {
				rc, err := client.ReplicationControllers(sr.Reference.Namespace).Get(sr.Reference.Name)
				// Assume a reason for an error is because the RC is either
				// gone/missing or that the rc has too few replicas configured.
				// TODO: replace the minReplica check with pod disruption budget.
				if err == nil && rc != nil {
					if rc.Spec.Replicas < minReplica {
						return []*api.Pod{}, fmt.Errorf("replication controller for %s/%s has too few replicas spec: %d min: %d",
							pod.Namespace, pod.Name, rc.Spec.Replicas, minReplica)
					} else {
						replicated = true
					}
				} else {
					return []*api.Pod{}, fmt.Errorf("replication controller for %s/%s is not available, err: %v", pod.Namespace, pod.Name, err)
				}
			} else {
				replicated = true
			}
		} else if refKind == "DaemonSet" {
			if checkReferences {
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
				} else {
					return []*api.Pod{}, fmt.Errorf("deamonset for %s/%s is not present, err: %v", pod.Namespace, pod.Name, err)
				}
			} else {
				daemonsetPod = true
			}
		} else if refKind == "Job" {
			if checkReferences {
				job, err := client.ExtensionsClient.Jobs(sr.Reference.Namespace).Get(sr.Reference.Name)

				// Assume the only reason for an error is because the Job is
				// gone/missing, not for any other cause.  TODO(mml): something more
				// sophisticated than this
				if err == nil && job != nil {
					replicated = true
				} else {
					return []*api.Pod{}, fmt.Errorf("job for %s/%s is not available: err: %v", pod.Namespace, pod.Name, err)
				}
			} else {
				replicated = true
			}
		} else if refKind == "ReplicaSet" {
			if checkReferences {
				rs, err := client.ExtensionsClient.ReplicaSets(sr.Reference.Namespace).Get(sr.Reference.Name)

				// Assume the only reason for an error is because the RS is
				// gone/missing, not for any other cause.  TODO(mml): something more
				// sophisticated than this
				if err == nil && rs != nil {
					if rs.Spec.Replicas < minReplica {
						return []*api.Pod{}, fmt.Errorf("replication controller for %s/%s has too few replicas spec: %d min: %d",
							pod.Namespace, pod.Name, rs.Spec.Replicas, minReplica)
					} else {
						replicated = true
					}
				} else {
					return []*api.Pod{}, fmt.Errorf("replication controller for %s/%s is not available, err: %v", pod.Namespace, pod.Name, err)
				}
			} else {
				replicated = true
			}
		}
		if !daemonsetPod && !replicated {
			return []*api.Pod{}, fmt.Errorf("%s/%s is not replicated", pod.Namespace, pod.Name)
		}
		if !daemonsetPod && pod.Namespace == "kube-system" && skipNodesWithSystemPods {
			return []*api.Pod{}, fmt.Errorf("non-deamons set, non-mirrored, kube-system pod present: %s", pod.Name)
		}
		if !daemonsetPod && HasLocalStorage(pod) && skipNodesWithLocalStorage {
			return []*api.Pod{}, fmt.Errorf("pod with local storage present: %s", pod.Name)
		}

		if !daemonsetPod {
			pods = append(pods, pod)
		}
	}
	return pods, nil
}

// CreatorRefKind returns the kind of the creator of the pod.
func CreatorRefKind(pod *api.Pod) (string, error) {
	sr, err := CreatorRef(pod)
	if err != nil {
		return "", err
	}
	if sr == nil {
		return "", nil
	}
	return sr.Reference.Kind, nil
}

// CreatorRefKind returns the kind of the creator reference of the pod.
func CreatorRef(pod *api.Pod) (*api.SerializedReference, error) {
	creatorRef, found := pod.ObjectMeta.Annotations[controller.CreatedByAnnotation]
	if !found {
		return nil, nil
	}
	var sr api.SerializedReference
	if err := runtime.DecodeInto(api.Codecs.UniversalDecoder(), []byte(creatorRef), &sr); err != nil {
		return nil, err
	}
	return &sr, nil
}

// IsMirrorPod checks whether the pod is a mirror pod.
func IsMirrorPod(pod *api.Pod) bool {
	_, found := pod.ObjectMeta.Annotations[types.ConfigMirrorAnnotationKey]
	return found
}

func HasLocalStorage(pod *api.Pod) bool {
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

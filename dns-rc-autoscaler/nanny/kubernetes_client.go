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

package nanny

import (
	"encoding/json"
	"fmt"
	"log"

	api "k8s.io/kubernetes/pkg/api"
	client "k8s.io/kubernetes/pkg/client/clientset_generated/release_1_3"
)

const replicationControllerKind = "ReplicationController"
const createdByAnnotationName = "kubernetes.io/created-by"

type kubernetesClient struct {
	namespace, pod string
	clientset      *client.Clientset
}

// Find this pod's owner references, find the replication controller parent and return its name
// This is acquired from the pod's annotation "kubernetes.io/created-by"
//  annotations:
//    kubernetes.io/created-by: |
//      {"kind":"SerializedReference","apiVersion":"v1","reference":
//      {"kind":"ReplicationController","namespace":"default",
//      "name":"pod-autoscaler-nanny",
//      "uid":"7ccf55b9-5073-11e6-8321-42010af00002",
//      "apiVersion":"v1",
//      "resourceVersion":"20339"}}
//

func (k *kubernetesClient) GetParentRc(namespace, podname string) (string, error) {
	pod, err := k.clientset.CoreClient.Pods(namespace).Get(podname)
	if err != nil {
		return "", err
	}
	createdBy, ok := pod.ObjectMeta.Annotations[createdByAnnotationName]
	if !ok || len(createdBy) == 0 {
		// Empty or missing created-by annotation would mean this is a
		// standalone pod and does not have an RC/RS managing it
		return "", fmt.Errorf("Pod does not have a created-by annotation")
	}
	type RCInner struct {
		Kind string `json:"kind"`
		Name string `json:"name"`
	}
	type AnnotationT struct {
		Reference RCInner `json:"reference"`
	}
	var annot AnnotationT
	err = json.Unmarshal([]byte(createdBy), &annot)
	if err != nil {
		return "", fmt.Errorf("Failed to parse created-by annotation (%s)", err)
	}
	if annot.Reference.Kind != "ReplicationController" {
		return "", fmt.Errorf("This pod %s/%s was not created by a replication controller", namespace, podname)
	}
	return annot.Reference.Name, nil
}

// Count schedulable nodes in our cluster
func (k *kubernetesClient) CountNodes() (uint64, uint64, error) {
	opt := api.ListOptions{Watch: false}

	nodes, err := k.clientset.CoreClient.Nodes().List(opt)
	if err != nil {
		return 0, 0, err
	}
	var schedulableNodes uint64
	var totalNodes = uint64(len(nodes.Items))
	for _, node := range nodes.Items {
		if !node.Spec.Unschedulable {
			schedulableNodes = schedulableNodes + 1
		}
	}
	return totalNodes, schedulableNodes, nil
}

// Get number of replicas configured in the parent replication controller
func (k *kubernetesClient) PodReplicas() (uint64, error) {
	replicationController, err := k.GetParentRc(k.namespace, k.pod)
	if err != nil {
		return 0, err
	}
	rc, err := k.clientset.CoreClient.ReplicationControllers(k.namespace).Get(replicationController)
	if err != nil {
		return 0, err
	}
	return uint64(*rc.Spec.Replicas), nil
}

// Update the number of replicas in the parent replication controller
func (k *kubernetesClient) UpdateReplicas(replicas uint64) error {
	// First, get the parent replication controller.
	replicationController, err := k.GetParentRc(k.namespace, k.pod)
	if err != nil {
		return err
	}
	rc, err := k.clientset.CoreClient.ReplicationControllers(k.namespace).Get(replicationController)
	if err != nil {
		return err
	}
	*rc.Spec.Replicas = int32(replicas)
	if *rc.Spec.Replicas == 0 {
		log.Fatalf("Cannot update to 0 replicas")
	}
	_, err = k.clientset.CoreClient.ReplicationControllers(k.namespace).Update(rc)
	if err != nil {
		return err
	}
	return nil
}

// NewKubernetesClient gives a KubernetesClient with the given dependencies.
func NewKubernetesClient(namespace, pod string, clientset *client.Clientset) KubernetesClient {
	return &kubernetesClient{
		namespace: namespace,
		pod:       pod,
		clientset: clientset,
	}
}

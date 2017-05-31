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

/*
Package nanny implements logic to poll the k8s apiserver for cluster status,
and update a deployment based on that status.
*/
package nanny

import (
	"log"
	"time"
)

// KubernetesClient is an object that performs the nanny's requisite interactions with Kubernetes.
type KubernetesClient interface {
	CountNodes() (uint64, uint64, error)
	PodReplicas() (uint64, error)
	UpdateReplicas(uint64) error
	GetParentRc(namespace, podname string) (string, error)
}

// PollAPIServer periodically counts the number of nodes, estimates the expected
// number of replicas, compares them to the actual replicas, and
// updates the parent ReplicationController with the expected replicas if necessary.
func PollAPIServer(k8s KubernetesClient, scaler Scaler, pollPeriod time.Duration, verbose bool) {
	for i := 0; true; i++ {
		if i != 0 {
			// Sleep for the poll period.
			time.Sleep(pollPeriod)
		}

		// Query the apiserver for the number of nodes.
		total, num, err := k8s.CountNodes()
		if err != nil {
			log.Println(err)
			continue
		}
		if verbose {
			log.Printf("The number of nodes is %d, schedulable nodes: %d\n", total, num)
		}

		// Query the apiserver for this pod's information.
		replicas, err := k8s.PodReplicas()
		if err != nil {
			log.Printf("Error while querying apiserver for pod replicas: %v\n", err)
			continue
		}
		if verbose {
			log.Printf("There are %d pod replicas\n", replicas)
		}

		// Get the expected resourcereplicas
		expReplicas := scaler.scaleWithNodes(num)
		if verbose {
			log.Printf("The expected number of replicas is %d\n", expReplicas)
		}

		if expReplicas < 1 {
			log.Fatalf("Cannot scale to replica count of %d\n", expReplicas)
		}

		// If there's a difference, go ahead and set the new values.
		if replicas == expReplicas {
			if verbose {
				log.Println("Replicas are within the expected limits.")
			}
			continue
		}
		log.Printf("Replicas are not within the expected limits: updating the parent replication controller to %d replicas\n",
			expReplicas)
		if err := k8s.UpdateReplicas(expReplicas); err != nil {
			log.Println(err)
			continue
		}
	}
}

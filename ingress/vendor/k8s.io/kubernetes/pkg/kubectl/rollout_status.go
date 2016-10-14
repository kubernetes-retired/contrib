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

package kubectl

import (
	"fmt"

	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	extensionsclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/extensions/unversioned"
)

// StatusViewer provides an interface for resources that provides rollout status.
type StatusViewer interface {
	Status(namespace, name string) (string, bool, error)
}

func StatusViewerFor(kind unversioned.GroupKind, c internalclientset.Interface) (StatusViewer, error) {
	switch kind {
	case extensions.Kind("Deployment"):
		return &DeploymentStatusViewer{c.Extensions()}, nil
	}
	return nil, fmt.Errorf("no status viewer has been implemented for %v", kind)
}

type DeploymentStatusViewer struct {
	c extensionsclient.DeploymentsGetter
}

// Status returns a message describing deployment status, and a bool value indicating if the status is considered done
func (s *DeploymentStatusViewer) Status(namespace, name string) (string, bool, error) {
	deployment, err := s.c.Deployments(namespace).Get(name)
	if err != nil {
		return "", false, err
	}
	if deployment.Generation <= deployment.Status.ObservedGeneration {
		if deployment.Status.UpdatedReplicas < deployment.Spec.Replicas {
			return fmt.Sprintf("Waiting for rollout to finish: %d out of %d new replicas have been updated...\n", deployment.Status.UpdatedReplicas, deployment.Spec.Replicas), false, nil
		}
		if deployment.Status.Replicas > deployment.Status.UpdatedReplicas {
			return fmt.Sprintf("Waiting for rollout to finish: %d old replicas are pending termination...\n", deployment.Status.Replicas-deployment.Status.UpdatedReplicas), false, nil
		}
		if deployment.Status.AvailableReplicas < deployment.Status.UpdatedReplicas {
			return fmt.Sprintf("Waiting for rollout to finish: %d of %d updated replicas are available...\n", deployment.Status.AvailableReplicas, deployment.Status.UpdatedReplicas), false, nil
		}
		return fmt.Sprintf("deployment %s successfully rolled out\n", name), true, nil
	}
	return fmt.Sprintf("Waiting for deployment spec update to be observed...\n"), false, nil
}

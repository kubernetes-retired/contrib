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

package controller

const (
	// these pair of constants are used by the provisioner.
	// The key is a kube namespaced key that denotes a ingress service requires provisioning.
	// The value is set only when provisioning is completed.  Any other value will tell the provisioner
	// that provisioning has not yet occurred.
	ingressProvisioningRequiredAnnotationKey    = "ingress.alpha.k8s.io/provisioning-required"
	ingressProvisioningCompletedAnnotationValue = "ingress.alpha.k8s.io/provisioning-completed"
	ingressProvisioningFailedAnnotationValue 	= "ingress.alpha.k8s.io/provisioning-failed"

	IngressProvisioningClassKey   = "ingress.alpha.k8s.io/ingress-class"

	ingressParameterCPUKey = "ingress.alpha.k8s.io/ingress-cpu"
	ingressParameterMEMKey = "ingress.alpha.k8s.io/ingress-mem"
	IngressParameterVIPKey = "ingress.alpha.k8s.io/ingress-vip"
)


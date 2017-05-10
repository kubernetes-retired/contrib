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

import (
	"fmt"
	"math/rand"
	"time"

	"k8s.io/client-go/1.5/pkg/api/resource"
	"k8s.io/client-go/1.5/pkg/api/v1"
	"k8s.io/contrib/ingress-admin/loadbalancer-controller/api"
)

func isProvisioningNeeded(annotation map[string]string) bool {
	if annotation == nil {
		return false
	}
	return annotation[IngressProvisioningClassKey] != "" &&
		annotation[ingressProvisioningRequiredAnnotationKey] != "" &&
		annotation[ingressProvisioningRequiredAnnotationKey] != ingressProvisioningCompletedAnnotationValue &&
		annotation[ingressProvisioningRequiredAnnotationKey] != ingressProvisioningFailedAnnotationValue
}

func getResourceList(annotation map[string]string) (*v1.ResourceList, error) {
	if annotation == nil {
		return nil, fmt.Errorf("annotation is nil")
	}
	if _, exist := annotation[ingressParameterCPUKey]; !exist {
		return nil, fmt.Errorf("cpu is not specified")
	}
	if _, exist := annotation[ingressParameterMEMKey]; !exist {
		return nil, fmt.Errorf("mem is not specified")
	}
	cpu, err := resource.ParseQuantity(annotation[ingressParameterCPUKey])
	if err != nil {
		return nil, fmt.Errorf("can not parse cpu")
	}

	mem, err := resource.ParseQuantity(annotation[ingressParameterMEMKey])
	if err != nil {
		return nil, fmt.Errorf("can not parse mem")
	}
	return &v1.ResourceList{
		v1.ResourceCPU:    cpu,
		v1.ResourceMemory: mem,
	}, nil
}

func generateLoadBalancerName(claim *api.LoadBalancerClaim) string {
	return claim.Name + "-aotoprovision-" + randStringBytesRmndr(4)
}

// randStringBytesRmndr returns a randome string.
func randStringBytesRmndr(n int) string {
	rand.Seed(int64(time.Now().Nanosecond()))
	var letterBytes = "abcdefghijklmnopqrstuvwxyz1234567890"
	b := make([]byte, n)
	b[0] = letterBytes[rand.Int63()%26]
	for i := 1; i < n; i++ {
		b[i] = letterBytes[rand.Int63()%int64(len(letterBytes))]
	}
	return string(b)
}

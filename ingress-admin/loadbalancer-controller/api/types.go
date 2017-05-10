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
package api

import (
	"k8s.io/client-go/1.5/pkg/api/unversioned"
	"k8s.io/client-go/1.5/pkg/api/v1"
)

type LoadBalancer struct{
	unversioned.TypeMeta `json:",inline"`
	v1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LoadBalancerSpec   `json:"spec,omitempty"`
	Status LoadBalancerStatus `json:"status,omitempty"`
}

type LoadBalancerSpec struct {
	NginxLoadBalancer *NginxLoadBalancer `json:"nginxLoadBalancer,omitempty"`
	//HaproxyLoadBalancer *HaproxyLoadBalancer
	//AliyunLoadBalancer *AliyunLoadBalancer
	//AnchnetLoadBalancer *AnchnetLoadBalancer
}

type NginxLoadBalancer struct {
	Service v1.ObjectReference `json:"service,omitempty"`
}

type LoadBalancerStatus struct {
	Phase LoadBalancerPhase `json:"phase,omitempty"`
	Message string `json:"message,omitempty"`
	Reason string `json:"reason,omitempty"`
}

type LoadBalancerPhase string

const (
	LoadBalancerAvailable LoadBalancerPhase = "Available"
	LoadBalancerBound LoadBalancerPhase = "Bound"
	LoadBalancerReleased LoadBalancerPhase = "Released"
	LoadBalancerFailed LoadBalancerPhase = "Failed"
)

type LoadBalancerClaim struct {
	unversioned.TypeMeta `json:",inline"`
	v1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LoadBalancerClaimSpec   `json:"spec,omitempty"`
	Status LoadBalancerClaimStatus `json:"status,omitempty"`
}

type LoadBalancerClaimSpec struct {
	// the binding reference to the LoadBalancer backing this claim.
	LoadBalancerName string	`json:"loadBalancerName,omitempty"`
}

type LoadBalancerClaimStatus struct {
	Phase LoadBalancerClaimPhase `json:"phase,omitempty"`
	Message string `json:"message,omitempty"`
}

type LoadBalancerClaimPhase string

const (
	LoadBalancerClaimPending LoadBalancerClaimPhase = "Pending"
	LoadBalancerClaimBound LoadBalancerClaimPhase = "Bound"
	LoadBalancerClaimFailed LoadBalancerClaimPhase = "Failed"
)


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

package eviction

import (
	"time"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	statsapi "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/stats"
)

// Signal defines a signal that can trigger eviction of pods on a node.
type Signal string

const (
	// SignalMemoryAvailable is memory available (i.e. capacity - workingSet), in bytes.
	SignalMemoryAvailable Signal = "memory.available"
)

// ThresholdOperator is the operator used to express a Threshold.
type ThresholdOperator string

const (
	// OpLessThan is the operator that expresses a less than operator.
	OpLessThan ThresholdOperator = "LessThan"
)

// Config holds information about how eviction is configured.
type Config struct {
	// PressureTransitionPeriod is duration the kubelet has to wait before transititioning out of a pressure condition.
	PressureTransitionPeriod time.Duration
	// Maximum allowed grace period (in seconds) to use when terminating pods in response to a soft eviction threshold being met.
	MaxPodGracePeriodSeconds int64
	// Thresholds define the set of conditions monitored to trigger eviction.
	Thresholds []Threshold
}

// Threshold defines a metric for when eviction should occur.
type Threshold struct {
	// Signal defines the entity that was measured.
	Signal Signal
	// Operator represents a relationship of a signal to a value.
	Operator ThresholdOperator
	// value is a quantity associated with the signal that is evaluated against the specified operator.
	Value *resource.Quantity
	// GracePeriod represents the amount of time that a threshold must be met before eviction is triggered.
	GracePeriod time.Duration
}

// Manager evaluates when an eviction threshold for node stability has been met on the node.
type Manager interface {
	// Start starts the control loop to monitor eviction thresholds at specified interval.
	Start(podFunc ActivePodsFunc, monitoringInterval time.Duration)

	// IsUnderMemoryPressure returns true if the node is under memory pressure.
	IsUnderMemoryPressure() bool
}

// KillPodFunc kills a pod.
// The pod status is updated, and then it is killed with the specified grace period.
// This function must block until either the pod is killed or an error is encountered.
// Arguments:
// pod - the pod to kill
// status - the desired status to associate with the pod (i.e. why its killed)
// gracePeriodOverride - the grace period override to use instead of what is on the pod spec
type KillPodFunc func(pod *api.Pod, status api.PodStatus, gracePeriodOverride *int64) error

// ActivePodsFunc returns pods bound to the kubelet that are active (i.e. non-terminal state)
type ActivePodsFunc func() []*api.Pod

// statsFunc returns the usage stats if known for an input pod.
type statsFunc func(pod *api.Pod) (statsapi.PodStats, bool)

// rankFunc sorts the pods in eviction order
type rankFunc func(pods []*api.Pod, stats statsFunc)

// signalObservations maps a signal to an observed quantity
type signalObservations map[Signal]*resource.Quantity

// thresholdsObservedAt maps a threshold to a time that it was observed
type thresholdsObservedAt map[Threshold]time.Time

// nodeConditionsObservedAt maps a node condition to a time that it was observed
type nodeConditionsObservedAt map[api.NodeConditionType]time.Time

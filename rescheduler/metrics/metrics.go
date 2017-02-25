/*
Copyright 2017 The Kubernetes Authors.

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

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// UnschedulableCriticalPodsCount tracks the number of time when a critical pod was unschedublable.
	UnschedulableCriticalPodsCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "rescheduler",
			Name:      "unschedulable_ciritical_pods_count",
			Help:      "Number of times a critical pod was unschedulable.",
		},
		[]string{"k8s_app"})
	// DeletedPodsCount tracks the number of deletion of pods in order to schedule a critical one.
	DeletedPodsCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "rescheduler",
			Name:      "deleted_pods_count",
			Help:      "Number of pods deleted in order to schedule a critical pod.",
		})
)

func init() {
	prometheus.MustRegister(UnschedulableCriticalPodsCount)
	prometheus.MustRegister(DeletedPodsCount)
}

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

package stackdriver

import (
	"github.com/prometheus/client_golang/prometheus"

	"k8s.io/contrib/fluentd/event-exporter/sinks"
)

// SdSinkName is a name of the Stackdriver sink, by which the appropriate
// SinkFactory can be queried from the KnownSinks.
const SdSinkName = "stackdriver"

func init() {
	prometheus.MustRegister(
		receivedEntryCount,
		successfullySentEntryCount,
		requestCount,
	)

	sinks.KnownSinks[SdSinkName] = newSdSinkFactory()
}

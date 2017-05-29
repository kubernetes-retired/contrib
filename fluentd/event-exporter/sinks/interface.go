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

package sinks

import (
	api_v1 "k8s.io/client-go/pkg/api/v1"
	"k8s.io/contrib/fluentd/event-exporter/watchers/events"
)

var (
	// KnownSinks contains list of all known sinks that can be used. In order to
	// make the new sink available, init function has to add an appropriate
	// sink factory to this map.
	KnownSinks = make(map[string]SinkFactory)
)

// Sink interface represents a generic sink that is responsible for handling
// actions upon the event objects and filter the initial events list.
type Sink interface {
	events.EventHandler

	OnList(*api_v1.EventList)

	Run(stopCh <-chan struct{})
}

// SinkFactory creates a new sink, using user-provided parameters.
type SinkFactory interface {
	CreateNew(opts []string) (Sink, error)
}

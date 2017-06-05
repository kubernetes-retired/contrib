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

package events

import (
	"time"

	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	api_v1 "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/contrib/fluentd/event-exporter/watchers"
)

const (
	// Since events live in the apiserver only for 1 hour, we have to remove
	// old objects to avoid memory leaks. If TTL is exactly 1 hour, race
	// can occur in case of the event being updated right before the end of
	// the hour, since it takes some time to deliver this event via watch.
	// 2 hours ought to be enough for anybody.
	eventStorageTTL = 2 * time.Hour
)

// OnListFunc represent an action on the initial list of object received
// from the Kubernetes API server before starting watching for the updates.
type OnListFunc func(*api_v1.EventList)

// EventWatcherConfig represents the configuration for the watcher that
// only watches the events resource.
type EventWatcherConfig struct {
	// Note, that this action will be executed on each List request, of which
	// there can be many, e.g. because of network problems. Note also, that
	// items in the List response WILL NOT trigger OnAdd method in handler,
	// instead Store contents will be completely replaced.
	OnList       OnListFunc
	ResyncPeriod time.Duration
	Handler      EventHandler
	// If true, watcher will initialize a cache to process event object updates
	// as OnUpdate actions, with populating previous events. Otherwise, store
	// will be faked, so that each update will trigger OnAdd method.
	StoreEvents bool
}

// NewEventWatcher create a new watcher that only watches the events resource.
func NewEventWatcher(client kubernetes.Interface, config *EventWatcherConfig) watchers.Watcher {
	storageType := watchers.FakeStorage
	if config.StoreEvents {
		storageType = watchers.TTLStorage
	}

	return watchers.NewWatcher(&watchers.WatcherConfig{
		ListerWatcher: &cache.ListWatch{
			ListFunc: func(options meta_v1.ListOptions) (runtime.Object, error) {
				list, err := client.CoreV1().Events(meta_v1.NamespaceAll).List(options)
				if err == nil {
					config.OnList(list)
				}
				return list, err
			},
			WatchFunc: func(options meta_v1.ListOptions) (watch.Interface, error) {
				return client.CoreV1().Events(meta_v1.NamespaceAll).Watch(options)
			},
		},
		ExpectedType: &api_v1.Event{},
		StoreConfig: &watchers.WatcherStoreConfig{
			KeyFunc:     cache.DeletionHandlingMetaNamespaceKeyFunc,
			Handler:     newEventHandlerWrapper(config.Handler),
			StorageType: storageType,
			StorageTTL:  eventStorageTTL,
		},
		ResyncPeriod: config.ResyncPeriod,
	})
}

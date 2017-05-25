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
	eventStorageTTL = 2 * time.Hour
)

// FilterListFunc represent an action on the initial list of object received
// from the Kubernetes API server before starting watching for the updates.
type FilterListFunc func([]api_v1.Event) []api_v1.Event

// EventWatcherConfig represents the configuration for the watcher that
// only watches the events resource.
type EventWatcherConfig struct {
	FilterListFunc FilterListFunc
	ResyncPeriod   time.Duration
	Consumer       EventWatchConsumer
}

// NewEventWatcher create a new watcher that only watches the events resource.
func NewEventWatcher(client kubernetes.Interface, config *EventWatcherConfig) watchers.Watcher {
	return watchers.NewWatcher(&watchers.WatcherConfig{
		ListerWatcher: &cache.ListWatch{
			ListFunc: func(options meta_v1.ListOptions) (runtime.Object, error) {
				list, err := client.CoreV1().Events(meta_v1.NamespaceAll).List(options)
				list.Items = config.FilterListFunc(list.Items)
				return list, err
			},
			WatchFunc: func(options meta_v1.ListOptions) (watch.Interface, error) {
				return client.CoreV1().Events(meta_v1.NamespaceAll).Watch(options)
			},
		},
		ExpectedType: &api_v1.Event{},
		StoreConfig: &watchers.WatcherStoreConfig{
			KeyFunc:     cache.DeletionHandlingMetaNamespaceKeyFunc,
			Consumer:    newWatchConsumer(config.Consumer),
			StorageType: watchers.TTLStorage,
			StorageTTL:  eventStorageTTL,
		},
		ResyncPeriod: config.ResyncPeriod,
	})
}

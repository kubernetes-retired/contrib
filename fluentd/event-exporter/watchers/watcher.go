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

package watchers

import (
	"time"

	"k8s.io/client-go/tools/cache"
)

// WatcherConfig represents the configuration of the Kubernetes API watcher.
type WatcherConfig struct {
	ListerWatcher cache.ListerWatcher
	ExpectedType  interface{}
	StoreConfig   *WatcherStoreConfig
	ResyncPeriod  time.Duration
}

// Watcher is an interface of the generic proactive API watcher.
type Watcher interface {
	Run(stopCh <-chan struct{})
}

type watcher struct {
	reflector *cache.Reflector
}

func (w *watcher) Run(stopCh <-chan struct{}) {
	w.reflector.Run()
	<-stopCh
}

// NewWatcher creates a new Kubernetes API watcher using provided configuration.
func NewWatcher(config *WatcherConfig) Watcher {
	return &watcher{
		reflector: cache.NewReflector(
			config.ListerWatcher,
			config.ExpectedType,
			newWatcherStore(config.StoreConfig),
			config.ResyncPeriod,
		),
	}
}

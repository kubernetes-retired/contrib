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

// StorageType defines what storage should be used as a cache for the watcher.
type StorageType int

const (
	// SimpleStorage storage type indicates thread-safe map as backing storage.
	SimpleStorage StorageType = iota
	// TTLStorage storage type indicates storage with expiration. When this
	// type of storage is used, TTL should be specified.
	TTLStorage
)

// WatcherStoreConfig represents the configuration of the storage backing the watcher.
type WatcherStoreConfig struct {
	KeyFunc     cache.KeyFunc
	Consumer    WatchConsumer
	StorageType StorageType
	StorageTTL  time.Duration
}

type watcherStore struct {
	cache.Store

	cacheStorage cache.Store
	consumer     WatchConsumer
}

func (s *watcherStore) Add(obj interface{}) error {
	if err := s.cacheStorage.Add(obj); err != nil {
		return err
	}
	s.consumer.Add(obj)
	return nil
}

func (s *watcherStore) Update(obj interface{}) error {
	oldObj, ok, err := s.cacheStorage.Get(obj)
	if err != nil {
		return err
	}
	if !ok {
		oldObj = nil
	}

	if err = s.cacheStorage.Update(obj); err != nil {
		return err
	}
	s.consumer.Update(oldObj, obj)
	return nil
}

func (s *watcherStore) Delete(obj interface{}) error {
	if err := s.cacheStorage.Delete(obj); err != nil {
		return err
	}
	s.consumer.Delete(obj)
	return nil
}

func (s *watcherStore) List() []interface{} {
	return s.cacheStorage.List()
}

func (s *watcherStore) ListKeys() []string {
	return s.cacheStorage.ListKeys()
}

func (s *watcherStore) Get(obj interface{}) (interface{}, bool, error) {
	return s.cacheStorage.Get(obj)
}

func (s *watcherStore) GetByKey(key string) (interface{}, bool, error) {
	return s.cacheStorage.GetByKey(key)
}

func (s *watcherStore) Replace(list []interface{}, rv string) error {
	return s.cacheStorage.Replace(list, rv)
}

func (s *watcherStore) Resync() error {
	return s.cacheStorage.Resync()
}

func newWatcherStore(config *WatcherStoreConfig) *watcherStore {
	var cacheStorage cache.Store
	switch config.StorageType {
	case TTLStorage:
		cacheStorage = cache.NewTTLStore(config.KeyFunc, config.StorageTTL)
		break
	case SimpleStorage:
	default:
		cacheStorage = cache.NewStore(config.KeyFunc)
		break
	}

	return &watcherStore{
		cacheStorage: cacheStorage,
		consumer:     config.Consumer,
	}
}

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

package cache

import (
	"time"

	expirationCache "k8s.io/kubernetes/pkg/client/cache"
)

// ObjectCache is a simple wrapper of expiration cache that
// 1. use string type key
// 2. has a updater to get value directly if it is expired
// 3. then update the cache
type ObjectCache struct {
	cache   expirationCache.Store
	updater func() (interface{}, error)
}

// objectEntry is a object with string type key.
type objectEntry struct {
	key string
	obj interface{}
}

// NewObjectCache creates ObjectCache with a updater.
// updater returns a object to cache.
func NewObjectCache(f func() (interface{}, error), ttl time.Duration) *ObjectCache {
	return &ObjectCache{
		updater: f,
		cache:   expirationCache.NewTTLStore(stringKeyFunc, ttl),
	}
}

// stringKeyFunc is a string as cache key function
func stringKeyFunc(obj interface{}) (string, error) {
	key := obj.(objectEntry).key
	return key, nil
}

// Get gets cached objectEntry by using a unique string as the key.
func (c *ObjectCache) Get(key string) (interface{}, error) {
	value, ok, err := c.cache.Get(objectEntry{key: key})
	if err != nil {
		return nil, err
	}
	if !ok {
		obj, err := c.updater()
		if err != nil {
			return nil, err
		}
		err = c.cache.Add(objectEntry{
			key: key,
			obj: obj,
		})
		if err != nil {
			return nil, err
		}
		return obj, nil
	}
	return value.(objectEntry).obj, nil
}

func (c *ObjectCache) Add(key string, obj interface{}) error {
	err := c.cache.Add(objectEntry{key: key, obj: obj})
	if err != nil {
		return err
	}
	return nil
}

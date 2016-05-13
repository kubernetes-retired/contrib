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

package storagebackend

import (
	"fmt"

	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"
)

const (
	StorageTypeUnset = ""
	StorageTypeETCD2 = "etcd2"
	StorageTypeETCD3 = "etcd3"
)

// Config is configuration for creating a storage backend.
type Config struct {
	// Type defines the type of storage backend, e.g. "etcd2", etcd3". Default ("") is "etcd2".
	Type string
	// Codec is used to serialize/deserialize objects.
	Codec runtime.Codec
	// Prefix is the prefix to all keys passed to storage.Interface methods.
	Prefix string
	// ServerList is the list of storage servers to connect with.
	ServerList []string
	// TLS credentials
	KeyFile  string
	CertFile string
	CAFile   string
	// Quorum indicates that whether read operations should be quorum-level consistent.
	Quorum bool
	// DeserializationCacheSize is the size of cache of deserialized objects.
	// Currently this is only supported in etcd2.
	// We will drop the cache once using protobuf.
	DeserializationCacheSize int
}

// Create creates a storage backend based on given config.
func Create(c Config) (storage.Interface, error) {
	switch c.Type {
	case StorageTypeUnset, StorageTypeETCD2:
		return newETCD2Storage(c)
	case StorageTypeETCD3:
		// TODO: We have the following features to implement:
		// - Support secure connection by using key, cert, and CA files.
		// - Honor "https" scheme to support secure connection in gRPC.
		// - Support non-quorum read.
		return newETCD3Storage(c)
	default:
		return nil, fmt.Errorf("unknown storage type: %s", c.Type)
	}
}

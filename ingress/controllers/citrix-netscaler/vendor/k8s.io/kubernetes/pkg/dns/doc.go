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

// Package DNS provides a backend for the skydns DNS server started by the
// kubedns cluster addon. It exposes the 2 interface method: Records and
// ReverseRecord, which skydns invokes according to the DNS queries it
// receives. It serves these records by consulting an in memory tree
// populated with Kubernetes Services and Endpoints received from the Kubernetes
// API server.
package dns

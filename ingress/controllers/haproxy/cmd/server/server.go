/*
Copyright 2015 The Kubernetes Authors.

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

package server

import (
	"fmt"
	"net/http"

	"k8s.io/kubernetes/pkg/healthz"

	log "github.com/golang/glog"
	"k8s.io/contrib/ingress/controllers/haproxy/pkg/balancer"
)

// StartServer starts the server that exposes the internal balancer API
func StartServer(listen string, port int, m *balancer.Manager) {

	mux := http.NewServeMux()
	healthz.InstallHandler(mux, *m)
	mux.HandleFunc("/version", versionHandler)

	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", listen, port),
		Handler: mux,
	}

	log.Infof("starting Haddock API server at %s:%d", listen, port)
	log.Fatal(server.ListenAndServe())
}

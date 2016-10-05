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
	"net/http"

	log "github.com/golang/glog"
	"k8s.io/contrib/ingress/controllers/haproxy/cmd/model"
)

// ErrOutHandler exits the app with output code 2
func versionHandler(w http.ResponseWriter, r *http.Request) {
	log.Infof("version handler has been requested from: %s", r.RemoteAddr)

	ai := model.GetAppInfo()
	if e := ObjectResponse(200, ai, w); e != nil {
		ErrorResponse("Error sending version", e, w)
	}

}

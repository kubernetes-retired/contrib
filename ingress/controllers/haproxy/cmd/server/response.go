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
	"encoding/json"
	"fmt"
	"net/http"

	log "github.com/golang/glog"
)

// StandardResponse is a standard response message
type StandardResponse struct {
	Message string `json:"message"`
	ID      string `json:"_id,,omitempty"`
}

// ObjectResponse answers a request with a JSON message containing an object
func ObjectResponse(status int, object interface{}, w http.ResponseWriter) error {
	j, e := json.Marshal(object)

	if e != nil {
		return e
	}

	log.Infof("building response. status: %d, object: %s", status, object)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(j)

	return nil
}

// ErrorResponse answers a request with a JSON error
func ErrorResponse(context string, err error, w http.ResponseWriter) {
	message := &StandardResponse{
		Message: fmt.Sprintf("%s: %v", context, err),
	}

	log.Errorf("building error response. status: %d, message: %s", http.StatusInternalServerError, message)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	if e := json.NewEncoder(w).Encode(message); e != nil {
		panic(e)
	}
}

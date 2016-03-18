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

package stats

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"time"

	"github.com/emicklei/go-restful"
	"github.com/golang/glog"
	cadvisorapi "github.com/google/cadvisor/info/v1"

	"k8s.io/kubernetes/pkg/api"
	kubecontainer "k8s.io/kubernetes/pkg/kubelet/container"
	"k8s.io/kubernetes/pkg/types"
)

// Host methods required by stats handlers.
type StatsProvider interface {
	GetContainerInfo(podFullName string, uid types.UID, containerName string, req *cadvisorapi.ContainerInfoRequest) (*cadvisorapi.ContainerInfo, error)
	GetRawContainerInfo(containerName string, req *cadvisorapi.ContainerInfoRequest, subcontainers bool) (map[string]*cadvisorapi.ContainerInfo, error)
	GetPodByName(namespace, name string) (*api.Pod, bool)
}

type handler struct {
	provider StatsProvider
}

func CreateHandlers(provider StatsProvider) *restful.WebService {
	h := &handler{provider}

	ws := &restful.WebService{}
	ws.Path("/stats/").
		Produces(restful.MIME_JSON)

	endpoints := []struct {
		path    string
		handler restful.RouteFunction
	}{
		{"", h.handleStats},
		{"/container", h.handleSystemContainer},
		{"/{podName}/{containerName}", h.handlePodContainer},
		{"/{namespace}/{podName}/{uid}/{containerName}", h.handlePodContainer},
	}

	for _, e := range endpoints {
		for _, method := range []string{"GET", "POST"} {
			ws.Route(ws.
				Method(method).
				Path(e.path).
				To(e.handler))
		}
	}

	return ws
}

type StatsRequest struct {
	// The name of the container for which to request stats.
	// Default: /
	ContainerName string `json:"containerName,omitempty"`

	// Max number of stats to return.
	// If start and end time are specified this limit is ignored.
	// Default: 60
	NumStats int `json:"num_stats,omitempty"`

	// Start time for which to query information.
	// If omitted, the beginning of time is assumed.
	Start time.Time `json:"start,omitempty"`

	// End time for which to query information.
	// If omitted, current time is assumed.
	End time.Time `json:"end,omitempty"`

	// Whether to also include information from subcontainers.
	// Default: false.
	Subcontainers bool `json:"subcontainers,omitempty"`
}

func (r *StatsRequest) cadvisorRequest() *cadvisorapi.ContainerInfoRequest {
	return &cadvisorapi.ContainerInfoRequest{
		NumStats: r.NumStats,
		Start:    r.Start,
		End:      r.End,
	}
}

func parseStatsRequest(request *restful.Request) (StatsRequest, error) {
	// Default request.
	query := StatsRequest{
		NumStats: 60,
	}

	err := json.NewDecoder(request.Request.Body).Decode(&query)
	if err != nil && err != io.EOF {
		return query, err
	}
	return query, nil
}

// Handles root container stats requests to /stats
func (h *handler) handleStats(request *restful.Request, response *restful.Response) {
	query, err := parseStatsRequest(request)
	if err != nil {
		handleError(response, err)
		return
	}

	// Root container stats.
	statsMap, err := h.provider.GetRawContainerInfo("/", query.cadvisorRequest(), false)
	if err != nil {
		handleError(response, err)
		return
	}
	writeResponse(response, statsMap["/"])
}

// Handles non-kubernetes container stats requests to /stats/container/
func (h *handler) handleSystemContainer(request *restful.Request, response *restful.Response) {
	query, err := parseStatsRequest(request)
	if err != nil {
		handleError(response, err)
		return
	}

	// Non-Kubernetes container stats.
	containerName := path.Join("/", query.ContainerName)
	stats, err := h.provider.GetRawContainerInfo(
		containerName, query.cadvisorRequest(), query.Subcontainers)
	if err != nil {
		handleError(response, err)
		return
	}
	writeResponse(response, stats)
}

// Handles kubernetes pod/container stats requests to:
// /stats/<pod name>/<container name>
// /stats/<namespace>/<pod name>/<uid>/<container name>
func (h *handler) handlePodContainer(request *restful.Request, response *restful.Response) {
	query, err := parseStatsRequest(request)
	if err != nil {
		handleError(response, err)
		return
	}

	// Default parameters.
	params := map[string]string{
		"namespace": api.NamespaceDefault,
		"uid":       "",
	}
	for k, v := range request.PathParameters() {
		params[k] = v
	}

	if params["podName"] == "" || params["containerName"] == "" {
		response.WriteErrorString(http.StatusBadRequest,
			fmt.Sprintf("Invalid pod container request: %v", params))
		return
	}

	pod, ok := h.provider.GetPodByName(params["namespace"], params["podName"])
	if !ok {
		glog.V(4).Infof("Container not found: %v", params)
		handleError(response, kubecontainer.ErrContainerNotFound)
		return
	}
	stats, err := h.provider.GetContainerInfo(
		kubecontainer.GetPodFullName(pod),
		types.UID(params["uid"]),
		params["containerName"],
		query.cadvisorRequest())

	if err != nil {
		handleError(response, err)
		return
	}
	writeResponse(response, stats)
}

func writeResponse(response *restful.Response, stats interface{}) {
	if stats == nil {
		return
	}
	err := response.WriteAsJson(stats)
	if err != nil {
		handleError(response, err)
	}
}

// handleError serializes an error object into an HTTP response.
func handleError(response *restful.Response, err error) {
	switch err {
	case kubecontainer.ErrContainerNotFound:
		response.WriteError(http.StatusNotFound, err)
	default:
		msg := fmt.Sprintf("Internal Error: %v", err)
		glog.Infof("HTTP InternalServerError: %s", msg)
		response.WriteErrorString(http.StatusInternalServerError, msg)
	}
}

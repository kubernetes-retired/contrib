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

package stackdriver

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/compute/metadata"
	"github.com/golang/glog"
	sd "google.golang.org/api/logging/v2"
)

const (
	defaultFlushDelay     = 5 * time.Second
	defaultMaxBufferSize  = 100
	defaultMaxConcurrency = 10

	eventsLogName = "events"
)

type sdSinkConfig struct {
	FlushDelay     time.Duration
	MaxBufferSize  int
	MaxConcurrency int
	LogName        string
	Resource       *sd.MonitoredResource
}

func newGceSdSinkConfig() (*sdSinkConfig, error) {
	if !metadata.OnGCE() {
		return nil, errors.New("not running on GCE, which is not supported for Stackdriver sink")
	}

	projectID, err := metadata.ProjectID()
	if err != nil {
		return nil, fmt.Errorf("failed to get project id: %v", err)
	}
	logName := fmt.Sprintf("projects/%s/logs/%s", projectID, eventsLogName)

	clusterName, err := metadata.InstanceAttributeValue("cluster-name")
	if err != nil {
		glog.Warningf("'cluster-name' label is not specified on the VM, defaulting to the empty value")
		clusterName = ""
	}
	clusterName = strings.TrimSpace(clusterName)

	resource := &sd.MonitoredResource{
		Type: "gke_cluster",
		Labels: map[string]string{
			"cluster_name": clusterName,
			// TODO: Replace with the actual zone of the cluster
			"location":   "",
			"project_id": projectID,
		},
	}

	return &sdSinkConfig{
		FlushDelay:     defaultFlushDelay,
		MaxBufferSize:  defaultMaxBufferSize,
		MaxConcurrency: defaultMaxConcurrency,
		LogName:        logName,
		Resource:       resource,
	}, nil
}

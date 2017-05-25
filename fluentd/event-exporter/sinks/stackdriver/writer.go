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
	"net/http"
	"strconv"
	"time"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/api/googleapi"
	sd "google.golang.org/api/logging/v2"
)

const (
	retryDelay = 10 * time.Second
)

var (
	requestCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:      "request_count",
			Help:      "Number of request, issued to Stackdriver API",
			Subsystem: "stackdriver_sink",
		},
		[]string{"code"},
	)
)

type sdWriter interface {
	Write([]*sd.LogEntry, string, *sd.MonitoredResource) int
}

type sdWriterImpl struct {
	service *sd.Service
}

func newSdWriter(service *sd.Service) sdWriter {
	return &sdWriterImpl{
		service: service,
	}
}

func (w sdWriterImpl) Write(entries []*sd.LogEntry, logName string, resource *sd.MonitoredResource) int {
	req := &sd.WriteLogEntriesRequest{
		Entries:  entries,
		LogName:  logName,
		Resource: resource,
	}

	for {
		res, err := w.service.Entries.Write(req).Do()

		if err == nil {
			requestCount.WithLabelValues(strconv.Itoa(res.HTTPStatusCode)).Inc()
			break
		}

		apiErr, ok := err.(*googleapi.Error)
		if ok {
			requestCount.WithLabelValues(strconv.Itoa(apiErr.Code)).Inc()

			// Bad request from Stackdriver most probably indicates that some entries
			// are in bad format, which means they won't be ingested after retry also,
			// so it doesn't make sense to try again.
			// TODO: Check response properly and return the actual number of
			// successfully ingested entries, parsed out from the response body.
			if apiErr.Code == http.StatusBadRequest {
				glog.Warningf("Recieved bad request response from server, "+
					"assuming some entries were rejected: %s", err)
				break
			}
		}

		glog.Warningf("Failed to send request to Stackdriver: %s", err)
		time.Sleep(retryDelay)
	}

	return len(entries)
}

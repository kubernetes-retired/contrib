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
	"sync/atomic"
	"testing"
	"time"

	sd "google.golang.org/api/logging/v2"

	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/apimachinery/pkg/util/wait"
	api_v1 "k8s.io/client-go/pkg/api/v1"
)

type fakeSdWriter struct {
	writeFunc func([]*sd.LogEntry, string, *sd.MonitoredResource) int
}

func (w *fakeSdWriter) Write(entries []*sd.LogEntry, logName string, resource *sd.MonitoredResource) int {
	if w.writeFunc != nil {
		return w.writeFunc(entries, logName, resource)
	}
	return 0
}

func TestMaxConcurrency(t *testing.T) {
	var writeCalledTimes int32
	w := &fakeSdWriter{
		writeFunc: func([]*sd.LogEntry, string, *sd.MonitoredResource) int {
			atomic.AddInt32(&writeCalledTimes, 1)
			time.Sleep(2 * time.Second)
			return 0
		},
	}
	config := &sdSinkConfig{
		Resource:       nil,
		FlushDelay:     100 * time.Millisecond,
		LogName:        "logname",
		MaxConcurrency: 10,
		MaxBufferSize:  10,
	}
	s := newSdSink(w, clock.NewFakeClock(time.Time{}), config)
	go s.Run(wait.NeverStop)

	for i := 0; i < 110; i++ {
		s.OnAdd(&api_v1.Event{})
	}

	if writeCalledTimes != int32(config.MaxConcurrency) {
		t.Fatalf("writeCalledTimes = %d, expected %d", writeCalledTimes, config.MaxConcurrency)
	}
}

func TestBatchTimeout(t *testing.T) {
	var writeCalledTimes int32
	w := &fakeSdWriter{
		writeFunc: func([]*sd.LogEntry, string, *sd.MonitoredResource) int {
			atomic.AddInt32(&writeCalledTimes, 1)
			return 0
		},
	}
	config := &sdSinkConfig{
		Resource:       nil,
		FlushDelay:     100 * time.Millisecond,
		LogName:        "logname",
		MaxConcurrency: 10,
		MaxBufferSize:  10,
	}
	s := newSdSink(w, clock.NewFakeClock(time.Time{}), config)
	go s.Run(wait.NeverStop)

	s.OnAdd(&api_v1.Event{})
	time.Sleep(200 * time.Millisecond)

	if writeCalledTimes != 1 {
		t.Fatalf("writeCalledTimes = %d, expected 1", writeCalledTimes)
	}
}

func TestBatchSizeLimit(t *testing.T) {
	var writeCalledTimes int32
	w := &fakeSdWriter{
		writeFunc: func([]*sd.LogEntry, string, *sd.MonitoredResource) int {
			atomic.AddInt32(&writeCalledTimes, 1)
			return 0
		},
	}
	config := &sdSinkConfig{
		Resource:       nil,
		FlushDelay:     1 * time.Second,
		LogName:        "logname",
		MaxConcurrency: 10,
		MaxBufferSize:  10,
	}
	s := newSdSink(w, clock.NewFakeClock(time.Time{}), config)
	go s.Run(wait.NeverStop)

	for i := 0; i < 15; i++ {
		s.OnAdd(&api_v1.Event{})
	}

	time.Sleep(100 * time.Millisecond)

	if writeCalledTimes != 1 {
		t.Fatalf("writeCalledTimes = %d, expected 1", writeCalledTimes)
	}
}

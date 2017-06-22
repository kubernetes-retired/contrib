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
	"time"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	sd "google.golang.org/api/logging/v2"

	"k8s.io/apimachinery/pkg/util/clock"
	api_v1 "k8s.io/client-go/pkg/api/v1"
)

var (
	receivedEntryCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:      "received_entry_count",
			Help:      "Number of entries, recieved by the Stackdriver sink",
			Subsystem: "stackdriver_sink",
		},
		[]string{"component", "host"},
	)

	successfullySentEntryCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name:      "successfully_sent_entry_count",
			Help:      "Number of entries, successfully ingested by Stackdriver",
			Subsystem: "stackdriver_sink",
		},
	)
)

type sdSink struct {
	logEntryChannel chan *sd.LogEntry
	config          *sdSinkConfig
	logEntryFactory *sdLogEntryFactory
	writer          sdWriter
	logName         string

	currentBuffer   []*sd.LogEntry
	timer           *time.Timer
	fakeTimeChannel chan time.Time
	// Channel for controlling how many requests are being sent at the same
	// time. It's empty initially, each request adds an object at the start
	// and takes it out upon completion. Channel's capacity is set to the
	// maximum level of parallelism, so any extra request will lock on addition.
	concurrencyChannel chan struct{}

	beforeFirstList bool
}

func newSdSink(writer sdWriter, clock clock.Clock, config *sdSinkConfig) *sdSink {
	return &sdSink{
		logEntryChannel: make(chan *sd.LogEntry, config.MaxBufferSize),
		config:          config,
		logEntryFactory: newSdLogEntryFactory(clock),
		writer:          writer,
		logName:         config.LogName,

		currentBuffer:      []*sd.LogEntry{},
		timer:              nil,
		fakeTimeChannel:    make(chan time.Time),
		concurrencyChannel: make(chan struct{}, config.MaxConcurrency),

		beforeFirstList: true,
	}
}

func (s *sdSink) OnAdd(event *api_v1.Event) {
	receivedEntryCount.WithLabelValues(event.Source.Component, event.Source.Host).Inc()

	logEntry := s.logEntryFactory.FromEvent(event)
	s.logEntryChannel <- logEntry
}

func (s *sdSink) OnUpdate(oldEvent *api_v1.Event, newEvent *api_v1.Event) {
	if newEvent.Count != oldEvent.Count+1 {
		// Sink doesn't send a LogEntry to Stackdriver, b/c event compression might
		// indicate that part of the watch history was lost, which may result in
		// multiple events being compressed. This may create an unecessary
		// flood in Stackdriver.
		glog.V(2).Infof("Event count has increased by %d != 1.\n"+
			"\tOld event: %+v\n\tNew event: %+v", newEvent.Count-oldEvent.Count, oldEvent, newEvent)
	}

	receivedEntryCount.WithLabelValues(newEvent.Source.Component, newEvent.Source.Host).Inc()

	logEntry := s.logEntryFactory.FromEvent(newEvent)
	s.logEntryChannel <- logEntry
}

func (s *sdSink) OnDelete(*api_v1.Event) {
	// Nothing to do here
}

func (s *sdSink) OnList(list *api_v1.EventList) {
	if s.beforeFirstList {
		s.logEntryChannel <- s.logEntryFactory.FromMessage("Event exporter started watching. " +
			"Some events may have been lost up to this point.")
		s.beforeFirstList = false
	}
}

func (s *sdSink) Run(stopCh <-chan struct{}) {
	glog.Info("Starting Stackdriver sink")
	for {
		select {
		case entry := <-s.logEntryChannel:
			s.currentBuffer = append(s.currentBuffer, entry)
			if len(s.currentBuffer) >= s.config.MaxBufferSize {
				s.flushBuffer()
			} else if len(s.currentBuffer) == 1 {
				s.setTimer()
			}
			break
		case <-s.getTimerChannel():
			s.flushBuffer()
			break
		case <-stopCh:
			glog.Info("Stackdriver sink recieved stop signal, waiting for all requests to finish")
			for i := 0; i < s.config.MaxConcurrency; i++ {
				s.concurrencyChannel <- struct{}{}
			}
			glog.Info("All requests to Stackdriver finished, exiting Stackdriver sink")
			return
		}
	}
}

func (s *sdSink) flushBuffer() {
	entries := s.currentBuffer
	s.currentBuffer = nil
	s.concurrencyChannel <- struct{}{}
	go s.sendEntries(entries)
}

func (s *sdSink) sendEntries(entries []*sd.LogEntry) {
	glog.V(4).Infof("Sending %d entries to Stackdriver", len(entries))

	written := s.writer.Write(entries, s.logName, s.config.Resource)
	successfullySentEntryCount.Add(float64(written))

	<-s.concurrencyChannel

	glog.V(4).Infof("Successfully sent %d entries to Stackdriver", len(entries))
}

func (s *sdSink) getTimerChannel() <-chan time.Time {
	if s.timer == nil {
		return s.fakeTimeChannel
	}
	return s.timer.C
}

func (s *sdSink) setTimer() {
	if s.timer == nil {
		s.timer = time.NewTimer(s.config.FlushDelay)
	} else {
		s.timer.Reset(s.config.FlushDelay)
	}
}

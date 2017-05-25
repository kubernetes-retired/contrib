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

package events

import (
	"github.com/golang/glog"

	api_v1 "k8s.io/client-go/pkg/api/v1"
	"k8s.io/contrib/fluentd/event-exporter/watchers"
)

// EventWatchConsumer interface provides a way to act upon signals from
// watcher that only watches the events resource.
type EventWatchConsumer interface {
	Add(*api_v1.Event)
	Update(*api_v1.Event, *api_v1.Event)
	Delete(*api_v1.Event)
}

type eventWatchConsumer struct {
	consumer EventWatchConsumer
}

func newWatchConsumer(consumer EventWatchConsumer) watchers.WatchConsumer {
	return &eventWatchConsumer{
		consumer: consumer,
	}
}

func (c *eventWatchConsumer) Add(obj interface{}) {
	if event, ok := c.convert(obj); ok {
		c.consumer.Add(event)
	}
}

func (c *eventWatchConsumer) Update(oldObj interface{}, newObj interface{}) {
	oldEvent, oldOk := c.convert(oldObj)
	newEvent, newOk := c.convert(newObj)
	if oldOk && newOk {
		c.consumer.Update(oldEvent, newEvent)
	}
}

func (c *eventWatchConsumer) Delete(obj interface{}) {
	if event, ok := c.convert(obj); ok {
		c.consumer.Delete(event)
	}
}

func (c *eventWatchConsumer) convert(obj interface{}) (*api_v1.Event, bool) {
	if event, ok := obj.(*api_v1.Event); ok {
		return event, true
	}
	glog.V(2).Infof("Event watch consumer recieved not event, but %v", obj)
	return nil, false
}

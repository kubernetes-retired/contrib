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
	"testing"

	api_v1 "k8s.io/client-go/pkg/api/v1"
)

type fakeEventConsumer struct {
	EventWatchConsumer

	addFunc    func(*api_v1.Event)
	updateFunc func(*api_v1.Event, *api_v1.Event)
	deleteFunc func(*api_v1.Event)
}

func (c *fakeEventConsumer) Add(event *api_v1.Event) {
	if c.addFunc != nil {
		c.addFunc(event)
	}
}

func (c *fakeEventConsumer) Update(oldEvent, newEvent *api_v1.Event) {
	if c.updateFunc != nil {
		c.updateFunc(oldEvent, newEvent)
	}
}

func (c *fakeEventConsumer) Delete(event *api_v1.Event) {
	if c.deleteFunc != nil {
		c.deleteFunc(event)
	}
}

func TestEventWatchConsumerAdd(t *testing.T) {
	testCases := []struct {
		desc     string
		obj      interface{}
		expected bool
	}{
		{
			"obj=nil",
			nil,
			false,
		},
		{
			"obj=non-event",
			42,
			false,
		},
		{
			"obj=event",
			&api_v1.Event{},
			true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			isTriggered := false
			fakeConsumer := &fakeEventConsumer{
				addFunc: func(*api_v1.Event) { isTriggered = true },
			}

			c := newWatchConsumer(fakeConsumer)
			c.Add(tc.obj)

			if isTriggered != tc.expected {
				t.Fatalf("Add is triggered = %v, expected %v", isTriggered, tc.expected)
			}
		})
	}
}

func TestEventWatchConsumerUpdate(t *testing.T) {
	testCases := []struct {
		desc     string
		oldObj   interface{}
		newObj   interface{}
		expected bool
	}{
		{
			"oldObj=nil,newObj=event",
			nil,
			&api_v1.Event{},
			false,
		},
		{
			"oldObj=non-event,newObj=event",
			42,
			&api_v1.Event{},
			false,
		},
		{
			"oldObj=event,newObj=nil",
			&api_v1.Event{},
			nil,
			false,
		},
		{
			"oldObj=event,newObj=non-event",
			&api_v1.Event{},
			42,
			false,
		},
		{
			"oldObj=event,newObj=event",
			&api_v1.Event{},
			&api_v1.Event{},
			true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			isTriggered := false
			fakeConsumer := &fakeEventConsumer{
				updateFunc: func(*api_v1.Event, *api_v1.Event) { isTriggered = true },
			}

			c := newWatchConsumer(fakeConsumer)
			c.Update(tc.oldObj, tc.newObj)

			if isTriggered != tc.expected {
				t.Fatalf("Update is triggered = %v, expected %v", isTriggered, tc.expected)
			}
		})
	}
}

func TestEventWatchConsumerDelete(t *testing.T) {
	testCases := []struct {
		desc     string
		obj      interface{}
		expected bool
	}{
		{
			"obj=nil",
			nil,
			false,
		},
		{
			"obj=non-event",
			42,
			false,
		},
		{
			"obj=event",
			&api_v1.Event{},
			true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			isTriggered := false
			fakeConsumer := &fakeEventConsumer{
				deleteFunc: func(*api_v1.Event) { isTriggered = true },
			}

			c := newWatchConsumer(fakeConsumer)
			c.Delete(tc.obj)

			if isTriggered != tc.expected {
				t.Fatalf("Delete is triggered = %v, expected %v", isTriggered, tc.expected)
			}
		})
	}
}

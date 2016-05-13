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

package producers

import (
	"fmt"
	"time"

	"github.com/golang/glog"
)

type PushObj struct {
	Name        string
	Value       string
	CollectedAt time.Time
	Instance    string
}

type Producer interface {
	Name() string
	Produce(chan PushObj)
}

var producerMap = map[string]Producer{}

func GetAllProducers() []Producer {
	out := []Producer{}
	for _, producer := range producerMap {
		out = append(out, producer)
	}
	return out
}

func RegisterProducer(producer Producer) error {
	if _, found := producerMap[producer.Name()]; found {
		return fmt.Errorf("a producer with that name (%s) already exists", producer.Name())
	}
	producerMap[producer.Name()] = producer
	glog.Infof("Registered %#v at %s", producer, producer.Name())
	return nil
}

func RegisterProducerOrDie(producer Producer) {
	if err := RegisterProducer(producer); err != nil {
		glog.Fatalf("Failed to register producer: %s", err)
	}
}

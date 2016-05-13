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

import "time"

type DummyProducer struct {
}

func init() {
	d := DummyProducer{}
	RegisterProducerOrDie(d)
}

func (DummyProducer) Name() string {
	return "dummy-producer"
}

func (d DummyProducer) Produce(channel chan PushObj) {
	ticker := time.NewTicker(time.Second)
	go func() {
		for range ticker.C {
			channel <- PushObj{"Name", "Value", time.Now(), "Instance"}
		}
	}()
	time.Sleep(time.Second * 5)
	ticker.Stop()
}

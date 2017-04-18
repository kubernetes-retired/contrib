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

package main

import (
	"sync"

	"k8s.io/contrib/metrics/consumers"
	"k8s.io/contrib/metrics/producers"
)

func run_producer(producer producers.Producer, channel chan producers.PushObj, wg *sync.WaitGroup) {
	defer wg.Done()
	producer.Produce(channel)
}

func run_consumer(consumer consumers.Consumer, channel chan producers.PushObj, wg *sync.WaitGroup) {
	defer wg.Done()
	consumer.Consume(channel)
}

func main() {
	channel := make(chan producers.PushObj)
	var producer_waiter sync.WaitGroup
	var consumer_waiter sync.WaitGroup

	consumer := consumers.DummyConsumer{}
	consumer_waiter.Add(1)
	go run_consumer(consumer, channel, &consumer_waiter)

	allProducers := producers.GetAllProducers()
	for _, producer := range allProducers {
		producer_waiter.Add(1)
		go run_producer(producer, channel, &producer_waiter)
	}

	// Wait for producers to be done. When they are, notify consumer
	// and wait for it to finish.
	producer_waiter.Wait()
	close(channel)
	consumer_waiter.Wait()
}

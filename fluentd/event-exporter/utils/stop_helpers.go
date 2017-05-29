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

package utils

import (
	"sync"
)

// StoppableFunc is a type of function that blocks on run, but can be
// stopped via stop channel passed as an argument
type StoppableFunc func(<-chan struct{})

// RunConcurrentlyUntil runs several stoppable functions in parallel and blocks
// until all of them are executed.
func RunConcurrentlyUntil(stopCh <-chan struct{}, funcs ...StoppableFunc) {
	var wg sync.WaitGroup
	stopChs := make([]chan struct{}, len(funcs))
	for i, f := range funcs {
		stopChs[i] = make(chan struct{})
		wg.Add(1)
		go func() {
			f(stopChs[i])
			wg.Done()
		}()
	}

	<-stopCh

	for _, c := range stopChs {
		c <- struct{}{}
	}

	wg.Wait()
}

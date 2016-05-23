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

package flakesync

import (
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/google/gofuzz"
)

func makeRandom(Job, Number) (*Result, error) {
	r := &Result{}
	fuzz.New().Fuzz(r)
	time.Sleep(time.Millisecond / 4)
	return r, nil
}

func TestBasic(t *testing.T) {
	c := NewCache(makeRandom)
	r1, err := c.Get("foo", 5)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	r2, _ := c.Get("foo", 5)
	if !reflect.DeepEqual(r1, r2) {
		t.Errorf("expected to match: %#v, %#v", r1, r2)
	}
	i := 6
	for len(c.Flakes()) == 0 {
		c.Get("foo", Number(i))
	}
}

func TestThreading(t *testing.T) {
	c := NewCache(makeRandom)
	wg := sync.WaitGroup{}
	const threads = 100
	wg.Add(threads)
	for i := 0; i < threads; i++ {
		go func(s int) {
			defer wg.Done()
			for n := 0; n < 80; n++ {
				// n*s means many collide a few times, but some do not
				c.Get("foo", Number(n*s))
			}
		}(i)
	}
	wg.Wait()
}

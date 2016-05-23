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
	"sort"
	"sync"
	"time"
)

// Job is a test job, e.g. "kubernetes-e2e-gce"
type Job string

// Number is a run of a test job.
type Number int

// Test is the name of an individual test that runs in a Job.
type Test string

// Result records a test job completion.
type Result struct {
	Job
	Number
	Pass bool

	StartTime time.Time
	EndTime   time.Time

	// test name to reason/desc
	Flakes         map[Test]string
	UnlistedFlakes bool // i.e., non-JUnit reported
}

// Flake records a single flake occurrence.
type Flake struct {
	Job
	Number
	Test
	Reason string
}

type flakeKey struct {
	Job
	Number
	Test
}

type flakeMap map[flakeKey]*Flake
type key struct {
	Job
	Number
}
type jobMap map[key]*Result

// Cache caches test result lookups, and aggregates flakes in a single place.
// TODO: evict based on time.
// TODO: evict when issue filed.
// TODO: backfill to given time.
type Cache struct {
	lock       sync.Mutex
	byJob      jobMap
	flakeQueue flakeMap

	// only one expensive lookup at a time. Also, don't lock the cache
	// while we're doing an expensive update. If you lock both locks, you
	// must lock this one first.
	expensiveLookupLock sync.Mutex
	doExpensiveLookup   ResultFunc
}

// ResultFunc should look up the job & number from its source (GCS or
// wherever).
type ResultFunc func(Job, Number) (*Result, error)

// NewCache returns a new Cache.
func NewCache(getFunc ResultFunc) *Cache {
	c := &Cache{
		byJob:             jobMap{},
		flakeQueue:        flakeMap{},
		doExpensiveLookup: getFunc,
	}
	return c
}

// Get returns an item from the cache, possibly calling the lookup function
// passed at construction time.
func (c *Cache) Get(j Job, n Number) (*Result, error) {
	if r, ok := c.lookup(j, n); ok {
		return r, nil
	}
	return c.populate(j, n)
}

func (c *Cache) lookup(j Job, n Number) (*Result, bool) {
	c.lock.Lock()
	defer c.lock.Unlock()
	k := key{j, n}
	r, ok := c.byJob[k]
	return r, ok
}

func (c *Cache) populate(j Job, n Number) (*Result, error) {
	c.expensiveLookupLock.Lock()
	defer c.expensiveLookupLock.Unlock()
	if r, ok := c.lookup(j, n); ok {
		// added to the queue in the time it took us to get the lock.
		return r, nil
	}

	r, err := c.doExpensiveLookup(j, n)
	if err != nil {
		return nil, err
	}

	c.lock.Lock()
	defer c.lock.Unlock()
	k := key{j, n}
	c.byJob[k] = r

	// Add any flakes to the queue.
	for f, reason := range r.Flakes {
		c.flakeQueue[flakeKey{j, n, f}] = &Flake{
			Job:    j,
			Number: n,
			Test:   f,
			Reason: reason,
		}
	}
	return r, nil
}

// Flakes is a sortable list of flakes.
type Flakes []Flake

func (f Flakes) Len() int      { return len(f) }
func (f Flakes) Swap(i, j int) { f[i], f[j] = f[j], f[i] }
func (f Flakes) Less(i, j int) bool {
	if f[i].Test < f[j].Test {
		return true
	}
	if f[i].Test > f[j].Test {
		return false
	}
	if f[i].Job < f[j].Job {
		return true
	}
	if f[i].Job > f[j].Job {
		return false
	}
	if f[i].Number < f[j].Number {
		return true
	}
	if f[i].Number > f[j].Number {
		return false
	}
	return f[i].Reason < f[j].Reason
}

// Flakes lists all the current flakes, sorted.
func (c *Cache) Flakes() Flakes {
	flakes := Flakes{}
	c.lock.Lock()
	defer c.lock.Unlock()
	for _, f := range c.flakeQueue {
		flakes = append(flakes, *f)
	}
	sort.Sort(flakes)
	return flakes
}

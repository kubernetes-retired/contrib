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

package condition

import (
	"reflect"
	"sync"
	"time"

	"k8s.io/contrib/node-problem-detector/pkg/problemclient"
	"k8s.io/contrib/node-problem-detector/pkg/types"
	problemutil "k8s.io/contrib/node-problem-detector/pkg/util"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util"

	"github.com/golang/glog"
)

const (
	// updatePeriod is the period which condition manager checks update.
	updatePeriod = 1 * time.Second
	// updateTimeout is the timeout of condition update operation.
	updateTimeout = 10 * time.Second
	// resyncPeriod is the period which condition manager does resync no matter whether these is any update.
	resyncPeriod = 30 * time.Second
)

// ConditionManager synchronizes node conditions with the apiserver with problem client.
// It makes sure that:
// 1) Node conditions are updated to apiserver as soon as possible.
// 2) Node problem detector won't flood apiserver.
// 3) No one else could change the node conditions maintained by node problem detector.
// ConditionManager checks every updatePeriod to see whether there is node condition update. If there are any,
// it will synchronize with the apiserver. This addresses 1) and 2).
// ConditionManager synchronizes with apiserver every resyncPeriod no matter there is node condition update or
// not. This addresses 3).
type ConditionManager interface {
	// Start starts the condition manager.
	Start()
	// UpdateCondition update specific condition.
	UpdateCondition(types.Condition)
}

type conditionManager struct {
	sync.Mutex
	clock      util.Clock
	latest     time.Time
	client     problemclient.Client
	updates    map[string]types.Condition
	conditions map[string]types.Condition
}

// NewConditionManager creates a condition manager.
func NewConditionManager(client problemclient.Client, clock util.Clock) ConditionManager {
	return &conditionManager{
		client:     client,
		clock:      clock,
		updates:    make(map[string]types.Condition),
		conditions: make(map[string]types.Condition),
	}
}

func (c *conditionManager) Start() {
	go c.syncLoop()
}

func (c *conditionManager) UpdateCondition(condition types.Condition) {
	c.Lock()
	defer c.Unlock()
	// New node condition will override the old condition, because we only need the newest
	// condition for each condition type.
	c.updates[condition.Type] = condition
}

func (c *conditionManager) syncLoop() {
	updateCh := c.clock.Tick(updatePeriod)
	for {
		select {
		case <-updateCh:
			if c.checkUpdates() || c.checkResync() {
				c.sync()
			}
		}
	}
}

// checkUpdates checks whether there are recent updates.
func (c *conditionManager) checkUpdates() bool {
	c.Lock()
	defer c.Unlock()
	needUpdate := false
	for t, update := range c.updates {
		if !reflect.DeepEqual(c.conditions[t], update) {
			needUpdate = true
			c.conditions[t] = update
		}
		delete(c.updates, t)
	}
	return needUpdate
}

// checkResync checks whether a resync is needed.
func (c *conditionManager) checkResync() bool {
	return c.clock.Now().Sub(c.latest) >= resyncPeriod
}

// sync synchronizes node conditions with the apiserver.
func (c *conditionManager) sync() {
	conditions := []api.NodeCondition{}
	for i := range c.conditions {
		conditions = append(conditions, problemutil.ConvertToAPICondition(c.conditions[i]))
	}
	if err := c.client.SetConditions(conditions, updateTimeout); err != nil {
		// The conditions will be updated again in future sync
		glog.Errorf("failed to update node conditions: %v", err)
		return
	}
	c.latest = c.clock.Now()
}

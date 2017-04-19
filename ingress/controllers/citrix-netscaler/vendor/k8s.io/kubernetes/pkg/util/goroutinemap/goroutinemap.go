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

/*
Package goroutinemap implements a data structure for managing go routines
by name. It prevents the creation of new go routines if an existing go routine
with the same name exists.
*/
package goroutinemap

import (
	"fmt"
	"sync"

	"k8s.io/kubernetes/pkg/util/runtime"
)

// GoRoutineMap defines the supported set of operations.
type GoRoutineMap interface {
	// Run adds operationName to the list of running operations and spawns a new
	// go routine to execute the operation. If an operation with the same name
	// already exists, an error is returned. Once the operation is complete, the
	// go routine is terminated and the operationName is removed from the list
	// of executing operations allowing a new operation to be started with the
	// same name without error.
	Run(operationName string, operation func() error) error

	// Wait blocks until all operations are completed. This is typically
	// necessary during tests - the test should wait until all operations finish
	// and evaluate results after that.
	Wait()
}

// NewGoRoutineMap returns a new instance of GoRoutineMap.
func NewGoRoutineMap() GoRoutineMap {
	return &goRoutineMap{
		operations: make(map[string]bool),
	}
}

type goRoutineMap struct {
	operations map[string]bool
	sync.Mutex
	wg sync.WaitGroup
}

func (grm *goRoutineMap) Run(operationName string, operation func() error) error {
	grm.Lock()
	defer grm.Unlock()
	if grm.operations[operationName] {
		// Operation with name exists
		return newAlreadyExistsError(operationName)
	}

	grm.operations[operationName] = true
	grm.wg.Add(1)
	go func() {
		defer grm.operationComplete(operationName)
		defer runtime.HandleCrash()
		operation()
	}()

	return nil
}

func (grm *goRoutineMap) operationComplete(operationName string) {
	defer grm.wg.Done()
	grm.Lock()
	defer grm.Unlock()
	delete(grm.operations, operationName)
}

func (grm *goRoutineMap) Wait() {
	grm.wg.Wait()
}

// alreadyExistsError is specific error returned when NewGoRoutine()
// detects that operation with given name is already running.
type alreadyExistsError struct {
	operationName string
}

var _ error = alreadyExistsError{}

func (err alreadyExistsError) Error() string {
	return fmt.Sprintf("Failed to create operation with name %q. An operation with that name already exists", err.operationName)
}

func newAlreadyExistsError(operationName string) error {
	return alreadyExistsError{operationName}
}

// IsAlreadyExists returns true if an error returned from NewGoRoutine indicates
// that operation with the same name already exists.
func IsAlreadyExists(err error) bool {
	switch err.(type) {
	case alreadyExistsError:
		return true
	default:
		return false
	}
}

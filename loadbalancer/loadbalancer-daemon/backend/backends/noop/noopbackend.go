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

package backends

import (
	"github.com/golang/glog"
	factory "k8s.io/contrib/loadbalancer/loadbalancer-daemon/backend"
)

// NoOpController Controller for noop backend
type NoOpController struct {
	variable string
	exitChan chan struct{}
}

func init() {
	factory.Register("noop", NewNoOpController)
}

// NewNoOpController creates a Noop controller
func NewNoOpController() (factory.BackendController, error) {

	cont := NoOpController{
		variable: "NOOP",
		exitChan: make(chan struct{}),
	}

	return &cont, nil
}

// Name returns the name of the backend controller
func (noop *NoOpController) Name() string {
	return "NOOP"
}

// AddConfig Add event
func (noop *NoOpController) AddConfig(name string, config factory.BackendConfig) {
	glog.Infof("Received config %s: %v", name, config)
}

// DeleteConfig delete event
func (noop *NoOpController) DeleteConfig(name string) {
	glog.Infof("Received delete config name %s", name)
}

// ExitChannel returns the channel used to communicate nginx process has exited
func (noop *NoOpController) ExitChannel() chan struct{} {
	return noop.exitChan
}

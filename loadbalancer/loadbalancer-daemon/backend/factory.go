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
	"fmt"
	"strings"

	"github.com/golang/glog"
)

// BackendConfig Config that have all data for backend
type BackendConfig struct {
	Host              string
	Namespace         string
	BindIp            string
	TargetServiceName string
	TargetIP          string
	Ports             []string
	SSL               bool
	SSLPort           int
	Path              string
	TlsCert           string
	TlsKey            string
}

// BackendController defines the functions needed to be implemented by every backend.
type BackendController interface {
	Name() string
	AddConfig(name string, config BackendConfig)
	DeleteConfig(name string)
	ExitChannel() chan struct{}
}

// BackendControllerFactory Factory for backend controllers
type BackendControllerFactory func() (BackendController, error)

var backendControllerFactories = make(map[string]BackendControllerFactory)

// Register register a backend controller to the factory
func Register(name string, factory BackendControllerFactory) {
	if factory == nil {
		glog.Errorf("Backend controller factory %s does not exist.", name)
	}
	_, registered := backendControllerFactories[name]
	if registered {
		glog.Errorf("Backend controller factory %s already registered. Ignoring.", name)
	}
	backendControllerFactories[name] = factory
}

// CreateBackendController returns a backend controller object based on the backendname
func CreateBackendController(backendName string) (BackendController, error) {
	// Query configuration for backend controller.
	engineName := backendName

	engineFactory, ok := backendControllerFactories[engineName]
	if !ok {
		// Factory has not been registered.
		// Make a list of all available backend controller factories for logging.
		availableBackendControllers := make([]string, len(backendControllerFactories))
		for k := range backendControllerFactories {
			availableBackendControllers = append(availableBackendControllers, k)
		}
		return nil, fmt.Errorf(fmt.Sprintf("Invalid backend controller name. Must be one of: %s", strings.Join(availableBackendControllers, ", ")))
	}

	// Run the factory with the configuration.
	return engineFactory()
}

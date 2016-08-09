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

package backend

import (
	"fmt"
	"strings"
	"sync"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/unversioned"

	"github.com/golang/glog"
)

// BackendController Interface for all Backends
type BackendController interface {
	Name() string
	GetBindIP(name string) (string, error)
	HandleConfigMapCreate(configMap *api.ConfigMap) error
	HandleConfigMapDelete(configMap *api.ConfigMap)
	HandleNodeCreate(node *api.Node)
	HandleNodeDelete(node *api.Node)
	HandleNodeUpdate(oldNode *api.Node, curNode *api.Node)
}

// BackendControllerFactory Factory for Backend controllers
type BackendControllerFactory func(kubeClient *unversioned.Client, watchNamespace string, conf map[string]string, configLabelKey, configLabelValue string) (BackendController, error)

var backendsMutex sync.Mutex
var backendControllerFactories = make(map[string]BackendControllerFactory)

// Register registers a backend factory by name
func Register(name string, factory BackendControllerFactory) {
	backendsMutex.Lock()
	defer backendsMutex.Unlock()
	if factory == nil {
		glog.Errorf("Backend controller factory %s does not exist.", name)
	}
	_, registered := backendControllerFactories[name]
	if registered {
		glog.Errorf("Backend controller factory %s already registered. Ignoring.", name)
	}
	backendControllerFactories[name] = factory
	glog.Infof("Registered backend %v.", name)
}

// CreateBackendController creates a backend controller factory for a specific backend
func CreateBackendController(kubeClient *unversioned.Client, watchNamespace string, conf map[string]string, configLabelKey, configLabelValue string) (BackendController, error) {
	backendsMutex.Lock()
	defer backendsMutex.Unlock()

	// Query configuration for backend controller.
	engineName := conf["BACKEND"]

	engineFactory, ok := backendControllerFactories[engineName]
	if !ok {
		// Factory has not been registered.
		// Make a list of all available backend controller factories for logging.
		availableBackendControllers := make([]string, len(backendControllerFactories))
		for k := range backendControllerFactories {
			availableBackendControllers = append(availableBackendControllers, k)
		}
		return nil, fmt.Errorf("Invalid backend controller name. Must be one of: %s", strings.Join(availableBackendControllers, ", "))
	}

	// Run the factory with the configuration.
	return engineFactory(kubeClient, watchNamespace, conf, configLabelKey, configLabelValue)
}

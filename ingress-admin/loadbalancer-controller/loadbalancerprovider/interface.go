/*
Copyright 2016 The Kubernetes Authors.

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
package loadbalancerprovider

import (
	"sync"
	"fmt"
	"strings"

	"k8s.io/contrib/ingress-admin/loadbalancer-controller/api"

	"k8s.io/client-go/1.5/pkg/api/v1"
	"k8s.io/client-go/1.5/pkg/util/validation"
	"k8s.io/client-go/1.5/kubernetes"
	"k8s.io/client-go/1.5/dynamic"
)

type LoadBalancerPlugin interface {
	GetPluginName() string
	CanSupport(spec *api.LoadBalancerClaim) bool
	NewProvisioner(options LoadBalancerOptions) Provisioner
}

type LoadBalancerOptions struct {
	Resources v1.ResourceRequirements
	LoadBalancerName string
	LoadBalancerVIP string
}

type Provisioner interface {
	Provision(clientset *kubernetes.Clientset, dynamicClient *dynamic.Client) (string, error)
}

// LoadBalancerPluginMgr tracks registered plugins.
type LoadBalancerPluginMgr struct {
	mutex   sync.Mutex
	plugins map[string]LoadBalancerPlugin
}

var PluginMgr LoadBalancerPluginMgr = LoadBalancerPluginMgr{
	mutex: sync.Mutex{},
	plugins: map[string]LoadBalancerPlugin{},
}

func RegisterPlugin(plugin LoadBalancerPlugin) error {
	PluginMgr.mutex.Lock()
	defer PluginMgr.mutex.Unlock()

	name := plugin.GetPluginName()
	if errs := validation.IsQualifiedName(name); len(errs) != 0 {
		return fmt.Errorf("volume plugin has invalid name: %q: %s", name, strings.Join(errs, ";"))
	}
	if _, found := PluginMgr.plugins[name]; found {
		return fmt.Errorf("volume plugin %q was registered more than once", name)
	}
	
	PluginMgr.plugins[name] = plugin
	
	return nil
}

func(pm *LoadBalancerPluginMgr) FindPluginBySpec(claim *api.LoadBalancerClaim) (LoadBalancerPlugin, error) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	matches := []string{}
	for k, v := range pm.plugins {
		if v.CanSupport(claim) {
			matches = append(matches, k)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no ingress service plugin matched")
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("multiple ingress service plugins matched: %s", strings.Join(matches, ","))
	}
	return pm.plugins[matches[0]], nil
}




/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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

package unversioned

import (
	"fmt"

	"k8s.io/kubernetes/pkg/api/latest"
	"k8s.io/kubernetes/pkg/apis/extensions"
)

// Interface holds the experimental methods for clients of Kubernetes
// to allow mock testing.
// Features of Extensions group are not supported and may be changed or removed in
// incompatible ways at any time.
type ExtensionsInterface interface {
	HorizontalPodAutoscalersNamespacer
	ScaleNamespacer
	DaemonSetsNamespacer
	DeploymentsNamespacer
	JobsNamespacer
	IngressNamespacer
	ThirdPartyResourceNamespacer
	ConfigMapsNamespacer
}

// ExtensionsClient is used to interact with experimental Kubernetes features.
// Features of Extensions group are not supported and may be changed or removed in
// incompatible ways at any time.
type ExtensionsClient struct {
	*RESTClient
}

func (c *ExtensionsClient) HorizontalPodAutoscalers(namespace string) HorizontalPodAutoscalerInterface {
	return newHorizontalPodAutoscalers(c, namespace)
}

func (c *ExtensionsClient) Scales(namespace string) ScaleInterface {
	return newScales(c, namespace)
}

func (c *ExtensionsClient) DaemonSets(namespace string) DaemonSetInterface {
	return newDaemonSets(c, namespace)
}

func (c *ExtensionsClient) Deployments(namespace string) DeploymentInterface {
	return newDeployments(c, namespace)
}

func (c *ExtensionsClient) Jobs(namespace string) JobInterface {
	return newJobs(c, namespace)
}

func (c *ExtensionsClient) Ingress(namespace string) IngressInterface {
	return newIngress(c, namespace)
}

func (c *ExtensionsClient) ConfigMaps(namespace string) ConfigMapsInterface {
	return newConfigMaps(c, namespace)
}

func (c *ExtensionsClient) ThirdPartyResources(namespace string) ThirdPartyResourceInterface {
	return newThirdPartyResources(c, namespace)
}

// NewExtensions creates a new ExtensionsClient for the given config. This client
// provides access to experimental Kubernetes features.
// Features of Extensions group are not supported and may be changed or removed in
// incompatible ways at any time.
func NewExtensions(c *Config) (*ExtensionsClient, error) {
	config := *c
	if err := setExtensionsDefaults(&config); err != nil {
		return nil, err
	}
	client, err := RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &ExtensionsClient{client}, nil
}

// NewExtensionsOrDie creates a new ExtensionsClient for the given config and
// panics if there is an error in the config.
// Features of Extensions group are not supported and may be changed or removed in
// incompatible ways at any time.
func NewExtensionsOrDie(c *Config) *ExtensionsClient {
	client, err := NewExtensions(c)
	if err != nil {
		panic(err)
	}
	return client
}

func setExtensionsDefaults(config *Config) error {
	// if experimental group is not registered, return an error
	g, err := latest.Group(extensions.GroupName)
	if err != nil {
		return err
	}
	config.Prefix = "apis/"
	if config.UserAgent == "" {
		config.UserAgent = DefaultKubernetesUserAgent()
	}
	// TODO: Unconditionally set the config.Version, until we fix the config.
	//if config.Version == "" {
	copyGroupVersion := g.GroupVersion
	config.GroupVersion = &copyGroupVersion
	//}

	versionInterfaces, err := g.InterfacesFor(*config.GroupVersion)
	if err != nil {
		return fmt.Errorf("Extensions API group/version '%v' is not recognized (valid values: %v)",
			config.GroupVersion, latest.GroupOrDie(extensions.GroupName).GroupVersions)
	}
	config.Codec = versionInterfaces.Codec
	if config.QPS == 0 {
		config.QPS = 5
	}
	if config.Burst == 0 {
		config.Burst = 10
	}
	return nil
}

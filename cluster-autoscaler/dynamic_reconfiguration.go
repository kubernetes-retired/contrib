package main

import (
	"fmt"
	"github.com/golang/glog"
	"k8s.io/contrib/cluster-autoscaler/cloudprovider/builder"
	"k8s.io/contrib/cluster-autoscaler/dynamic"
)

// DynamicReconfiguration dynamically reconfigure cluster-autoscaler with node group specs read from a configmap
type DynamicReconfiguration struct {
	configMapName        string
	autoscalingContext   *AutoscalingContext
	configFetcher        *dynamic.ConfigFetcher
	cloudProviderBuilder builder.CloudProviderBuilder
}

// NewDynamicReconfiguration builds a new dynamic reconfiguration object
func NewDynamicReconfiguration(configMapName string, autoscalingContext *AutoscalingContext, configFetcher *dynamic.ConfigFetcher, cloudProviderBuilder builder.CloudProviderBuilder) DynamicReconfiguration {
	return DynamicReconfiguration{
		configMapName:        configMapName,
		autoscalingContext:   autoscalingContext,
		configFetcher:        configFetcher,
		cloudProviderBuilder: cloudProviderBuilder,
	}
}

// Run dynamic reconfiguration once
func (r DynamicReconfiguration) Run() error {
	var updatedConfig *dynamic.Config
	var err error

	if updatedConfig, err = r.configFetcher.FetchConfigIfUpdated(); err != nil {
		return fmt.Errorf("failed to fetch updated config: %v", err)
	}

	if updatedConfig != nil {
		r.autoscalingContext.CloudProvider = r.cloudProviderBuilder.Build(updatedConfig.NodeGroupSpecStrings())
		glog.V(4).Infof("Dynamic reconfiguration finished: updatedConfig=%v", updatedConfig)
	}

	return nil
}

// Enabled returns true if reconfiguration is enabled. The `Run()` func shouldn't be called if this returned `false`.
func (r DynamicReconfiguration) Enabled() bool {
	return r.configMapName != ""
}

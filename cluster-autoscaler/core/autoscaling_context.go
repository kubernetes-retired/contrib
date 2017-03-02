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

package core

import (
	"time"

	kube_record "k8s.io/client-go/tools/record"
	"k8s.io/contrib/cluster-autoscaler/cloudprovider"
	"k8s.io/contrib/cluster-autoscaler/cloudprovider/builder"
	"k8s.io/contrib/cluster-autoscaler/clusterstate"
	"k8s.io/contrib/cluster-autoscaler/expander"
	"k8s.io/contrib/cluster-autoscaler/expander/factory"
	"k8s.io/contrib/cluster-autoscaler/simulator"
	kube_client "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
)

// AutoscalingContext contains user-configurable constant and configuration-related objects passed to
// scale up/scale down functions.
type AutoscalingContext struct {
	// Options to customize how autoscaling works
	AutoscalingOptions
	// CloudProvider used in CA.
	CloudProvider cloudprovider.CloudProvider
	// CloudProviderBuilder is used to new CloudProvider
	CloudProviderBuilder builder.CloudProviderBuilder
	// ClientSet interface.
	ClientSet kube_client.Interface
	// ClusterState for maintaining the state of custer nodes.
	ClusterStateRegistry *clusterstate.ClusterStateRegistry
	// Recorder for recording events.
	Recorder kube_record.EventRecorder
	// PredicateChecker to check if a pod can fit into a node.
	PredicateChecker *simulator.PredicateChecker
	// ExpanderStrategy is the strategy used to choose which node group to expand when scaling up
	ExpanderStrategy expander.Strategy
}

// AutoscalingOptions contain various options to customize how autoscaling works
type AutoscalingOptions struct {
	// MaxEmptyBulkDelete is a number of empty nodes that can be removed at the same time.
	MaxEmptyBulkDelete int
	// ScaleDownUtilizationThreshold sets threshould for nodes to be considered for scale down.
	// Well-utilized nodes are not touched.
	ScaleDownUtilizationThreshold float64
	// ScaleDownUnneededTime sets the duriation CA exepects a node to be unneded/eligible for removal
	// before scaling down the node.
	ScaleDownUnneededTime time.Duration
	// ScaleDownUnreadyTime represents how long an unready node should be unneeded before it is eligible for scale down
	ScaleDownUnreadyTime time.Duration
	// MaxNodesTotal sets the maximum number of nodes in the whole cluster
	MaxNodesTotal int
	// UnregisteredNodeRemovalTime represents how long CA waits before removing nodes that are not registered in Kubernetes")
	UnregisteredNodeRemovalTime time.Duration
	// EstimatorName is the estimator used to estimate the number of needed nodes in scale up.
	EstimatorName string
	// ExpanderName sets the type of node group expander to be used in scale up
	ExpanderName string
	// MaxGratefulTerminationSec is maximum number of seconds scale down waits for pods to terminante before
	// removing the node from cloud provider.
	MaxGratefulTerminationSec int
	//  Maximum time CA waits for node to be provisioned
	MaxNodeProvisionTime time.Duration
	// MaxTotalUnreadyPercentage is the maximum percentage of unready nodes after which CA halts operations
	MaxTotalUnreadyPercentage float64
	// OkTotalUnreadyCount is the number of allowed unready nodes, irrespective of max-total-unready-percentage
	OkTotalUnreadyCount int
	// CloudConfig is the path to the cloud provider configuration file. Empty string for no configuration file.
	CloudConfig string
	// CloudProviderName sets the type of the cloud provider CA is about to run in. Allowed values: gce, aws
	CloudProviderName string
	// NodeGroups is the list of node groups a.k.a autoscaling targets
	NodeGroups []string
	// VerifyUnschedulable is used to enable verification to ensure that each pod marked by Scheduler as unschedulable actually can't be scheduled on any node.
	// This prevents from adding unnecessary nodes in situation when CA and Scheduler have different configuration.
	VerifyUnschedulablePods bool
	// ScaleDownEnabled is used to allow CA to scale down the cluster
	ScaleDownEnabled bool
	// ScaleDownDelay sets the duration from the last scale up to the time when CA starts to check scale down options
	ScaleDownDelay time.Duration
	// ScaleDownTrialInterval sets how often scale down possibility is check
	ScaleDownTrialInterval time.Duration
	// WriteStatusConfigMap tells if the status information should be written to a ConfigMap
	WriteStatusConfigMap bool
}

// NewAutoscalingContext returns an autoscaling context from all the necessary parameters passed via arguments
func NewAutoscalingContext(options AutoscalingOptions, predicateChecker *simulator.PredicateChecker, kubeClient kube_client.Interface, kubeEventRecorder kube_record.EventRecorder) *AutoscalingContext {
	cloudProviderBuilder := builder.NewCloudProviderBuilder(options.CloudProviderName, options.CloudConfig)
	cloudProvider := cloudProviderBuilder.Build(options.NodeGroups)
	expanderStrategy := factory.ExpanderStrategyFromString(options.ExpanderName)
	clusterStateConfig := clusterstate.ClusterStateRegistryConfig{
		MaxTotalUnreadyPercentage: options.MaxTotalUnreadyPercentage,
		OkTotalUnreadyCount:       options.OkTotalUnreadyCount,
	}
	clusterStateRegistry := clusterstate.NewClusterStateRegistry(cloudProvider, clusterStateConfig)

	autoscalingContext := AutoscalingContext{
		AutoscalingOptions:   options,
		CloudProvider:        cloudProvider,
		CloudProviderBuilder: cloudProviderBuilder,
		ClusterStateRegistry: clusterStateRegistry,
		ClientSet:            kubeClient,
		Recorder:             kubeEventRecorder,
		PredicateChecker:     predicateChecker,
		ExpanderStrategy:     expanderStrategy,
	}

	return &autoscalingContext
}

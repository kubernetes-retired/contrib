package core

import (
	"time"

	kube_client "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	kube_record "k8s.io/kubernetes/pkg/client/record"

	"k8s.io/contrib/cluster-autoscaler/config/dynamic"
)

// AutoscalerOptions is the whole set of options for configuring an autoscaler
type AutoscalerOptions struct {
	AutoscalingOptions
	dynamic.ConfigFetcherOptions
}

// Autoscaler is the main component of CA which scales up/down node groups according to its configuration
// The configuration can be injected at the creation of an autoscaler
type Autoscaler interface {
	// RunOnce represents an iteration in the control-loop of CA
	RunOnce(currentTime time.Time)
}

// NewAutoscaler creates an autoscaler of an appropriate type according to the parameters
func NewAutoscaler(opts AutoscalerOptions, kubeClient kube_client.Interface, kubeEventRecorder kube_record.EventRecorder) Autoscaler {
	var autoscaler Autoscaler
	if opts.ConfigMapName != "" {
		autoscalerBuilder := NewAutoscalerBuilder(opts.AutoscalingOptions, kubeClient, kubeEventRecorder)
		configFetcher := dynamic.NewConfigFetcher(opts.ConfigFetcherOptions, kubeClient, kubeEventRecorder)
		autoscaler = NewDynamicAutoscaler(autoscalerBuilder, configFetcher)
	} else {
		autoscaler = NewStaticAutoscaler(opts.AutoscalingOptions, kubeClient, kubeEventRecorder)
	}
	return autoscaler
}

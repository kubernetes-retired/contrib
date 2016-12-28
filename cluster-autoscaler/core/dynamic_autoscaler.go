package core

import (
	"fmt"
	"time"

	"k8s.io/contrib/cluster-autoscaler/config/dynamic"
	"k8s.io/contrib/cluster-autoscaler/metrics"

	kube_client "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	kube_record "k8s.io/kubernetes/pkg/client/record"

	"github.com/golang/glog"
)

// DynamicAutoscaler is a variant of autoscaler which supports dynamic reconfiguration at runtime
type DynamicAutoscaler struct {
	autoscaler Autoscaler
	autoscalerBuilder    AutoscalerBuilder
	configFetcher dynamic.ConfigFetcher
}

// NewDynamicAutoscaler builds a DynamicAutoscaler from required parameters
func NewDynamicAutoscaler(autoscalerBuilder AutoscalerBuilder, configFetcher dynamic.ConfigFetcher) *DynamicAutoscaler {
	return &DynamicAutoscaler{
		autoscaler: autoscalerBuilder.Build(),
		autoscalerBuilder: autoscalerBuilder,
		configFetcher: configFetcher,
	}
}

// RunOnce represents a single iteration of a dynamic autoscaler inside the CA's control-loop
func (a *DynamicAutoscaler) RunOnce(currentTime time.Time) {
	reconfigureStart := time.Now()
	metrics.UpdateLastTime("reconfigure")
	if err := a.Reconfigure(); err != nil {
		glog.Errorf("Failed to reconfigure : %v", err)
	}
	metrics.UpdateDuration("reconfigure", reconfigureStart)
	a.autoscaler.RunOnce(currentTime)
}

// Reconfigure this dynamic autoscaler if the configmap is updated
func (a *DynamicAutoscaler) Reconfigure() error {
	var updatedConfig *dynamic.Config
	var err error

	if updatedConfig, err = a.configFetcher.FetchConfigIfUpdated(); err != nil {
		return fmt.Errorf("failed to fetch updated config: %v", err)
	}

	if updatedConfig != nil {
		// For safety, any config change should stop and recreate all the stuff running in CA hence recreating all the Autoscaler instance here
		// See https://github.com/kubernetes/contrib/pull/2226#discussion_r94126064
		a.autoscaler = a.autoscalerBuilder.SetDynamicConfig(*updatedConfig).Build()
		glog.V(4).Infof("Dynamic reconfiguration finished: updatedConfig=%v", updatedConfig)
	}

	return nil
}

type AutoscalerBuilder interface {
	SetDynamicConfig(config dynamic.Config) AutoscalerBuilder
	Build() Autoscaler
}

// AutoscalerBuilderImpl builds new autoscalers from its state including initial `AutoscalingOptions` given at startup and
// `dynamic.Config` read on demand from the configmap
type AutoscalerBuilderImpl struct {
	autoscalingOptions AutoscalingOptions
	dynamicConfig *dynamic.Config
	kubeClient kube_client.Interface
	kubeEventRecorder kube_record.EventRecorder
}

// NewBuilder builds an AutoscalerBuilder from required parameters
func NewAutoscalerBuilder(autoscalingOptions AutoscalingOptions, kubeClient kube_client.Interface, kubeEventRecorder kube_record.EventRecorder) *AutoscalerBuilderImpl {
	return &AutoscalerBuilderImpl{
		autoscalingOptions: autoscalingOptions,
		kubeClient: kubeClient,
		kubeEventRecorder: kubeEventRecorder,
	}
}

// SetDynamicConfig sets an instance of dynamic.Config read from a configmap so that
// the new autoscaler built afterwards reflect the latest configuration contained in the configmap
func (b *AutoscalerBuilderImpl) SetDynamicConfig(config dynamic.Config) AutoscalerBuilder {
	b.dynamicConfig = &config
	return b
}

// Build an autoscaler according to the builder's state
func (b *AutoscalerBuilderImpl) Build() Autoscaler {
	options := b.autoscalingOptions
	if b.dynamicConfig != nil {
		c := *(b.dynamicConfig)
		options.NodeGroups = c.NodeGroupSpecStrings()
	}
	return NewStaticAutoscaler(options, b.kubeClient, b.kubeEventRecorder)
}

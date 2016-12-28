package dynamic

import (
	"fmt"
	apiv1 "k8s.io/kubernetes/pkg/api/v1"
	kube_client "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	kube_record "k8s.io/kubernetes/pkg/client/record"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConfigFetcher fetches the up-to-date dynamic configuration from the apiserver
type ConfigFetcher interface {
	FetchConfigIfUpdated() (*Config, error)
}

type configFetcherImpl struct {
	configMapName string
	namespace     string
	kubeClient    kube_client.Interface
	lastConfig    Config
	// Recorder for recording events.
	recorder kube_record.EventRecorder
}

// ConfigFetcherOptions contains the various options to customize ConfigFetcher
type ConfigFetcherOptions struct {
	ConfigMapName string
	Namespace string
}

// NewConfigFetcher builds a config fetcher from the parameters and dependencies
func NewConfigFetcher(options ConfigFetcherOptions, kubeClient kube_client.Interface, recorder kube_record.EventRecorder) *configFetcherImpl {
	return &configFetcherImpl{
		configMapName: options.ConfigMapName,
		namespace:     options.Namespace,
		kubeClient:    kubeClient,
		lastConfig:    NewDefaultConfig(),
		recorder: recorder,
	}
}

// Returns the config if it has changed since the last sync. Returns nil if it has not changed.
func (c *configFetcherImpl) FetchConfigIfUpdated() (*Config, error) {
	opts := metav1.GetOptions{}
	cm, err := c.kubeClient.Core().ConfigMaps(c.namespace).Get(c.configMapName, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch config map named %s in namespace %s. please confirm if the configmap name and the namespace are correctly spelled and you've already created the configmap: %v", c.configMapName, c.namespace, err)
	}

	configFromServer, err := ConfigFromConfigMap(cm)
	if err != nil {
		c.recorder.Eventf(cm, apiv1.EventTypeNormal, "FailedToBeLoaded",
			"cluster-autoscaler tried to load this configmap but failed: %v", err)
		return nil, fmt.Errorf("failed to load dyamic config: %v", err)
	}

	if c.lastConfig.VersionMismatchesAgainst(*configFromServer) {
		c.lastConfig = *configFromServer
		return configFromServer, nil
	}

	return nil, nil
}

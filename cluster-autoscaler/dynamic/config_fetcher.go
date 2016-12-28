package dynamic

import (
	"fmt"
	kube_client "k8s.io/kubernetes/pkg/client/clientset_generated/release_1_5"
)

// ConfigFetcher fetches the up-to-date dynamic configuration from the apiserver
type ConfigFetcher struct {
	configMapName string
	namespace     string
	kubeClient    kube_client.Interface
	lastConfig    Config
}

// Builds a config fetcher from the parameters and dependencies
func NewConfigFetcher(configMapName string, namespace string, kubeClient kube_client.Interface) *ConfigFetcher {
	return &ConfigFetcher{
		configMapName: configMapName,
		namespace:     namespace,
		kubeClient:    kubeClient,
		lastConfig:    NewDefaultConfig(),
	}
}

// Returns the config if it has changed since the last sync. Returns nil if it has not changed.
func (c *ConfigFetcher) FetchConfigIfUpdated() (*Config, error) {
	cm, err := c.kubeClient.Core().ConfigMaps(c.namespace).Get(c.configMapName)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch config map named %s: %v", c.configMapName, err)
	}

	configFromServer, err := ConfigFromConfigMap(cm)
	if err != nil {
		return nil, fmt.Errorf("failed to load dyamic config: %v", err)
	}

	if c.lastConfig.VersionMismatchesAgainst(*configFromServer) {
		c.lastConfig = *configFromServer
		return configFromServer, nil
	}

	return nil, nil
}

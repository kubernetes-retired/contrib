package utils

import (
	"sync"

	"github.com/golang/glog"
	"github.com/spf13/pflag"
	"k8s.io/kubernetes/pkg/client/unversioned"
	kubectl_util "k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

var kubeClient K8Client
var once sync.Once

// K8Client represents a client object used to access the kubernetes api
type K8Client struct {
	Client    *unversioned.Client
	Namespace string
}

// ClientConfig struct has the variable needed to create a k8 client
type ClientConfig struct {
	InCluster bool
	Namespace string
	Flags     *pflag.FlagSet
}

// GetK8Client return a client to access kubernetes api.
func GetK8Client(cfg ClientConfig) K8Client {
	once.Do(func() {
		var err error
		var k8Client *unversioned.Client
		if cfg.InCluster || cfg.Flags == nil {
			k8Client, err = unversioned.NewInCluster()
		} else {
			clientConfig := kubectl_util.DefaultClientConfig(cfg.Flags)
			config, connErr := clientConfig.ClientConfig()
			if connErr != nil {
				glog.Fatalf("error connecting to the client: %v", err)
			}
			k8Client, err = unversioned.New(config)
		}

		if err != nil {
			glog.Fatalf("failed to create client: %v", err)
		}
		kubeClient = K8Client{
			Client:    k8Client,
			Namespace: cfg.Namespace,
		}

	})
	return kubeClient
}

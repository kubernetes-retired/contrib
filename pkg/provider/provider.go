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

package provider

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/clientset_generated/release_1_4"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	clientcmdapi "k8s.io/kubernetes/pkg/client/unversioned/clientcmd/api"
)

const (
	UserAgent = "terraform-kubernetes"

	PollInterval = 10 * time.Second
	PollTimeout  = 10 * time.Minute
)

func Provider() terraform.ResourceProvider {
	return &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"kubernetes_kubeconfig": resourceKubeconfig(),
		},
	}
}

func resourceKubeconfig() *schema.Resource {
	return &schema.Resource{
		Create: CreateKubeconfig,
		Delete: DeleteKubeconfig,
		Read:   ReadKubeconfig,

		Schema: map[string]*schema.Schema{
			"server": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				Description: "Domain name or IP address of the API server",
				ForceNew:    true,
			},
			"configdata": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				Description: "kubeconfig in the serialized JSON format",
				ForceNew:    true,
			},
		},
	}
}

func CreateKubeconfig(d *schema.ResourceData, meta interface{}) error {
	server := d.Get("server").(string)
	clientConfig, err := clientcmd.BuildConfigFromKubeconfigGetter(server, kubeConfigGetter(d))
	if err != nil {
		return fmt.Errorf("couldn't parse the supplied config: %v", err)
	}

	clientset, err := release_1_4.NewForConfig(restclient.AddUserAgent(clientConfig, UserAgent))
	if err != nil {
		return fmt.Errorf("failed to initialize the cluster client: %v", err)
	}

	if !clusterHealthy(clientset, PollInterval, PollTimeout) {
		return fmt.Errorf("cluster components never turned healthy")
	}

	configAccess := clientcmd.NewDefaultPathOptions()
	kubeConfig, err := kubeConfigGetter(d)()
	if err != nil {
		return fmt.Errorf("couldn't parse the supplied config: %v", err)
	}

	if err := modifyConfig(configAccess, kubeConfig); err != nil {
		return fmt.Errorf("couldn't update kubeconfig: %v", err)
	}

	return nil
}

func DeleteKubeconfig(d *schema.ResourceData, meta interface{}) error {
	return nil
}

func ReadKubeconfig(d *schema.ResourceData, meta interface{}) error {
	return nil
}

func poll(pollInterval, pollTimeout time.Duration, cond func() (bool, error)) bool {
	interval := time.NewTicker(pollInterval)
	defer interval.Stop()
	timeout := time.NewTimer(pollTimeout)
	defer timeout.Stop()

	// Try the first time before waiting.
	if ok, err := cond(); ok {
		log.Printf("[DEBUG] condition succeeded, error: %v", err)
		return true
	} else if err != nil {
		log.Printf("[DEBUG] condition error: %v", err)
		return false
	} else {
		log.Printf("[DEBUG] condition has failed, retrying...")
	}

	for {
		select {
		case <-interval.C:
			if ok, err := cond(); ok {
				log.Printf("[DEBUG] condition succeeded, error: %v", err)
				return true
			} else if err != nil {
				log.Printf("[DEBUG] condition error: %v", err)
				return false
			} else {
				log.Printf("[DEBUG] condition has failed, retrying...")
			}
		case <-timeout.C:
			return false
		}
	}
	// Something went wrong
	log.Printf("[DEBUG] something went wrong while polling, that's all we know")
	return false
}

func allComponentsHealthy(clientset release_1_4.Interface) func() (bool, error) {
	return func() (bool, error) {
		csList, err := clientset.Core().ComponentStatuses().List(api.ListOptions{})
		if err != nil || len(csList.Items) <= 0 {
			log.Printf("[DEBUG] Listing components failed %s", err)
			return false, nil
		}
		for _, cs := range csList.Items {
			if !(len(cs.Conditions) > 0 && cs.Conditions[0].Type == "Healthy" && cs.Conditions[0].Status == "True") {
				log.Printf("[DEBUG] %s isn't healthy. Conditions: %+v", cs.Name, cs.Conditions)
				return false, nil
			}
		}
		return true, nil
	}
}

func kubeConfigGetter(d *schema.ResourceData) clientcmd.KubeconfigGetter {
	return func() (*clientcmdapi.Config, error) {
		kubeConfigStr := d.Get("configdata").(string)
		return clientcmd.Load([]byte(kubeConfigStr))
	}
}

func updateConfig(configAccess clientcmd.ConfigAccess, suppliedConfig *clientcmdapi.Config) func() (bool, error) {
	return func() (bool, error) {
		err := modifyConfig(configAccess, suppliedConfig)
		if err != nil {
			// TODO: We are relying too much on the fact that this error is going
			// to be an *os.PathError returned by file locking mechanism. This is
			// dangerous. Try to introduce a specific error for this in
			// "k8s.io/kubernetes/pkg/client/unversioned/clientcmd" package.
			if os.IsExist(err) {
				return false, nil
			}
			return false, fmt.Errorf("couldn't update kubeconfig: %v", err)
		}
		return true, nil
	}
}

func modifyConfig(configAccess clientcmd.ConfigAccess, suppliedConfig *clientcmdapi.Config) error {
	config, err := configAccess.GetStartingConfig()
	if err != nil {
		return err
	}

	for name, authInfo := range suppliedConfig.AuthInfos {
		initial, ok := config.AuthInfos[name]
		if !ok {
			initial = clientcmdapi.NewAuthInfo()
		}
		modifiedAuthInfo := *initial

		var setToken, setBasic bool

		if len(authInfo.ClientCertificate) > 0 {
			modifiedAuthInfo.ClientCertificate = authInfo.ClientCertificate
		}
		if len(authInfo.ClientCertificateData) > 0 {
			modifiedAuthInfo.ClientCertificateData = authInfo.ClientCertificateData
		}

		if len(authInfo.ClientKey) > 0 {
			modifiedAuthInfo.ClientKey = authInfo.ClientKey
		}
		if len(authInfo.ClientKeyData) > 0 {
			modifiedAuthInfo.ClientKeyData = authInfo.ClientKeyData
		}

		if len(authInfo.Token) > 0 {
			modifiedAuthInfo.Token = authInfo.Token
			setToken = len(modifiedAuthInfo.Token) > 0
		}

		if len(authInfo.Username) > 0 {
			modifiedAuthInfo.Username = authInfo.Username
			setBasic = setBasic || len(modifiedAuthInfo.Username) > 0
		}
		if len(authInfo.Password) > 0 {
			modifiedAuthInfo.Password = authInfo.Password
			setBasic = setBasic || len(modifiedAuthInfo.Password) > 0
		}

		// If any auth info was set, make sure any other existing auth types are cleared
		if setToken || setBasic {
			if !setToken {
				modifiedAuthInfo.Token = ""
			}
			if !setBasic {
				modifiedAuthInfo.Username = ""
				modifiedAuthInfo.Password = ""
			}
		}
		config.AuthInfos[name] = &modifiedAuthInfo
	}

	for name, cluster := range suppliedConfig.Clusters {
		initial, ok := config.Clusters[name]
		if !ok {
			initial = clientcmdapi.NewCluster()
		}
		modifiedCluster := *initial

		if len(cluster.Server) > 0 {
			modifiedCluster.Server = cluster.Server
		}
		if cluster.InsecureSkipTLSVerify {
			modifiedCluster.InsecureSkipTLSVerify = cluster.InsecureSkipTLSVerify
			// Specifying insecure mode clears any certificate authority
			if modifiedCluster.InsecureSkipTLSVerify {
				modifiedCluster.CertificateAuthority = ""
				modifiedCluster.CertificateAuthorityData = nil
			}
		}
		if len(cluster.CertificateAuthorityData) > 0 {
			modifiedCluster.CertificateAuthorityData = cluster.CertificateAuthorityData
			modifiedCluster.InsecureSkipTLSVerify = false
		}
		if len(cluster.CertificateAuthority) > 0 {
			modifiedCluster.CertificateAuthority = cluster.CertificateAuthority
			modifiedCluster.InsecureSkipTLSVerify = false
		}
		config.Clusters[name] = &modifiedCluster
	}

	for name, context := range suppliedConfig.Contexts {
		initial, ok := config.Contexts[name]
		if !ok {
			initial = clientcmdapi.NewContext()
		}
		modifiedContext := *initial

		if len(context.Cluster) > 0 {
			modifiedContext.Cluster = context.Cluster
		}
		if len(context.AuthInfo) > 0 {
			modifiedContext.AuthInfo = context.AuthInfo
		}
		if len(context.Namespace) > 0 {
			modifiedContext.Namespace = context.Namespace
		}
		config.Contexts[name] = &modifiedContext
	}

	if err := clientcmd.ModifyConfig(configAccess, *config, true); err != nil {
		return err
	}

	return nil
}

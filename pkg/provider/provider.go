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

func clusterHealthy(clientset release_1_4.Interface, pollInterval, pollTimeout time.Duration) bool {
	interval := time.NewTicker(pollInterval)
	defer interval.Stop()
	timeout := time.NewTimer(pollTimeout)
	defer timeout.Stop()

	for {
		select {
		case <-interval.C:
			// // Try re-creating the client until all the components turn healthy
			// // or we timeout. Until the server comes up, the clientset creation
			// // completely fails. Also, do not re-use this clientset outside this
			// // block, just to be safe.
			// healthClientSet, err := release_1_4.NewForConfig(restclient.AddUserAgent(clientConfig, UserAgent))
			if allComponentsHealthy(clientset) {
				log.Printf("[DEBUG] all components are healthy")
				return true
			} else {
				log.Printf("[DEBUG] components aren't healthy")
			}
		case <-timeout.C:
			return false
		}
	}
	// Something went wrong
	return false
}

func allComponentsHealthy(clientset release_1_4.Interface) bool {
	csList, err := clientset.Core().ComponentStatuses().List(api.ListOptions{})
	if err != nil || len(csList.Items) <= 0 {
		log.Printf("[DEBUG] Listing components failed %s", err)
		return false
	}
	for _, cs := range csList.Items {
		if !(len(cs.Conditions) > 0 && cs.Conditions[0].Type == "Healthy" && cs.Conditions[0].Status == "True") {
			log.Printf("[DEBUG] %s isn't healthy. Conditions: %+v", cs.Name, cs.Conditions)
			return false
		}
	}
	return true
}

func kubeConfigGetter(d *schema.ResourceData) clientcmd.KubeconfigGetter {
	return func() (*clientcmdapi.Config, error) {
		kubeConfigStr := d.Get("configdata").(string)
		return clientcmd.Load([]byte(kubeConfigStr))
	}
}

func modifyConfig(configAccess clientcmd.ConfigAccess, suppliedConfig *clientcmdapi.Config) error {
	config, err := configAccess.GetStartingConfig()
	if err != nil {
		return err
	}

	for name, cluster := range suppliedConfig.Clusters {
		initial, ok := config.Clusters[name]
		if !ok {
			initial = clientcmdapi.NewCluster()
		}
		modified := *initial

		if len(cluster.Server) > 0 {
			modified.Server = cluster.Server
		}
		if cluster.InsecureSkipTLSVerify {
			modified.InsecureSkipTLSVerify = cluster.InsecureSkipTLSVerify
			// Specifying insecure mode clears any certificate authority
			if modified.InsecureSkipTLSVerify {
				modified.CertificateAuthority = ""
				modified.CertificateAuthorityData = nil
			}
		}
		if len(cluster.CertificateAuthorityData) > 0 {
			modified.CertificateAuthorityData = cluster.CertificateAuthorityData
			modified.InsecureSkipTLSVerify = false
			modified.CertificateAuthority = ""
		} else if len(cluster.CertificateAuthority) > 0 {
			modified.CertificateAuthority = cluster.CertificateAuthority
			modified.InsecureSkipTLSVerify = false
			modified.CertificateAuthorityData = nil
		}
		config.Clusters[name] = &modified
	}

	if err := clientcmd.ModifyConfig(configAccess, *config, true); err != nil {
		return err
	}

	return nil
}

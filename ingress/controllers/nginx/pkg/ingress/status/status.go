/*
Copyright 2015 The Kubernetes Authors.

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

package status

import (
	"fmt"
	"os"
	"time"

	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/api"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/leaderelection"
	"k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/util/wait"
)

// Sync ...
type Sync interface {
	Start()
	Stop()
}

type sync struct {
	client *unversioned.Client

	podInfo *podInfo

	elector *leaderelection.LeaderElector
}

// Start ...
func (s sync) Start() {
	go wait.Forever(s.elector.Run, 0)
}

// Stop ...
func (s sync) Stop() {
	//s.removeFromIngress()
}

// NewStatusSyncer ...
func NewStatusSyncer(client *unversioned.Client, leaderElectionClient *clientset.Clientset) Sync {
	podInfo, err := getPodDetails(client)
	if err != nil {
		glog.Fatalf("unexpected error obtaining pod information: %v", err)
	}
	st := sync{
		client:  client,
		podInfo: podInfo,
	}

	callback := func(leader string) {
		if leader == podInfo.PodName {
			st.updateIngressStatus()
		}
	}

	le, err := NewElection("leader-ingress-controller", podInfo.PodName, podInfo.PodNamespace, 30*time.Second, callback, client, leaderElectionClient)
	if err != nil {
		glog.Fatalf("unexpected error starting leader election: %v", err)
	}
	st.elector = le

	return st
}

func (s *sync) updateIngressStatus() error {
	glog.Infof("updating status of Ingress rules")
	/*
		obj, ingExists, err := lbc.ingLister.Store.GetByKey(key)
		if err != nil {
			return err
		}

		if !ingExists {
			// TODO: what's the correct behavior here?
			return nil
		}

		ing := obj.(*extensions.Ingress)

		ingClient := s.client.Extensions().Ingress(ing.Namespace)

		currIng, err := ingClient.Get(ing.Name)
		if err != nil {
			return fmt.Errorf("unexpected error searching Ingress %v/%v: %v", ing.Namespace, ing.Name, err)
		}

			lbIPs := ing.Status.LoadBalancer.Ingress
			if !s.isStatusIPDefined(lbIPs) {
				glog.Infof("Updating loadbalancer %v/%v with IP %v", ing.Namespace, ing.Name, lbc.podInfo.NodeIP)
				currIng.Status.LoadBalancer.Ingress = append(currIng.Status.LoadBalancer.Ingress, api.LoadBalancerIngress{
					IP: lbc.podInfo.NodeIP,
				})
				if _, err := ingClient.UpdateStatus(currIng); err != nil {
					lbc.recorder.Eventf(currIng, api.EventTypeWarning, "UPDATE", "error: %v", err)
					return err
				}

				lbc.recorder.Eventf(currIng, api.EventTypeNormal, "CREATE", "ip: %v", lbc.podInfo.NodeIP)
			}
	*/
	return nil
}

// removeFromIngress removes the IP address of the node where the Ingres
// controller is running before shutdown to avoid incorrect status
// information in Ingress rules
func (s *sync) removeFromIngress() { //ings []interface{}) {
	if !s.elector.IsLeader() {
		return
	}
	//glog.Infof("updating %v Ingress rule/s", len(ings))
	/*for _, cur := range ings {
		ing := cur.(*extensions.Ingress)

		ingClient := lbc.client.Extensions().Ingress(ing.Namespace)
		currIng, err := ingClient.Get(ing.Name)
		if err != nil {
			glog.Errorf("unexpected error searching Ingress %v/%v: %v", ing.Namespace, ing.Name, err)
			continue
		}

		lbIPs := ing.Status.LoadBalancer.Ingress
		if len(lbIPs) > 0 && lbc.isStatusIPDefined(lbIPs) {
			glog.Infof("Updating loadbalancer %v/%v. Removing IP %v", ing.Namespace, ing.Name, lbc.podInfo.NodeIP)

			for idx, lbStatus := range currIng.Status.LoadBalancer.Ingress {
				if lbStatus.IP == lbc.podInfo.NodeIP {
					currIng.Status.LoadBalancer.Ingress = append(currIng.Status.LoadBalancer.Ingress[:idx],
						currIng.Status.LoadBalancer.Ingress[idx+1:]...)
					break
				}
			}

			if _, err := ingClient.UpdateStatus(currIng); err != nil {
				lbc.recorder.Eventf(currIng, api.EventTypeWarning, "UPDATE", "error: %v", err)
				continue
			}

			lbc.recorder.Eventf(currIng, api.EventTypeNormal, "DELETE", "ip: %v", lbc.podInfo.NodeIP)
		}
	}*/
}

// podInfo contains runtime information about the pod
type podInfo struct {
	PodName      string
	PodNamespace string
	NodeIP       string
}

// getPodDetails  returns runtime information about the pod: name, namespace and IP of the node
func getPodDetails(kubeClient *unversioned.Client) (*podInfo, error) {
	podName := os.Getenv("POD_NAME")
	podNs := os.Getenv("POD_NAMESPACE")

	if podName == "" && podNs == "" {
		return nil, fmt.Errorf("unable to get POD information (missing POD_NAME or POD_NAMESPACE environment variable")
	}

	pod, _ := kubeClient.Pods(podNs).Get(podName)
	if pod == nil {
		return nil, fmt.Errorf("unable to get POD information")
	}

	node, err := kubeClient.Nodes().Get(pod.Spec.NodeName)
	if err != nil {
		return nil, err
	}

	var externalIP string
	for _, address := range node.Status.Addresses {
		if address.Type == api.NodeExternalIP {
			if address.Address != "" {
				externalIP = address.Address
				break
			}
		}

		if externalIP == "" && address.Type == api.NodeLegacyHostIP {
			externalIP = address.Address
		}
	}

	return &podInfo{
		PodName:      podName,
		PodNamespace: podNs,
		NodeIP:       externalIP,
	}, nil
}

func (s *sync) isStatusIPDefined(lbings []api.LoadBalancerIngress) bool {
	/*for _, lbing := range lbings {
		if lbing.IP == lbc.podInfo.NodeIP {
			return true
		}
	}*/

	return false
}

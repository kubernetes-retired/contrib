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

	cache_store "k8s.io/contrib/ingress/controllers/nginx/pkg/cache"
	"k8s.io/contrib/ingress/controllers/nginx/pkg/task"

	"k8s.io/kubernetes/pkg/api"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/leaderelection"
	"k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/util/wait"
)

const (
	updateInterval = 15 * time.Second
)

// Sync ...
type Sync interface {
	Run(stopCh <-chan struct{})
}

// Config ...
type Config struct {
	Client         *unversioned.Client
	ElectionClient *clientset.Clientset
	PublishService string
	IngressLister  cache_store.StoreToIngressLister
}

type sync struct {
	Config

	podInfo *podInfo

	elector *leaderelection.LeaderElector

	syncQueue *task.Queue
}

type dummyObject struct {
	api.ObjectMeta
}

// Run ...
func (s sync) Run(stopCh <-chan struct{}) {
	go wait.Forever(s.elector.Run, 0)
	go s.syncQueue.Run(time.Second, stopCh)
	go s.run()

	<-stopCh

	s.syncQueue.Shutdown()
	// remove IP from Ingress
	s.removeFromIngress()
}

func (s *sync) run() {
	err := wait.PollInfinite(updateInterval, func() (bool, error) {
		if s.syncQueue.IsShuttingDown() {
			return true, nil
		}
		// send a dummy object to the queue to force a sync
		s.syncQueue.Enqueue(&dummyObject{
			ObjectMeta: api.ObjectMeta{
				Name:      "dummy",
				Namespace: "default",
			},
		})
		return false, nil
	})
	if err != nil {
		glog.Errorf("error waiting shutdown: %v", err)
	}
}

func (s *sync) sync(key string) error {
	if !s.elector.IsLeader() {
		glog.V(2).Infof("skipping Ingress status update (I am not the current leader)")
		return nil
	}

	s.updateIngressStatus()
	return nil
}

// callback invoked function when a new leader is elected
func (s *sync) callback(leader string) {
	if s.syncQueue.IsShuttingDown() {
		return
	}

	glog.V(2).Infof("new leader elected (%v)", leader)
	if leader == s.podInfo.podName {
		glog.V(2).Infof("I am the new status update leader")
	}
}

// NewStatusSyncer ...
func NewStatusSyncer(config Config) Sync {
	podInfo, err := getPodDetails(config.Client)
	if err != nil {
		glog.Fatalf("unexpected error obtaining pod information: %v", err)
	}

	st := sync{
		podInfo: podInfo,
	}
	st.Config = config
	st.syncQueue = task.NewTaskQueue(st.sync)

	le, err := NewElection("ingress-controller-leader",
		podInfo.podName, podInfo.podNamespace, 30*time.Second,
		st.callback, config.Client, config.ElectionClient)
	if err != nil {
		glog.Fatalf("unexpected error starting leader election: %v", err)
	}
	st.elector = le
	return st
}

func (s *sync) updateIngressStatus() {
	if !s.elector.IsLeader() {
		return
	}

	glog.Infof("updating status of Ingress rules")
}

// removeFromIngress removes the IP address of the node where the Ingres
// controller is running before shutdown to avoid incorrect status
// information in Ingress rules
func (s *sync) removeFromIngress() {
	if !s.elector.IsLeader() {
		return
	}

	glog.Infof("updating status of Ingress rules (remove)")
	if s.syncQueue.IsShuttingDown() {
		glog.Infof("removing my ip (%v)", s.podInfo.nodeIP)
	}
}

// podInfo contains runtime information about the pod
type podInfo struct {
	podName      string
	podNamespace string
	nodeIP       string
	labels       map[string]string
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

	return &podInfo{
		podName:      podName,
		podNamespace: podNs,
		nodeIP:       getNodeIP(kubeClient, pod.Spec.NodeName),
		labels:       pod.GetLabels(),
	}, nil
}

func (s *sync) isStatusIPDefined(ip string, lbings []api.LoadBalancerIngress) bool {
	for _, lbing := range lbings {
		if lbing.IP == ip {
			return true
		}
	}
	return false
}

func getNodeIP(kubeClient *unversioned.Client, name string) string {
	var externalIP string
	node, err := kubeClient.Nodes().Get(name)
	if err != nil {
		return externalIP
	}

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
	return externalIP
}

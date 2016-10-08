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
	"sort"
	"time"

	"github.com/golang/glog"

	cache_store "k8s.io/contrib/ingress/controllers/nginx/pkg/cache"
	"k8s.io/contrib/ingress/controllers/nginx/pkg/k8s"
	"k8s.io/contrib/ingress/controllers/nginx/pkg/task"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/client/leaderelection"
	"k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/labels"
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
	// pod contains runtime information about this pod
	pod *k8s.PodInfo

	elector *leaderelection.LeaderElector
	// workqueue used to keep in sync the status IP/s
	// in the Ingress rules
	syncQueue *task.Queue
}

// dummyObject is used as a helper to interact with the
// workqueue without real Kubernetes API Objects
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
	if !s.elector.IsLeader() {
		return
	}

	ips, err := s.getRunningIPs()
	if err != nil {
		return
	}

	if len(ips) > 1 {
		// leave the job to the next leader
		return
	}

	glog.Infof("updating status of Ingress rules (remove)")
	glog.Infof("removing my ip (%v)", ips)
	s.updateStatus(sliceToStatus(ips))
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

	ips, err := s.getRunningIPs()
	if err != nil {
		return err
	}
	s.updateStatus(sliceToStatus(ips))

	return nil
}

// callback invoked function when a new leader is elected
func (s *sync) callback(leader string) {
	if s.syncQueue.IsShuttingDown() {
		return
	}

	glog.V(2).Infof("new leader elected (%v)", leader)
	if leader == s.pod.Name {
		glog.V(2).Infof("I am the new status update leader")
	}
}

// NewStatusSyncer ...
func NewStatusSyncer(config Config) Sync {
	pod, err := k8s.GetPodDetails(config.Client)
	if err != nil {
		glog.Fatalf("unexpected error obtaining pod information: %v", err)
	}

	st := sync{
		pod: pod,
	}
	st.Config = config
	st.syncQueue = task.NewTaskQueue(st.sync)

	le, err := NewElection("ingress-controller-leader",
		pod.Name, pod.Namespace, 30*time.Second,
		st.callback, config.Client, config.ElectionClient)
	if err != nil {
		glog.Fatalf("unexpected error starting leader election: %v", err)
	}
	st.elector = le
	return st
}

func (s *sync) getRunningIPs() ([]string, error) {
	ips := []string{}
	if s.PublishService != "" {
		ns, name, _ := k8s.ParseNameNS(s.PublishService)
		svc, err := s.Client.Services(ns).Get(name)
		if err != nil {
			return nil, err
		}

		for _, ip := range svc.Status.LoadBalancer.Ingress {
			ips = append(ips, ip.IP)
		}
	} else {
		// get information about all the pods running the ingress controller
		pods, err := s.Client.Pods(s.pod.Namespace).List(api.ListOptions{
			LabelSelector: labels.SelectorFromSet(s.pod.Labels),
		})
		if err != nil {
			return nil, err
		}

		for _, pod := range pods.Items {
			ips = append(ips, pod.Status.HostIP)
		}
	}

	return ips, nil
}

func sliceToStatus(ips []string) []api.LoadBalancerIngress {
	lbi := []api.LoadBalancerIngress{}
	for _, ip := range ips {
		lbi = append(lbi, api.LoadBalancerIngress{IP: ip})
	}

	sort.Sort(LoadBalancerIngressByIP(lbi))
	return lbi
}

func (s *sync) updateStatus(newIPs []api.LoadBalancerIngress) {
	ings := s.IngressLister.List()
	glog.Infof("updating %v Ingress rule/s", len(ings))
	for _, cur := range ings {
		ing := cur.(*extensions.Ingress)

		ingClient := s.Client.Extensions().Ingress(ing.Namespace)
		currIng, err := ingClient.Get(ing.Name)
		if err != nil {
			glog.Errorf("unexpected error searching Ingress %v/%v: %v", ing.Namespace, ing.Name, err)
			continue
		}

		curIPs := ing.Status.LoadBalancer.Ingress
		sort.Sort(LoadBalancerIngressByIP(curIPs))

		if ingressSliceEqual(newIPs, curIPs) {
			continue
		}

		glog.Infof("updating Ingress status %v/%v to %v", ing.Namespace, ing.Name, newIPs)
		currIng.Status.LoadBalancer.Ingress = newIPs

		if _, err := ingClient.UpdateStatus(currIng); err != nil {
			//s.recorder.Eventf(currIng, api.EventTypeWarning, "UPDATE", "error: %v", err)
			glog.Infof("update error: %v", err)
			//continue
		}
	}
}

func ingressSliceEqual(lhs, rhs []api.LoadBalancerIngress) bool {
	if len(lhs) != len(rhs) {
		return false
	}
	for i := range lhs {
		if !ingressEqual(&lhs[i], &rhs[i]) {
			return false
		}
	}
	return true
}

func ingressEqual(lhs, rhs *api.LoadBalancerIngress) bool {
	if lhs.IP != rhs.IP {
		return false
	}
	if lhs.Hostname != rhs.Hostname {
		return false
	}
	return true
}

// LoadBalancerIngressByIP sorts LoadBalancerIngress using the field IP
type LoadBalancerIngressByIP []api.LoadBalancerIngress

func (c LoadBalancerIngressByIP) Len() int      { return len(c) }
func (c LoadBalancerIngressByIP) Swap(i, j int) { c[i], c[j] = c[j], c[i] }
func (c LoadBalancerIngressByIP) Less(i, j int) bool {
	return c[i].IP < c[j].IP
}

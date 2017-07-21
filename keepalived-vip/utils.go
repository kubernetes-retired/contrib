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

package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/golang/glog"

	apierrs "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	api "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	k8sexec "k8s.io/kubernetes/pkg/util/exec"
	"k8s.io/kubernetes/pkg/util/sysctl"
)

var (
	invalidIfaces = []string{"lo", "docker0", "flannel.1", "cbr0"}
	nsSvcLbRegex  = regexp.MustCompile(`(.*)/(.*):(.*)|(.*)/(.*)`)
	vethRegex     = regexp.MustCompile(`^veth.*`)
	caliRegex     = regexp.MustCompile(`^cali.*`)
	lvsRegex      = regexp.MustCompile(`NAT|PROXY`)
)

type nodeInfo struct {
	iface   string
	ip      string
	netmask int
}

// StoreToEndpointsLister makes a Store that lists Endpoints.
type StoreToEndpointsLister struct {
	cache.Store
}

// GetServiceEndpoints returns the endpoints of a service, matched on service name.
func (s *StoreToEndpointsLister) GetServiceEndpoints(svc *api.Service) (ep api.Endpoints, err error) {
	for _, m := range s.Store.List() {
		ep = *m.(*api.Endpoints)
		if svc.Name == ep.Name && svc.Namespace == ep.Namespace {
			return ep, nil
		}
	}
	err = fmt.Errorf("could not find endpoints for service: %v", svc.Name)
	return
}

// StoreToServiceLister makes a Store that lists Service.
type StoreToServiceLister struct {
	cache.Indexer
}

// getNetworkInfo returns information of the node where the pod is running
func getNetworkInfo(ip string) (*nodeInfo, error) {
	iface, _, mask := interfaceByIP(ip)
	return &nodeInfo{
		iface:   iface,
		ip:      ip,
		netmask: mask,
	}, nil
}

type podInfo struct {
	PodName      string
	PodNamespace string
	NodeIP       string
}

// getPodDetails  returns runtime information about the pod: name, namespace and IP of the node
func getPodDetails(kubeClient *kubernetes.Clientset) (*podInfo, error) {
	podName := os.Getenv("POD_NAME")
	podNs := os.Getenv("POD_NAMESPACE")

	if podName == "" || podNs == "" {
		return nil, fmt.Errorf("Please check the manifest (for missing POD_NAME or POD_NAMESPACE env variables)")
	}

	err := waitForPodRunning(kubeClient, podNs, podName, time.Millisecond*200, time.Second*30)
	if err != nil {
		return nil, err
	}

	pod, _ := kubeClient.Pods(podNs).Get(podName, meta_v1.GetOptions{})
	if pod == nil {
		return nil, fmt.Errorf("Unable to get POD information")
	}

	n, err := kubeClient.Nodes().Get(pod.Spec.NodeName, meta_v1.GetOptions{})
	if err != nil {
		return nil, err
	}

	externalIP, err := getNodeHostIP(n)
	if err != nil {
		return nil, err
	}

	return &podInfo{
		PodName:      podName,
		PodNamespace: podNs,
		NodeIP:       externalIP.String(),
	}, nil
}

// netInterfaces returns a slice containing the local network interfaces
// excluding lo, docker0, flannel.1 and veth interfaces.
func netInterfaces() []net.Interface {
	validIfaces := []net.Interface{}
	ifaces, err := net.Interfaces()
	if err != nil {
		glog.Errorf("unexpected error obtaining network interfaces: %v", err)
		return validIfaces
	}

	for _, iface := range ifaces {
		if !vethRegex.MatchString(iface.Name) &&
			!caliRegex.MatchString(iface.Name) &&
			stringSlice(invalidIfaces).pos(iface.Name) == -1 {
			validIfaces = append(validIfaces, iface)
		}
	}

	glog.Infof("network interfaces: %+v", validIfaces)
	return validIfaces
}

// interfaceByIP returns the local network interface name that is using the
// specified IP address. If no interface is found returns an empty string.
func interfaceByIP(ip string) (string, string, int) {
	for _, iface := range netInterfaces() {
		ifaceIP, mask, err := ipByInterface(iface.Name)
		if err == nil && ip == ifaceIP {
			return iface.Name, ip, mask
		}
	}

	glog.Warningf("no interface with IP address %v detected in the node", ip)
	return "", "", 0
}

func ipByInterface(name string) (string, int, error) {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return "", 32, err
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return "", 32, err
	}

	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ip := ipnet.IP.String()
				ones, _ := ipnet.Mask.Size()
				mask := ones
				return ip, mask, nil
			}
		}
	}

	return "", 32, errors.New("found no IPv4 addresses.")
}

type stringSlice []string

// pos returns the position of a string in a slice.
// If it does not exists in the slice returns -1.
func (slice stringSlice) pos(value string) int {
	for p, v := range slice {
		if v == value {
			return p
		}
	}

	return -1
}

// getClusterNodesIP returns the IP address of each node in the kubernetes cluster
func getClusterNodesIP(kubeClient *kubernetes.Clientset, nodeSelector string) (clusterNodes []string) {
	listOpts := meta_v1.ListOptions{}

	if nodeSelector != "" {
		label, err := labels.Parse(nodeSelector)
		if err != nil {
			glog.Fatalf("'%v' is not a valid selector: %v", nodeSelector, err)
		}
		listOpts.LabelSelector = label.String()
	}

	nodes, err := kubeClient.Core().Nodes().List(listOpts)
	if err != nil {
		glog.Fatalf("Error getting running nodes: %v", err)
	}

	for _, nodo := range nodes.Items {
		nodeIP, err := getNodeHostIP(&nodo)
		if err == nil {
			clusterNodes = append(clusterNodes, nodeIP.String())
		}
	}
	sort.Strings(clusterNodes)

	return
}

// getNodeNeighbors returns a list of IP address of the nodes
func getNodeNeighbors(nodeInfo *nodeInfo, clusterNodes []string) (neighbors []string) {
	for _, neighbor := range clusterNodes {
		if nodeInfo.ip != neighbor {
			neighbors = append(neighbors, neighbor)
		}
	}
	sort.Strings(neighbors)
	return
}

// getPriority returns the priority of one node using the
// IP address as key. It starts in 100
func getNodePriority(ip string, nodes []string) int {
	return 100 + stringSlice(nodes).pos(ip)
}

// loadIPVModule load module require to use keepalived
func loadIPVModule() error {
	out, err := k8sexec.New().Command("modprobe", "ip_vs").CombinedOutput()
	if err != nil {
		glog.V(2).Infof("Error loading ip_vip: %s, %v", string(out), err)
		return err
	}

	_, err = os.Stat("/proc/net/ip_vs")
	return err
}

// changeSysctl changes the required network setting in /proc to get
// keepalived working in the local system.
func changeSysctl() error {
	sys := sysctl.New()
	for k, v := range sysctlAdjustments {
		if err := sys.SetSysctl(k, v); err != nil {
			return err
		}
	}

	return nil
}

func appendIfMissing(slice []string, item string) []string {
	for _, elem := range slice {
		if elem == item {
			return slice
		}
	}
	return append(slice, item)
}

func parseNsName(input string) (string, string, error) {
	nsName := strings.Split(input, "/")
	if len(nsName) != 2 {
		return "", "", fmt.Errorf("invalid format (namespace/name) found in '%v'", input)
	}

	return nsName[0], nsName[1], nil
}

func parseNsSvcLVS(input string) (string, string, string, error) {
	nsSvcLb := nsSvcLbRegex.FindStringSubmatch(input)
	if len(nsSvcLb) != 6 {
		return "", "", "", fmt.Errorf("invalid format (namespace/service name[:NAT|PROXY]) found in '%v'", input)
	}

	ns := nsSvcLb[1]
	svc := nsSvcLb[2]
	kind := nsSvcLb[3]

	if ns == "" {
		ns = nsSvcLb[4]
	}

	if svc == "" {
		svc = nsSvcLb[5]
	}

	if kind == "" {
		kind = "NAT"
	}

	if !lvsRegex.MatchString(kind) {
		return "", "", "", fmt.Errorf("invalid LVS method. Only NAT and PROXY are supported: %v", kind)
	}

	return ns, svc, kind, nil
}

type nodeSelector map[string]string

func (ns nodeSelector) String() string {
	kv := []string{}
	for key, val := range ns {
		kv = append(kv, fmt.Sprintf("%v=%v", key, val))
	}

	return strings.Join(kv, ",")
}

func parseNodeSelector(data map[string]string) string {
	return nodeSelector(data).String()
}

func waitForPodRunning(kubeClient *kubernetes.Clientset, ns, podName string, interval, timeout time.Duration) error {
	condition := func(pod *api.Pod) (bool, error) {
		if pod.Status.Phase == api.PodRunning {
			return true, nil
		}
		return false, nil
	}

	return waitForPodCondition(kubeClient, ns, podName, condition, interval, timeout)
}

// waitForPodCondition waits for a pod in state defined by a condition (func)
func waitForPodCondition(kubeClient *kubernetes.Clientset, ns, podName string, condition func(pod *api.Pod) (bool, error),
	interval, timeout time.Duration) error {
	err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		pod, err := kubeClient.Pods(ns).Get(podName, meta_v1.GetOptions{})
		if err != nil {
			if apierrs.IsNotFound(err) {
				return false, nil
			}
		}

		done, err := condition(pod)
		if err != nil {
			return false, err
		}
		if done {
			return true, nil
		}

		return false, nil
	})

	if err != nil {
		return fmt.Errorf("timed out waiting to observe own status as Running")
	}

	return nil
}

// taskQueue manages a work queue through an independent worker that
// invokes the given sync function for every work item inserted.
type taskQueue struct {
	// queue is the work queue the worker polls
	queue workqueue.RateLimitingInterface
	// sync is called for each item in the queue
	sync func(string) error
	// workerDone is closed when the worker exits
	workerDone chan struct{}
}

func (t *taskQueue) run(period time.Duration, stopCh <-chan struct{}) {
	wait.Until(t.worker, period, stopCh)
}

// enqueue enqueues ns/name of the given api object in the task queue.
func (t *taskQueue) enqueue(obj interface{}) {
	key, err := keyFunc(obj)
	if err != nil {
		glog.Infof("could not get key for object %+v: %v", obj, err)
		return
	}
	t.queue.Add(key)
}

func (t *taskQueue) requeue(key string) {
	t.queue.AddRateLimited(key)
}

// worker processes work in the queue through sync.
func (t *taskQueue) worker() {
	for {
		key, quit := t.queue.Get()
		if quit {
			close(t.workerDone)
			return
		}
		glog.V(3).Infof("syncing %v", key)
		if err := t.sync(key.(string)); err != nil {
			glog.V(3).Infof("requeuing %v, err %v", key, err)
			t.requeue(key.(string))
		} else {
			t.queue.Forget(key)
		}

		t.queue.Done(key)
	}
}

// shutdown shuts down the work queue and waits for the worker to ACK
func (t *taskQueue) shutdown() {
	t.queue.ShutDown()
	<-t.workerDone
}

// NewTaskQueue creates a new task queue with the given sync function.
// The sync function is called for every element inserted into the queue.
func NewTaskQueue(syncFn func(string) error) *taskQueue {
	return &taskQueue{
		queue:      workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		sync:       syncFn,
		workerDone: make(chan struct{}),
	}
}

// copyHaproxyCfg copies the default haproxy configuration file
// to the mounted directory (the mount overwrites the default file)
func copyHaproxyCfg() {
	data, err := ioutil.ReadFile("/haproxy.cfg")
	if err != nil {
		glog.Fatalf("unexpected error reading haproxy.cfg: %v", err)
	}
	err = ioutil.WriteFile("/etc/haproxy/haproxy.cfg", data, 0644)
	if err != nil {
		glog.Fatalf("unexpected error writing haproxy.cfg: %v", err)
	}
}

// GetNodeHostIP returns the provided node's IP, based on the priority:
// 1. NodeInternalIP
// 2. NodeExternalIP
func getNodeHostIP(node *api.Node) (net.IP, error) {
	addresses := node.Status.Addresses
	addressMap := make(map[api.NodeAddressType][]api.NodeAddress)
	for i := range addresses {
		addressMap[addresses[i].Type] = append(addressMap[addresses[i].Type], addresses[i])
	}
	if addresses, ok := addressMap[api.NodeInternalIP]; ok {
		return net.ParseIP(addresses[0].Address), nil
	}
	if addresses, ok := addressMap[api.NodeExternalIP]; ok {
		return net.ParseIP(addresses[0].Address), nil
	}
	return nil, fmt.Errorf("host IP unknown; known addresses: %v", addresses)
}

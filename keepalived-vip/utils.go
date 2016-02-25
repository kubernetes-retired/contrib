/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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
	"net"
	"os"
	"regexp"
	"sort"
	"strconv"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/unversioned"
	k8sexec "k8s.io/kubernetes/pkg/util/exec"
	"k8s.io/kubernetes/pkg/util/node"
	"k8s.io/kubernetes/pkg/util/sysctl"
)

const (
	vethRegex = "^veth.*"
)

var (
	invalidIfaces = []string{"lo", "docker0", "flannel.1", "cbr0"}
)

type nodeInfo struct {
	iface   string
	ip      string
	netmask int
}

// getNodeInfo returns information of the node where the pod is running
func getNodeInfo(nodes []string, forcedIP string) (*nodeInfo, error) {
	ip, err := getEffectiveIP(nodes, forcedIP)
	if err != nil {
		return &nodeInfo{}, err
	}

	return &nodeInfo{
		iface:   interfaceByIP(ip),
		ip:      ip,
		netmask: maskForLocalIP(ip),
	}, nil
}

// getEffectiveIP returns the forcedIP if it was given on the cmdline.
// If forcedIP is not set it will return the node address
func getEffectiveIP(nodes []string, forcedIP string) (string, error) {
	if len(forcedIP) > 0 {
		return forcedIP, nil
	} else {
		ip, err := myIP(nodes)
		return ip, err
	}
}

// myIP returns the local IP address of this node comparing the
// local addresses with the published by the cluster nodes
func myIP(nodes []string) (string, error) {
	var err error
	for _, iface := range netInterfaces() {
		ip, _, err := ipByInterface(iface.Name)
		if err == nil && stringSlice(nodes).pos(ip) != -1 {
			return ip, nil
		}
	}

	glog.Errorf("error getting local IP: %v", err)
	return "", err
}

// netInterfaces returns a slice containing the local network interfaces
// excluding lo, docker0, flannel.1 and veth interfaces.
func netInterfaces() []net.Interface {
	r, _ := regexp.Compile(vethRegex)

	validIfaces := []net.Interface{}
	ifaces, err := net.Interfaces()
	if err != nil {
		return validIfaces
	}

	for _, iface := range ifaces {
		if !r.MatchString(iface.Name) && stringSlice(invalidIfaces).pos(iface.Name) == -1 {
			validIfaces = append(validIfaces, iface)
		}
	}

	return validIfaces
}

// interfaceByIP returns the local network interface name that is using the
// specified IP address. If no interface is found returns an empty string.
func interfaceByIP(ip string) string {
	for _, iface := range netInterfaces() {
		ifaceIP, _, err := ipByInterface(iface.Name)
		if err == nil && ip == ifaceIP {
			return iface.Name
		}
	}

	return ""
}

func maskForLocalIP(ip string) int {
	for _, iface := range netInterfaces() {
		ifaceIP, mask, err := ipByInterface(iface.Name)
		if err == nil && ip == ifaceIP {
			return mask
		}
	}

	return 32
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

	return "", 32, errors.New("Found no IPv4 addresses.")
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
func getClusterNodesIP(kubeClient *unversioned.Client) (clusterNodes []string) {
	nodes, err := kubeClient.Nodes().List(api.ListOptions{})
	if err != nil {
		glog.Fatalf("Error getting running nodes: %v", err)
	}

	for _, nodo := range nodes.Items {
		nodeIP, err := node.GetNodeHostIP(&nodo)
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
func loadKernelModules() error {
	for k, v := range kernelModules {
		out, err := k8sexec.New().Command("modprobe", k).CombinedOutput()
		if err != nil {
			glog.V(2).Infof("Error loading %s: %s, %v", k, string(out), err)
			return err
		}

		if len(v) > 0 {
			_, err := os.Stat(v)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// changeSysctl changes the required network setting in /proc to get
// keepalived working in the local system.
func changeSysctl() error {
	for k, v := range sysctlAdjustments {
		if err := sysctl.SetSysctl(k, v); err != nil {
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

func calculateNetwork(ip string, netmask int) (string, error) {
	_, parsedNet, err := net.ParseCIDR(ip + "/" + strconv.FormatInt(int64(netmask), 10))

	if err != nil {
		return "", err
	}

	return parsedNet.IP.String(), nil
}

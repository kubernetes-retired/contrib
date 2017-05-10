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
package main

import (
	"errors"
	"net"
	"regexp"
	"sort"
)

var (
	invalidIfaces = []string{"lo", "docker0", "flannel.1", "cbr0"}
	vethRegex     = regexp.MustCompile(`^veth.*`)
)

type networkInfo struct {
	iface   string
	ip      string
	netmask int
}

// getNetworkInfo returns information of the node where the pod is running
func getNetworkInfo(ip string) (*networkInfo, error) {
	iface, _, mask := interfaceByIP(ip)
	return &networkInfo{
		iface:   iface,
		ip:      ip,
		netmask: mask,
	}, nil
}

// netInterfaces returns a slice containing the local network interfaces
// excluding lo, docker0, flannel.1 and veth interfaces.
func netInterfaces() []net.Interface {
	validIfaces := []net.Interface{}
	ifaces, err := net.Interfaces()
	if err != nil {
		return validIfaces
	}

	for _, iface := range ifaces {
		if !vethRegex.MatchString(iface.Name) && stringSlice(invalidIfaces).pos(iface.Name) == -1 {
			validIfaces = append(validIfaces, iface)
		}
	}

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

	return "", 32, errors.New("Found no IPv4 addresses.")
}

// getPriority returns the priority of one node using the
// IP address as key. It starts in 100
func getPriority(ip string, peers []string) int {
	return 100 + stringSlice(peers).pos(ip)
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

// getNeighbors returns a list of IP address of the nodes
func getNeighbors(self string, all []string) (neighbors []string) {
	for _, neighbor := range all {
		if self == neighbor {
			continue
		}
		neighbors = append(neighbors, neighbor)
	}
	sort.Strings(neighbors)
	return
}

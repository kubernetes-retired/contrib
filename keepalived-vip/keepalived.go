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
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"text/template"
	"net"

	"github.com/golang/glog"
	k8sexec "k8s.io/kubernetes/pkg/util/exec"
	"k8s.io/kubernetes/pkg/util/iptables"
)

const (
	iptablesChain = "KUBE-KEEPALIVED-VIP"
	keepalivedCfg = "/etc/keepalived/keepalived.conf"
)

var keepalivedTmpl = "keepalived.tmpl"

// Structure for VIP (since we no longer just handle ip addresses)
type VipInfo struct {
        IP          string
        Subnet      int
        VlanId      int
	Gateway     string
	FullSubnet  string
}

// moved from utils.go to keep next to VipInfo structure
func appendIfMissing(slice []VipInfo, item VipInfo) []VipInfo {
        for _, elem := range slice {
                if elem == item {
                        return slice
                }
        }
        return append(slice, item)
}

type keepalived struct {
	iface       string
	ip          string
	netmask     int
	priority    int
	nodes       []string
	neighbors   []string
	useUnicast  bool
	started     bool
	vips        []VipInfo
	vlans       []int
	tmpl        *template.Template
	cmd         *exec.Cmd
	ipt         iptables.Interface
	vrid        int
	vrrpVersion int
	customIface string

}

// WriteCfg creates a new keepalived configuration file.
// In case of an error with the generation it returns the error
func (k *keepalived) WriteCfg(svcs []vip) error {
	w, err := os.Create(keepalivedCfg)
	if err != nil {
		return err
	}
	defer w.Close()

	k.vips = getVIPs(svcs)
	k.vlans = ListVlans(svcs)

	conf := make(map[string]interface{})
	conf["iptablesChain"] = iptablesChain
	conf["iface"] = k.iface
	conf["myIP"] = k.ip
	conf["netmask"] = k.netmask
	conf["svcs"] = svcs
	conf["vips"] = k.vips
	conf["vlans"] = k.vlans
	conf["nodes"] = k.neighbors
	conf["priority"] = k.priority
	conf["useUnicast"] = k.useUnicast
	conf["vrid"] = k.vrid
	conf["vrrpVersion"] = k.vrrpVersion
	conf["customIface"] = k.customIface

	if glog.V(2) {
		b, _ := json.Marshal(conf)
		glog.Infof("%v", string(b))
	}

	return k.tmpl.Execute(w, conf)
}

// getVIPs returns a list of the virtual IP addresses to be used in keepalived
// without duplicates (a service can use more than one port)
func getVIPs(svcs []vip) []VipInfo {
	result := []VipInfo{}
	for _, svc := range svcs {
		_, ipnet, _ := net.ParseCIDR(fmt.Sprintf("%v/%v",svc.IP,svc.Subnet))
		result = appendIfMissing(result, VipInfo{IP: svc.IP, Subnet: svc.Subnet, VlanId: svc.VlanId, Gateway: svc.Gateway, FullSubnet: fmt.Sprintf("%v",ipnet)})
	}

	return result
}

// Start starts a keepalived process in foreground.
// In case of any error it will terminate the execution with a fatal error
func (k *keepalived) Start() {
	ae, err := k.ipt.EnsureChain(iptables.TableFilter, iptables.Chain(iptablesChain))
	if err != nil {
		glog.Fatalf("unexpected error: %v", err)
	}
	if ae {
		glog.V(2).Infof("chain %v already existed", iptablesChain)
	}

	k.cmd = exec.Command("keepalived",
		"--dont-fork",
		"--log-console",
		"--release-vips",
		"--pid", "/keepalived.pid")

	k.cmd.Stdout = os.Stdout
	k.cmd.Stderr = os.Stderr

	k.started = true

	if err := k.cmd.Start(); err != nil {
		glog.Errorf("keepalived error: %v", err)
	}

	if err := k.cmd.Wait(); err != nil {
		glog.Fatalf("keepalived error: %v", err)
	}
}

// Reload sends SIGHUP to keepalived to reload the configuration.
func (k *keepalived) Reload() error {
	if !k.started {
		// TODO: add a warning indicating that keepalived is not started?
		return nil
	}

	glog.Info("reloading keepalived")
	err := syscall.Kill(k.cmd.Process.Pid, syscall.SIGHUP)
	if err != nil {
		return fmt.Errorf("error reloading keepalived: %v", err)
	}

	return nil
}

// Stop stop keepalived process
func (k *keepalived) Stop() {
	for _, vip := range k.vips {
		k.removeVIP(vip)
	}

	err := k.ipt.FlushChain(iptables.TableFilter, iptables.Chain(iptablesChain))
	if err != nil {
		glog.V(2).Infof("unexpected error flushing iptables chain %v: %v", err, iptablesChain)
	}

	err = syscall.Kill(k.cmd.Process.Pid, syscall.SIGTERM)
	if err != nil {
		glog.Errorf("error stopping keepalived: %v", err)
	}
}

func resetIPVS() error {
	glog.Info("cleaning ipvs configuration")
	_, err := k8sexec.New().Command("ipvsadm", "-C").CombinedOutput()
	if err != nil {
		return fmt.Errorf("error removing ipvs configuration: %v", err)
	}

	return nil
}

func (k *keepalived) removeVIP(vip VipInfo) error {
	glog.Infof("removing configured VIP %v", vip)
	iface := CoalesceString(k.customIface, k.iface)
	if vip.VlanId != 0 {
		iface = fmt.Sprintf("%v.%v", iface, vip.VlanId)
	}
	out, err := k8sexec.New().Command("ip", "addr", "del", fmt.Sprintf("%v/%v",vip.IP,vip.Subnet), "dev", iface).CombinedOutput()
	if err != nil {
		return fmt.Errorf("error reloading keepalived: %v\n%s", err, out)
	}
	return nil
}

func (k *keepalived) loadTemplate() error {
	tmpl, err := template.ParseFiles(keepalivedTmpl)
	if err != nil {
		return err
	}
	k.tmpl = tmpl
	return nil
}


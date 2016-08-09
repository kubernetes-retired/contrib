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

package keepalived

import (
	"hash/fnv"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"text/template"

	"github.com/golang/glog"
	"k8s.io/contrib/loadbalancer/loadbalancer-daemon/utils"
	k8sexec "k8s.io/kubernetes/pkg/util/exec"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/sysctl"
)

var (
	keepalivedTmpl = "keepalived.tmpl"
	keepalivedCfg  = "/etc/keepalived/keepalived.conf"
	// sysctl changes required by keepalived
	sysctlAdjustments = map[string]int{
		// allows processes to bind() to non-local IP addresses
		"net/ipv4/ip_nonlocal_bind": 1,
	}
	keepAlivedPIDFile      = "/var/run/keepalived.pid"
	defaultVirtualRouterID = "50"
)

// KeepalivedController is a object that manages the keepalived config
type KeepalivedController struct {
	keepalived *Keepalived
	exitChan   chan struct{}
}

// Keepalived represents all the elements needed for constructing a keepalived conf
type Keepalived struct {
	Interface       string
	Vips            sets.String
	VirtualRouterID string
	Password        string
}

// NewKeepalivedController creates a new keepalived controller
func NewKeepalivedController(nodeInterface string) KeepalivedController {

	// System init
	err := changeSysctl()
	if err != nil {
		glog.Errorf("Unexpected error for system settings: %v", err)
	}

	virtualRouterID := os.Getenv("VIRTUAL_ROUTER_ID")
	if len(virtualRouterID) == 0 {
		virtualRouterID = defaultVirtualRouterID
	}
	hash := fnv.New32a()
	hash.Write([]byte(virtualRouterID))
	k := Keepalived{
		Interface:       nodeInterface,
		Vips:            sets.NewString(),
		VirtualRouterID: virtualRouterID,
		Password:        strconv.FormatUint(uint64(hash.Sum32()), 10),
	}

	kaControl := KeepalivedController{
		keepalived: &k,
		exitChan:   make(chan struct{}),
	}
	return kaControl
}

// Start starts a keepalived process in foreground.
// In case of any error it will terminate the execution with a fatal error
func (k *KeepalivedController) Start() {

	opts := os.Getenv("KEEPALIVED_OPTS")
	var args []string
	if len(opts) == 0 {
		args = append(args, "--log-console")
		args = append(args, "--vrrp")
		args = append(args, "--release-vips")
	} else {
		args = strings.Split(opts, ",")
		for index, arg := range args {
			args[index] = "--" + arg
		}
	}

	glog.Infof("Starting keepalived with options %v", args)
	cmd := exec.Command("keepalived", args...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// in case the pod is terminated we need to check that the vips are removed
	c := make(chan os.Signal, 2)
	signal.Notify(c, syscall.SIGTERM)
	go func() {
		for range c {
			glog.Warning("TERM signal received. freeing vips")
			k.Clean()
		}
	}()

	if err := cmd.Start(); err != nil {
		glog.Errorf("keepalived error: %v", err)
	}

	if err := cmd.Wait(); err != nil {
		glog.Fatalf("keepalived error: %v", err)
	}

	// Monitors the keepalived process
	go utils.MonitorProcess(keepAlivedPIDFile, k.ExitChannel())
}

// Clean removes all VIPs created by keepalived.
func (k *KeepalivedController) Clean() {
	for vip := range k.keepalived.Vips {
		glog.Infof("removing configured VIP %v", vip)
		k8sexec.New().Command("ip", "addr", "del", vip+"/32", "dev", k.keepalived.Interface).CombinedOutput()
	}
}

// ExitChannel returns the channel used to communicate keepalived process has exited
func (k *KeepalivedController) ExitChannel() chan struct{} {
	return k.exitChan
}

// AddVIP adds a new VIP to the keepalived config and reload keepalived process
func (k *KeepalivedController) AddVIP(vip string) {
	glog.Infof("Adding VIP %v", vip)
	if k.keepalived.Vips.Has(vip) {
		glog.Errorf("VIP %v has already been added", vip)
		return
	}
	k.keepalived.Vips.Insert(vip)
	k.writeCfg()
	k.reload()
}

// DeleteVIP removes a VIP from the keepalived config and reload keepalived process
func (k *KeepalivedController) DeleteVIP(vip string) {
	glog.Infof("Deleing VIP %v", vip)
	if !k.keepalived.Vips.Has(vip) {
		glog.Errorf("VIP %v had not been added.", vip)
		return
	}
	k.keepalived.Vips.Delete(vip)
	k.writeCfg()
	k.reload()
}

// DeleteAllVIPs Delete all VIPs from the keepalived config and reload keepalived process
func (k *KeepalivedController) DeleteAllVIPs() {
	glog.Infof("Deleing all VIPs")
	k.keepalived.Vips.Delete(k.keepalived.Vips.List()...)
	k.writeCfg()
	k.reload()
}

// writeCfg creates a new keepalived configuration file.
// In case of an error with the generation it returns the error
func (k *KeepalivedController) writeCfg() {
	tmpl, err := template.New(keepalivedTmpl).ParseFiles(keepalivedTmpl)
	w, err := os.Create(keepalivedCfg)
	if err != nil {
		glog.Fatalf("Failed to open %v: %v", keepalivedCfg, err)
	}
	defer w.Close()

	conf := make(map[string]interface{})
	conf["interface"] = k.keepalived.Interface
	conf["vips"] = k.keepalived.Vips.List()
	conf["virtualRouterID"] = k.keepalived.VirtualRouterID
	conf["password"] = k.keepalived.Password
	if err := tmpl.Execute(w, conf); err != nil {
		glog.Fatalf("Failed to write template %v", err)
	}
}

// reload sends SIGHUP to keepalived to reload the configuration.
func (k *KeepalivedController) reload() {
	glog.Infof("Reloading keepalived")
	pid, _ := utils.ReadPID(keepAlivedPIDFile)
	err := syscall.Kill(pid, syscall.SIGHUP)
	if err != nil {
		glog.Fatalf("Could not reload keepalived: %v", err)
	}
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

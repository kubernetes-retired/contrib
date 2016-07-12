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
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"text/template"

	"github.com/golang/glog"
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
		// enable connection tracking for LVS connections
		"net/ipv4/vs/conntrack": 1,
	}
)

type KeepalivedController struct {
	keepalived *Keepalived
}

type Keepalived struct {
	Interface string
	Vips      sets.String
}

// NewKeepalivedController creates a new keepalived controller
func NewKeepalivedController(nodeInterface string) KeepalivedController {

	// System init
	changeSysctl()

	k := Keepalived{
		Interface: nodeInterface,
		Vips:      sets.NewString(),
	}

	kaControl := KeepalivedController{
		keepalived: &k,
	}

	return kaControl
}

// Start starts a keepalived process in foreground.
// In case of any error it will terminate the execution with a fatal error
func (k *KeepalivedController) Start() {

	glog.Infof("Starting keepalived")
	cmd := exec.Command("keepalived",
		"--log-console",
		"--release-vips")

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// in case the pod is terminated we need to check that the vips are removed
	c := make(chan os.Signal, 2)
	signal.Notify(c, syscall.SIGTERM)
	go func() {
		for range c {
			glog.Warning("TERM signal received. freeing vips")
			for vip := range k.keepalived.Vips {
				k.freeVIP(vip)
			}
		}
	}()

	if err := cmd.Start(); err != nil {
		glog.Errorf("keepalived error: %v", err)
	}

	if err := cmd.Wait(); err != nil {
		glog.Fatalf("keepalived error: %v", err)
	}
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
	if err := tmpl.Execute(w, conf); err != nil {
		glog.Fatalf("Failed to write template %v", err)
	}
}

// reload sends SIGHUP to keepalived to reload the configuration.
func (k *KeepalivedController) reload() {
	glog.Infof("Reloading keepalived")
	keepalivedPID, err := ioutil.ReadFile("/var/run/keepalived.pid")
	if err != nil {
		glog.Fatalf("Unable to get PID for keepalived. %v", err)
	}
	pidInt, _ := strconv.Atoi(strings.Trim(string(keepalivedPID), "\n"))
	glog.Infof("Pid of keepalived is %v", pidInt)

	err = syscall.Kill(pidInt, syscall.SIGHUP)
	if err != nil {
		glog.Fatalf("Could not reload keepalived: %v", err)
	}
}

func (k *KeepalivedController) freeVIP(vip string) error {
	glog.Infof("removing configured VIP %v", vip)
	out, err := k8sexec.New().Command("ip", "addr", "del", vip+"/32", "dev", k.keepalived.Interface).CombinedOutput()
	if err != nil {
		return fmt.Errorf("error reloading keepalived: %v\n%s", err, out)
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

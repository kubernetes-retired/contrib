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
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"text/template"

	"github.com/golang/glog"
	k8sexec "k8s.io/kubernetes/pkg/util/exec"
)

const (
	keepalivedTmpl = `{{ $iface := .iface }}{{ $netmask := .netmask }}
vrrp_sync_group VG_1 
  group {
    vips
  }
}

vrrp_instance vips {
  state BACKUP
  interface {{ $iface }}
  virtual_router_id 50
  priority {{ .priority }}
  nopreempt
  advert_int 1

  track_interface {
    {{ $iface }}
  }

  {{ if .useUnicast }}
  unicast_src_ip {{ .myIP }}
  unicast_peer { {{ range .nodes }}
    {{ . }}{{ end }}
  }
  {{ end }}

  virtual_ipaddress { {{ range .vips }}
    {{ . }}{{ end }}
  }

  authentication {
    auth_type AH
    auth_pass {{ .authPass }}
  }
}

{{ range $i, $svc := .svcs }}# Start {{$svc.Name}}
{{ range $j, $port := $svc.Ports }}# Start {{$port.Port}} {{$port.Protocol}}
virtual_server {{ $svc.IP }} {{ $port.Port }} {
  delay_loop 5
  lvs_sched wlc
  lvs_method NAT
  persistence_timeout 1800
  protocol {{ $port.Protocol }}

  {{ range $k, $backend := $port.Backends }}
  real_server {{ $backend.IP }} {{ $backend.Port }} {
    weight 1
    TCP_CHECK {
      connect_port {{ $backend.Port }}
      connect_timeout 3
    }
  }
  {{ end }}
}
# End {{$port.Port}} {{$port.Protocol}}
{{ end }}# End {{$svc.Name}}
{{ end }}
`
)

type keepalived struct {
	iface          string
	ip             string
	netmask        int
	priority       int
	nodes          []string
	neighbors      []string
	useUnicast     bool
	password       string
	vips           []string
	keepalivedTmpl *template.Template
	cmd            *exec.Cmd
}

var (
	keepalivedPath = flags.String("keepalived", "keepalived",
		`Path to keepalived`)
	keepalivedTemplateFilename = flags.String("keepalivedconftemplate",
		"/etc/keepalived/keepalived.conf.template", `Path to keepalived.conf.template`)
	keepalivedConfFilename = flags.String("keepalivedconf", "/etc/keepalived/keepalived.conf",
		`Path to keepalived.conf`)
	keepalivedPidFilename = flags.String("keepalivedpid", "/run/keepalived.pid",
		`Path to keepalived.conf`)
	debugKeeplivedConf = flags.Bool("debug-keepalived-conf", false,
		`debug keepalived.conf generation`)
)

// WriteCfg creates a new keepalived configuration file.
// In case of an error with the generation it returns the error
func (k *keepalived) WriteCfg(svcs []service, routes []vroute) error {
	if k.keepalivedTmpl == nil {
		err := k.loadTemplate()
		if err != nil {
			return err
		}
	}

	w, err := os.Create(*keepalivedConfFilename)
	if err != nil {
		return err
	}
	defer w.Close()

	k.vips = getVIPs(svcs)

	conf := make(map[string]interface{})
	conf["iface"] = k.iface
	conf["myIP"] = k.ip
	conf["netmask"] = k.netmask
	conf["svcs"] = svcs
	conf["vips"] = k.vips
	conf["routes"] = routes
	conf["nodes"] = k.neighbors
	conf["priority"] = k.priority
	// password to protect the access to the vrrp_instance group
	if k.password != "" {
		conf["authPass"] = k.password[0:8]
	} else {
		conf["authPass"] = k.getSha()[0:8]
	}
	conf["useUnicast"] = k.useUnicast

	if glog.V(2) {
		b, _ := json.Marshal(conf)
		glog.Infof("%v", string(b))
	}

	return k.keepalivedTmpl.Execute(w, conf)
}

// getVIPs returns a list of the virtual IP addresses to be used in keepalived
// without duplicates (a service can use more than one port)
func getVIPs(svcs []service) []string {
	result := []string{}
	for _, svc := range svcs {
		result = appendIfMissing(result, svc.IP)
	}

	return result
}

// Start starts a keepalived process in foreground.
// In case of any error it will terminate the execution with a fatal error
func (k *keepalived) Start() error {
	if *debugKeeplivedConf {
		return nil
	}
	glog.Info("starting keepalived to announce VIPs")

	k.cmd = exec.Command(*keepalivedPath,
		"--dont-fork",
		"--log-console",
		"--release-vips",
		"-D",
		"--pid", *keepalivedPidFilename,
		"--use-file", *keepalivedConfFilename)

	k.cmd.Stdout = os.Stdout
	k.cmd.Stderr = os.Stderr

	// in case the pod is terminated we need to check that the vips are removed
	c := make(chan os.Signal, 2)
	signal.Notify(c, syscall.SIGTERM)
	go func() {
		for range c {
			glog.Warning("TERM signal received. removing vips")
			for _, vip := range k.vips {
				k.removeVIP(vip)
			}
		}
	}()

	if err := k.cmd.Start(); err != nil {
		glog.Errorf("keepalived error: %v", err)
		return err
	}

	/*
	if err := k.cmd.Wait(); err != nil {
		glog.Fatalf("keepalived error: %v", err)
		return err
	}
	*/
	return nil
}

// Reload sends SIGHUP to keepalived to reload the configuration.
func (k *keepalived) Reload() error {
	if k.cmd == nil {
		// TODO: add a warning indicating that keepalived is not started?
		return nil
	}

	glog.Info("reloading keepalived")
	err := syscall.Kill(k.cmd.Process.Pid, syscall.SIGHUP)

	if err != nil {
		return fmt.Errorf("error reloading keepalived: %v\n", err)
	}

	return nil
}

// getSha returns a sha1 of the list of nodes in the cluster using the IP
// address to create a password to be used in the authentication of the
// vrrp_instance
func (k *keepalived) getSha() string {
	h := sha1.New()
	h.Write([]byte(fmt.Sprintf("%v", k.nodes)))
	return hex.EncodeToString(h.Sum(nil))
}

// Reset virtual ips
func resetIPVS() error {
	glog.Info("cleaning ipvs configuration")
	_, err := k8sexec.New().Command("ipvsadm", "-C").CombinedOutput()
	if err != nil {
		return fmt.Errorf("error removing ipvs configuration: %v", err)
	}

	return nil
}

// Remove virtual ips
func (k *keepalived) removeVIP(vip string) error {
	glog.Infof("removing configred VIP %v", vip)
	out, err := k8sexec.New().Command("ip", "addr", "del", vip+"/32", "dev", k.iface).CombinedOutput()
	if err != nil {
		return fmt.Errorf("error reloading keepalived: %v\n%s", err, out)
	}
	return nil
}

// Initialize template from file or constant
func (k *keepalived) loadTemplate() error {
	if _, err := os.Stat(*keepalivedTemplateFilename); os.IsNotExist(err) {
		k.keepalivedTmpl, err = template.New("keepalived").Parse(keepalivedTmpl)
		if err != nil {
			return err
		}
	} else {
		k.keepalivedTmpl, err = template.ParseFiles(*keepalivedTemplateFilename)

		if err != nil {
			glog.Fatalf("cannot read keepalived.conf template at %v: %v",
				*keepalivedTemplateFilename, err)
			return err
		}
	}
	return nil
}

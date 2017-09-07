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

package haproxy

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"text/template"

	log "github.com/golang/glog"
	"k8s.io/contrib/ingress/controllers/haproxy/pkg/balancer"
	hatemplate "k8s.io/contrib/ingress/controllers/haproxy/pkg/balancer/haproxy/template"
)

const (
	tmpSubDir = "tmp"

	hostPrio = 4 >> iota
	pathPrio
	affinityPrio
)

// Manager is an HAProxy manager
type Manager struct {
	config         HAProxy
	configFile     string
	balancerScript string
	certsDir       string
	template       *template.Template
	mu             sync.Mutex
}

// NewManager returns the HAProxy balancer manager
func NewManager(configFile string, balancerScript string, certsDir string) (balancer.Manager, error) {
	log.Infof("new haproxy manager with config: %s script: %s certs: %s", configFile, balancerScript, certsDir)

	t, e := template.New("haproxy").Parse(hatemplate.HAProxy)
	if e != nil {
		panic(e)
	}

	if configFile == "" {
		return nil, fmt.Errorf("no haproxy configuration file informed")
	}

	if balancerScript == "" {
		return nil, fmt.Errorf("no haproxy shell script informed")
	}

	if certsDir == "" {
		return nil, fmt.Errorf("no certs directory informed")
	}

	if _, e = os.Stat(balancerScript); e != nil {
		return nil, fmt.Errorf("can't read balancer shell script file '%s': %+v", balancerScript, e)
	}

	if _, e = os.Stat(certsDir); e != nil {
		if err := os.MkdirAll(certsDir, 0755); err != nil {
			return nil, fmt.Errorf("can't create certificates directory '%s': %+v", certsDir, err)
		}
	}

	absCerts, e := filepath.Abs(certsDir)
	if e != nil {
		return nil, fmt.Errorf("can't get absolute dir for directory '%s': %+v", certsDir, e)
	}

	m := &Manager{
		template:       t,
		configFile:     configFile,
		balancerScript: balancerScript,
		certsDir:       absCerts,
	}

	return balancer.Manager(m), nil
}

// WriteConfigAndRestart if not forced, compares argument config with cached config
// writes a new config and restarts balancer
func (m *Manager) WriteConfigAndRestart(config *balancer.Config, force bool) error {

	ok, e := m.writeConfig(config, force)
	if e != nil {
		return e
	}

	if !ok {
		return nil
	}

	e = m.ReloadBalancer()

	return e
}

// writeConfig returns true if the file was written
func (m *Manager) writeConfig(config *balancer.Config, force bool) (bool, error) {
	log.V(4).Infof("writing config:%+v", config)
	m.mu.Lock()
	defer m.mu.Unlock()

	// from balancer.config to haproxy
	c := m.generateHAProxyConfig(config)
	log.V(4).Infof("haproxy config structure is:%+v", config)

	if !force {
		if reflect.DeepEqual(m.config, *c) {
			log.V(2).Info("no configuration changes detected, skipping haproxy configuration re-write")
			return false, nil
		}
	}

	tmpConfig := fmt.Sprintf("%s.tmp", m.configFile)
	cfgTemp, e := os.Create(tmpConfig)
	if e != nil {
		return false, e
	}

	log.Infof("generating configuration file '%s'", m.configFile)
	cfgWriter := bufio.NewWriter(cfgTemp)

	e = m.template.ExecuteTemplate(cfgWriter, "haproxy", *c)
	if e != nil {
		return false, e
	}
	cfgWriter.Flush()

	tmpCerts := path.Join(m.certsDir, tmpSubDir)
	if e = m.writeCertificates(c, tmpCerts); e != nil {
		return false, e
	}

	m.switchCerts(tmpCerts, m.certsDir)

	e = os.Rename(tmpConfig, m.configFile)
	if e != nil {
		return false, e
	}

	m.config = *c
	return true, nil
}

// generateHAProxyConfig translates a generic configuration to an haproxy friendly configuration
func (m *Manager) generateHAProxyConfig(config *balancer.Config) *HAProxy {
	ha := NewDefaultBalancer()
	ha.CertsDir = m.certsDir

	for _, exposed := range config.Exposed {

		if len(exposed.Upstream.Endpoints) == 0 {
			log.Warningf("exposed frontend %s with backend %s had no endpoints. Skipping", exposed.Name(), exposed.Upstream.Name)
			continue
		}

		// grab haproxy frontend or create it if it doesn't exists
		fe, ok := ha.Frontends[exposed.BindPort]
		if !ok {
			fe = FrontEnd{
				Name: fmt.Sprintf("port%d", exposed.BindPort),
				Bind: Bind{Port: exposed.BindPort, IP: "*"},
				ACLs: make(map[string]ACL),
			}
		}

		// if backend doesn't already exists, create it
		if _, ok := ha.Backends[exposed.Upstream.Name]; !ok {
			// get upstream endpoint into backend
			backend := Backend{}
			backend.Servers = make(map[string]Server)
			for _, ep := range exposed.Upstream.Endpoints {
				s := NewDefaultBackendServer()
				s.Address = ep.IP
				s.Port = ep.Port

				// remove duplicates
				n := s.Name()
				if _, ok := backend.Servers[n]; ok {
					log.Infof("found existing backend server for %s. Overwriting with backend server %+v", n, s)
				}
				backend.Servers[n] = *s
			}
			ha.Backends[exposed.Upstream.Name] = backend
		}

		var ub UseBackend
		ub.Backend = exposed.Upstream.Name

		// if IsDefault no rules apply, add certs and continue to next exposed service
		if exposed.IsDefault {
			// if default, certificate must be placed first
			fe.Bind.Certs = append(exposed.Certificates, fe.Bind.Certs...)
			ub.Priority = 0
			fe.DefaultBackend = ub
			ha.Frontends[exposed.BindPort] = fe
			continue
		}

		fe.Bind.Certs = append(fe.Bind.Certs, exposed.Certificates...)

		// priority manages the order in which the set of ACLs is matched.
		prio := 0
		var hostACL ACL
		if exposed.HostName != "" {
			prio += hostPrio
			hostACL = *NewHostNameACL(exposed.HostName)
			ub.ACLs = append(ub.ACLs, hostACL)
			fe.ACLs[hostACL.Name] = hostACL
		}

		if p := strings.TrimSpace(exposed.PathBegins); p != "" && p != "/" {
			prio += pathPrio
			pathACL := *NewPathACL(p)
			ub.ACLs = append(ub.ACLs, pathACL)
			fe.ACLs[pathACL.Name] = pathACL
		}

		ub.Priority = prio
		fe.UseBackends = append(fe.UseBackends, ub)
		ha.Frontends[exposed.BindPort] = fe
	}

	return ha
}

// writeCertificates to disk
func (m *Manager) writeCertificates(ha *HAProxy, dir string) error {
	log.Info("generating certificates")

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("error creating certificates folder: %+v", err)
	}

	for _, fe := range ha.Frontends {
		for _, c := range fe.Bind.Certs {
			f, err := os.Create(path.Join(dir, fmt.Sprintf("%s.pem", c.Name)))
			if err != nil {
				return err
			}
			defer f.Close()
			if _, e := f.Write(c.Public); e != nil {
				return fmt.Errorf("error writing public key to %s: %+v", f.Name(), err)
			}
			if _, e := f.Write(c.Private); e != nil {
				return fmt.Errorf("error writing private key to %s: %+v", f.Name(), err)
			}
		}
	}

	return nil
}

// switchCerts moves temporary writen certificates to be used by haproxy
func (m *Manager) switchCerts(from, to string) error {

	files, _ := ioutil.ReadDir(to)
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		log.V(4).Infof("cert %s is being deleted", f.Name())
		if e := os.Remove(path.Join(to, f.Name())); e != nil {
			// move on, write new certs to avoid leaving inconsistent certs
			log.Errorf("error removing certificate file %s: %+v", f.Name(), e)
		}
	}

	files, _ = ioutil.ReadDir(from)
	var err error
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		log.V(4).Infof("cert %s is being moved to be used by haproxy", f.Name())
		if e := os.Rename(path.Join(from, f.Name()), path.Join(to, f.Name())); e != nil {
			// keep last error but contine moving files
			err = e
			log.Errorf("error moving certificate file %s: %+v", f.Name(), e)
		}
	}

	if err != nil {
		return fmt.Errorf("errors occurred moving certificates from temp folder: %+v", err)
	}

	return nil

}

/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package nginx

import (
	"bytes"
	"fmt"
	"html/template"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strconv"
	"sync"

	factory "k8s.io/contrib/loadbalancer/loadbalancer-daemon/backend"
	"k8s.io/contrib/loadbalancer/loadbalancer-daemon/utils"

	"github.com/golang/glog"
)

var (
	configPath   = "/etc/nginx"
	nginxPIDFile = "/var/run/nginx.pid"
)

// NGINXController Updates NGINX configuration, starts and reloads NGINX
type NGINXController struct {
	nginxConfdPath    string
	nginxTCPConfdPath string
	nginxCertsPath    string
	exitChan          chan struct{}
}

// NGINXConfig describes the upstreams and servers needed for the conf file
type NGINXConfig struct {
	Upstreams []Upstream
	Servers   []Server
}

// Upstream describes an NGINX upstream
type Upstream struct {
	Name           string
	UpstreamServer UpstreamServer
}

// UpstreamServer describes a server in an NGINX upstream
type UpstreamServer struct {
	Address string
	Port    string
}

// Server describes an NGINX server
type Server struct {
	Name              string
	BindIP            string
	BindPort          string
	Location          Location
	SSL               bool
	SSLPort           string
	SSLCertificate    string
	SSLCertificateKey string
}

// Location describes an NGINX location
type Location struct {
	Path     string
	Upstream Upstream
}

func init() {
	factory.Register("nginx", NewNGINXController)
}

var reloadMutex sync.Mutex

// NewNGINXController creates a NGINX controller
func NewNGINXController() (factory.BackendController, error) {

	// Generate nginx.conf file
	configString :=
		`
worker_processes auto;
pid /run/nginx.pid;

events {
	worker_connections 768;
}

`
	generateNginxCfg(configString)

	ngxc := NGINXController{
		nginxConfdPath:    path.Join(configPath, "conf.d"),
		nginxTCPConfdPath: path.Join(configPath, "conf.d", "tcp"),
		nginxCertsPath:    path.Join(configPath, "ssl"),
		exitChan:          make(chan struct{}),
	}

	// This is to add a include directive for tcp apps
	appendStreamDirectiveToCfg(path.Join(configPath, "nginx.conf"))
	// Create tcp config dir
	createDir(ngxc.nginxTCPConfdPath)
	// Create cert dir
	createDir(ngxc.nginxCertsPath)
	start()

	// Monitors the nginx process
	go utils.MonitorProcess(nginxPIDFile, ngxc.exitChan)

	return &ngxc, nil
}

// Name returns the name of the backend controller
func (nginx *NGINXController) Name() string {
	return "NGINXController"
}

// AddConfig creates or updates a file with
// the specified configuration for the specified config
func (nginx *NGINXController) AddConfig(name string, config factory.BackendConfig) {
	glog.Infof("Updating NGINX configuration")
	glog.Infof("Received config %s: %v", name, config)
	nginxConfig := generateNGINXCfg(nginx.nginxCertsPath, name, config)

	var configFile string
	if config.Path != "" {
		// HTTP app so put config file under conf.d
		configFile = getNGINXConfigFileName(nginx.nginxConfdPath, name)
	} else {
		// TCP app so put config file under conf.d/tcp
		configFile = getNGINXConfigFileName(nginx.nginxTCPConfdPath, name)
	}
	templateIt(nginxConfig, configFile)
	reload()
}

// DeleteConfig deletes the configuration file, which corresponds for the
// specified configmap from NGINX conf directory
func (nginx *NGINXController) DeleteConfig(name string) {
	filename := getNGINXConfigFileName(nginx.nginxTCPConfdPath, name)

	// check if filename exist.
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return
	}

	glog.Infof("Deleting %v", filename)
	if err := os.Remove(filename); err != nil {
		glog.Warningf("Failed to delete %v: %v", filename, err)
	}
	reload()
}

// ExitChannel returns the channel used to communicate nginx process has exited
func (nginx *NGINXController) ExitChannel() chan struct{} {
	return nginx.exitChan
}

func getNGINXConfigFileName(cnfPath string, name string) string {
	return path.Join(cnfPath, name+".conf")
}

func templateIt(config NGINXConfig, filename string) {
	tmpl, err := template.New("nginx.tmpl").ParseFiles("nginx.tmpl")
	if err != nil {
		glog.Fatalf("Failed to parse template. %v", err)
	}

	glog.Infof("Writing NGINX conf to %v", filename)

	w, err := os.Create(filename)
	if err != nil {
		glog.Fatalf("Failed to open %v: %v", filename, err)
	}
	defer w.Close()

	if err := tmpl.Execute(w, config); err != nil {
		glog.Fatalf("Failed to write template %v", err)
	}

	glog.Infof("NGINX configuration file had been updated")
}

// Reload reloads NGINX
func reload() {
	reloadMutex.Lock()
	shellOut("nginx -s reload")
	reloadMutex.Unlock()
}

// Start starts NGINX
func start() {
	shellOut("nginx")
}

func createDir(path string) {
	if err := os.Mkdir(path, os.ModeDir); err != nil {
		if os.IsExist(err) {
			glog.Infof("%v already exists", err)
			return
		}
		glog.Fatalf("Couldn't create directory %v: %v", path, err)
	}
}

func generateNginxCfg(configFile string) {
	ioutil.WriteFile(path.Join(configPath, "nginx.conf"), []byte(configFile), 0644)
}

func appendStreamDirectiveToCfg(configFile string) {
	directive :=
		`
stream {
    include /etc/nginx/conf.d/tcp/*.conf;
}
`

	f, err := os.OpenFile(configFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		glog.Fatalf("Couldn't open config file %v: %v", configFile, err)
	}

	defer f.Close()

	if _, err = f.WriteString(directive); err != nil {
		glog.Fatalf("Couldn't create stream directive to config file %v: %v", configFile, err)
	}
}

func generateNGINXCfg(certPath string, name string, config factory.BackendConfig) NGINXConfig {

	var nginxConfig NGINXConfig
	if config.TargetServiceName == "" || config.TargetIP == "" {
		glog.Errorf("Target service name or IP was not provided in config %v.", name)
		return nginxConfig
	}

	upstreams := []Upstream{}
	servers := []Server{}
	for _, port := range config.Ports {
		upsName := getNameForUpstream(name, config.Host, config.TargetServiceName, port)
		upstream := createUpstream(upsName, config.TargetIP, port)
		upstreams = append(upstreams, upstream)

		serverName := config.Host
		server := Server{
			Name:     serverName,
			BindIP:   config.BindIp,
			BindPort: port,
		}
		if config.SSL {
			pemFile := addOrUpdateCertAndKey(certPath, name, config.TlsCert, config.TlsKey)
			server.SSLPort = strconv.Itoa(config.SSLPort)
			server.SSL = true
			server.SSLCertificate = pemFile
			server.SSLCertificateKey = pemFile
		}

		loc := Location{
			Path:     config.Path,
			Upstream: upstream,
		}
		server.Location = loc
		servers = append(servers, server)
	}

	nginxConfig = NGINXConfig{
		Upstreams: upstreams,
		Servers:   servers,
	}

	return nginxConfig
}

func createUpstream(name, address, port string) Upstream {
	ups := Upstream{
		Name: name,
		UpstreamServer: UpstreamServer{
			Address: address,
			Port:    port,
		},
	}
	return ups
}

func addOrUpdateCertAndKey(path string, name string, cert string, key string) string {
	pemFileName := path + "/" + name + ".pem"

	pem, err := os.Create(pemFileName)
	if err != nil {
		glog.Fatalf("Couldn't create pem file %v: %v", pemFileName, err)
	}
	defer pem.Close()

	_, err = pem.WriteString(key)
	if err != nil {
		glog.Fatalf("Couldn't write to pem file %v: %v", pemFileName, err)
	}

	_, err = pem.WriteString("\n")
	if err != nil {
		glog.Fatalf("Couldn't write to pem file %v: %v", pemFileName, err)
	}

	_, err = pem.WriteString(cert)
	if err != nil {
		glog.Fatalf("Couldn't write to pem file %v: %v", pemFileName, err)
	}

	return pemFileName
}

func getNameForUpstream(name string, host string, service string, port string) string {
	return fmt.Sprintf("%v-%v-%v-%v", name, host, service, port)
}

func shellOut(cmd string) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	glog.Infof("executing %s", cmd)

	command := exec.Command("sh", "-c", cmd)
	command.Stdout = &stdout
	command.Stderr = &stderr

	err := command.Start()
	if err != nil {
		glog.Fatalf("Failed to execute %v, err: %v", cmd, err)
	}

	err = command.Wait()
	if err != nil {
		glog.Errorf("Command %v stdout: %q", cmd, stdout.String())
		glog.Errorf("Command %v stderr: %q", cmd, stderr.String())
		glog.Fatalf("Command %v finished with error: %v", cmd, err)
	}
}

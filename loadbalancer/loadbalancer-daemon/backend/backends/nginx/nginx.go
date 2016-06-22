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

	"github.com/golang/glog"
)

var (
	configPath = "/etc/nginx"
)

// NGINXController Updates NGINX configuration, starts and reloads NGINX
type NGINXController struct {
	nginxConfdPath    string
	nginxTCPConfdPath string
	nginxCertsPath    string
}

type NGINXConfig struct {
	Upstream Upstream
	Server   Server
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
	}

	// This is to add a include directive for tcp apps
	appendStreamDirectiveToCfg(path.Join(configPath, "nginx.conf"))
	// Create tcp config dir
	createDir(ngxc.nginxTCPConfdPath)
	// Create cert dir
	createDir(ngxc.nginxCertsPath)
	start()

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
	if nginxConfig == (NGINXConfig{}) {
		glog.Errorf("Could not generate nginx config for %v", name)
		return
	}

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

	upsName := getNameForUpstream(name, config.Host, config.TargetServiceName)
	upstream := createUpstream(upsName, config)

	serverName := config.Host
	server := Server{
		Name:     serverName,
		BindIP:   config.BindIp,
		BindPort: strconv.Itoa(config.BindPort),
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

	nginxConfig = NGINXConfig{
		Upstream: upstream,
		Server:   server,
	}

	return nginxConfig
}

func createUpstream(name string, backend factory.BackendConfig) Upstream {
	ups := Upstream{
		Name: name,
		UpstreamServer: UpstreamServer{
			Address: backend.TargetIP,
			Port:    strconv.Itoa(backend.TargetPort),
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

func getNameForUpstream(name string, host string, service string) string {
	return fmt.Sprintf("%v-%v-%v", name, host, service)
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

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
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"reflect"
	"text/template"

	vault "github.com/hashicorp/vault/api"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/util"
)

const (
	version = "1.6.0"
	nginxConf = `
daemon off;

events {
	worker_connections 1024;
}
http {
	# http://nginx.org/en/docs/http/ngx_http_core_module.html
	types_hash_max_size 2048;
	server_names_hash_max_size 512;
	server_names_hash_bucket_size 64;
	# bite-460
	client_max_body_size 128m;

	log_format proxied_combined '"$http_x_forwarded_for" - $remote_user [$time_local] "$request" '
											'$status $body_bytes_sent "$http_referer" '
											'"$http_user_agent"';
	
	error_log /dev/stderr info;
	access_log /dev/stdout proxied_combined;

	server {

		listen 443 ssl default_server;
		ssl_certificate /etc/nginx/certs/localhost.crt;
		ssl_certificate_key /etc/nginx/certs/localhost.key;

		listen	 80 default_server;

		location / {
			root	 /usr/share/nginx/html;
			index	index.html index.htm;
		}
	}
{{range $i := .}}

	server {
		server_name {{$i.Host}};
{{if $i.Ssl}}
		listen 443 ssl;
		ssl_certificate		/etc/nginx/certs/{{$i.Host}}.crt;
		ssl_certificate_key	/etc/nginx/certs/{{$i.Host}}.key;
{{end}}
{{if $i.Nonssl}}		listen 80;{{end}}
{{ range $path := $i.Paths }}
		location {{$path.Location}} {
			proxy_set_header Host $host;
			proxy_pass {{$i.Scheme}}://{{$path.Service}}.{{$i.Namespace}}.svc.cluster.local:{{$path.Port}};
		}
{{end}}
	}{{end}}
}`
)

const nginxConfDir = "/etc/nginx"
const nginxCommand = "nginx"

// shellOut runs an external command.
// stdout and stderr are attached to this external process
func shellOut(shellCmd string, args []string) {
	cmd := exec.Command(shellCmd, args...)
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	fmt.Printf("Starting %v %v\n", shellCmd, args)

	go io.Copy(os.Stdout, stdout)
	go io.Copy(os.Stderr, stderr)
	err := cmd.Start()
	if err != nil {
		log.Fatalf("Failed to execute %v: err: %v", cmd, err)
	}
}

type Path struct {
	Location	string
	Service		string
	Port			int32
}

type Ingress struct {
	Host			string
	Namespace	string
	Paths 		[]*Path
	Ssl				bool
	Nonssl		bool
	Scheme		string
}

func main() {
	var ingClient client.IngressInterface
	if kubeClient, err := client.NewInCluster(); err != nil {
		log.Fatalf("Failed to create client: %v.", err)
	} else {
		ingClient = kubeClient.Extensions().Ingress(api.NamespaceAll)
	}
	/* vaultEnabled
	The following environment variables should be set:
	VAULT_ADDR
	VAULT_TOKEN
	VAULT_SKIP_VERIFY (if using self-signed SSL on vault)
	The only one we need to explicitly introduce is VAULT_ADDR, but we can check the others
	Se the following to disable Vault integration entirely:
	VAULT_ENABLED = "false"
	*/
	nginxTemplate := nginxConf
	vaultEnabledFlag := os.Getenv("VAULT_ENABLED")
	vaultAddress := os.Getenv("VAULT_ADDR")
	vaultToken := os.Getenv("VAULT_TOKEN")
	debug := os.Getenv("DEBUG")

	nginxArgs := []string{
		"-c",
		nginxConfDir + "/nginx.conf",
	}

	shellOut(nginxCommand, nginxArgs)

	vaultEnabled := "true"

	fmt.Printf("\n Ingress Controller version: %v\n", version)

	if vaultEnabledFlag == "" {
		vaultEnabled = "true"
	} else {
		vaultEnabled = vaultEnabledFlag
	}
 
	if vaultAddress == "" || vaultToken == "" {
		fmt.Printf("\nVault not configured\n")
		vaultEnabled = "false"
	}

	config := vault.DefaultConfig()
	config.Address = vaultAddress

	vault, err := vault.NewClient(config)
	if err != nil {
		fmt.Printf("WARN: Vault config failed.\n")
		vaultEnabled = "false"
	}

	token := vaultToken
		_ = token

	tmpl, _ := template.New("nginx").Parse(nginxTemplate)
	rateLimiter := util.NewTokenBucketRateLimiter(0.1, 1)
	known := &extensions.IngressList{}

	// Controller loop
	for {
		rateLimiter.Accept()
		ingresses, err := ingClient.List(api.ListOptions{})
		if err != nil {
			fmt.Printf("Error retrieving ingresses: %v\n", err)
			continue
		}
		if reflect.DeepEqual(ingresses.Items, known.Items) {
			continue
		}
		known = ingresses

		type IngressList []*Ingress

		var ingresslist IngressList = IngressList{}

		for _, ingress := range ingresses.Items {

			ingressHost := ingress.Spec.Rules[0].Host		

			// Setup ingress defaults

			i := new(Ingress)
			i.Host = ingressHost
			i.Namespace = ingress.Namespace
			i.Ssl = false
			i.Nonssl = true
			i.Scheme = "http"

			// Parse labels
			l := ingress.GetLabels()
			for k, v := range(l) {
				if k == "ssl" && v == "true" {
					i.Ssl = true
				}
				if k == "httpsOnly" && v == "true" {
					i.Nonssl = false
				}
				if k == "httpsBackend" && v == "true" {
					i.Scheme = "https"
				}
			}

			// Parse Paths
			for _, r := range(ingress.Spec.Rules) {
				for _, p := range(r.HTTP.Paths) {
					l := new(Path)
					l.Location = p.Path
					l.Service = p.Backend.ServiceName
					l.Port = p.Backend.ServicePort.IntVal
					i.Paths = append(i.Paths, l)
				}
			}

			if vaultEnabled == "true" && i.Ssl {

				vaultPath := "secret/ssl/" + ingressHost

				keySecretData, err := vault.Logical().Read(vaultPath)
				if err != nil || keySecretData == nil {
					fmt.Printf("No secret for %v\n", ingressHost)
					i.Ssl = false
					continue
				} else {
					fmt.Printf("Found secret for %v\n", ingressHost)
				}

				var keySecret string = fmt.Sprintf("%v", keySecretData.Data["key"])
				if err != nil || keySecret == "" {
					fmt.Printf("WARN: No secret keys found at %v\n", vaultPath)
					i.Ssl = false
					continue
				}
				fmt.Printf("Found key for %v\n", ingressHost)
				keyFileName := nginxConfDir + "/certs/" + ingressHost + ".key"
				if err := ioutil.WriteFile(keyFileName, []byte(keySecret), 0400); err != nil {
					log.Fatalf("failed to write file %v: %v\n", keyFileName, err)
					i.Ssl = false
					continue
				}
				var crtSecret string = fmt.Sprintf("%v", keySecretData.Data["crt"])
				if err != nil || crtSecret == "" {
					fmt.Printf("WARN: No crt found at %v\n", vaultPath)
					i.Ssl = false
					continue
				}

				fmt.Printf("Found crt for %v\n", ingressHost)
				crtFileName := nginxConfDir + "/certs/" + ingressHost + ".crt"
				if err := ioutil.WriteFile(crtFileName, []byte(crtSecret), 0400); err != nil {
					log.Fatalf("failed to write file %v: %v\n", crtFileName, err)
					i.Ssl = false
					continue
				}
			}
			ingresslist = append(ingresslist, i)

		}

		if w, err := os.Create(nginxConfDir + "/nginx.conf"); err != nil {
			log.Fatalf("failed to open %v: %v\n", nginxTemplate, err)
		} else if err := tmpl.Execute(w, ingresslist); err != nil {
			log.Fatalf("failed to write template %v\n", err)
		}

		if debug  == "true" {
			conf, _ := ioutil.ReadFile(nginxConfDir + "/nginx.conf")
			fmt.Printf(string(conf))
		}

		verifyArgs := []string{
			"-t",
			"-c",
			nginxConfDir + "/nginx.conf",
		}
		reloadArgs := []string{
			"-s",
			"reload",
		}

		err = exec.Command(nginxCommand, verifyArgs...).Run()
		if err != nil {
			fmt.Printf("ERR: nginx config failed validation: %v\n", err)
		} else {
			shellOut(nginxCommand, reloadArgs)
			fmt.Printf("nginx config updated.\n")
		}
	}
}

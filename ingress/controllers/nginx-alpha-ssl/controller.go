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

	log_format default '$host $remote_addr - $remote_user [$time_local] \"$request\" $status $body_bytes_sent \"$http_referer\" \"$http_user_agent\"';

{{range $ing := .Items}}
{{range $rule := $ing.Spec.Rules}}
	server {
		server_name {{$rule.Host}};
{{range $key, $val := $ing.Labels}}
{{if (and (eq $key "ssl") (eq $val "true"))}}
		listen 443 ssl;
		ssl_certificate	/etc/nginx/certs/{{$rule.Host}}.crt;
		ssl_certificate_key	/etc/nginx/certs/{{$rule.Host}}.key;
{{- end}}{{end}}
		error_log /dev/stdout info;
		access_log /dev/stdout default;

		listen 80;
{{ range $path := $rule.HTTP.Paths }}
		location {{$path.Path}} {
			proxy_set_header Host $host;
			proxy_pass http://{{$path.Backend.ServiceName}}.{{$ing.Namespace}}.svc.cluster.local:{{$path.Backend.ServicePort}};
		}{{end}}
	}{{end}}{{end}}
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
	*/
	vaultEnabled := true
	config := vault.DefaultConfig()
	config.Address = os.Getenv("VAULT_ADDR")
	vault, err := vault.NewClient(config)
	if err != nil {
		fmt.Printf("WARN: VAULT_ADDR is not set\n")
		vaultEnabled = false
	}

	token := os.Getenv("VAULT_TOKEN")
	if token == "" {
		fmt.Printf("WARN: VAULT_TOKEN is not set\n")
		vaultEnabled = false
	}

	tmpl, _ := template.New("nginx").Parse(nginxConf)
	rateLimiter := util.NewTokenBucketRateLimiter(0.1, 1)
	known := &extensions.IngressList{}

	// Controller loop
	nginxArgs := []string{
		"-c",
		nginxConfDir + "/nginx.conf",
	}

	shellOut(nginxCommand, nginxArgs)
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

		if vaultEnabled {
			for _, ingress := range ingresses.Items {
				ingressHost := ingress.Spec.Rules[0].Host
				vaultPath := "secret/ssl/" + ingressHost

				keySecretData, err := vault.Logical().Read(vaultPath)
				if err != nil {
					log.Fatal(err)
				}
				if keySecretData == nil {
					fmt.Printf("No secret for %v\n", ingressHost)
					continue
				}
				fmt.Printf("Found secret for %v\n", ingressHost)

				var keySecret string = fmt.Sprintf("%v", keySecretData.Data["key"])
				if err != nil {
					fmt.Printf("WARN: No secret keys found at %v\n", vaultPath)
					continue
				}
				if keySecret == "" {
					fmt.Printf("WARN: No value found at %v\n", vaultPath)
					continue
				} 
				fmt.Printf("Found key for %v\n", ingressHost)
				keyFileName := nginxConfDir + "/certs/" + ingressHost + ".key"
				if err := ioutil.WriteFile(keyFileName, []byte(keySecret), 0400); err != nil {
					log.Fatalf("failed to write file %v: %v\n", keyFileName, err)
					continue
				}
				var crtSecret string = fmt.Sprintf("%v", keySecretData.Data["crt"])
				if crtSecret == "" {
					fmt.Printf("WARN: Failed to find crt secret at %v\n", vaultPath)
					continue
				}
				fmt.Printf("Found crt for %v\n", ingressHost)
				crtFileName := nginxConfDir + "/certs/" + ingressHost + ".crt"
				if err := ioutil.WriteFile(crtFileName, []byte(crtSecret), 0400); err != nil {
					log.Fatalf("failed to write file %v: %v\n", crtFileName, err)
				}
			}
		}
		if w, err := os.Create(nginxConfDir + "/nginx.conf"); err != nil {
			log.Fatalf("failed to open %v: %v\n", nginxConf, err)
		} else if err := tmpl.Execute(w, ingresses); err != nil {
			log.Fatalf("failed to write template %v\n", err)
		}

		verifyArgs := []string{
			"-t",
			"-c",
			nginxConfDir + "/nginx.conf",
		}
		stopArgs := []string{
			"-s",
			"quit",
		}

		err = exec.Command(nginxCommand, verifyArgs...).Run()
		if err != nil {
			fmt.Printf("ERR: nginx config failed validation: %v\n", err)
		} else {
			err = exec.Command(nginxCommand, stopArgs...).Run()
			if err != nil {
				fmt.Printf("ERR: nginx restart failed: %v\n", err)
			}
			shellOut(nginxCommand, nginxArgs)
			fmt.Printf("nginx config updated.\n")
		}
	}
}

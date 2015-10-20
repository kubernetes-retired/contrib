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
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ziutek/syslog"
)

var (
	//
	pem = flag.String("pem", "/etc/haproxy/ssl/haproxyhttps.pem",
		`path to .pem file. This needs to match the directory specified in haproxy.cfg.`)
	key = flag.String("key", "/ssl/haproxyhttps.key",
		`Path to the .key file from the secret. This cannot be in the same dir as the pem.`)
	crt = flag.String("crt", "/ssl/haproxyhttps.crt",
		`Path to the .crt file from the secret. This cannot be in the same dir as the pem.`)
)

type handler struct {
	*syslog.BaseHandler
}

type syslogServer struct {
	*syslog.Server
}

// startSyslogServer start a syslog server using a unix socket to listen for connections
func startSyslogServer(path string) (*syslogServer, error) {
	log.Printf("Starting syslog server for haproxy using %v as socket", path)
	// remove the socket file if exists
	os.Remove(path)

	server := &syslogServer{syslog.NewServer()}
	server.AddHandler(newHandler())
	err := server.Listen(path)
	if err != nil {
		return nil, err
	}

	return server, nil
}

func newHandler() *handler {
	h := handler{syslog.NewBaseHandler(1000, nil, false)}
	go h.mainLoop()
	return &h
}

func (h *handler) mainLoop() {
	for {
		message := h.Get()
		if message == nil {
			break
		}

		fmt.Printf("servicelb [%s] %s%s\n", strings.ToUpper(message.Severity.String()), message.Tag, message.Content)
	}

	h.End()
}

func reloadHaproxy() error {
	output, err := exec.Command("sh", "-c", "/haproxy_reload").CombinedOutput()
	msg := fmt.Sprintf("haproxy -- %v", string(output))
	if err != nil {
		return fmt.Errorf("error restarting %v: %v", msg, err)
	}
	log.Printf(msg)
	return nil
}

func catFiles(in1, in2, out string) error {
	var content bytes.Buffer
	for _, f := range []string{in1, in2} {
		c, err := ioutil.ReadFile(f)
		if err != nil {
			return fmt.Errorf("Could not parse files from %v: %v", f, err)
		}
		content.Write(c)
	}
	err := ioutil.WriteFile(out, content.Bytes(), 0644)
	if err != nil {
		return fmt.Errorf("Could not write to %v: %v", out, err)
	}
	return nil
}

func createPEM(key, crt, pem string) error {
	if _, err := os.Stat(pem); err == nil {
		log.Printf("%v already exists", pem)
		return nil
	}
	pemDir := filepath.Dir(pem)
	if filepath.Dir(crt) == pemDir || filepath.Dir(key) == pemDir {
		return fmt.Errorf("Key and crt cannot share the same parent dir as pem.")
	}
	if err := os.MkdirAll(pemDir, 0644); err != nil {
		return err
	}
	return catFiles(crt, key, pem)
}

func main() {
	flag.Parse()
	// Secrets are mounted as .key and .crt, haproxy expects a .pem
	if err := createPEM(*key, *crt, *pem); err != nil {
		log.Fatalf("Could not create pem file %v", err)
	}
	_, err := startSyslogServer("/var/run/haproxy.log.socket")
	if err != nil {
		log.Fatalf("Failed to start syslog server: %v", err)
	}

	if err := reloadHaproxy(); err != nil {
		log.Fatalf("Failed to reload haproxy: %v", err)
	}
	select {}
}

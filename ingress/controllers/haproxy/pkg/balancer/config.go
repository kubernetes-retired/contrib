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

package balancer

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
)

// Config is load balancer configuration scheme not tied to any LB implementation
type Config struct {
	Options   map[string]string
	Exposed   map[string]Exposed
	Upstreams map[string]Upstream
}

// Exposed is a exposed server specification
type Exposed struct {
	BindPort     int
	Certificates []Certificate
	Options      map[string]string
	HostName     string
	PathBegins   string
	IsDefault    bool
	Upstream     *Upstream
}

// Name returns an unique name based on BindPorts, Hostname and Path
func (exp *Exposed) Name() string {

	n := fmt.Sprintf("port%d", exp.BindPort)

	if exp.IsDefault {
		n = fmt.Sprintf("%s.defaulthost", n)
	} else {
		n = fmt.Sprintf("%s.host-%s", n, exp.HostName)
	}

	if exp.PathBegins != "" && exp.PathBegins != "/" {
		n = fmt.Sprintf("%s.path-%s", n, exp.PathBegins)
	}

	return n
}

// Upstream servers manage request after they are received at the balancer
type Upstream struct {
	Name      string
	Options   map[string]string
	Endpoints []Endpoint
}

// Certificate x509 in PEM format
type Certificate struct {
	Name    string
	Domains []string
	Private []byte
	Public  []byte
}

// Endpoint for an upstream server
type Endpoint struct {
	Options map[string]string
	IP      string
	Port    int
}

// NewDefaultLBConfig returns an LBConfig with defaults
func NewDefaultLBConfig() Config {
	return Config{}
}

// NewCertificate creates a certificate
func NewCertificate(priv, pub []byte, name string) (*Certificate, error) {

	cert := append(pub, '\n')
	cert = append(cert, priv...)
	block, _ := pem.Decode(cert)
	if block == nil {
		return nil, fmt.Errorf("certificate %v is not pem formatted", name)
	}

	c509, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("certificate %v is not a valid x509 certificate: %v", name, err)
	}

	d := []string{c509.Subject.CommonName}
	if len(c509.DNSNames) > 0 {
		d = append(d, c509.DNSNames...)
	}

	c := &Certificate{
		Name:    name,
		Private: priv,
		Public:  pub,
		Domains: d,
	}

	return c, nil
}

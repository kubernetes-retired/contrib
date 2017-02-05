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

package ingress

import (
	"k8s.io/contrib/ingress/controllers/nginx/nginx/auth"
	"k8s.io/contrib/ingress/controllers/nginx/nginx/authreq"
	"k8s.io/contrib/ingress/controllers/nginx/nginx/ipwhitelist"
	"k8s.io/contrib/ingress/controllers/nginx/nginx/ratelimit"
	"k8s.io/contrib/ingress/controllers/nginx/nginx/rewrite"
	"k8s.io/contrib/ingress/controllers/nginx/nginx/timeout"
)

// Configuration describes an NGINX configuration
type Configuration struct {
	Upstreams    []*Upstream
	Servers      []*Server
	TCPUpstreams []*Location
	UDPUpstreams []*Location
}

// Upstream describes an NGINX upstream
type Upstream struct {
	Name     string
	Backends []UpstreamServer
	Secure   bool
}

// UpstreamByNameServers sorts upstreams by name
type UpstreamByNameServers []*Upstream

func (c UpstreamByNameServers) Len() int      { return len(c) }
func (c UpstreamByNameServers) Swap(i, j int) { c[i], c[j] = c[j], c[i] }
func (c UpstreamByNameServers) Less(i, j int) bool {
	return c[i].Name < c[j].Name
}

// UpstreamServer describes a server in an NGINX upstream
type UpstreamServer struct {
	Address     string
	Port        string
	MaxFails    int
	FailTimeout int
}

// UpstreamServerByAddrPort sorts upstream servers by address and port
type UpstreamServerByAddrPort []UpstreamServer

func (c UpstreamServerByAddrPort) Len() int      { return len(c) }
func (c UpstreamServerByAddrPort) Swap(i, j int) { c[i], c[j] = c[j], c[i] }
func (c UpstreamServerByAddrPort) Less(i, j int) bool {
	iName := c[i].Address
	jName := c[j].Address
	if iName != jName {
		return iName < jName
	}

	iU := c[i].Port
	jU := c[j].Port
	return iU < jU
}

// Server describes an NGINX server
type Server struct {
	Name              string
	Locations         []*Location
	SSL               bool
	SSLCertificate    string
	SSLCertificateKey string
	SSLPemChecksum    string
}

// ServerByName sorts server by name
type ServerByName []*Server

func (c ServerByName) Len() int      { return len(c) }
func (c ServerByName) Swap(i, j int) { c[i], c[j] = c[j], c[i] }
func (c ServerByName) Less(i, j int) bool {
	return c[i].Name < c[j].Name
}

// Location describes an NGINX location
type Location struct {
	Path            string
	IsDefBackend    bool
	Upstream        Upstream
	Auth            auth.Nginx
	RateLimit       ratelimit.RateLimit
	Redirect        rewrite.Redirect
	SecureUpstream  bool
	Whitelist       ipwhitelist.SourceRange
	EnableCORS      bool
	ExternalAuthURL authreq.Auth
	Timeout         timeout.Timeout
}

// LocationByPath sorts location by path
// Location / is the last one
type LocationByPath []*Location

func (c LocationByPath) Len() int      { return len(c) }
func (c LocationByPath) Swap(i, j int) { c[i], c[j] = c[j], c[i] }
func (c LocationByPath) Less(i, j int) bool {
	return c[i].Path > c[j].Path
}

// SSLCert describes a SSL certificate to be used in NGINX
type SSLCert struct {
	CertFileName string
	KeyFileName  string
	// PemFileName contains the path to the file with the certificate and key concatenated
	PemFileName string
	// PemSHA contains the sha1 of the pem file.
	// This is used to detect changes in the secret that contains the certificates
	PemSHA string
	// CN contains all the common names defined in the SSL certificate
	CN []string
}

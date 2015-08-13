/*
Copyright 2014 The Kubernetes Authors All rights reserved.

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

package client

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"

	"k8s.io/kubernetes/pkg/probe"
	httprobe "k8s.io/kubernetes/pkg/probe/http"
)

// KubeletClient is an interface for all kubelet functionality
type KubeletClient interface {
	KubeletHealthChecker
	ConnectionInfoGetter
}

// KubeletHealthchecker is an interface for healthchecking kubelets
type KubeletHealthChecker interface {
	HealthCheck(host string) (result probe.Result, output string, err error)
}

type ConnectionInfoGetter interface {
	GetConnectionInfo(host string) (scheme string, port uint, transport http.RoundTripper, err error)
}

// HTTPKubeletClient is the default implementation of KubeletHealthchecker, accesses the kubelet over HTTP.
type HTTPKubeletClient struct {
	Client *http.Client
	Config *KubeletConfig
}

func MakeTransport(config *KubeletConfig) (http.RoundTripper, error) {
	cfg := &Config{TLSClientConfig: config.TLSClientConfig}
	if config.EnableHttps {
		hasCA := len(config.CAFile) > 0 || len(config.CAData) > 0
		if !hasCA {
			cfg.Insecure = true
		}
	}
	tlsConfig, err := TLSConfigFor(cfg)
	if err != nil {
		return nil, err
	}
	if config.Dial != nil || tlsConfig != nil {
		return &http.Transport{
			Dial:            config.Dial,
			TLSClientConfig: tlsConfig,
		}, nil
	} else {
		return http.DefaultTransport, nil
	}
}

// TODO: this structure is questionable, it should be using client.Config and overriding defaults.
func NewKubeletClient(config *KubeletConfig) (KubeletClient, error) {
	transport, err := MakeTransport(config)
	if err != nil {
		return nil, err
	}
	c := &http.Client{
		Transport: transport,
		Timeout:   config.HTTPTimeout,
	}
	return &HTTPKubeletClient{
		Client: c,
		Config: config,
	}, nil
}

func (c *HTTPKubeletClient) GetConnectionInfo(host string) (string, uint, http.RoundTripper, error) {
	scheme := "http"
	if c.Config.EnableHttps {
		scheme = "https"
	}
	return scheme, c.Config.Port, c.Client.Transport, nil
}

func (c *HTTPKubeletClient) url(host, path, query string) *url.URL {
	scheme := "http"
	if c.Config.EnableHttps {
		scheme = "https"
	}

	return &url.URL{
		Scheme:   scheme,
		Host:     net.JoinHostPort(host, strconv.FormatUint(uint64(c.Config.Port), 10)),
		Path:     path,
		RawQuery: query,
	}
}

func (c *HTTPKubeletClient) HealthCheck(host string) (probe.Result, string, error) {
	return httprobe.DoHTTPProbe(c.url(host, "/healthz", ""), c.Client)
}

// FakeKubeletClient is a fake implementation of KubeletClient which returns an error
// when called.  It is useful to pass to the master in a test configuration with
// no kubelets.
type FakeKubeletClient struct{}

func (c FakeKubeletClient) HealthCheck(host string) (probe.Result, string, error) {
	return probe.Unknown, "", errors.New("Not Implemented")
}

func (c FakeKubeletClient) GetConnectionInfo(host string) (string, uint, http.RoundTripper, error) {
	return "", 0, nil, errors.New("Not Implemented")
}

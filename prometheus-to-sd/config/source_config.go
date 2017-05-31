/*
Copyright 2017 The Kubernetes Authors.

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

package config

import (
	"fmt"
	"k8s.io/contrib/prometheus-to-sd/flags"
	"net"
	"strconv"
	"strings"
)

// SourceConfig contains data specific for scraping one component.
type SourceConfig struct {
	Component   string
	Host        string
	Port        uint
	Whitelisted []string
}

// NewSourceConfig creates a new SourceConfig based on string representation of fields.
func NewSourceConfig(component string, host string, port string, whitelisted string) (*SourceConfig, error) {
	if port == "" {
		return nil, fmt.Errorf("No port provided.")
	}

	portNum, err := strconv.ParseUint(port, 10, 32)
	if err != nil {
		return nil, err
	}

	var whitelistedList []string
	if whitelisted != "" {
		whitelistedList = strings.Split(whitelisted, ",")
	}

	return &SourceConfig{
		Component:   component,
		Host:        host,
		Port:        uint(portNum),
		Whitelisted: whitelistedList,
	}, nil
}

// ParseSourceConfig creates a new SourceConfig based on the provided flags.Uri instance.
func ParseSourceConfig(uri flags.Uri) (*SourceConfig, error) {
	host, port, err := net.SplitHostPort(uri.Val.Host)
	if err != nil {
		return nil, err
	}

	component := uri.Key
	values := uri.Val.Query()
	whitelisted := values.Get("whitelisted")

	return NewSourceConfig(component, host, port, whitelisted)
}

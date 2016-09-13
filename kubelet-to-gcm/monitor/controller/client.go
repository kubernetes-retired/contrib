/*
Copyright 2016 The Kubernetes Authors.

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

package controller

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	log "github.com/golang/glog"
)

// Metrics are parsed values from the kube-controller.
type Metrics struct {
	NodeEvictions int64
}

// parseEvictions parses the node_collector_evictions_number metric, and
// sets its value in the struct.
func (c *Metrics) parseEvictions(line string) error {
	// The value is the final token.
	fields := strings.Fields(line)
	value := fields[len(fields)-1]
	// Try to parse the value as a uint.
	count, err := strconv.ParseInt(value, 0, 64)
	if err != nil {
		return fmt.Errorf("Failed to parse node_collector_evictions_number value: %v", err)
	}
	c.NodeEvictions = count
	return nil
}

// NewMetrics creates a Metrics object from a Prometheus response body.
func NewMetrics(body []byte) (*Metrics, error) {
	metrics := &Metrics{}
	// If there's a better way to parse Prometheus metrics, I'd love to know.
	for _, line := range strings.Split(string(body), "\n") {
		if match, err := regexp.MatchString("^node_collector_evictions_number{", line); err != nil {
			return nil, fmt.Errorf("Failed to match node evictions: %v", err)
		} else if match {
			err = metrics.parseEvictions(line)
			if err != nil {
				return nil, err
			}
			break
		}
	}
	return metrics, nil
}

// Client queries metrics from the controller process.
type Client struct {
	client     *http.Client
	metricsURL *url.URL
}

// NewClient generates a client to hit the given controller.
func NewClient(host string, port uint, client *http.Client) (*Client, error) {
	// Parse our URL upfront, so we can fail fast.
	urlStr := fmt.Sprintf("http://%s:%d/metrics", host, port)
	metricsURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}
	return &Client{
		client:     client,
		metricsURL: metricsURL,
	}, nil
}

// doRequest makes the request to the controller, and returns the body.
func (c *Client) doRequestAndParse(req *http.Request) (*Metrics, error) {
	log.Infof("Preparing to perform request: %v", req)
	response, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body - %v", err)
	}
	if response.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%q not found", req.URL.String())
	} else if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed - %q, response: %q", response.Status, string(body))
	}

	metrics, err := NewMetrics(body)
	return metrics, err
}

// GetMetrics returns the latest Metrics parsed from the kube-controller endpoint.
func (c *Client) GetMetrics() (*Metrics, error) {
	req, err := http.NewRequest("GET", c.metricsURL.String(), nil)
	if err != nil {
		return nil, err
	}
	metrics, err := c.doRequestAndParse(req)
	return metrics, err
}

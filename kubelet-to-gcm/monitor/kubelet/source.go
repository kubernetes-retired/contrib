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

package kubelet

import (
	"fmt"

	"k8s.io/contrib/kubelet-to-gcm/monitor"

	v3 "google.golang.org/api/monitoring/v3"
)

// Source pulls data from the controller source, and translates it for GCMv3.
type Source struct {
	translator *Translator
	client     *Client
}

// NewSource creates a new Source for a kubelet.
func NewSource(cfg *monitor.SourceConfig) (*Source, error) {
	// Create objects for kubelet monitoring.
	trans := NewTranslator(cfg.Zone, cfg.Project, cfg.Cluster, cfg.Host, cfg.Resolution)

	// NewClient validates its own inputs.
	client, err := NewClient(cfg.Host, cfg.Port, nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to create a kubelet client with config %v: %v", cfg, err)
	}

	return &Source{
		translator: trans,
		client:     client,
	}, nil
}

// GetTSReq returns the GCM v3 TimeSeries data.
func (s *Source) GetTSReq() (*v3.CreateTimeSeriesRequest, error) {
	// Get the latest summary.
	summary, err := s.client.GetSummary()
	if err != nil {
		return nil, fmt.Errorf("Failed to get summary from kubelet: %v", err)
	}

	// Translate kubelet's data to GCM v3's format.
	tsReq, err := s.translator.Translate(summary)
	if err != nil {
		return nil, fmt.Errorf("Failed to translate data from summary %v: %v", summary, err)
	}

	return tsReq, nil
}

// Name returns the name of the component being monitored.
func (s *Source) Name() string {
	return "kubelet"
}

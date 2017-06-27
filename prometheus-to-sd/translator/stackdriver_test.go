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

package translator

import (
	"github.com/stretchr/testify/assert"
	"k8s.io/contrib/prometheus-to-sd/config"
	"testing"
)

func TestGetMetricType(t *testing.T) {
	testConfig := &config.CommonConfig{
		GceConfig:     &config.GceConfig{MetricsPrefix: "container.googleapis.com/master"},
		ComponentName: "component",
	}
	expected := "container.googleapis.com/master/component/name"
	assert.Equal(t, expected, getMetricType(testConfig, "name"))
}

func TestParseMetricType(t *testing.T) {
	testConfig := &config.GceConfig{MetricsPrefix: "container.googleapis.com/master"}
	correct := "container.googleapis.com/master/component/name"
	component, metricName, err := parseMetricType(testConfig, correct)

	if assert.NoError(t, err) {
		assert.Equal(t, "component", component)
		assert.Equal(t, "name", metricName)
	}

	incorrect1 := "container.googleapis.com/master/component"
	_, _, err = parseMetricType(testConfig, incorrect1)
	assert.Error(t, err)

	incorrect2 := "incorrect.prefix.com/master/component"
	_, _, err = parseMetricType(testConfig, incorrect2)
	assert.Error(t, err)
}

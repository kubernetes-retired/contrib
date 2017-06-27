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
	"github.com/stretchr/testify/assert"
	"k8s.io/contrib/prometheus-to-sd/flags"
	"net/url"
	"testing"
)

func TestNewSourceConfig(t *testing.T) {
	correct := [...]struct {
		component   string
		host        string
		port        string
		whitelisted string
		output      SourceConfig
	}{
		{"testComponent", "localhost", "1234", "a,b,c,d",
			SourceConfig{
				Component:   "testComponent",
				Host:        "localhost",
				Port:        1234,
				Whitelisted: []string{"a", "b", "c", "d"},
			},
		},

		{"testComponent", "localhost", "1234", "",
			SourceConfig{
				Component:   "testComponent",
				Host:        "localhost",
				Port:        1234,
				Whitelisted: nil,
			},
		},
	}

	for _, c := range correct {
		res, err := NewSourceConfig(c.component, c.host, c.port, c.whitelisted)
		if assert.NoError(t, err) {
			assert.Equal(t, c.output, *res)
		}
	}
}

func TestParseSourceConfig(t *testing.T) {
	correct := struct {
		in     flags.Uri
		output SourceConfig
	}{
		flags.Uri{
			Key: "testComponent",
			Val: url.URL{
				Scheme:   "http",
				Host:     "hostname:1234",
				RawQuery: "whitelisted=a,b,c,d",
			},
		},
		SourceConfig{
			Component:   "testComponent",
			Host:        "hostname",
			Port:        1234,
			Whitelisted: []string{"a", "b", "c", "d"},
		},
	}

	res, err := ParseSourceConfig(correct.in)
	if assert.NoError(t, err) {
		assert.Equal(t, correct.output, *res)
	}

	incorrect := [...]flags.Uri{
		{
			Key: "incorrectHost",
			Val: url.URL{
				Scheme:   "http",
				Host:     "hostname[:1234",
				RawQuery: "whitelisted=a,b,c,d",
			},
		},
		{
			Key: "noPort",
			Val: url.URL{
				Scheme:   "http",
				Host:     "hostname",
				RawQuery: "whitelisted=a,b,c,d",
			},
		},
	}

	for _, c := range incorrect {
		_, err = ParseSourceConfig(c)
		assert.Error(t, err)
	}
}

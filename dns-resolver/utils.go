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
	"os"
	"strings"
	"text/template"

	"github.com/golang/glog"
)

const (
	hexDigit = "0123456789abcdef"
)

func mergeTemplate(tmpl, output string, data map[string]interface{}) error {
	w, err := os.Create(output)
	if err != nil {
		return err
	}
	defer w.Close()

	t, err := template.ParseFiles(tmpl)
	if err != nil {
		return err
	}

	return t.Execute(w, data)
}

func parseServers(input string) []string {
	return strings.Split(input, ",")
}

func parseForwards(input string) []forward {
	forwards := []forward{}
	domains := strings.Split(input, ",")
	for _, domain := range domains {
		domainPort := strings.Split(domain, ":")
		if len(domainPort) == 2 {
			forwards = append(forwards, forward{
				Name: domainPort[0],
				IP:   domainPort[1],
			})
		} else {
			glog.V(2).Infof("invalid forward format (%v)", domainPort)
		}
	}
	return forwards
}

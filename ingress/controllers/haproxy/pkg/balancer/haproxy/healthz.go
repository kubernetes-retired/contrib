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

package haproxy

import (
	"fmt"
	"net"
	"net/http"

	log "github.com/golang/glog"
)

// Name for the healtz item
func (m *Manager) Name() string {
	return "HAProxy"
}

// Check that AHProxy is listening on frontends
func (m *Manager) Check(req *http.Request) error {

	for _, fe := range m.config.Frontends {

		if fe.Bind.Port == 0 {
			continue
		}

		if fe.DefaultBackend.Backend == "" && len(fe.UseBackends) == 0 {
			continue
		}

		// address := b.IP
		// if address == "*" {
		// 	address = ""
		// }
		//address = fmt.Sprintf("%s:%d", address, b.Port)
		address := fmt.Sprintf(":%d", fe.Bind.Port)
		c, err := net.Dial("tcp", address)
		if err != nil {
			log.Errorf("healthz check dialing '%s' failed: %+v", address, err)
			return err
		}
		defer c.Close()
	}

	return nil
}

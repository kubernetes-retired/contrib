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
	"os/exec"
)

// StartBalancer starts the balancer process
func (m *Manager) StartBalancer() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.executeCommand("start")

	return nil
}

// ReloadBalancer starts the balancer process
func (m *Manager) ReloadBalancer() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.executeCommand("reload")

	return nil
}

// StopBalancer stops balancer process
func (m *Manager) StopBalancer() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.executeCommand("stop")

	return nil
}

func (m *Manager) executeCommand(command string) error {
	if out, e := exec.Command(m.balancerScript, command).Output(); e != nil {
		return fmt.Errorf("error executing haproxy %s: %+v. %s", command, e, string(out))
	}
	return nil
}

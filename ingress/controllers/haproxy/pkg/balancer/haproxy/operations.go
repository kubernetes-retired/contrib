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

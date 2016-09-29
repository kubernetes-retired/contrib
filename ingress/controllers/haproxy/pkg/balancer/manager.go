package balancer

import "net/http"

// Manager holds balancer configuration public functions
type Manager interface {
	WriteConfigAndRestart(config *Config, force bool) error
	StartBalancer() error
	StopBalancer() error
	ReloadBalancer() error

	// healtz
	Name() string
	Check(req *http.Request) error
}

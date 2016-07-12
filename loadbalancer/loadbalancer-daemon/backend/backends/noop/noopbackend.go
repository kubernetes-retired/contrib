package backends

import (
	"github.com/golang/glog"
	factory "k8s.io/contrib/loadbalancer/loadbalancer-daemon/backend"
)

// NoOpController Controller for noop backend
type NoOpController struct {
	variable string
}

func init() {
	factory.Register("noop", NewNoOpController)
}

// NewNoOpController creates a Noop controller
func NewNoOpController() (factory.BackendController, error) {

	cont := NoOpController{
		variable: "NOOP",
	}

	return &cont, nil
}

// Name returns the name of the backend controller
func (noop *NoOpController) Name() string {
	return "NOOP"
}

// AddConfig Add event
func (noop *NoOpController) AddConfig(name string, config factory.BackendConfig) {
	glog.Infof("Received config %s: %v", name, config)
}

// DeleteConfig delete event
func (noop *NoOpController) DeleteConfig(name string) {
	glog.Infof("Received delete config name %s", name)
}

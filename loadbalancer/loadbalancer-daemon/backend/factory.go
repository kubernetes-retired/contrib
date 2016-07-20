package backends

import (
	"errors"
	"fmt"
	"strings"

	"github.com/golang/glog"
)

// BackendConfig Config that have all data for backend
type BackendConfig struct {
	Host              string
	Namespace         string
	BindIp            string
	TargetServiceName string
	TargetIP          string
	Ports             []string
	SSL               bool
	SSLPort           int
	Path              string
	TlsCert           string
	TlsKey            string
}

type BackendController interface {
	Name() string
	AddConfig(name string, config BackendConfig)
	DeleteConfig(name string)
	ExitChannel() chan struct{}
}

// BackendControllerFactory Factory for backend controllers
type BackendControllerFactory func() (BackendController, error)

var backendControllerFactories = make(map[string]BackendControllerFactory)

func Register(name string, factory BackendControllerFactory) {
	if factory == nil {
		glog.Errorf("Backend controller factory %s does not exist.", name)
	}
	_, registered := backendControllerFactories[name]
	if registered {
		glog.Errorf("Backend controller factory %s already registered. Ignoring.", name)
	}
	backendControllerFactories[name] = factory
}

func CreateBackendController(backendName string) (BackendController, error) {
	// Query configuration for backend controller.
	engineName := backendName

	engineFactory, ok := backendControllerFactories[engineName]
	if !ok {
		// Factory has not been registered.
		// Make a list of all available backend controller factories for logging.
		availableBackendControllers := make([]string, len(backendControllerFactories))
		for k, _ := range backendControllerFactories {
			availableBackendControllers = append(availableBackendControllers, k)
		}
		return nil, errors.New(fmt.Sprintf("Invalid backend controller name. Must be one of: %s", strings.Join(availableBackendControllers, ", ")))
	}

	// Run the factory with the configuration.
	return engineFactory()
}

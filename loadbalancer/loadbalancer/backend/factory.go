package backend

import (
	"fmt"
	"strings"
	"sync"

	"github.com/golang/glog"
)

// BackendConfig Config that have all data for backend
type BackendConfig struct {
	Host              string
	Namespace         string
	Nodes             []string
	BindIP            string
	BindPort          int
	TargetServiceName string
	TargetPort        int
	NodePort          int
	SSL               bool
	SSLPort           int
	Path              string
	TLSCert           string
	TLSKey            string
	Protocol          string
}

// BackendController Interface for all Backends
type BackendController interface {
	Name() string
	Create(name string, config BackendConfig)
	Delete(name string)
	AddNodeHandler(ip string, configMapNodePortMap map[string]int)
	DeleteNodeHandler(ip string, configMapNodePortMap map[string]int)
	UpdateNodeHandler(oldIP string, newIP string, configMapNodePortMap map[string]int)
}

// BackendControllerFactory Factory for Backend controllers
type BackendControllerFactory func(conf map[string]string) (BackendController, error)

var backendsMutex sync.Mutex
var backendControllerFactories = make(map[string]BackendControllerFactory)

// Register registers a backend factory by name
func Register(name string, factory BackendControllerFactory) {
	backendsMutex.Lock()
	defer backendsMutex.Unlock()
	if factory == nil {
		glog.Errorf("Backend controller factory %s does not exist.", name)
	}
	_, registered := backendControllerFactories[name]
	if registered {
		glog.Errorf("Backend controller factory %s already registered. Ignoring.", name)
	}
	backendControllerFactories[name] = factory
	glog.Infof("Registed backend %v.", name)
}

// CreateBackendController creates a backend controller factory for a specific backend
func CreateBackendController(conf map[string]string) (BackendController, error) {
	backendsMutex.Lock()
	defer backendsMutex.Unlock()

	// Query configuration for backend controller.
	engineName := conf["BACKEND"]

	engineFactory, ok := backendControllerFactories[engineName]
	if !ok {
		// Factory has not been registered.
		// Make a list of all available backend controller factories for logging.
		availableBackendControllers := make([]string, len(backendControllerFactories))
		for k := range backendControllerFactories {
			availableBackendControllers = append(availableBackendControllers, k)
		}
		return nil, fmt.Errorf("Invalid backend controller name. Must be one of: %s", strings.Join(availableBackendControllers, ", "))
	}

	// Run the factory with the configuration.
	return engineFactory(conf)
}

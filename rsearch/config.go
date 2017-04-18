package rsearch

import (
	"gopkg.in/gcfg.v1"
)

// Done is an alias for empty struct, used for termination channels
type Done struct{}

// Config is a top level struct describing expected structure of config file.
type Config struct {
	Resource Resource
	Server   Server
	Api      Api
}

// Server is a config section describing server instance of this package.
type Server struct {
	Port  string
	Host  string
	Debug bool
}

// API is a config section describing kubernetes API parameters.
type Api struct {
	Url          string
	NamespaceUrl string
}

// Resource is a config section describing kubernetes resource to be cached and searched for.
type Resource struct {
	Name       string
	Type       string
	Selector   string
	Namespaced string
	UrlPrefix  string
	UrlPostfix string
}

// NewConfig parsing config file and returning initialized instance of Config structure
func NewConfig(configFile string) (Config, error) {
	cfg := Config{}
	if err := gcfg.ReadFileInto(&cfg, configFile); err != nil {
		return cfg, err
	}

	return cfg, nil
}

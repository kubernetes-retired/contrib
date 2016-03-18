package rsearch

import (
	"gopkg.in/gcfg.v1"
)

type Done struct{}

type Config struct {
	Resource Resource
	Server   Server
	Api      Api
}

type Server struct {
	Port  string
	Debug bool
}

type Api struct {
	Url          string
	NamespaceUrl string
}

type Resource struct {
	Name       string
	Type       string
	Selector   string
	Namespaced string
	UrlPrefix  string
	UrlPostfix string
}

func NewConfig(configFile string) (Config, error) {
	cfg := Config{}
	if err := gcfg.ReadFileInto(&cfg, configFile); err != nil {
		return cfg, err
	}

	return cfg, nil
}

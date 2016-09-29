package server

import (
	"fmt"
	"net/http"

	"k8s.io/kubernetes/pkg/healthz"

	"k8s.io/contrib/ingress/controllers/haproxy/pkg/balancer"
	log "github.com/golang/glog"
)

// StartServer starts the server that exposes the internal balancer API
func StartServer(listen string, port int, m *balancer.Manager) {

	mux := http.NewServeMux()
	healthz.InstallHandler(mux, *m)
	mux.HandleFunc("/version", versionHandler)

	server := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", listen, port),
		Handler: mux,
	}

	log.Infof("starting Haddock API server at %s:%d", listen, port)
	log.Fatal(server.ListenAndServe())
}

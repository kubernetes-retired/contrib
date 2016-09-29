package server

import (
	"net/http"

	"k8s.io/contrib/ingress/controllers/haproxy/cmd/model"
	log "github.com/golang/glog"
)

// ErrOutHandler exits the app with output code 2
func versionHandler(w http.ResponseWriter, r *http.Request) {
	log.Infof("version handler has been requested from: %s", r.RemoteAddr)

	ai := model.GetAppInfo()
	if e := ObjectResponse(200, ai, w); e != nil {
		ErrorResponse("Error sending version", e, w)
	}

}

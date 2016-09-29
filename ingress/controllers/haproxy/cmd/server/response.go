package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	log "github.com/golang/glog"
)

// StandardResponse is a standard response message
type StandardResponse struct {
	Message string `json:"message"`
	ID      string `json:"_id,,omitempty"`
}

// ObjectResponse answers a request with a JSON message containing an object
func ObjectResponse(status int, object interface{}, w http.ResponseWriter) error {
	j, e := json.Marshal(object)

	if e != nil {
		return e
	}

	log.Infof("building response. status: %d, object: %s", status, object)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(j)

	return nil
}

// ErrorResponse answers a request with a JSON error
func ErrorResponse(context string, err error, w http.ResponseWriter) {
	message := &StandardResponse{
		Message: fmt.Sprintf("%s: %v", context, err),
	}

	log.Errorf("building error response. status: %d, message: %s", http.StatusInternalServerError, message)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	if e := json.NewEncoder(w).Encode(message); e != nil {
		panic(e)
	}
}

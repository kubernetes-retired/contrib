package rsearch

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// TODO will become obsolete as soon as handler will be redefined on searchRequest
type IO struct {
	req  chan<- SearchRequest
	resp <-chan SearchResponse
}


// TODO func responseWaiter is a goroutine that receives  http.responseWriter and KubeRequests from handlers
// it will then pass requests to processor and await response
// if responce arrives in time then it will be delivered to HTTP client
// otherwise timeout will be served

// TODO extracts KubeRequest from a HTTP body and spawns responseWaiter goroutine per request
// TODO handler can be defined on SearchRequest
func (io IO) handler(w http.ResponseWriter, req *http.Request) {
	decoder := json.NewDecoder(req.Body)
	request := SearchRequest{}
	err := decoder.Decode(&request)
	if err != nil {
		panic(err)
	}

	log.Printf("Server passing request %s  to Processor", request)
	io.req <- request

	// TODO there is no guarantee that response we're getting is related
	// to our request, not some other request from another client.
	response := <-io.resp
	fmt.Println("Sending response ", response)
	r, err := json.Marshal(response)
	if err != nil {
		panic(err)
	}
	fmt.Println("Sending response ", string(r))
	fmt.Fprint(w, string(r))
}

// TODO search response channel not needed
// handler can be a method of search request, IO not needed
func Serve(config Config, req chan<- SearchRequest, resp <-chan SearchResponse) {
	io := IO{req, resp}
	http.HandleFunc("/", io.handler)
	log.Fatal(http.ListenAndServe(":"+config.Server.Port, nil))
}

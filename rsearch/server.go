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

// TODO add response channel into request
// TODO maybe move request/responce defenitions to the server.go ?
type SearchRequest struct {
	Tag  string              `json:"tag"`
	Resp chan SearchResponse `json:",omitempty"`
}

type SearchResponse []KubeObject

// TODO func responseWaiter is a goroutine that receives  http.responseWriter and KubeRequests from handlers
// it will then pass requests to processor and await response
// if responce arrives in time then it will be delivered to HTTP client
// otherwise timeout will be served
func (io IO) responseWaiter(w http.ResponseWriter, request SearchRequest) {
	request.Resp = make(chan SearchResponse)
	io.req <- request
	fmt.Println("ResponseWaiter is waiting for answer")
	response := <-request.Resp
	defer close(request.Resp)

	fmt.Println("Sending response ", response)
	r, err := json.Marshal(response)
	if err != nil {
		panic(err)
	}
	fmt.Println("Sending response ", string(r))
	fmt.Fprint(w, string(r))
	fmt.Fprint(w, "DEBUG")
}

func (io IO) handler(w http.ResponseWriter, req *http.Request) {
	decoder := json.NewDecoder(req.Body)
	request := SearchRequest{}
	err := decoder.Decode(&request)
	if err != nil {
		panic(err)
	}

	log.Printf("Server passing request %s  to Processor", request)
	io.responseWaiter(w, request)
}

// TODO search response channel not needed
// handler can be a method of search request, IO not needed
func Serve(config Config, req chan<- SearchRequest) {
	io := IO{req: req}
	http.HandleFunc("/", io.handler)
	log.Fatal(http.ListenAndServe(":"+config.Server.Port, nil))
}

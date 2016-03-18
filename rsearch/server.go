package search

import (
	"net/http"
	"encoding/json"
	"fmt"
	"log"
)

type IO struct {
	req chan<- SearchRequest
	resp <-chan SearchResponse
	
}

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
	response := <- io.resp
	fmt.Println("Sending response ", response)
	r, err :=  json.Marshal(response)
	if err != nil {
		panic(err)
	}
	fmt.Println("Sending response ", string(r))
	fmt.Fprint(w, string(r))
}

func Serve(config Config, req chan<- SearchRequest, resp <-chan SearchResponse)  {
	io := IO{req, resp}
	http.HandleFunc("/", io.handler)
	log.Fatal(http.ListenAndServe(":" + config.Server.Port, nil))
}

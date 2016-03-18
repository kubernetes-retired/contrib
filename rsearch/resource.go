package search

import (
//	"fmt"
	"log"
//	"net/http"
//	"encoding/json"
//	"time"
)

/*
{"type":"ADDED","object":{"apiVersion":"romana.io/demo/v1","kind":"NetworkPolicy","metadata":{"name":"pol1","namespace":"default","selfLink":"/apis/romana.io/demo/v1/namespaces/default/networkpolicys/pol1","uid":"d7036130-e119-11e5-aab8-0213e1312dc5","resourceVersion":"119875","creationTimestamp":"2016-03-03T08:28:00Z","labels":{"owner":"t1"}},"spec":{"allowIncoming":{"from":[{"pods":{"tier":"frontend"}}],"toPorts":[{"port":80,"protocol":"TCP"}]},"podSelector":{"tier":"backend"}}}}
*/

type Event struct {
	Type string	`json:"Type"`
	Object KubeObject	`json:"object"`
}

type KubeObject struct {
	Kind	string	`json:"kind"`
	Spec	Spec	`json:"spec"`
	ApiVersion	string	`json:"apiVersion"`
	Metadata	Metadata	`json:"metadata"`
}

func (o KubeObject) makeId () string {
	id := o.Metadata.Name + "/" + o.Metadata.Namespace
	return id
}

func (o KubeObject) getSelector (config Config) string {
	var selector string
	// TODO this should use Config.Resource.Selector path instead of podSelector
	for k, v := range o.Spec.PodSelector {
		selector = k + "/" + v + "#"
	}
	return selector
}

// TODO need to find a way to use different specs for different resources
type Spec struct {
	AllowIncoming	map[string]interface{}	`json:"allowIncoming"`
	ToPorts		map[string]interface{}	`json:"toPorts"`
	PodSelector	map[string]string	`json:"podSelector"`
}
	

type Metadata struct {
	Name	string	`json:"name"`
	Namespace	string	`json:"namespace"`
	SelfLink	string	`json:"selfLink"`
	Uid	string	`json:"uid"`
	ResourceVersion string `json:"resourceVersion"`
	CreationTimestamp string `json:"creationTimestamp"`
	Labels	map[string]string	`json:"labels"`
}

type SearchRequest struct {
	Tag	string	`json:"tag"`
}

type SearchResponse []KubeObject

func Process(in <-chan Event, done chan Done, config Config) (chan<- SearchRequest, <-chan SearchResponse) {
	req := make(chan SearchRequest)
	resp := make(chan SearchResponse, 100)
	// search is allowed by Config.Resource.Selector tag
	// which is hardcoded to Spec.PodSelector which is dict

	// NPid := event.object.metadata.name + event.object.metadata.namespace
	// Selector := event.object.spec.PodSelector.k + event.object.spec.PodSelector.v
	// storage struct is map[NPid]KubeObject
	// search struct is map[Selector]map[NPid]bool
	storage := make(map[string]KubeObject)
	search  := make(map[string]map[string]bool)

	// maintains storage map
	// on event.type == ADDED:
	// 	storage[NPid]KubeObject
	// on event.type == DELETE
	//	delete(storage, NPid)

	// maintain search map
	// on event.type == ADDED:
	//	search[Selector][NPid] = true
	// on event.type == DELETED:
	//	delete(search[Selector],NPid)
	// on SearchRequest:
	//	resp := []KubeObject
	//	for NPid, _ := range search[SearchRequest]:
	//		append(resp, storage[NPid])
	//	return resp

	
	go func() {
		for {
			select {
				case e := <- in:
					updateStorage(e, storage, search, config)
				case request := <- req:
					resp <- processSearchRequest(storage, search, request)
				case <-done:
					return
			}
		}
	}()

	return req, resp
}

func processSearchRequest(storage map[string]KubeObject, search map[string]map[string]bool, req SearchRequest) SearchResponse {
	log.Println("Received request", req)
	var resp []KubeObject
	log.Printf("Index map has following %s, request tag is %s ", search, req.Tag)
	for NPid, _ := range search[string(req.Tag)] {
		log.Printf("Assembling response adding %s to %s", resp, storage[NPid])
		resp = append(resp, storage[NPid])
	}
	log.Printf("Dispatching final response %s", resp)
	return resp
}

func updateStorage(e Event, storage map[string]KubeObject, search map[string]map[string]bool, config Config) {
	NPid := e.Object.makeId()
	Selector := e.Object.getSelector(config)

	if e.Type == "ADDED" {
		log.Printf("Processing ADD request for %s", e.Object.Metadata.Name)
		storage[NPid] = e.Object
		if _, ok := search[Selector]; !ok {
			m := make(map[string]bool)
			search[Selector] = m
		}
		search[Selector][NPid] = true
	} else if e.Type == "DELETED" {
		log.Printf("Processing DELETE request for %s", e.Object.Metadata.Name)
		delete(storage, NPid)
		delete(search[Selector], NPid)
	} else {
		log.Printf("Received unindentified request %s for %s", e.Type, e.Object.Metadata.Name)
	}
}

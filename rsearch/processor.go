package rsearch

import (
	"log"
)

/*
{"type":"ADDED","object":{"apiVersion":"romana.io/demo/v1","kind":"NetworkPolicy","metadata":{"name":"pol1","namespace":"default","selfLink":"/apis/romana.io/demo/v1/namespaces/default/networkpolicys/pol1","uid":"d7036130-e119-11e5-aab8-0213e1312dc5","resourceVersion":"119875","creationTimestamp":"2016-03-03T08:28:00Z","labels":{"owner":"t1"}},"spec":{"allowIncoming":{"from":[{"pods":{"tier":"frontend"}}],"toPorts":[{"port":80,"protocol":"TCP"}]},"podSelector":{"tier":"backend"}}}}
*/

// Process is a goroutine that consumes resource update events and maintains a searchable
// cache of all known resources. It also accepts search requests and perform searches.
func Process(in <-chan Event, done chan Done, config Config) chan<- SearchRequest {
	// Channel to submit SearchRequest's into
	req := make(chan SearchRequest)

	// storage map is a cache of known KubeObjects
	// arranged by NPid
	storage := make(map[string]KubeObject)

	// search map is a cache of known NPid's
	// arranged by Selectors, where selector being
	// a field by which we search
	search := make(map[string]map[string]bool)

	go func() {
		for {
			select {
			case e := <-in:
				// On incoming event update caches
				updateStorage(e, storage, search, config)
			case request := <-req:
				// On incoming search request return a list
				// of resources with matching Selectors
				processSearchRequest(storage, search, request, config)
			case <-done:
				return
			}
		}
	}()

	return req
}

func processSearchRequest(storage map[string]KubeObject, search map[string]map[string]bool, req SearchRequest, config Config) SearchResponse {
	if config.Server.Debug {
		log.Println("Received request", req)
	}

	var resp []KubeObject

	if config.Server.Debug {
		log.Printf("Index map has following %s, request tag is %s ", search, req.Tag)
	}

	// Assembling response.
	for NPid, _ := range search[string(req.Tag)] {
		if config.Server.Debug {
			log.Printf("Assembling response adding %s to %s", resp, storage[NPid])
		}
		resp = append(resp, storage[NPid])
	}

	if config.Server.Debug {
		log.Printf("Dispatching final response %s", resp)
	}

	req.Resp <- resp // TODO see if it may hang up here
	return resp
}

func updateStorage(e Event, storage map[string]KubeObject, search map[string]map[string]bool, config Config) {
	NPid := e.Object.makeId()
	Selector := e.Object.getSelector(config)

	if e.Type == "ADDED" {
		if config.Server.Debug {
			log.Printf("Processing ADD request for %s", e.Object.Metadata.Name)
		}
		storage[NPid] = e.Object
		if _, ok := search[Selector]; !ok {
			m := make(map[string]bool)
			search[Selector] = m
		}
		search[Selector][NPid] = true
	} else if e.Type == "DELETED" {
		if config.Server.Debug {
			log.Printf("Processing DELETE request for %s", e.Object.Metadata.Name)
		}
		delete(storage, NPid)
		delete(search[Selector], NPid)
	} else {
		if config.Server.Debug {
			log.Printf("Received unindentified request %s for %s", e.Type, e.Object.Metadata.Name)
		}
	}
}

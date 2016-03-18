package search

import (
	"fmt"
	"log"
	"time"
	"net/http"
	"encoding/json"
)

/*
{"type":"ADDED","object":{"kind":"Namespace","apiVersion":"v1","metadata":{"name":"default","selfLink":"/api/v1/namespaces/default","uid":"d10db271-dc03-11e5-9c86-0213e1312dc5","resourceVersion":"6","creationTimestamp":"2016-02-25T21:07:45Z"},"spec":{"finalizers":["kubernetes"]},"status":{"phase":"Active"}}}
*/

type NsEvent struct {
	Type string	`json:"Type"`
	Object NsObject	`json:"object"`
}

type NsObject struct {
	Kind	string	`json:"kind"`
	Spec	Spec	`json:"spec"`
	ApiVersion	string	`json:"apiVersion"`
	Metadata	Metadata	`json:"metadata"`
	Status	map[string]string	`json:"status"`
}

func NsWatch(done <-chan Done,  url string) (<-chan NsEvent, error) {
	out := make(chan NsEvent)
	resp, err := http.Get(url)
	if err != nil {
		return out, err
	}
	tick := time.Tick(1 * time.Second)
	fmt.Println(resp.Body)
	dec := json.NewDecoder(resp.Body)
	var e NsEvent

	go func() {
		for {
			select {
			case <- tick:
				dec.Decode(&e)
				out <- e
			case <- done:
				return
			}
		}
	}()

	return out, nil
}

func (ns NsEvent) Produce(out chan Event, done <-chan Done, config  Config) error {
	url := fmt.Sprintf("%s/%s/%s/%s", config.Api.Url, config.Resource.UrlPrefix, ns.Object.Metadata.Name, config.Resource.UrlPostfix)
	log.Println("Launching producer to listen on ", url)
	tick := time.Tick(1 * time.Second)

	resp, err  := http.Get(url)
	if err != nil {
		return  err
	}

	dec := json.NewDecoder(resp.Body)
	var e Event
	go func() {
		for {
			select {
			case <- tick:
				dec.Decode(&e)
				out <- e
			case <- done:
				return
			}
		}
	}()

	return nil
}

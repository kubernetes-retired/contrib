package rsearch

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

/*
{"type":"ADDED","object":{"kind":"Namespace","apiVersion":"v1","metadata":{"name":"default","selfLink":"/api/v1/namespaces/default","uid":"d10db271-dc03-11e5-9c86-0213e1312dc5","resourceVersion":"6","creationTimestamp":"2016-02-25T21:07:45Z"},"spec":{"finalizers":["kubernetes"]},"status":{"phase":"Active"}}}
*/

// NsEvent is an alias to Event to visually distinguish
// namespace related events
type NsEvent Event

// NsWatch generates events related to kubernetes namespaces
func NsWatch(done <-chan Done, url string) (<-chan NsEvent, error) {
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
			case <-tick:
				dec.Decode(&e)
				out <- e
			case <-done:
				return
			}
		}
	}()

	return out, nil
}

// Produce generates events in a namespace of base object
func (ns KubeObject) Produce(out chan Event, done <-chan Done, config Config) error {
	url := fmt.Sprintf("%s/%s/%s/%s", config.Api.Url, config.Resource.UrlPrefix, ns.Metadata.Name, config.Resource.UrlPostfix)
	log.Println("Launching producer to listen on ", url)
	tick := time.Tick(1 * time.Second)

	resp, err := http.Get(url)
	if err != nil {
		return err
	}

	dec := json.NewDecoder(resp.Body)
	var e Event
	go func() {
		for {
			select {
			case <-tick:
				dec.Decode(&e)
				out <- e
			case <-done:
				return
			}
		}
	}()

	return nil
}

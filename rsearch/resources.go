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

type Event struct {
	Type   string     `json:"Type"`
	Object KubeObject `json:"object"`
}

type KubeObject struct {
	Kind       string            `json:"kind"`
	Spec       Spec              `json:"spec"`
	ApiVersion string            `json:"apiVersion"`
	Metadata   Metadata          `json:"metadata"`
	Status     map[string]string `json:"status,omitempty"`
}

func (o KubeObject) makeId() string {
	id := o.Metadata.Name + "/" + o.Metadata.Namespace
	return id
}

func (o KubeObject) getSelector(config Config) string {
	var selector string
	// TODO this should use Config.Resource.Selector path instead of podSelector
	for k, v := range o.Spec.PodSelector {
		selector = k + "/" + v + "#"
	}
	return selector
}

// TODO need to find a way to use different specs for different resources
type Spec struct {
	AllowIncoming map[string]interface{} `json:"allowIncoming"`
	PodSelector   map[string]string      `json:"podSelector"`
}

type Metadata struct {
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	SelfLink          string            `json:"selfLink"`
	Uid               string            `json:"uid"`
	ResourceVersion   string            `json:"resourceVersion"`
	CreationTimestamp string            `json:"creationTimestamp"`
	Labels            map[string]string `json:"labels"`
}

// NsEvent is an alias to Event to visually distinguish
// namespace related events
type NsEvent Event

// NsWatch is a generator that watches namespace related events in
// kubernetes API and publishes this events to a channel.
func NsWatch(done <-chan Done, url string, config Config) (<-chan NsEvent, error) {
	out := make(chan NsEvent)
	// TODO need to handle GET timeouts probably.
	resp, err := http.Get(url)
	if err != nil {
		return out, err
	}
	tick := time.Tick(1 * time.Second)
	if config.Server.Debug {
		log.Println("Received namespace related event from kubernetes", resp.Body)
	}
	dec := json.NewDecoder(resp.Body)
	var e NsEvent

	go func() {
		for {
			select {
			case <-tick:
				if err = dec.Decode(&e); err != nil {
					if config.Server.Debug {
						log.Printf("Failed to decode message from conenction %s due to %s\n. Attempting to re-establish", url, err)
					}
					out <- NsEvent{Type: "_CRASH"}
					resp, err2 := http.Get(url)
					if (err2 != nil) && (config.Server.Debug) {
						log.Printf("Failed establish conenction %s due to %s\n.", url, err)
					} else if err2 == nil {
						dec = json.NewDecoder(resp.Body)
					}
				} else {
					out <- e
				}
			case <-done:
				return
			}
		}
	}()

	return out, nil
}

// Produce method listens for resource updates happening within givcen namespace
// and publishes this updates in a channel
func (ns KubeObject) Produce(out chan Event, done <-chan Done, config Config) error {
	url := fmt.Sprintf("%s/%s/%s/%s", config.Api.Url, config.Resource.UrlPrefix, ns.Metadata.Name, config.Resource.UrlPostfix)
	if config.Server.Debug {
		log.Println("Launching producer to listen on ", url)
	}
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
				if err = dec.Decode(&e); err != nil {
					if config.Server.Debug {
						log.Printf("Failed to decode message from conenction %s due to %s\n. Attempting to re-establish", url, err)
					}
					resp, err2 := http.Get(url)
					if (err2 != nil) && (config.Server.Debug) {
						log.Printf("Failed establish conenction %s due to %s\n.", url, err)
					} else if err2 == nil {
						dec = json.NewDecoder(resp.Body)
					}
				} else {
					out <- e
				}
			case <-done:
				return
			}
		}
	}()

	return nil
}

package rsearch

import (
	"fmt"
	"gopkg.in/gcfg.v1"
	"log"
	"math/rand"
	"net/http"
	"testing"
	"time"
)

const cfgStr = `
; comment
[api]
url=http://127.0.0.1
namespaceUrl=api/v1/namespaces

[resource]
type=3rdParty
namespaced=true
urlPrefix=apis/romana.io/demo/v1/namespaces
urlPostfix=networkpolicys
name=NetworkPolicy
selector=podSelector

[server]
port=9700
debug=false
`

const testNs = `{"type":"ADDED","object":{"kind":"Namespace","apiVersion":"v1","metadata":{"name":"default","selfLink":"/api/v1/namespaces/default","uid":"d10db271-dc03-11e5-9c86-0213e1312dc5","resourceVersion":"6","creationTimestamp":"2016-02-25T21:07:45Z"},"spec":{"finalizers":["kubernetes"]},"status":{"phase":"Active"}}}
`

const testPolicy = `{"type":"ADDED","object":{"apiVersion":"romana.io/demo/v1","kind":"NetworkPolicy","metadata":{"name":"pol1","namespace":"default","selfLink":"/apis/romana.io/demo/v1/namespaces/default/networkpolicys/pol1","uid":"d7036130-e119-11e5-aab8-0213e1312dc5","resourceVersion":"119875","creationTimestamp":"2016-03-03T08:28:00Z","labels":{"owner":"t1"}},"spec":{"allowIncoming":{"from":[{"pods":{"tier":"frontend"}}],"toPorts":[{"port":80,"protocol":"TCP"}]},"podSelector":{"tier":"backend"}}}}
`

func fakeNsHandler(w http.ResponseWriter, req *http.Request) {
	fmt.Fprint(w, testNs)
}

func fakePolicyHandler(w http.ResponseWriter, req *http.Request) {
	fmt.Fprint(w, testPolicy)
}

func fakeServer(config Config) {
	http.HandleFunc("/"+config.Api.NamespaceUrl, fakeNsHandler)
	policyUrl := "/" + config.Resource.UrlPrefix + "/default/" + config.Resource.UrlPostfix
	http.HandleFunc(policyUrl, fakePolicyHandler)
	log.Fatal(http.ListenAndServe(config.Api.Url[7:], nil))
}

// Testing ability of nswatch to watch ns events
func TestNsWatch(t *testing.T) {
	config := Config{}
	err := gcfg.ReadStringInto(&config, cfgStr)
	if err != nil {
		t.Errorf("Failed to parse gcfg data: %s", err)
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	port := 50000 + r.Intn(1000)
	config.Api.Url = fmt.Sprintf("%s:%d", config.Api.Url, port)

	go fakeServer(config)
	time.Sleep(time.Duration(5 * time.Second))

	done := make(chan Done)
	url := config.Api.Url + "/" + config.Api.NamespaceUrl
	nsEvents, err := NsWatch(done, url)
	if err != nil {
		t.Error("Namespace watcher failed to start", err)
	}

	ns := <-nsEvents
	if ns.Object.Metadata.Name != "default" {
		t.Error("Expecting namespace name = default, got =", ns.Object.Metadata.Name)
	}

	events := make(chan Event)
	err = ns.Object.Produce(events, done, config)
	if err != nil {
		t.Error("Namespace producer failed to start", err)
	}

	e := <-events
	if e.Object.Metadata.Name != "pol1" {
		t.Error("Expecting policy name = pol1, got =", e.Object.Metadata.Name)
	}
}

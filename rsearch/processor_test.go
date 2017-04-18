package rsearch

import (
	"bytes"
	"encoding/json"
	"gopkg.in/gcfg.v1"
	"testing"
	"time"
)

func TestResoureProcessor(t *testing.T) {
	config := Config{}
	err := gcfg.ReadStringInto(&config, cfgStr)
	if err != nil {
		t.Errorf("Failed to parse gcfg data: %s", err)
	}

	done := make(chan Done)
	events := make(chan Event)

	req := Process(events, done, config)
	time.Sleep(time.Duration(1 * time.Second))

	var e Event
	policyReader := bytes.NewBufferString(testPolicy)
	dec := json.NewDecoder(policyReader)
	dec.Decode(&e)

	events <- e

	responseChannel := make(chan SearchResponse)
	searchRequest := SearchRequest{Tag: "tier/backend#", Resp: responseChannel}
	req <- searchRequest

	result := <-searchRequest.Resp
	if result[0].Metadata.Name != "pol1" {
		t.Error("Unexpected search response = expect policy name = pol1, got ", result[0].Metadata.Name)
	}
}

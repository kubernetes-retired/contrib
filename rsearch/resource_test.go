package search

import (
	"gopkg.in/gcfg.v1"
	"bytes"
	"testing"
	"time"
	"encoding/json"
)

func TestResoureProcessor(t *testing.T) {
	config := Config{}
	err := gcfg.ReadStringInto(&config, cfgStr)
	if err != nil {
		t.Errorf("Failed to parse gcfg data: %s", err)
	}

	done := make(chan Done)
	events := make(chan Event)

	req, resp := Process(events, done, config)
	time.Sleep(time.Duration(1 * time.Second))

	var e Event
	policyReader := bytes.NewBufferString(testPolicy)
	dec := json.NewDecoder(policyReader)
	dec.Decode(&e)

	events <- e
	searchRequest := SearchRequest{ Tag: "tier/backend#" }
	req <- searchRequest

	result := <- resp
	if result[0].Metadata.Name != "pol1" {
		t.Error("Unexpected search response = expect policy name = pol1, got ", result[0].Metadata.Name)
	}
}

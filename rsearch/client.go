package rsearch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// SearchResource
func SearchResource(config Config, req SearchRequest) SearchResponse {
	url := "http://localhost:" + config.Server.Port
	data := []byte(`{ "tag" : "` + req.Tag + `"}`)
	if config.Server.Debug {
		log.Println("Making request with", string(data))
	}

	request, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()
	decoder := json.NewDecoder(response.Body)

	sr := SearchResponse{}

	if config.Server.Debug {
		log.Println("Trying to decode", response.Body)
	}
	err = decoder.Decode(&sr)
	if err != nil {
		panic(err)
	}

	if config.Server.Debug {
		fmt.Println("Got response form a server", sr)
	}
	return sr
}

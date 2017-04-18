package rsearch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// SearchResource connects to instance of a server
// and resolves SearchRequest
func SearchResource(config Config, req SearchRequest) SearchResponse {
	// TODO need to make url configurable
	url := config.Server.Host + ":" + config.Server.Port
	data := []byte(`{ "tag" : "` + req.Tag + `"}`)
	if config.Server.Debug {
		log.Println("Making request with", string(data))
	}

	// Make request
	request, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		log.Println("HTTP request failed", url, err)
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
		log.Println("Failed to decode", response.Body)
		panic(err)
	}

	if config.Server.Debug {
		fmt.Println("Decoded response form a server", sr)
	}
	return sr
}

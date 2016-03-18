package main

import (
	"flag"
	"fmt"
	search "github.com/romana/contrib/rsearch"
	"log"
	//	"net/http"
	//    "encoding/json"

	//	"io"
)

func main() {
	var cfgFile = flag.String("c", "", "Kubernetes reverse search config file")
	var server = flag.Bool("s", false, "Start a server")
	var searchTag = flag.String("r", "", "Search resources by tag")
	flag.Parse()

	done := make(chan search.Done)

	config, err := search.NewConfig(*cfgFile)
	if err != nil {
		fmt.Printf("Can not read config file %s, %s\n", *cfgFile, err)
		return
	}

	if *server {
		fmt.Println("Starting server")
		nsUrl := fmt.Sprintf("%s/%s", config.Api.Url, config.Api.NamespaceUrl)
		nsEvents, err := search.NsWatch(done, nsUrl)
		if err != nil {
			log.Fatal("Namespace watcher failed to start", err)
		}

		events := search.Conductor(nsEvents, done, config)
		req, resp := search.Process(events, done, config)
		log.Println("All routines started")
		search.Serve(config, req, resp)
	} else if len(*searchTag) > 0 {
		if config.Server.Debug {
			fmt.Println("Making request t the server")
		}
		r := search.SearchResource(config, search.SearchRequest{*searchTag})
		fmt.Println(r)
	}

}

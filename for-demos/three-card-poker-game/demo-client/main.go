package main

import (
	"flag"
	"log"
	"net/http"
)

func main() {

	router := NewRouter()
	var dir string

	flag.StringVar(&dir, "dir", "./static", "the directory to serve files from. Defaults to the current dir")
	flag.Parse()

	// This will serve files under http://localhost:8000/static/<filename>
	router.PathPrefix("/demo/").Handler(http.StripPrefix("/demo/", http.FileServer(http.Dir(dir))))

	log.Fatal(http.ListenAndServe(":8083", router))
}

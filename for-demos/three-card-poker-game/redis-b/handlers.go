package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func Index(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Welcome!\n")
}

func GetRandom(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	rNum := GetRandomNumber()
	if err := json.NewEncoder(w).Encode(rNum); err != nil {
		panic(err)
	}
}

/*
Test with this curl command:
curl -H "Content-Type: application/json" -d '{"key":"value"}' http://localhost:8080/something
*/

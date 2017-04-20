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

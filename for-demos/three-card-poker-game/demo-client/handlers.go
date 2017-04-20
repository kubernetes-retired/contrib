package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

func Index(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Welcome! to Demo Client\n")
}

func GetPods(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
        namespace := r.FormValue("namespace")
	data := pods(namespace)
	w.Write(data)

	return
}

func pods(namespace string) []byte {

	masterService := os.Getenv("MASTER_SERVICE")
	if len(masterService) == 0 {
		masterService = "10.65.226.67"
	}

	podsURL := "http://" + masterService + ":8080/" + "api/v1/namespaces/" + namespace + "/pods"

	//call to get Number
	req, err := http.NewRequest("GET", podsURL, nil)
	if err != nil {
		log.Fatal("NewRequest: ", err)
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal("Do: ", err)
	}

	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body) //<--- here!

	if err != nil {
		log.Fatal(err)
	}
	return data
}

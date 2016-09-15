package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

type Number struct {
	Id string `json:"Id"`
}

type Suit struct {
	Id string `json:"Id"`
}

func Index(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Welcome!\n")
}

func GetCard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	s1 := composite()
	s2 := composite()
	s3 := composite()

	for s1 == s2 || s2 == s3 || s3 == s1 {
		s1 = composite()
		s2 = composite()
		//hack to avoid match
		fmt.Println("****Hack Used****")

	}

	fmt.Printf(s1)
	fmt.Printf(s2)
	fmt.Printf(s3)

	card := Card{Card1: s1, Card2: s2, Card3: s3}
	fmt.Println("Card*****", card)

	if err := json.NewEncoder(w).Encode(card); err != nil {
		panic(err)
	}
	return
}

func composite() string {

	redisAAPPService := os.Getenv("REDIS_A_APP_SERVICE")
	//redisAAPPHost := os.Getenv(redisAAPPService)
	redisA := os.Getenv(redisAAPPService)

	redisBAPPService := os.Getenv("REDIS_B_APP_SERVICE")
	//redisBAPPHost := os.Getenv(redisBAPPService)
	redisB := os.Getenv(redisBAPPService)

	//redisA := os.Getenv(redisAAPPHost)
	//redisB := os.Getenv(redisBAPPHost)

	//url for number
	url1 := "http://" + redisA + ":8080/randomNumber"
	//url for suit
	url2 := "http://" + redisB + ":8081/randomSuit"

	//log.Print(url1)
	//log.Print(url2)

	var suit Suit
	var number Number

	//call to get Number
	req, err := http.NewRequest("GET", url1, nil)
	if err != nil {
		log.Fatal("NewRequest: ", err)
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal("Do: ", err)
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(&number); err != nil {
		log.Println(err)
	}

	//call to get Suit
	req1, err := http.NewRequest("GET", url2, nil)
	if err != nil {
		log.Fatal("NewRequest: ", err)
	}
	client1 := &http.Client{}
	resp1, err := client1.Do(req1)
	if err != nil {
		log.Fatal("Do: ", err)
	}
	defer resp1.Body.Close()
	if err := json.NewDecoder(resp1.Body).Decode(&suit); err != nil {
		log.Println(err)
	}

	s := number.Id + "_of_" + suit.Id
	return s
}

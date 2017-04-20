package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/garyburd/redigo/redis"
)

func init() {
	redisService := os.Getenv("REDIS_SERVICE")
	redisHost := os.Getenv(redisService)
	c, err := redis.Dial("tcp", redisHost+":6379")
	if err != nil {
		panic(err)
	}
	defer c.Close()

	//set
	c.Do("SET", "21", "hearts")
	c.Do("SET", "22", "diamonds")
	c.Do("SET", "23", "clubs")
	c.Do("SET", "24", "spades")
}

//GetRandomNumber returns a random number between Ace to King
func GetRandomNumber() RandomNum {
	//INIT OMIT
	redisService := os.Getenv("REDIS_SERVICE")
	redisHost := os.Getenv(redisService)
	c, err := redis.Dial("tcp", redisHost+":6379")
	if err != nil {
		panic(err)
	}
	defer c.Close()

	rand.Seed(time.Now().UTC().UnixNano())
	rNum := 21 + rand.Intn(4)
	fmt.Println(rNum)

	//get
	r, err := redis.String(c.Do("GET", rNum))
	if err != nil {
		fmt.Println("key not found")
	}

	fmt.Println(r)
	s := RandomNum{r}
	return s
	//ENDINIT OMIT
}

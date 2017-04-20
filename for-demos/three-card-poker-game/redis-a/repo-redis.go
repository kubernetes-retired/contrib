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
	c.Do("SET", "1", "ace")
	c.Do("SET", "2", "2")
	c.Do("SET", "3", "3")
	c.Do("SET", "4", "4")
	c.Do("SET", "5", "5")
	c.Do("SET", "6", "6")
	c.Do("SET", "7", "7")
	c.Do("SET", "8", "8")
	c.Do("SET", "9", "9")
	c.Do("SET", "10", "10")
	c.Do("SET", "11", "jack")
	c.Do("SET", "12", "queen")
	c.Do("SET", "13", "king")
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
	rNum := 1 + rand.Intn(13)
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

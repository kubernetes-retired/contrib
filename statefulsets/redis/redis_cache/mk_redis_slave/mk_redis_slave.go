/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"github.com/mediocregopher/radix.v2/redis"
)

func main() {

	MP, err := ioutil.ReadFile("/config/master.txt")

	if err != nil {
		log.Printf("Unable to read master.txti err=%v, must be the master", err)
	} else {

		hostname, err := os.Hostname()
		if err != nil {
			log.Fatalf("Failed to get hostname: %s", err)
		}
		MyName := fmt.Sprintf("%s:6379", hostname)

		var C *redis.Client
		var rerr error

		for i := 100; i > 0; i-- {
			C, rerr = redis.Dial("tcp", MyName)
			if rerr != nil {
				log.Printf("error connecting to redis %v instance failed err=%v", MyName, rerr)
				if i <= 1 {
					log.Printf("retry exhausted. Exiting...")
					return
				}
			} else {
				break
			}
			time.Sleep(1 * time.Second)
		}

		hostport := strings.Split(string(MP), " ")

		if len(hostport) != 2 {
			log.Printf("Error invalid Master Endpoint %v", hostport)
			return
		}

		resp := C.Cmd("SLAVEOF", hostport[0], hostport[1]).String()

		if !strings.Contains(resp, "OK") {
			log.Printf("Unable to set SLAVEOF %v %v command err=%v", hostport[0], hostport[1], resp)
			return
		}

		log.Printf("this slave is now replicating from %v %v", hostport[0], hostport[1])
	}

	log.Printf("peer-finder: Will sleep for ever")
	for {
		time.Sleep(time.Hour * 24)
	}

}

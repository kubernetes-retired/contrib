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
	"flag"
	"log"
	"os"
	"text/template"
)

//RedisStatefulSet This type will have the template
type RedisStatefulSet struct {
	Name             string
	Replica          int
	DisruptionBudget int
}

//processRedisTemplate This function takes name=, replica=r and disrutptionBudget = dB as argument and process the template
func processRedisTemplate(n string, r int, dB int) error {

	tmpl, err := template.ParseFiles("./redis.tempalate")
	if err != nil {
		log.Printf("Unable to create new template")
		return err
	}
	RSS := RedisStatefulSet{n, r, dB}
	err = tmpl.Execute(os.Stdout, RSS)
	if err != nil {
		log.Printf("Unable to execute the template RSS=%v", RSS)
		return err
	}
	return nil

}

func main() {

	//Process all the flags
	name := flag.String("name", "cache", "Name of the redis statefulset")
	replica := flag.Int("replicas", 3, "Number of redis instances you need in this master slave setup")
	disruptionBudget := flag.Int("disruptionbudge", 2, "Number number of guaranteed instances during K8s Maintenance activity")
	flag.Parse()

	//Open the template file
	err := processRedisTemplate(*name, *replica, *disruptionBudget)

	//Unable to
	if err != nil {
		log.Fatalf("Unable to generate Yml file err=%v", err)
		os.Exit(1)
	}

}

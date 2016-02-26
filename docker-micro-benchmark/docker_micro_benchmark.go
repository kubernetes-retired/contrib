/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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
	"os"

	docker "github.com/fsouza/go-dockerclient"
)

func main() {
	usage := func() {
		fmt.Printf("Usage: %s -[o|c|i|r]\n", os.Args[0])
	}
	if len(os.Args) != 2 {
		usage()
		return
	}
	client, _ := docker.NewClient(endpoint)
	client.PullImage(docker.PullImageOptions{Repository: "ubuntu", Tag: "latest"}, docker.AuthConfiguration{})
	switch os.Args[1] {
	case "-o":
		benchmarkContainerStart(client)
	case "-c":
		benchmarkVariesContainerNumber(client)
	case "-i":
		benchmarkVariesInterval(client)
	case "-r":
		benchmarkVariesRoutineNumber(client)
	default:
		usage()
	}
}

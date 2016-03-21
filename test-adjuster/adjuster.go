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
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/golang/glog"
)

var (
	repoPath       = flag.String("repo-path", "/kubernetes", "Path to the clone of Kubernetes repository.")
	testConfigPath = flag.String("test-config-path", "hack/jenkins/job-configs/kubernetes-jenkins/kubernetes-e2e.yaml",
		"path to the config file to change, relative to the repo-path")
	pollFrequency  = 10*time.Minute
	scalabilityHighFrequencyCron = "@hourly"
	scalablityLowFrequencyCron = "H H/12 * * *"
)

func substituteCronStringAfter(delim string, targetCronString string) string {
	file, err := os.Open(*repoPath + "/" + *testConfigPath)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	data, err := ioutil.ReadAll(file)
	currentConfig := string(data)
	if err != nil {
		panic(err)
	}
	index := strings.Index(currentConfig, delim)
	cronIndex := strings.Index(currentConfig[index:len(currentConfig)], "cron-string")
	cronIndex += index
	newConfig := currentConfig[0:cronIndex]
	crondEndlineIndex := strings.Index(currentConfig[cronIndex:len(currentConfig)], "\n")
	crondEndlineIndex += cronIndex
	newConfig = newConfig + fmt.Sprintf("cron-string: '%v'", targetCronString)
	newConfig = newConfig + currentConfig[crondEndlineIndex:len(currentConfig)]
	return newConfig
}

func IncreaseScalability100Frequency() string {
	return substituteCronStringAfter("gce-scalability", scalabilityHighFrequencyCron)
}

func DecreaseScalability100Frequency() string {
	return substituteCronStringAfter("gce-scalability", scalablityLowFrequencyCron)
}

func NumberOfFailuresInLast10Builds() map[string]int {
	return map[string]int{"kubernetes-kubemark-500-gce": 5}
}

func main() {
	flag.Parse()

	for {
		failedBuilds := NumberOfFailuresInLast10Builds()
		for build, numberOfFailures := range failedBuilds {
			if build == "kubernetes-kubemark-500-gce" {
				if numberOfFailures > 2 {
					glog.Infof("Increasing")
					newConfig := IncreaseScalability100Frequency()
					ioutil.WriteFile(*repoPath + "/" + *testConfigPath, []byte(newConfig), 0644)
				} else {
					glog.Infof("Decreasing")
					newConfig := DecreaseScalability100Frequency()
					ioutil.WriteFile(*repoPath + "/" + *testConfigPath, []byte(newConfig), 0644)
				}
			}
		}

		time.Sleep(pollFrequency)
	}
}

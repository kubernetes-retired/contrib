/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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
	"bufio"
	goflag "flag"
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/pflag"
	"k8s.io/contrib/compare/src"
)

const (
	urlPrefix = "https://storage.googleapis.com/kubernetes-jenkins/logs/kubernetes-e2e-gce-scalability/"
	urlSuffix = "/build-log.txt"
)

var (
	leftBuildNumber, rightBuildNumber int
	enableOutputColoring              bool
)

func registerFlags(fs *pflag.FlagSet) {
	fs.IntVar(&leftBuildNumber, "left-build-number", 0, "Id of the build to serve as a left hand side of comparison.")
	fs.IntVar(&rightBuildNumber, "right-build-number", 0, "Id of the build to serve as a right hand side of comparison.")
	fs.BoolVar(&enableOutputColoring, "enable-output-coloring", true, "If set to true tool will print offending values in color")
}

func main() {
	registerFlags(pflag.CommandLine)
	pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)
	pflag.Parse()

	if leftBuildNumber == 0 || rightBuildNumber == 0 {
		fmt.Fprintf(os.Stderr, "Need both left and right build numbers")
		return
	}

	leftResp, err := http.Get(fmt.Sprintf("%v%v%v", urlPrefix, leftBuildNumber, urlSuffix))
	if err != nil {
		panic(err)
	}
	leftBody := leftResp.Body
	defer leftBody.Close()
	leftBodyScanner := bufio.NewScanner(leftBody)
	leftLogs, leftResources, leftMetrics := src.ProcessSingleTest(leftBodyScanner, leftBuildNumber)

	rightResp, err := http.Get(fmt.Sprintf("%v%v%v", urlPrefix, rightBuildNumber, urlSuffix))
	if err != nil {
		panic(err)
	}
	rightBody := rightResp.Body
	defer rightBody.Close()
	rightBodyScanner := bufio.NewScanner(rightBody)
	rightLogs, rightResources, rightMetrics := src.ProcessSingleTest(rightBodyScanner, rightBuildNumber)

	if len(leftLogs) != 0 && len(rightLogs) != 0 {
		for k := range leftLogs {
			if _, ok := rightLogs[k]; !ok {
				fmt.Printf("Right logs missing for test %v\n", k)
				continue
			}
			violatingLogs := src.CompareLogGenerationSpeed(leftLogs[k], rightLogs[k])
			fmt.Printf("Differences for test %v\n", k)
			violatingLogs.PrintToStdout(leftBuildNumber, rightBuildNumber, enableOutputColoring)
		}
	}
	fmt.Println("")

	if len(leftResources) != 0 && len(rightResources) != 0 {
		for k := range leftResources {
			if _, ok := rightResources[k]; !ok {
				fmt.Printf("Right resources missing for test %v\n", k)
				continue
			}
			violatingResources := src.CompareResourceUsages(leftResources[k], rightResources[k])
			fmt.Printf("Differences for test %v\n", k)
			violatingResources.PrintToStdout(leftBuildNumber, rightBuildNumber, enableOutputColoring)
		}
	}
	fmt.Println("")

	if len(leftMetrics) != 0 && len(rightMetrics) != 0 {
		for k := range rightMetrics {
			if _, ok := rightMetrics[k]; !ok {
				fmt.Printf("Right resources missing for test %v\n", k)
				continue
			}
			violatingMetrics := src.CompareMetrics(leftMetrics[k], rightMetrics[k])
			fmt.Printf("Differences for test %v\n", k)
			violatingMetrics.PrintToStdout(leftBuildNumber, rightBuildNumber, enableOutputColoring)
		}
	}
}

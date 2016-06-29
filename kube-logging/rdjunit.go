package main

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"io/ioutil"
	"strings"
	"flag"
)

// taken from contrib/e2e.go
type Testcase struct {
	Name      string `xml:"name,attr"`
	ClassName string `xml:"classname,attr"`
	Failure   string `xml:"failure"`
}
type Testsuite struct {
	TestCount int        `xml:"tests,attr"`
	FailCount int        `xml:"failures,attr"`
	Testcases []Testcase `xml:"testcase"`
}

// look for junit files in each of the folders for each image located
// in the artifacts folder and call the functions for getting the 
// information about each of the failed tests
func main() {	
	folder := flag.String("f", "", "the folder name")
	kubeFilter := flag.String("kblt", "all", "filter for kubelet log")
	flag.Parse()

	images, err := filepath.Glob(*folder + "/artifacts/tmp*")
	check(err)

	kubeFilters := map[string]bool{"all":true, "p":true, "1":true}


	if !kubeFilters[*kubeFilter] {
		fmt.Println("Invalid kubelet log filter")
		os.Exit(1)
	}

	mapPodError := map[string]string{}
	mapPodCont := map[string]string{}

	if len(images) != 0 {
		for _, fp := range images {
			junits, err := filepath.Glob(fp + "/junit*")
			check(err)
			for _, ju := range junits {
				pods := getFailedPods(ju, mapPodError, mapPodCont)
				if len(pods) > 0 {
					mapPodKubelet := rdKubelet(fp, *kubeFilter, pods)
					rdKubeAPI(fp, mapPodKubelet)
				}
			}
		}
	} 

	// fmt.Println("mapPodError")
	// fmt.Println(mapPodError)
	// fmt.Println("")
	// fmt.Println("mapPodCont")
	// fmt.Println(mapPodCont)
}

// from a junit xml file, populate the testsuite struct and extract
// the failed pods and the containers they are associated with
func getFailedPods(fp string, mapPodError map[string]string, mapPodCont map[string]string) ([]string){
	testSuite := &Testsuite{}
	
	failures := map[string]string{}
	
	file, err := os.Open(fp)
	check(err)
	defer file.Close()
	data, err := ioutil.ReadAll(file)
	check(err)
	
	err = xml.Unmarshal(data, testSuite)

	if testSuite.FailCount == 0 {
		// no pods failed in this file
		return nil
	}

	check(err)

	for _, tc := range testSuite.Testcases{
		if  tc.Failure != "" {
			failures[fmt.Sprintf("%v {%v}", tc.Name, tc.ClassName)] = tc.Failure
		}
	}
	
	fmt.Println("Failed Pods in file", fp)
	fmt.Println("")

	podNames := make([]string, 0)
	for _, v := range failures {
		lines := strings.SplitAfter(v, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if (strings.HasPrefix(line, "pod")) {
				podInfo := strings.Split(line, "'")
				podName := podInfo[1]
				
				fmt.Println("----------")
				fmt.Println(podName)
				fmt.Println("----------")

				fmt.Println(line)
				
				//map the pod to the error
				mapPodError[podName] = line

				//map the pod to the container
				cont := strings.Split(strings.Split(line, "ContainerID:")[1], "}")
				mapPodCont[podName] = cont[0]
				podNames = append(podNames, podName)
				fmt.Println("")
			}
		}
	}

	return podNames
}
	
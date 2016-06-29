package main

import (
	"fmt"
	"os"
	"bufio"
	"strings"
)

func check(e error) {
       if e != nil {
		panic(e)
        }
}

// given the filepath containing the kubelet log file and a slice
// of the failed pods, returns a map of the pods to the relevant lines
// which right now are all the lines containing the pod name 
// TODO: add other filters/options like all the lines from when the pod
// first appeared to when it dissappears, lines that contain container id, etc 
func rdKubelet(fp string, filter string, pods []string) map[string]string{
	file, err := os.Open(fp + "/kubelet.log")
	check(err)
	defer file.Close()

	fmt.Println(filter)

	blackList := []string{"blob data"}
	if filter == "all" {
		blackList = []string{}
	}


	// mapPodKubelet: all the lines that contain the pod name
	mapPodKubelet := map[string]string{}
	mapPodLines := map[string]string{}

	scanner := bufio.NewScanner(file)
	check(scanner.Err())
		
	for _, pod := range(pods) {
		mapPodKubelet[pod] = ""
		mapPodLines[pod] = ""
	}
	
	if filter == "p" {
		for scanner.Scan() {
			line := scanner.Text()
			for _, pod := range(pods) {
				if strings.Contains(line, pod) {
					a := []string{mapPodKubelet[pod], line}
					mapPodKubelet[pod] = strings.Join(a,"\n")
				}
			}
		}
	} else {
		for scanner.Scan() {
			badLine := false
			line := scanner.Text()
			for _, word := range(blackList) {
				if strings.Contains(line, word) {
					badLine = true
				}
			}
			if badLine {
				continue
			}
			for _, pod := range(pods) {
				if strings.Contains(line, pod) && mapPodKubelet[pod] == "" {
					mapPodKubelet[pod] = line 
				} else if strings.Contains(line, pod) {
					mapPodLines[pod] = mapPodLines[pod] + "\n" + line
					mapPodKubelet[pod] = mapPodKubelet[pod] + "\n" + mapPodLines[pod]
				} else if mapPodKubelet[pod] != "" {
					mapPodLines[pod] = mapPodLines[pod] + "\n" + line
				}
			}
		}
	}

	fo, err :=  os.Create("filteredkubelet.txt")
	check(err)
	defer fo.Close()

	for pod, lines := range mapPodKubelet {
		fmt.Println("=================================")
		fmt.Println("Kubelet log for pod:", pod)
		fmt.Println(lines)
		fmt.Println("=================================")
	}

	return mapPodKubelet
}


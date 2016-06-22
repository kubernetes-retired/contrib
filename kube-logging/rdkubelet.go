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
func rdKubelet(fp string, pods []string) map[string]string{
	file, err := os.Open(fp + "/kubelet.log")
	check(err)
	defer file.Close()

	// mapPodKubelet: all the lines that contain the pod name
	mapPodKubelet := map[string]string{}

	scanner := bufio.NewScanner(file)
	check(scanner.Err())
		
	for _, pod := range(pods) {
		mapPodKubelet[pod] = ""
	}
	
	for scanner.Scan() {
		line := scanner.Text()
		for _, pod := range(pods) {
			if strings.Contains(line, pod) {
				a := []string{mapPodKubelet[pod], line}
				mapPodKubelet[pod] = strings.Join(a,"\n")
			}
		}
	}
	
	for pod, lines := range mapPodKubelet {
		fmt.Println("=================================")
		fmt.Println("Kubelet log for pod:", pod)
		fmt.Println(lines)
		fmt.Println("=================================")
	}

	return mapPodKubelet
}


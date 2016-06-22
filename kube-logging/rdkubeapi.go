package main

import (
	"fmt"
	"os"
	"bufio"
	"strings"
	"strconv"
)

func timeToInt(timeStampString string) ([]int) {
	timeStamp := strings.Split(timeStampString, ":")
	intStamp := make([]int, len(timeStamp))
	for i, v := range timeStamp {
		intTime, err := strconv.Atoi(v)
		if err != nil {
			seconds := strings.Split(v, ".")
			intTime, _ := strconv.Atoi(seconds[0])
			intStamp[i] = intTime
			intTime1, _ := strconv.Atoi(seconds[1])
			intStamp = append(intStamp, intTime1)
		} else {		
			intStamp[i] = intTime
		}
	}
	return intStamp
}

// read through the kube-apiserver log and return the part of the log based
// on the time the pod was mentioned in the kubelet log
func rdKubeAPI(fp string, mapPodKubelet map[string]string) {
	file, err := os.Open(fp + "/kube-apiserver.log")
	check(err)
	defer file.Close()

	mapPodKubeAPI := map[string]string{}
	podTimes := map[string][]string{}
	
	
	for pod, kubelet := range mapPodKubelet {
		splitKubelet := strings.Split(kubelet, "\n")	
		startTime := strings.Split(splitKubelet[1], " ")[1]
		endTime := strings.Split(splitKubelet[len(splitKubelet) - 1], " ")[1]
		podTimes[pod] = []string{startTime, endTime}
		mapPodKubeAPI[pod] = ""
	}


	scanner := bufio.NewScanner(file)
	check(scanner.Err())	

	for scanner.Scan() {
		line := scanner.Text()
		timeString := strings.Split(line, " ")[1]

		if len(strings.Split(timeString, ":")) == 3 {
			for pod, log := range mapPodKubeAPI {
				timeStamp := timeToInt(timeString)
				startTime := timeToInt(podTimes[pod][0])
				endTime := timeToInt(podTimes[pod][1])
				if checkInChunk(timeStamp, startTime, endTime) {
					a := []string{log, line}
					mapPodKubeAPI[pod] = strings.Join(a, "\n")
				}
			}
		}
	}

	fmt.Println("======== in rdkubeapi.go ========")
	for pod, log := range mapPodKubeAPI {
		fmt.Println(pod)
		fmt.Println("")
		fmt.Println(log)
		fmt.Println("---------------")
	}
	fmt.Println("=================================")

}

// given a timestamp check if it is between a start and end time
// TODO: put the calls to the timeToInt function in here instead
func checkInChunk(timeNow []int, startTime []int, endTime []int) (bool) {
	if (timeNow[0] >= startTime[0] && timeNow[0] <= endTime[0]) {
		if (timeNow[1] >= startTime[1] && timeNow[1] <= endTime[1]) {
			if (timeNow[2] >= startTime[2] && timeNow[2] <= endTime[2]) {
				if (timeNow[3] >= startTime[3] && timeNow[3] <= endTime[3]) {
					return true
				}
			}
		}
	}
	return false
}
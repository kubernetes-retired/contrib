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

func getTime(lines []string) ([]string) {
	startEndTimes := []string{}

	firstLine := lines[0]
	
	if len(firstLine) == 0 {
		firstLine = lines[1]
	}

	lastLine := lines[len(lines) - 1]

	for _, word := range strings.Split(firstLine, " ") {
		if len(strings.Split(word, ":")) == 3 {
			startEndTimes = append(startEndTimes, word)
			break
		}
	}
	for _, word := range strings.Split(lastLine, " ") {
		if len(strings.Split(word, ":")) == 3 {
			startEndTimes = append(startEndTimes, word)
			break
		}
	}
	return startEndTimes
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
		fmt.Println(splitKubelet[0], splitKubelet[1], splitKubelet[2])	
		startEndTimes := getTime(splitKubelet)
		podTimes[pod] = startEndTimes
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
	inChunk := true
	for i, _ := range startTime {
		inChunk = inChunk && timeNow[i] >= startTime[i] && timeNow[i] <= endTime[i]
	}

	return inChunk
	
}

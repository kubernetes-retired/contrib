package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path"
	"regexp"
	"sort"
	"time"
)

// This parser does not require additional probes for kubelet tracing. It is used by node perf dash.

const (
	// TODO(coufon): use constants defined in Kubernetes packages
	tracingVersion = "v1"
	// TODO(coufon): We need a string of year because year is missing in log timestamp.
	// Hardcoding the year here is a simple temporary solution.
	currentYear = "2016"
	// Timestamp format of test result log (build-log.txt)
	testLogTimeFormat = "2006 " + time.StampMilli
	// Timestamp format of kubelet log (kubelet.log)
	kubeletLogTimeFormat = "2006 0102 15:04:05.000000"
)

var (
	// Kubelet parser do not leverage on tracing probes. It uses the native log of Kubelet instead.
	// The mapping from native log to tracing probes is as follows:
	// TODO(coufon): there is no volume now in our node-e2e performance test. Add probe for volume manager in future.
	//          probe                                      log
	//     kubelet_firstseen           SyncLoop (ADD, "api")
	//     kubelet_runtime             docker_manager.go.*restart pod infra container
	//     kubelet_container_start     kubelet.go:2346] SyncLoop (PLEG):.*Type:"ContainerStarted"
	//     kubelet_status              status_manager.go.*updated successfully: {status:{Phase:Running
	regexMap = map[string]*regexp.Regexp{
		"kubelet_firstseen":       regexp.MustCompile(`[IW](\d{2}\d{2} \d{2}:\d{2}:\d{2}.\d{6}).*kubelet.go.*SyncLoop \(ADD, \"api\"\).*`),
		"kubelet_runtime":         regexp.MustCompile(`[IW](\d{2}\d{2} \d{2}:\d{2}:\d{2}.\d{6}).*docker_manager.go.*Need to restart pod infra container for.*because it is not found.*`),
		"kubelet_container_start": regexp.MustCompile(`[IW](\d{2}\d{2} \d{2}:\d{2}:\d{2}.\d{6}).*kubelet.go.*SyncLoop \(PLEG\): \".*\((.*)\)\".*Type:"ContainerStarted", Data:"(.*)".*`),
		"kubelet_status":          regexp.MustCompile(`[IW](\d{2}\d{2} \d{2}:\d{2}:\d{2}.\d{6}).*status_manager.go.*updated successfully.*Phase:Running.*`),
	}
)

// EndTimeTuple is the tuple of a test and its end time.
type EndTimeTuple struct {
	TestName  string
	TimeInLog time.Time
}

// SortedEndTime is a sorted list of EndTimeTuple.
type SortedEndTime []EndTimeTuple

func (a SortedEndTime) Len() int           { return len(a) }
func (a SortedEndTime) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a SortedEndTime) Less(i, j int) bool { return a[i].TimeInLog.Before(a[j].TimeInLog) }

// SortedEndTimePerNode records SortedEndTime for each node.
type SortedEndTimePerNode map[string](SortedEndTime)

// TestEndTime records the end time for a node and a test.
type TestEndTime map[string](map[string]time.Time)

// Add adds one end time into TestEndTime
func (ete TestEndTime) Add(testName, nodeName, timeInLog string) {
	if _, ok := ete[nodeName]; !ok {
		ete[nodeName] = make(map[string]time.Time)
	}
	if _, ok := ete[nodeName][testName]; !ok {
		ts, err := time.Parse(testLogTimeFormat, currentYear+" "+timeInLog)
		if err != nil {
			log.Fatal(err)
		}
		ete[nodeName][testName] = ts
	}
}

// Sort sorts all arrays of SortedEndTime for each node and return a map of sorted array.
func (ete TestEndTime) Sort() SortedEndTimePerNode {
	sortedEndTimePerNode := make(SortedEndTimePerNode)
	for nodeName, testTimeMap := range ete {
		sortedEndTime := SortedEndTime{}
		for testName, timeInLog := range testTimeMap {
			sortedEndTime = append(sortedEndTime,
				EndTimeTuple{
					TestName:  testName,
					TimeInLog: timeInLog,
				})
		}
		sort.Sort(sortedEndTime)
		sortedEndTimePerNode[nodeName] = sortedEndTime
	}
	return sortedEndTimePerNode
}

// TracingData contains the tracing time series data of a test on a node
type TracingData struct {
	Labels  map[string]string   `json:"labels"`
	Version string              `json:"version"`
	Data    map[string]int64arr `json:"op_series"`
}

// int64arr is a wrapper of sortable int64 array.
type int64arr []int64

func (a int64arr) Len() int           { return len(a) }
func (a int64arr) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a int64arr) Less(i, j int) bool { return a[i] < a[j] }

// ParseKubeletLog extracts test end time from build-log.txt and calls GrabTracingKubelet to parse tracing data.
func ParseKubeletLog(d Downloader, job string, buildNumber int, testEndTime TestEndTime) string {
	sortedEndTimePerNode := testEndTime.Sort()
	result := ""
	for nodeName, sortedEndTime := range sortedEndTimePerNode {
		result += GrabTracingKubelet(d, job, buildNumber,
			nodeName, sortedEndTime)
	}
	return result
}

// GrabTracingKubelet parse tracing data using kubelet.log. It does not reuqire additional tracing probes.
func GrabTracingKubelet(d Downloader, job string, buildNumber int, nodeName string,
	sortedEndTime SortedEndTime) string {
	// Return empty string if there is no test in list
	if len(sortedEndTime) == 0 {
		return ""
	}

	file, err := d.GetFile(job, buildNumber, path.Join("artifacts", nodeName, kubeletLogFile))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error while fetching tracing data event: %v\n", err)
		log.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	result := ""
	currentTestIndex := 0

	tracingData := TracingData{
		Labels: map[string]string{
			"test": sortedEndTime[currentTestIndex].TestName,
			"node": nodeName,
		},
		Version: tracingVersion,
		Data:    map[string]int64arr{},
	}
	containerNrPerPod := map[string]int{}

	for scanner.Scan() {
		line := scanner.Text()
		// Find a tracing event in kubelet log
		detectedEntry := parseLogEntry([]byte(line), containerNrPerPod)
		if detectedEntry != nil {
			// Detect out of range
			if sortedEndTime[currentTestIndex].TimeInLog.Before(detectedEntry.Timestamp) {
				currentTestIndex++
				if currentTestIndex >= len(sortedEndTime) {
					break
				}
				tracingData.SortData()
				result += TimeSeriesTag + tracingData.ToSeriesData() + "\n\n" +
					TimeSeriesEnd + "\n"
				// Move on to the next Test
				tracingData = TracingData{
					Labels: map[string]string{
						"test": sortedEndTime[currentTestIndex].TestName,
						"node": nodeName,
					},
					Version: tracingVersion,
					Data:    map[string]int64arr{},
				}
				containerNrPerPod = map[string]int{}
			}
			tracingData.AppendData(detectedEntry.Probe, detectedEntry.Timestamp.UnixNano())
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	tracingData.SortData()
	return result + TimeSeriesTag + tracingData.ToSeriesData() + "\n\n" +
		TimeSeriesEnd + "\n"
}

// DetectedEntry records a parsed probe and timestamp tuple.
type DetectedEntry struct {
	Probe     string
	Timestamp time.Time
}

// parseLogEntry parses one line in Kubelet log.
func parseLogEntry(line []byte, containerNrPerPod map[string]int) *DetectedEntry {
	for probe, regex := range regexMap {
		if regex.Match(line) {
			matchResult := regex.FindSubmatch(line)
			if matchResult != nil {
				ts, err := time.Parse(kubeletLogTimeFormat, currentYear+" "+string(matchResult[1]))
				if err != nil {
					log.Fatal("Error: can not parse log timestamp in kubelet.log")
				}
				if probe == "kubelet_container_start" {
					pod := string(matchResult[2])
					nr, ok := containerNrPerPod[pod]
					if !ok {
						containerNrPerPod[pod] = 1
					} else {
						containerNrPerPod[pod] = nr + 1
					}
					probe += fmt.Sprintf("_%d", containerNrPerPod[pod])
				}
				return &DetectedEntry{Probe: probe, Timestamp: ts}
			}
		}
	}
	return nil
}

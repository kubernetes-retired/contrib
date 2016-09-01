package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"regexp"
	"sort"
	"time"
)

// This parser does not require additional probes for kubelet tracing. It is currently used by node perf dash.
// TODO(coufon): We plan to adopt event for tracing in future.

const (
	// TODO(coufon): use constants defined in Kubernetes packages.
	tracingVersion = "v1"
	// TODO(coufon): We need a string of year because year is missing in log timestamp.
	// Hardcoding the year here is a simple temporary solution.
	currentYear = "2016"

	// Timestamp format of test result log (build-log.txt)
	testLogTimeFormat = "2006 Jan 2 15:04:05.000"
	// Timestamp format of kubelet log (kubelet.log)
	kubeletLogTimeFormat = "2006 0102 15:04:05.000000"

	// Probe names
	probeFirstseen      = "kubelet_pod_config_change"
	probeRuntime        = "kubelet_runtime"
	probeContainerStart = "kubelet_container_start"
	probeStatusUpdate   = "kubelet_pod_status_running"
	// time when the infra container for the test pod starts
	probeInfraContainer = "kubelet_start_infra_container"
	// time when the test pod starts
	probeTestContainer = "kubelet_start_test_pod"
)

var (
	// Kubelet parser do not leverage on additional tracing probes. It uses the native log of Kubelet instead.
	// The mapping from native log to tracing probes is as follows:
	// TODO(coufon): there is no volume now in our node-e2e performance test. Add probe for volume manager in future.
	//          probe                                      log
	//     kubelet_firstseen           SyncLoop (ADD, "api")
	//     kubelet_runtime             docker_manager.go.*restart pod infra container
	//     kubelet_container_start     kubelet.go.*SyncLoop (PLEG):.*Type:"ContainerStarted"
	//     kubelet_status              status_manager.go.*updated successfully: {status:{Phase:Running
	regexMap = map[string]*regexp.Regexp{
		probeFirstseen:      regexp.MustCompile(`[IW](\d{2}\d{2} \d{2}:\d{2}:\d{2}.\d{6}).*kubelet.go.*SyncLoop \(ADD, \"api\"\): \".+\"`),
		probeRuntime:        regexp.MustCompile(`[IW](\d{2}\d{2} \d{2}:\d{2}:\d{2}.\d{6}).*docker_manager.go.*Need to restart pod infra container for.*because it is not found.*`),
		probeContainerStart: regexp.MustCompile(`[IW](\d{2}\d{2} \d{2}:\d{2}:\d{2}.\d{6}).*kubelet.go.*SyncLoop \(PLEG\): \".*\((.*)\)\".*Type:"ContainerStarted", Data:"(.*)".*`),
		probeStatusUpdate:   regexp.MustCompile(`[IW](\d{2}\d{2} \d{2}:\d{2}:\d{2}.\d{6}).*status_manager.go.*Status for pod \".*\((.*)\)\" updated successfully.*Phase:Running.*`),
	}
	// We do not process logs for cAdvisor pod. Use this regex to filter them out.
	regexMapCadvisorLog = regexp.MustCompile(`.*cadvisor.*`)
)

// TestTimeRange contains test name and its end time.
type TestTimeRange struct {
	TestName string
	EndTime  time.Time
}

// SortedTestTime is a sorted list of TestTimeRange.
type SortedTestTime []TestTimeRange

func (a SortedTestTime) Len() int           { return len(a) }
func (a SortedTestTime) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a SortedTestTime) Less(i, j int) bool { return a[i].EndTime.Before(a[j].EndTime) }

// SortedTestTimePerNode records SortedTestTime for each node.
type SortedTestTimePerNode map[string](SortedTestTime)

// TestTime records the end time for one test on one node.
type TestTime map[string](map[string]time.Time)

// Add adds one end time into TestTime
func (ete TestTime) Add(testName, nodeName, Endtime string) {
	if _, ok := ete[nodeName]; !ok {
		ete[nodeName] = make(map[string]time.Time)
	}
	if _, ok := ete[nodeName][testName]; !ok {
		end, err := time.Parse(testLogTimeFormat, currentYear+" "+Endtime)
		if err != nil {
			log.Fatal(err)
		}
		ete[nodeName][testName] = end
	}
}

// Sort sorts all end time of tests for each node and return a map of sorted array.
func (ete TestTime) Sort() SortedTestTimePerNode {
	sortedTestTimePerNode := make(SortedTestTimePerNode)
	for nodeName, testTimeMap := range ete {
		sortedTestTime := SortedTestTime{}
		for testName, endTime := range testTimeMap {
			sortedTestTime = append(sortedTestTime,
				TestTimeRange{
					TestName: testName,
					EndTime:  endTime,
				})
		}
		sort.Sort(sortedTestTime)
		sortedTestTimePerNode[nodeName] = sortedTestTime
	}
	return sortedTestTimePerNode
}

// TracingData contains the tracing time series data of a test on a node.
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

// ParseKubeletLog calls GrabTracingKubelet to parse tracing data while using test end time from build-log.txt to separate tests.
// It returns the parsed tracing data as a string in time series data format.
func ParseKubeletLog(d Downloader, job string, buildNumber int, testTime TestTime) string {
	sortedTestTimePerNode := testTime.Sort()
	result := ""
	for nodeName, sortedTestTime := range sortedTestTimePerNode {
		result += GrabTracingKubelet(d, job, buildNumber,
			nodeName, sortedTestTime)
	}
	return result
}

// PodState records the state of a pod from parsed kubelet log. The state is used in parsing.
type PodState struct {
	ContainerNr   int
	StatusUpdated bool
}

// GrabTracingKubelet parse tracing data using kubelet.log. It does not reuqire additional tracing probes.
func GrabTracingKubelet(d Downloader, job string, buildNumber int, nodeName string,
	sortedTestTime SortedTestTime) string {
	// Return empty string if there is no test in list
	if len(sortedTestTime) == 0 {
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
	testStarted := false

	tracingData := TracingData{
		Labels: map[string]string{
			"test": sortedTestTime[currentTestIndex].TestName,
			"node": nodeName,
		},
		Version: tracingVersion,
		Data:    map[string]int64arr{},
	}
	statePerPod := map[string]*PodState{}

	for scanner.Scan() {
		line := scanner.Text()
		if regexMapCadvisorLog.Match([]byte(line)) {
			continue
		}
		// Found a tracing event in kubelet log
		detectedEntry := parseLogEntry([]byte(line), statePerPod)
		if detectedEntry != nil {
			// Detect the log timestamp is out of test time range
			if sortedTestTime[currentTestIndex].EndTime.Before(detectedEntry.Timestamp) {
				currentTestIndex++
				if currentTestIndex >= len(sortedTestTime) {
					break
				}
				tracingData.SortData()
				result += timeSeriesTag + tracingData.ToSeriesData() + "\n\n" +
					timeSeriesEnd + "\n"
				// Move on to the next Test
				tracingData = TracingData{
					Labels: map[string]string{
						"test": sortedTestTime[currentTestIndex].TestName,
						"node": nodeName,
					},
					Version: tracingVersion,
					Data:    map[string]int64arr{},
				}
				statePerPod = map[string]*PodState{}
				testStarted = false
			}
			if detectedEntry.Probe == probeFirstseen {
				testStarted = true
			}
			if testStarted == false {
				continue
			}
			tracingData.AppendData(detectedEntry.Probe, detectedEntry.Timestamp.UnixNano())
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	tracingData.SortData()
	return result + timeSeriesTag + tracingData.ToSeriesData() + "\n\n" +
		timeSeriesEnd + "\n"
}

// DetectedEntry records a parsed probe and timestamp.
type DetectedEntry struct {
	Probe     string
	Timestamp time.Time
}

// parseLogEntry parses one line in Kubelet log.
func parseLogEntry(line []byte, statePerPod map[string]*PodState) *DetectedEntry {
	for probe, regex := range regexMap {
		if regex.Match(line) {
			matchResult := regex.FindSubmatch(line)
			if matchResult != nil {
				ts, err := time.Parse(kubeletLogTimeFormat, currentYear+" "+string(matchResult[1]))
				if err != nil {
					log.Fatal("Error: can not parse log timestamp in kubelet.log")
				}
				if probe == probeContainerStart {
					pod := string(matchResult[2])
					if _, ok := statePerPod[pod]; !ok {
						statePerPod[pod] = &PodState{ContainerNr: 1}
					} else {
						statePerPod[pod].ContainerNr++
					}
					// In our test the pod contains an infra container and test container.
					switch statePerPod[pod].ContainerNr {
					case 1:
						probe = probeInfraContainer
					case 2:
						probe = probeTestContainer
					default:
						return nil
					}
				}
				// We only trace the first status update event.
				if probe == probeStatusUpdate {
					pod := string(matchResult[2])
					if _, ok := statePerPod[pod]; !ok {
						statePerPod[pod] = &PodState{}
					}
					if statePerPod[pod].StatusUpdated {
						return nil
					}
					statePerPod[pod].StatusUpdated = true
				}
				return &DetectedEntry{Probe: probe, Timestamp: ts}
			}
		}
	}
	return nil
}

// AppendData adds a new timestamp of a probe into tracing data.
func (td *TracingData) AppendData(probe string, timestamp int64) {
	td.Data[probe] = append(td.Data[probe], timestamp)
}

// SortData have all time series data sorted
func (td *TracingData) SortData() {
	for _, arr := range td.Data {
		sort.Sort(arr)
	}
}

// ToSeriesData returns stringified tracing data in JSON
func (td *TracingData) ToSeriesData() string {
	seriesData, err := json.Marshal(td)
	if err != nil {
		log.Fatal(err)
	}
	return string(seriesData)
}

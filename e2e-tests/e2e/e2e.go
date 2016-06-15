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

package e2e

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	cache "k8s.io/contrib/e2e-tests/flakesync"
	"k8s.io/contrib/test-utils/utils"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/golang/glog"
	"io/ioutil"
)

type testStatus string

const (
	noData          testStatus = "Unable to retrieve data"
	manualOverride  testStatus = "Manual override"
	notStable       testStatus = "Not stable"
	stableStatus    testStatus = "Stable"
	ignoreableFlake testStatus = "Ignorable flake"
)

// E2ETester can be queried for E2E job stability.
type E2ETester interface {
	UpdateTests()
	AllowMerge() bool
	GetBlockingTestStatus() map[string]TestInfo
	GetNonBlockingTestStatus() map[string]TestInfo
	Flakes() cache.Flakes
}

// TestInfo tells the build ID and the build success
type TestInfo struct {
	Status testStatus
	ID     string
}

// RealE2ETester is the object which will get status from a google bucket
// information about recent jobs
type RealE2ETester struct {
	BlockingJobNames     []string
	NonBlockingJobNames  []string
	WeakBlockingJobNames []string

	sync.Mutex
	blockingBuildStatus    map[string]TestInfo // protect by mutex
	nonBlockingBuildStatus map[string]TestInfo // protect by mutex

	GoogleGCSBucketUtils *utils.Utils

	flakeCache        *cache.Cache
	resolutionTracker *ResolutionTracker
}

// HTTPHandlerInstaller is anything that can hook up HTTP requests to handlers.
// Used for installing admin functions.
type HTTPHandlerInstaller interface {
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
}

// Init does construction-- call once it after setting the public fields of 'e'.
// adminMux may be nil, in which case handlers for the resolution tracker won't
// be installed.
func (e *RealE2ETester) Init(adminMux HTTPHandlerInstaller) *RealE2ETester {
	e.blockingBuildStatus = map[string]TestInfo{}
	e.nonBlockingBuildStatus = map[string]TestInfo{}
	e.flakeCache = cache.NewCache(e.getGCSResult)
	e.resolutionTracker = NewResolutionTracker()
	if adminMux != nil {
		adminMux.HandleFunc("/api/mark-resolved", e.resolutionTracker.SetHTTP)
		adminMux.HandleFunc("/api/is-resolved", e.resolutionTracker.GetHTTP)
		adminMux.HandleFunc("/api/list-resolutions", e.resolutionTracker.ListHTTP)
	}
	return e
}

// AllowMerge tells if all of the 'blocking' builds are stable, manually allowed,
// or for whatever reason merges should be allowed
func (e *RealE2ETester) AllowMerge() bool {
	builds := e.GetBlockingTestStatus()
	for _, testInfo := range builds {
		switch testInfo.Status {
		case manualOverride:
		case stableStatus:
		case ignoreableFlake:
		case noData: // TODO: Should noData really allow merges?
		default:
			return false
		}
	}
	return true
}

// GetBlockingTestStatus returns the build status. This map is a copy and is thus safe
// for the caller to use in any way.
func (e *RealE2ETester) GetBlockingTestStatus() map[string]TestInfo {
	e.Lock()
	defer e.Unlock()
	out := map[string]TestInfo{}
	for k, v := range e.blockingBuildStatus {
		out[k] = v
	}
	return out
}

// GetNonBlockingTestStatus returns the build status. This map is a copy and is thus safe
// for the caller to use in any way.
func (e *RealE2ETester) GetNonBlockingTestStatus() map[string]TestInfo {
	e.Lock()
	defer e.Unlock()
	out := map[string]TestInfo{}
	for k, v := range e.nonBlockingBuildStatus {
		out[k] = v
	}
	return out
}

// Flakes returns a sorted list of current flakes.
func (e *RealE2ETester) Flakes() cache.Flakes {
	return e.flakeCache.Flakes()
}

func (e *RealE2ETester) setBuildStatus(build string, status testStatus, id string, blocking bool) {
	e.Lock()
	defer e.Unlock()
	bi := TestInfo{
		Status: status,
		ID:     id,
	}
	if blocking {
		e.blockingBuildStatus[build] = bi
	} else {
		e.nonBlockingBuildStatus[build] = bi
	}
}

const (
	// ExpectedXMLHeader is the expected header of junit_XX.xml file
	ExpectedXMLHeader = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>"
)

// GetBuildResult returns (or gets) the cached result of the job and build. Public.
func (e *RealE2ETester) GetBuildResult(job string, number int) (*cache.Result, error) {
	return e.flakeCache.Get(cache.Job(job), cache.Number(number))
}

func (e *RealE2ETester) getGCSResult(j cache.Job, n cache.Number) (*cache.Result, error) {
	stable, err := e.GoogleGCSBucketUtils.CheckFinishedStatus(string(j), int(n))
	if err != nil {
		glog.V(4).Infof("Error looking up job: %v, build number: %v", j, n)
		// Not actually fatal!
	}
	r := &cache.Result{
		Job:    j,
		Number: n,
		// TODO: StartTime:
	}
	if stable {
		r.Status = cache.ResultStable
		return r, nil
	}

	// This isn't stable-- see if we can find a reason.
	thisFailures, err := e.failureReasons(string(j), int(n), true)
	if err != nil {
		glog.V(4).Infof("Error looking up job failure reasons: %v, build number: %v: %v", j, n, err)
		thisFailures = nil // ensure we fall through
	}
	if len(thisFailures) == 0 {
		r.Status = cache.ResultFailed
		// We add a "flake" just to make sure this appears in the flake
		// cache as something that needs to be synced.
		r.Flakes = map[cache.Test]string{
			cache.RunBrokenTestName: "Unable to get data-- please look at the logs",
		}
		return r, nil
	}

	r.Flakes = map[cache.Test]string{}
	for testName, reason := range thisFailures {
		r.Flakes[cache.Test(testName)] = reason
	}

	r.Status = cache.ResultFlaky
	return r, nil
}

func (e *RealE2ETester) UpdateTests() {
	_, _ = e.Stable()
	_ = e.WeakStable()
}

// Stable is a version of Stable function that depends on files stored in GCS instead of Jenkis
func (e *RealE2ETester) Stable() (allStable, ignorableFlakes bool) {
	allStable = true

	for _, job := range e.BlockingJobNames {
		lastBuildNumber, err := e.GoogleGCSBucketUtils.GetLastestBuildNumberFromJenkinsGoogleBucket(job)
		glog.V(4).Infof("Checking status of %v, %v", job, lastBuildNumber)
		if err != nil {
			glog.Errorf("Error while getting data for %v: %v", job, err)
			e.setBuildStatus(job, noData, strconv.Itoa(lastBuildNumber), true)
			continue
		}

		if e.resolutionTracker.Resolved(cache.Job(job), cache.Number(lastBuildNumber)) {
			e.setBuildStatus(job, manualOverride, strconv.Itoa(lastBuildNumber), true)
			continue
		}

		thisResult, err := e.GetBuildResult(job, lastBuildNumber)
		if err != nil || thisResult.Status == cache.ResultFailed {
			glog.V(4).Infof("Found unstable job: %v, build number: %v: (err: %v) %#v", job, lastBuildNumber, err, thisResult)
			e.setBuildStatus(job, notStable, strconv.Itoa(lastBuildNumber), true)
			allStable = false
			continue
		}

		if thisResult.Status == cache.ResultStable {
			e.setBuildStatus(job, stableStatus, strconv.Itoa(lastBuildNumber), true)
			continue
		}

		lastResult, err := e.GetBuildResult(job, lastBuildNumber-1)
		if err != nil || lastResult.Status == cache.ResultFailed {
			glog.V(4).Infof("prev job doesn't help: %v, build number: %v (the previous build); (err %v) %#v", job, lastBuildNumber-1, err, lastResult)
			allStable = false
			e.setBuildStatus(job, notStable, strconv.Itoa(lastBuildNumber), true)
			continue
		}

		if lastResult.Status == cache.ResultStable {
			ignorableFlakes = true
			e.setBuildStatus(job, ignoreableFlake, strconv.Itoa(lastBuildNumber), true)
			continue
		}

		intersection := sets.NewString()
		for testName := range thisResult.Flakes {
			if _, ok := lastResult.Flakes[testName]; ok {
				intersection.Insert(string(testName))
			}
		}
		if len(intersection) == 0 {
			glog.V(2).Infof("Ignoring failure of %v/%v since it didn't happen the previous run this run = %v; prev run = %v.", job, lastBuildNumber, thisResult.Flakes, lastResult.Flakes)
			ignorableFlakes = true
			e.setBuildStatus(job, ignoreableFlake, strconv.Itoa(lastBuildNumber), true)
			continue
		}
		glog.V(2).Infof("Failure of %v/%v is legit. Tests that failed multiple times in a row: %v", job, lastBuildNumber, intersection)
		allStable = false
		e.setBuildStatus(job, notStable, strconv.Itoa(lastBuildNumber), true)
	}

	// Also get status for non-blocking jobs
	for _, job := range e.NonBlockingJobNames {
		lastBuildNumber, err := e.GoogleGCSBucketUtils.GetLastestBuildNumberFromJenkinsGoogleBucket(job)
		glog.V(4).Infof("Checking status of %v, %v", job, lastBuildNumber)
		if err != nil {
			glog.Errorf("Error while getting data for %v: %v", job, err)
			e.setBuildStatus(job, notStable, strconv.Itoa(lastBuildNumber), false)
			continue
		}

		if thisResult, err := e.GetBuildResult(job, lastBuildNumber); err != nil || thisResult.Status != cache.ResultStable {
			e.setBuildStatus(job, notStable, strconv.Itoa(lastBuildNumber), false)
		} else {
			e.setBuildStatus(job, stableStatus, strconv.Itoa(lastBuildNumber), false)
		}
	}

	return allStable, ignorableFlakes
}

func getJUnitFailures(r io.Reader) (failures map[string]string, err error) {
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
	type Testsuites struct {
		TestSuites []Testsuite `xml:"testsuite"`
	}
	var testSuiteList []Testsuite
	failures = map[string]string{}
	testSuites := &Testsuites{}
	testSuite := &Testsuite{}
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return failures, err
	}
	// first try to parse the result with <testsuites> as top tag
	err = xml.Unmarshal(b, testSuites)
	if err == nil && len(testSuites.TestSuites) > 0 {
		testSuiteList = testSuites.TestSuites
	} else {
		// second try to parse the result with <testsuite> as top tag
		err = xml.Unmarshal(b, testSuite)
		if err != nil {
			return nil, err
		}
		testSuiteList = []Testsuite{*testSuite}
	}
	for _, ts := range testSuiteList {
		if ts.FailCount == 0 {
			continue
		}
		for _, tc := range ts.Testcases {
			if tc.Failure != "" {
				failures[fmt.Sprintf("%v {%v}", tc.Name, tc.ClassName)] = tc.Failure
			}
		}
	}
	return failures, nil
}

// If completeList is true, collect every failure reason. Otherwise exit as soon as you see any failure.
func (e *RealE2ETester) failureReasons(job string, buildNumber int, completeList bool) (failedTests map[string]string, err error) {
	failuresFromResp := func(resp *http.Response) (failures map[string]string, err error) {
		defer resp.Body.Close()
		return getJUnitFailures(resp.Body)
	}
	failedTests = map[string]string{}

	// junit file prefix
	prefix := "artifacts/junit"
	junitList, err := e.GoogleGCSBucketUtils.ListFilesInBuild(job, buildNumber, prefix)
	if err != nil {
		glog.Errorf("Failed to list junit files for %v/%v/%v: %v", job, buildNumber, prefix, err)
	}

	// If we're here it means that build failed, so we need to look for a reason
	// by iterating over junit*.xml files and look for failures
	for _, filePath := range junitList {
		// if do not need complete list and we already have failed tests, then return
		if !completeList && len(failedTests) > 0 {
			break
		}
		if !strings.HasSuffix(filePath, ".xml") {
			continue
		}
		split := strings.Split(filePath, "/")
		junitFilePath := fmt.Sprintf("artifacts/%s", split[len(split)-1])
		response, err := e.GoogleGCSBucketUtils.GetFileFromJenkinsGoogleBucket(job, buildNumber, junitFilePath)
		if err != nil {
			return nil, fmt.Errorf("error while getting data for %v/%v/%v: %v", job, buildNumber, junitFilePath, err)
		}
		if response.StatusCode != http.StatusOK {
			response.Body.Close()
			break
		}
		failures, err := failuresFromResp(response) // closes response.Body for us
		if err != nil {
			return nil, fmt.Errorf("failed to read the response for %v/%v/%v: %v", job, buildNumber, junitFilePath, err)
		}
		for k, v := range failures {
			failedTests[k] = v
		}
	}

	return failedTests, nil
}

// WeakStable is a version of Stable with a slightly relaxed condition.
// This function says that e2e's are unstable only if there were real test failures
// (i.e. there was a test that failed, so no timeouts/cluster startup failures counts),
// or test failed for any reason 3 times in a row.
func (e *RealE2ETester) WeakStable() bool {
	allStable := true
	for _, job := range e.WeakBlockingJobNames {
		lastBuildNumber, err := e.GoogleGCSBucketUtils.GetLastestBuildNumberFromJenkinsGoogleBucket(job)
		glog.V(4).Infof("Checking status of %v, %v", job, lastBuildNumber)
		if err != nil {
			glog.Errorf("Error while getting data for %v: %v", job, err)
			e.setBuildStatus(job, notStable, strconv.Itoa(lastBuildNumber), true)
			continue
		}
		if stable, err := e.GoogleGCSBucketUtils.CheckFinishedStatus(job, lastBuildNumber); stable && err == nil {
			e.setBuildStatus(job, stableStatus, strconv.Itoa(lastBuildNumber), true)
			continue
		}

		if e.resolutionTracker.Resolved(cache.Job(job), cache.Number(lastBuildNumber)) {
			e.setBuildStatus(job, manualOverride, strconv.Itoa(lastBuildNumber), true)
			continue
		}

		failures, err := e.failureReasons(job, lastBuildNumber, false)
		if err != nil {
			glog.Errorf("Error while getting data for %v/%v: %v", job, lastBuildNumber, err)
			e.setBuildStatus(job, notStable, strconv.Itoa(lastBuildNumber), true)
			continue
		}

		thisStable := len(failures) == 0

		if thisStable == false {
			allStable = false
			e.setBuildStatus(job, notStable, strconv.Itoa(lastBuildNumber), true)
			glog.Infof("WeakStable failed because found a failure in JUnit file for build %v; %v and possibly more failed", lastBuildNumber, failures)
			continue
		}

		// If we're here it means that we weren't able to find a test that failed, which means that the reason of build failure is comming from the infrastructure
		// Check results of previous two builds.
		unstable := make([]int, 0)
		if stable, err := e.GoogleGCSBucketUtils.CheckFinishedStatus(job, lastBuildNumber-1); !stable || err != nil {
			unstable = append(unstable, lastBuildNumber-1)
		}
		if stable, err := e.GoogleGCSBucketUtils.CheckFinishedStatus(job, lastBuildNumber-2); !stable || err != nil {
			unstable = append(unstable, lastBuildNumber-2)
		}
		if len(unstable) > 1 {
			e.setBuildStatus(job, notStable, strconv.Itoa(lastBuildNumber), true)
			allStable = false
			glog.Infof("WeakStable failed because found a weak failure in build %v and builds %v failed.", lastBuildNumber, unstable)
			continue
		}
		e.setBuildStatus(job, stableStatus, strconv.Itoa(lastBuildNumber), true)
	}
	return allStable
}

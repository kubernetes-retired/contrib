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
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"testing"

	"k8s.io/contrib/mungegithub/mungers/jenkins"
	"k8s.io/contrib/test-utils/utils"
)

type testHandler struct {
	handler func(http.ResponseWriter, *http.Request)
}

func (t *testHandler) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	t.handler(res, req)
}

func marshalOrDie(obj interface{}, t *testing.T) []byte {
	data, err := json.Marshal(obj)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	return data
}

func TestCheckJenkinsBuilds(t *testing.T) {
	tests := []struct {
		paths          map[string][]byte
		expectStable   bool
		expectedStatus map[string]BuildInfo
	}{
		{
			paths: map[string][]byte{
				"/job/foo/lastCompletedBuild/api/json": marshalOrDie(jenkins.Job{
					Result: "SUCCESS",
				}, t),
				"/job/bar/lastCompletedBuild/api/json": marshalOrDie(jenkins.Job{
					Result: "SUCCESS",
				}, t),
			},
			expectStable:   true,
			expectedStatus: map[string]BuildInfo{"foo": {"Stable", ""}, "bar": {"Stable", ""}},
		},
		{
			paths: map[string][]byte{
				"/job/foo/lastCompletedBuild/api/json": marshalOrDie(jenkins.Job{
					Result: "SUCCESS",
				}, t),
				"/job/bar/lastCompletedBuild/api/json": marshalOrDie(jenkins.Job{
					Result: "UNSTABLE",
				}, t),
			},
			expectStable:   false,
			expectedStatus: map[string]BuildInfo{"foo": {"Stable", ""}, "bar": {"Not Stable", ""}},
		},
		{
			paths: map[string][]byte{
				"/job/foo/lastCompletedBuild/api/json": marshalOrDie(jenkins.Job{
					Result: "SUCCESS",
				}, t),
				"/job/bar/lastCompletedBuild/api/json": marshalOrDie(jenkins.Job{
					Result: "FAILURE",
				}, t),
			},
			expectStable:   false,
			expectedStatus: map[string]BuildInfo{"foo": {"Stable", ""}, "bar": {"Not Stable", ""}},
		},
		{
			paths: map[string][]byte{
				"/job/foo/lastCompletedBuild/api/json": marshalOrDie(jenkins.Job{
					Result: "FAILURE",
				}, t),
				"/job/bar/lastCompletedBuild/api/json": marshalOrDie(jenkins.Job{
					Result: "SUCCESS",
				}, t),
			},
			expectStable:   false,
			expectedStatus: map[string]BuildInfo{"foo": {"Not Stable", ""}, "bar": {"Stable", ""}},
		},
	}
	for _, test := range tests {
		server := httptest.NewServer(&testHandler{
			handler: func(res http.ResponseWriter, req *http.Request) {
				data, found := test.paths[req.URL.Path]
				if !found {
					res.WriteHeader(http.StatusNotFound)
					fmt.Fprintf(res, "Unknown path: %s", req.URL.Path)
					return
				}
				res.WriteHeader(http.StatusOK)
				res.Write(data)
			},
		})
		e2e := &RealE2ETester{
			JenkinsHost: server.URL,
			JobNames: []string{
				"foo",
				"bar",
			},
			BuildStatus: map[string]BuildInfo{},
		}
		stable := e2e.Stable()
		if stable != test.expectStable {
			t.Errorf("expected: %v, saw: %v", test.expectStable, stable)
		}
		if !reflect.DeepEqual(test.expectedStatus, e2e.BuildStatus) {
			t.Errorf("expected: %v, saw: %v", test.expectedStatus, e2e.BuildStatus)
		}
	}
}

func TestCheckGCSBuilds(t *testing.T) {
	latestBuildNumberFoo := 42
	latestBuildNumberBar := 44
	tests := []struct {
		paths             map[string][]byte
		expectStable      bool
		expectedLastBuild int
		expectedStatus    map[string]BuildInfo
	}{
		{
			paths: map[string][]byte{
				"/foo/latest-build.txt": []byte(strconv.Itoa(latestBuildNumberFoo)),
				fmt.Sprintf("/foo/%v/finished.json", latestBuildNumberFoo): marshalOrDie(utils.FinishedFile{
					Result:    "SUCCESS",
					Timestamp: 1234,
				}, t),
				"/bar/latest-build.txt": []byte(strconv.Itoa(latestBuildNumberBar)),
				fmt.Sprintf("/bar/%v/finished.json", latestBuildNumberBar): marshalOrDie(utils.FinishedFile{
					Result:    "SUCCESS",
					Timestamp: 1234,
				}, t),
			},
			expectStable: true,
			expectedStatus: map[string]BuildInfo{
				"foo": {Status: "Stable", ID: "42"},
				"bar": {Status: "Stable", ID: "44"},
			},
		},
		{
			paths: map[string][]byte{
				"/foo/latest-build.txt": []byte(strconv.Itoa(latestBuildNumberFoo)),
				fmt.Sprintf("/foo/%v/finished.json", latestBuildNumberFoo): marshalOrDie(utils.FinishedFile{
					Result:    "SUCCESS",
					Timestamp: 1234,
				}, t),
				"/bar/latest-build.txt": []byte(strconv.Itoa(latestBuildNumberBar)),
				fmt.Sprintf("/bar/%v/finished.json", latestBuildNumberBar): marshalOrDie(utils.FinishedFile{
					Result:    "UNSTABLE",
					Timestamp: 1234,
				}, t),
			},
			expectStable: false,
			expectedStatus: map[string]BuildInfo{
				"foo": {Status: "Stable", ID: "42"},
				"bar": {Status: "Not Stable", ID: "44"},
			},
		},
		{
			paths: map[string][]byte{
				"/foo/latest-build.txt": []byte(strconv.Itoa(latestBuildNumberFoo)),
				fmt.Sprintf("/foo/%v/finished.json", latestBuildNumberFoo): marshalOrDie(utils.FinishedFile{
					Result:    "SUCCESS",
					Timestamp: 1234,
				}, t),
				"/bar/latest-build.txt": []byte(strconv.Itoa(latestBuildNumberBar)),
				fmt.Sprintf("/bar/%v/finished.json", latestBuildNumberBar): marshalOrDie(utils.FinishedFile{
					Result:    "UNSTABLE",
					Timestamp: 1234,
				}, t),
				fmt.Sprintf("/bar/%v/artifacts/junit_01.xml", latestBuildNumberBar-1): getJUnit(5, 0),
				fmt.Sprintf("/bar/%v/artifacts/junit_02.xml", latestBuildNumberBar-1): getJUnit(5, 0),
				fmt.Sprintf("/bar/%v/artifacts/junit_03.xml", latestBuildNumberBar-1): getJUnit(5, 0),
				fmt.Sprintf("/bar/%v/artifacts/junit_01.xml", latestBuildNumberBar):   getJUnit(5, 0),
				fmt.Sprintf("/bar/%v/artifacts/junit_02.xml", latestBuildNumberBar):   getRealJUnitFailure(),
				fmt.Sprintf("/bar/%v/artifacts/junit_03.xml", latestBuildNumberBar):   getJUnit(5, 0),
				fmt.Sprintf("/bar/%v/finished.json", latestBuildNumberBar-1): marshalOrDie(utils.FinishedFile{
					Result:    "STABLE",
					Timestamp: 999,
				}, t),
			},
			expectStable: true,
			expectedStatus: map[string]BuildInfo{
				"foo": {Status: "Stable", ID: "42"},
				"bar": {Status: "Ignorable flake", ID: "44"},
			},
		},
		{
			paths: map[string][]byte{
				"/foo/latest-build.txt": []byte(strconv.Itoa(latestBuildNumberFoo)),
				fmt.Sprintf("/foo/%v/finished.json", latestBuildNumberFoo): marshalOrDie(utils.FinishedFile{
					Result:    "SUCCESS",
					Timestamp: 1234,
				}, t),
				"/bar/latest-build.txt": []byte(strconv.Itoa(latestBuildNumberBar)),
				fmt.Sprintf("/bar/%v/finished.json", latestBuildNumberBar): marshalOrDie(utils.FinishedFile{
					Result:    "UNSTABLE",
					Timestamp: 1234,
				}, t),
				fmt.Sprintf("/bar/%v/artifacts/junit_01.xml", latestBuildNumberBar-1): getJUnit(5, 0),
				fmt.Sprintf("/bar/%v/artifacts/junit_02.xml", latestBuildNumberBar-1): getRealJUnitFailure(),
				fmt.Sprintf("/bar/%v/artifacts/junit_03.xml", latestBuildNumberBar-1): getJUnit(5, 0),
				fmt.Sprintf("/bar/%v/artifacts/junit_01.xml", latestBuildNumberBar):   getJUnit(5, 0),
				fmt.Sprintf("/bar/%v/artifacts/junit_02.xml", latestBuildNumberBar):   getRealJUnitFailure(),
				fmt.Sprintf("/bar/%v/artifacts/junit_03.xml", latestBuildNumberBar):   getJUnit(5, 0),
				fmt.Sprintf("/bar/%v/finished.json", latestBuildNumberBar-1): marshalOrDie(utils.FinishedFile{
					Result:    "UNSTABLE",
					Timestamp: 999,
				}, t),
			},
			expectStable: false,
			expectedStatus: map[string]BuildInfo{
				"foo": {Status: "Stable", ID: "42"},
				"bar": {Status: "Not Stable", ID: "44"},
			},
		},

		{
			paths: map[string][]byte{
				"/foo/latest-build.txt": []byte(strconv.Itoa(latestBuildNumberFoo)),
				fmt.Sprintf("/foo/%v/finished.json", latestBuildNumberFoo): marshalOrDie(utils.FinishedFile{
					Result:    "SUCCESS",
					Timestamp: 1234,
				}, t),
				"/bar/latest-build.txt": []byte(strconv.Itoa(latestBuildNumberBar)),
				fmt.Sprintf("/bar/%v/finished.json", latestBuildNumberBar): marshalOrDie(utils.FinishedFile{
					Result:    "FAILURE",
					Timestamp: 1234,
				}, t),
			},
			expectStable: false,
			expectedStatus: map[string]BuildInfo{
				"foo": {Status: "Stable", ID: "42"},
				"bar": {Status: "Not Stable", ID: "44"},
			},
		},
		{
			paths: map[string][]byte{
				"/foo/latest-build.txt": []byte(strconv.Itoa(latestBuildNumberFoo)),
				fmt.Sprintf("/foo/%v/finished.json", latestBuildNumberFoo): marshalOrDie(utils.FinishedFile{
					Result:    "FAILURE",
					Timestamp: 1234,
				}, t),
				"/bar/latest-build.txt": []byte(strconv.Itoa(latestBuildNumberBar)),
				fmt.Sprintf("/bar/%v/finished.json", latestBuildNumberBar): marshalOrDie(utils.FinishedFile{
					Result:    "UNSTABLE",
					Timestamp: 1234,
				}, t),
			},
			expectStable: false,
			expectedStatus: map[string]BuildInfo{
				"foo": {Status: "Not Stable", ID: "42"},
				"bar": {Status: "Not Stable", ID: "44"},
			},
		},
		{
			paths: map[string][]byte{
				"/foo/latest-build.txt": []byte(strconv.Itoa(latestBuildNumberFoo)),
				fmt.Sprintf("/foo/%v/finished.json", latestBuildNumberFoo): marshalOrDie(utils.FinishedFile{
					Result:    "UNSTABLE",
					Timestamp: 1234,
				}, t),
				"/bar/latest-build.txt": []byte(strconv.Itoa(latestBuildNumberBar)),
				fmt.Sprintf("/bar/%v/finished.json", latestBuildNumberBar): marshalOrDie(utils.FinishedFile{
					Result:    "SUCCESS",
					Timestamp: 1234,
				}, t),
			},
			expectStable: false,
			expectedStatus: map[string]BuildInfo{
				"foo": {Status: "Not Stable", ID: "42"},
				"bar": {Status: "Stable", ID: "44"},
			},
		},
	}
	for _, test := range tests {
		server := httptest.NewServer(&testHandler{
			handler: func(res http.ResponseWriter, req *http.Request) {
				data, found := test.paths[req.URL.Path]
				if !found {
					res.WriteHeader(http.StatusNotFound)
					fmt.Fprintf(res, "Unknown path: %s", req.URL.Path)
					return
				}
				res.WriteHeader(http.StatusOK)
				res.Write(data)
			},
		})
		e2e := &RealE2ETester{
			JenkinsHost: server.URL,
			JobNames: []string{
				"foo",
				"bar",
			},
			BuildStatus:          map[string]BuildInfo{},
			GoogleGCSBucketUtils: utils.NewUtils(server.URL),
		}
		stable, _ := e2e.GCSBasedStable()
		if stable != test.expectStable {
			t.Errorf("expected: %v, saw: %v", test.expectStable, stable)
		}
		if !reflect.DeepEqual(test.expectedStatus, e2e.BuildStatus) {
			t.Errorf("expected: %v, saw: %v", test.expectedStatus, e2e.BuildStatus)
		}
	}
}

func getJUnit(testsNo int, failuresNo int) []byte {
	return []byte(fmt.Sprintf("%v\n<testsuite tests=\"%v\" failures=\"%v\" time=\"1234\">\n</testsuite>",
		ExpectedXMLHeader, testsNo, failuresNo))
}

func getRealJUnitFailure() []byte {
	return []byte(`<testsuite tests="7" failures="1" time="275.882258919">
<testcase name="[k8s.io] ResourceQuota should create a ResourceQuota and capture the life of a loadBalancer service." classname="Kubernetes e2e suite" time="17.759834805"/>
<testcase name="[k8s.io] ResourceQuota should create a ResourceQuota and capture the life of a secret." classname="Kubernetes e2e suite" time="21.201547548"/>
<testcase name="[k8s.io] Kubectl client [k8s.io] Kubectl patch should add annotations for pods in rc [Conformance]" classname="Kubernetes e2e suite" time="126.756441938">
<failure type="Failure">
/go/src/k8s.io/kubernetes/_output/dockerized/go/src/k8s.io/kubernetes/test/e2e/kubectl.go:972 May 18 13:02:24.715: No pods matched the filter.
</failure>
</testcase>
<testcase name="[k8s.io] hostPath should give a volume the correct mode [Conformance]" classname="Kubernetes e2e suite" time="9.246191421"/>
<testcase name="[k8s.io] Volumes [Feature:Volumes] [k8s.io] Ceph RBD should be mountable" classname="Kubernetes e2e suite" time="0">
<skipped/>
</testcase>
<testcase name="[k8s.io] Deployment deployment should label adopted RSs and pods" classname="Kubernetes e2e suite" time="16.557498555"/>
<testcase name="[k8s.io] ConfigMap should be consumable from pods in volume as non-root with FSGroup [Feature:FSGroup]" classname="Kubernetes e2e suite" time="0">
<skipped/>
</testcase>
<testcase name="[k8s.io] V1Job should scale a job down" classname="Kubernetes e2e suite" time="77.122626914"/>
<testcase name="[k8s.io] EmptyDir volumes volume on default medium should have the correct mode [Conformance]" classname="Kubernetes e2e suite" time="7.169679079"/>
<testcase name="[k8s.io] Reboot [Disruptive] [Feature:Reboot] each node by ordering unclean reboot and ensure they function upon restart" classname="Kubernetes e2e suite" time="0">
<skipped/>
</testcase>
</testsuite>`)
}

func TestCheckGCSWeakBuilds(t *testing.T) {
	latestBuildNumberFoo := 42
	latestBuildNumberBar := 44
	tests := []struct {
		paths             map[string][]byte
		expectStable      bool
		expectedLastBuild int
		expectedStatus    map[string]BuildInfo
	}{
		// Simple case - both succeeds
		{
			paths: map[string][]byte{
				"/foo/latest-build.txt": []byte(strconv.Itoa(latestBuildNumberFoo)),
				fmt.Sprintf("/foo/%v/finished.json", latestBuildNumberFoo): marshalOrDie(utils.FinishedFile{
					Result:    "SUCCESS",
					Timestamp: 1234,
				}, t),
				"/bar/latest-build.txt": []byte(strconv.Itoa(latestBuildNumberBar)),
				fmt.Sprintf("/bar/%v/finished.json", latestBuildNumberBar): marshalOrDie(utils.FinishedFile{
					Result:    "SUCCESS",
					Timestamp: 1234,
				}, t),
			},
			expectStable: true,
			expectedStatus: map[string]BuildInfo{
				"foo": {Status: "Stable", ID: "42"},
				"bar": {Status: "Stable", ID: "44"},
			},
		},
		// If last build was successful we shouldn't be looking any further
		{
			paths: map[string][]byte{
				"/foo/latest-build.txt": []byte(strconv.Itoa(latestBuildNumberFoo)),
				fmt.Sprintf("/foo/%v/finished.json", latestBuildNumberFoo): marshalOrDie(utils.FinishedFile{
					Result:    "SUCCESS",
					Timestamp: 1234,
				}, t),
				fmt.Sprintf("/foo/%v/finished.json", latestBuildNumberFoo-1): marshalOrDie(utils.FinishedFile{
					Result:    "UNSTABLE",
					Timestamp: 1234,
				}, t),
				"/bar/latest-build.txt": []byte(strconv.Itoa(latestBuildNumberBar)),
				fmt.Sprintf("/bar/%v/finished.json", latestBuildNumberBar): marshalOrDie(utils.FinishedFile{
					Result:    "SUCCESS",
					Timestamp: 1234,
				}, t),
				fmt.Sprintf("/bar/%v/finished.json", latestBuildNumberBar-1): marshalOrDie(utils.FinishedFile{
					Result:    "FAILURE",
					Timestamp: 1234,
				}, t),
			},
			expectStable: true,
			expectedStatus: map[string]BuildInfo{
				"foo": {Status: "Stable", ID: "42"},
				"bar": {Status: "Stable", ID: "44"},
			},
		},
		// If the last build was unsuccessful but there's no failures in JUnit file we assume that it was
		// an infrastructure failure. Build should succeed if at least one of two builds were fully successful.
		{
			paths: map[string][]byte{
				"/foo/latest-build.txt": []byte(strconv.Itoa(latestBuildNumberFoo)),
				fmt.Sprintf("/foo/%v/finished.json", latestBuildNumberFoo): marshalOrDie(utils.FinishedFile{
					Result:    "UNSTABLE",
					Timestamp: 1234,
				}, t),
				fmt.Sprintf("/foo/%v/artifacts/junit_01.xml", latestBuildNumberFoo): getJUnit(5, 0),
				fmt.Sprintf("/foo/%v/finished.json", latestBuildNumberFoo-1): marshalOrDie(utils.FinishedFile{
					Result:    "UNSTABLE",
					Timestamp: 1233,
				}, t),
				fmt.Sprintf("/foo/%v/finished.json", latestBuildNumberFoo-2): marshalOrDie(utils.FinishedFile{
					Result:    "SUCCESS",
					Timestamp: 1232,
				}, t),
				"/bar/latest-build.txt": []byte(strconv.Itoa(latestBuildNumberBar)),
				fmt.Sprintf("/bar/%v/finished.json", latestBuildNumberBar): marshalOrDie(utils.FinishedFile{
					Result:    "SUCCESS",
					Timestamp: 1234,
				}, t),
			},
			expectStable: true,
			expectedStatus: map[string]BuildInfo{
				"foo": {Status: "Stable", ID: "42"},
				"bar": {Status: "Stable", ID: "44"},
			},
		},
		// If the last build was unsuccessful but there's no failures in JUnit file we assume that it was
		// an infrastructure failure. Build should fail more than both recent builds failed.
		{
			paths: map[string][]byte{
				"/foo/latest-build.txt": []byte(strconv.Itoa(latestBuildNumberFoo)),
				fmt.Sprintf("/foo/%v/finished.json", latestBuildNumberFoo): marshalOrDie(utils.FinishedFile{
					Result:    "UNSTABLE",
					Timestamp: 1234,
				}, t),
				fmt.Sprintf("/foo/%v/artifacts/junit_01.xml", latestBuildNumberFoo): getJUnit(5, 0),
				fmt.Sprintf("/foo/%v/finished.json", latestBuildNumberFoo-1): marshalOrDie(utils.FinishedFile{
					Result:    "UNSTABLE",
					Timestamp: 1233,
				}, t),
				fmt.Sprintf("/foo/%v/finished.json", latestBuildNumberFoo-2): marshalOrDie(utils.FinishedFile{
					Result:    "UNSTABLE",
					Timestamp: 1232,
				}, t),
				"/bar/latest-build.txt": []byte(strconv.Itoa(latestBuildNumberBar)),
				fmt.Sprintf("/bar/%v/finished.json", latestBuildNumberBar): marshalOrDie(utils.FinishedFile{
					Result:    "SUCCESS",
					Timestamp: 1234,
				}, t),
			},
			expectStable: false,
			expectedStatus: map[string]BuildInfo{
				"foo": {Status: "Not Stable", ID: "42"},
				"bar": {Status: "Stable", ID: "44"},
			},
		},
		// If the last build was unsuccessful and there's a failed test in a JUnit file we should fail.
		{
			paths: map[string][]byte{
				"/foo/latest-build.txt": []byte(strconv.Itoa(latestBuildNumberFoo)),
				fmt.Sprintf("/foo/%v/finished.json", latestBuildNumberFoo): marshalOrDie(utils.FinishedFile{
					Result:    "UNSTABLE",
					Timestamp: 1234,
				}, t),
				fmt.Sprintf("/foo/%v/artifacts/junit_01.xml", latestBuildNumberFoo): getJUnit(5, 0),
				fmt.Sprintf("/foo/%v/artifacts/junit_02.xml", latestBuildNumberFoo): getJUnit(5, 1),
				fmt.Sprintf("/foo/%v/artifacts/junit_03.xml", latestBuildNumberFoo): getJUnit(5, 0),
				"/bar/latest-build.txt":                                             []byte(strconv.Itoa(latestBuildNumberBar)),
				fmt.Sprintf("/bar/%v/finished.json", latestBuildNumberBar): marshalOrDie(utils.FinishedFile{
					Result:    "SUCCESS",
					Timestamp: 1234,
				}, t),
			},
			expectStable: false,
			expectedStatus: map[string]BuildInfo{
				"foo": {Status: "Not Stable", ID: "42"},
				"bar": {Status: "Stable", ID: "44"},
			},
		},
		// Result shouldn't depend on order.
		{
			paths: map[string][]byte{
				"/foo/latest-build.txt": []byte(strconv.Itoa(latestBuildNumberFoo)),
				fmt.Sprintf("/foo/%v/finished.json", latestBuildNumberFoo): marshalOrDie(utils.FinishedFile{
					Result:    "SUCCESS",
					Timestamp: 1234,
				}, t),
				"/bar/latest-build.txt": []byte(strconv.Itoa(latestBuildNumberBar)),
				fmt.Sprintf("/bar/%v/finished.json", latestBuildNumberBar): marshalOrDie(utils.FinishedFile{
					Result:    "FAILURE",
					Timestamp: 1234,
				}, t),
				fmt.Sprintf("/bar/%v/artifacts/junit_01.xml", latestBuildNumberBar): getJUnit(5, 0),
				fmt.Sprintf("/bar/%v/artifacts/junit_02.xml", latestBuildNumberBar): getJUnit(5, 1),
				fmt.Sprintf("/bar/%v/artifacts/junit_03.xml", latestBuildNumberBar): getJUnit(5, 1),
			},
			expectStable: false,
			expectedStatus: map[string]BuildInfo{
				"foo": {Status: "Stable", ID: "42"},
				"bar": {Status: "Not Stable", ID: "44"},
			},
		},
	}
	for _, test := range tests {
		server := httptest.NewServer(&testHandler{
			handler: func(res http.ResponseWriter, req *http.Request) {
				data, found := test.paths[req.URL.Path]
				if !found {
					res.WriteHeader(http.StatusNotFound)
					fmt.Fprintf(res, "Unknown path: %s", req.URL.Path)
					return
				}
				res.WriteHeader(http.StatusOK)
				res.Write(data)
			},
		})
		e2e := &RealE2ETester{
			JenkinsHost: server.URL,
			WeakStableJobNames: []string{
				"foo",
				"bar",
			},
			BuildStatus:          map[string]BuildInfo{},
			GoogleGCSBucketUtils: utils.NewUtils(server.URL),
		}
		stable := e2e.GCSWeakStable()
		if stable != test.expectStable {
			t.Errorf("expected: %v, saw: %v", test.expectStable, stable)
		}
		if !reflect.DeepEqual(test.expectedStatus, e2e.BuildStatus) {
			t.Errorf("expected: %v, saw: %v", test.expectedStatus, e2e.BuildStatus)
		}
	}
}

func TestJUnitFailureParse(t *testing.T) {
	junitFailReader := bytes.NewReader(getRealJUnitFailure())
	got, err := getJUnitFailures(junitFailReader)
	if err != nil {
		t.Fatalf("Parse error? %v", err)
	}
	if e, a := []string{"[k8s.io] Kubectl client [k8s.io] Kubectl patch should add annotations for pods in rc [Conformance] {Kubernetes e2e suite}"}, got; !reflect.DeepEqual(e, a) {
		t.Errorf("Expected %v, got %v", e, a)
	}
}

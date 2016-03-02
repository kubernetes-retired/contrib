/*
Copyright 2016 The Kubernetes Authors All rights reserved.

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

package jenkins

import (
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/golang/glog"
)

// junitResults has the format of an JUnit XML output file
type junitTestSuites struct {
	TestSuites []*junitTestSuite `xml:"testsuite"`
}

type junitTestSuite struct {
	Tests    int     `xml:"tests,attr"`
	Failures int     `xml:"failures,attr"`
	Time     float32 `xml:"time,attr"`
	Name     string  `xml:"name,attr"`

	// TODO: Not sure about the marshalling of this one...
	Properties []*junitProperty `xml:"properties"`

	TestCases []*junitTestCase `xml:"testcase"`
}

type junitProperty struct {
	Name  string `xml:"name"`
	Value string `xml:"value"`
}

type junitTestCase struct {
	ClassName string            `xml:"classname,attr"`
	Name      string            `xml:"name,attr"`
	Time      float32           `xml:"time,attr"`
	skipped   *junitTestSkipped `xml:"skipped"`
	Failure   *junitTestFailure `xml:"failure"`
}

func (j *junitTestCase) Skipped() bool {
	return j.skipped != nil
}

type junitTestSkipped struct {
	Message string `xml:"message"`
}

type junitTestFailure struct {
	Type    string `xml:"type,attr"`
	Message string `xml:",chardata"`
}

type JUnitTestResult struct {
	Results []*junitTestSuite
}

func (j *JUnitTestResult) Failures() []*junitTestCase {
	var failures []*junitTestCase
	for _, suite := range j.Results {
		for _, testcase := range suite.TestCases {
			if testcase.Failure != nil {
				failures = append(failures, testcase)
			}
		}
	}
	return failures
}

func CombineJUnitTestResults(results map[string]*JUnitTestResult) *JUnitTestResult {
	var combined []*junitTestSuite

	for _, result := range results {
		combined = append(combined, result.Results...)
	}

	return &JUnitTestResult{Results: combined}
}

func ParseJUnitTestResult(data []byte) (*JUnitTestResult, error) {
	var suites []*junitTestSuite

	if strings.Contains(string(data), "<testsuites>") {
		o := &junitTestSuites{}
		err := xml.Unmarshal(data, o)
		if err != nil {
			glog.V(2).Infof("Failed to parse JUnit output: %s", string(data))
			return nil, fmt.Errorf("error parsing junit test results: %v", err)
		}

		suites = o.TestSuites
	} else {
		o := &junitTestSuite{}
		err := xml.Unmarshal(data, o)
		if err != nil {
			glog.V(2).Infof("Failed to parse JUnit output: %s", string(data))
			return nil, fmt.Errorf("error parsing junit test results: %v", err)
		}

		suites = append(suites, o)
	}
	return &JUnitTestResult{Results: suites}, nil
}

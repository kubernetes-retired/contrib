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
	"fmt"
	"strings"
	"time"

	"k8s.io/kubernetes/pkg/api"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/sets"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	// Interval to poll /stats/container on a node
	containerStatsPollingPeriod = 3 * time.Second
	// The monitoring time for one test.
	monitoringTime = 20 * time.Minute
	// The periodic reporting period.
	reportingPeriod = 5 * time.Minute
)

type resourceTest struct {
	podsPerNode int
	limits      containersCPUSummary
}

func logPodsOnNodes(c *client.Client, nodeNames []string) {
	for _, n := range nodeNames {
		podList, err := GetKubeletPods(c, n)
		if err != nil {
			Logf("Unable to retrieve kubelet pods for node %v", n)
			continue
		}
		Logf("%d pods are running on node %v", len(podList.Items), n)
	}
}

func runResourceTrackingTest(framework *Framework, podsPerNode int, nodeNames sets.String, rm *resourceMonitor, expected map[string]map[float64]float64) {
	numNodes := nodeNames.Len()
	totalPods := podsPerNode * numNodes
	By(fmt.Sprintf("Creating a RC of %d pods and wait until all pods of this RC are running", totalPods))
	rcName := fmt.Sprintf("resource%d-%s", totalPods, string(util.NewUUID()))

	// TODO: Use a more realistic workload
	Expect(RunRC(RCConfig{
		Client:    framework.Client,
		Name:      rcName,
		Namespace: framework.Namespace.Name,
		Image:     "gcr.io/google_containers/pause:2.0",
		Replicas:  totalPods,
	})).NotTo(HaveOccurred())

	// Log once and flush the stats.
	rm.LogLatest()
	rm.Reset()

	By("Start monitoring resource usage")
	// Periodically dump the cpu summary until the deadline is met.
	// Note that without calling resourceMonitor.Reset(), the stats
	// would occupy increasingly more memory. This should be fine
	// for the current test duration, but we should reclaim the
	// entries if we plan to monitor longer (e.g., 8 hours).
	deadline := time.Now().Add(monitoringTime)
	for time.Now().Before(deadline) {
		timeLeft := deadline.Sub(time.Now())
		Logf("Still running...%v left", timeLeft)
		if timeLeft < reportingPeriod {
			time.Sleep(timeLeft)
		} else {
			time.Sleep(reportingPeriod)
		}
		logPodsOnNodes(framework.Client, nodeNames.List())
	}

	By("Reporting overall resource usage")
	logPodsOnNodes(framework.Client, nodeNames.List())
	rm.LogLatest()

	summary := rm.GetCPUSummary()
	Logf("%s", rm.FormatCPUSummary(summary))
	verifyCPULimits(expected, summary)

	By("Deleting the RC")
	DeleteRC(framework.Client, framework.Namespace.Name, rcName)
}

func verifyCPULimits(expected containersCPUSummary, actual nodesCPUSummary) {
	if expected == nil {
		return
	}
	var errList []string
	for nodeName, perNodeSummary := range actual {
		var nodeErrs []string
		for cName, expectedResult := range expected {
			perContainerSummary, ok := perNodeSummary[cName]
			if !ok {
				nodeErrs = append(nodeErrs, fmt.Sprintf("container %q: missing", cName))
				continue
			}
			for p, expectedValue := range expectedResult {
				actualValue, ok := perContainerSummary[p]
				if !ok {
					nodeErrs = append(nodeErrs, fmt.Sprintf("container %q: missing percentile %v", cName, p))
					continue
				}
				if actualValue > expectedValue {
					nodeErrs = append(nodeErrs, fmt.Sprintf("container %q: expected %.0fth%% usage < %.3f; got %.3f",
						cName, p*100, expectedValue, actualValue))
				}
			}
		}
		if len(nodeErrs) > 0 {
			errList = append(errList, fmt.Sprintf("node %v:\n %s", nodeName, strings.Join(nodeErrs, ", ")))
		}
	}
	if len(errList) > 0 {
		Failf("CPU usage exceeding limits:\n %s", strings.Join(errList, "\n"))
	}
}

// Slow by design (1 hour)
var _ = Describe("Kubelet [Serial] [Slow]", func() {
	var nodeNames sets.String
	framework := NewFramework("kubelet-perf")
	var rm *resourceMonitor

	BeforeEach(func() {
		// It should be OK to list unschedulable Nodes here.
		nodes, err := framework.Client.Nodes().List(api.ListOptions{})
		expectNoError(err)
		nodeNames = sets.NewString()
		for _, node := range nodes.Items {
			nodeNames.Insert(node.Name)
		}
		rm = newResourceMonitor(framework.Client, targetContainers(), containerStatsPollingPeriod)
		rm.Start()
	})

	AfterEach(func() {
		rm.Stop()
	})
	Describe("regular resource usage tracking", func() {
		rTests := []resourceTest{
			{podsPerNode: 0,
				limits: containersCPUSummary{
					"/kubelet":       {0.50: 0.05, 0.95: 0.15},
					"/docker-daemon": {0.50: 0.03, 0.95: 0.06},
				},
			},
			{podsPerNode: 35,
				limits: containersCPUSummary{
					"/kubelet":       {0.50: 0.15, 0.95: 0.35},
					"/docker-daemon": {0.50: 0.06, 0.95: 0.30},
				},
			},
		}
		for _, testArg := range rTests {
			itArg := testArg
			podsPerNode := itArg.podsPerNode
			name := fmt.Sprintf(
				"for %d pods per node over %v", podsPerNode, monitoringTime)
			It(name, func() {
				runResourceTrackingTest(framework, podsPerNode, nodeNames, rm, itArg.limits)
			})
		}
	})
	Describe("experimental resource usage tracking", func() {
		density := []int{100}
		for i := range density {
			podsPerNode := density[i]
			name := fmt.Sprintf(
				"for %d pods per node over %v", podsPerNode, monitoringTime)
			It(name, func() {
				runResourceTrackingTest(framework, podsPerNode, nodeNames, rm, nil)
			})
		}
	})
})

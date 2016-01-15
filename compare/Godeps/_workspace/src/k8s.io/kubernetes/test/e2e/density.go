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
	"math"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/cache"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	controllerframework "k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/watch"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// NodeStartupThreshold is a rough estimate of the time allocated for a pod to start on a node.
const NodeStartupThreshold = 4 * time.Second

// Maximum container failures this test tolerates before failing.
var MaxContainerFailures = 0

// podLatencyData encapsulates pod startup latency information.
type podLatencyData struct {
	// Name of the pod
	Name string
	// Node this pod was running on
	Node string
	// Latency information related to pod startuptime
	Latency time.Duration
}

type latencySlice []podLatencyData

func (a latencySlice) Len() int           { return len(a) }
func (a latencySlice) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a latencySlice) Less(i, j int) bool { return a[i].Latency < a[j].Latency }

func extractLatencyMetrics(latencies []podLatencyData) LatencyMetric {
	length := len(latencies)
	perc50 := latencies[int(math.Ceil(float64(length*50)/100))-1].Latency
	perc90 := latencies[int(math.Ceil(float64(length*90)/100))-1].Latency
	perc99 := latencies[int(math.Ceil(float64(length*99)/100))-1].Latency
	return LatencyMetric{Perc50: perc50, Perc90: perc90, Perc99: perc99}
}

func density30AddonResourceVerifier() map[string]resourceConstraint {
	constraints := make(map[string]resourceConstraint)
	constraints["fluentd-elasticsearch"] = resourceConstraint{
		cpuConstraint:    0.1,
		memoryConstraint: 250 * (1024 * 1024),
	}
	constraints["elasticsearch-logging"] = resourceConstraint{
		cpuConstraint: 2,
		// TODO: bring it down to 750MB again, when we lower Kubelet verbosity level. I.e. revert #19164
		memoryConstraint: 5000 * (1024 * 1024),
	}
	constraints["heapster"] = resourceConstraint{
		cpuConstraint:    2,
		memoryConstraint: 1800 * (1024 * 1024),
	}
	constraints["kibana-logging"] = resourceConstraint{
		cpuConstraint:    0.05,
		memoryConstraint: 100 * (1024 * 1024),
	}
	constraints["kube-proxy"] = resourceConstraint{
		cpuConstraint:    0.05,
		memoryConstraint: 20 * (1024 * 1024),
	}
	constraints["l7-lb-controller"] = resourceConstraint{
		cpuConstraint:    0.05,
		memoryConstraint: 20 * (1024 * 1024),
	}
	constraints["influxdb"] = resourceConstraint{
		cpuConstraint:    2,
		memoryConstraint: 300 * (1024 * 1024),
	}
	return constraints
}

// This test suite can take a long time to run, and can affect or be affected by other tests.
// So by default it is added to the ginkgo.skip list (see driver.go).
// To run this suite you must explicitly ask for it by setting the
// -t/--test flag or ginkgo.focus flag.
var _ = Describe("Density [Skipped]", func() {
	var c *client.Client
	var nodeCount int
	var RCName string
	var additionalPodsPrefix string
	var ns string
	var uuid string

	// Gathers data prior to framework namespace teardown
	AfterEach(func() {
		// Verify latency metrics.
		highLatencyRequests, err := HighLatencyRequests(c)
		expectNoError(err)
		Expect(highLatencyRequests).NotTo(BeNumerically(">", 0), "There should be no high-latency requests")

		// Verify scheduler metrics.
		// TODO: Reset metrics at the beginning of the test.
		// We should do something similar to how we do it for APIserver.
		expectNoError(VerifySchedulerLatency(c))
	})

	// Explicitly put here, to delete namespace at the end of the test
	// (after measuring latency metrics, etc.).framework := NewFramework("density")
	framework := NewFramework("density")
	framework.NamespaceDeletionTimeout = time.Hour

	BeforeEach(func() {
		c = framework.Client
		ns = framework.Namespace.Name
		var err error

		nodes := ListSchedulableNodesOrDie(c)
		nodeCount = len(nodes.Items)
		Expect(nodeCount).NotTo(BeZero())

		// Terminating a namespace (deleting the remaining objects from it - which
		// generally means events) can affect the current run. Thus we wait for all
		// terminating namespace to be finally deleted before starting this test.
		err = checkTestingNSDeletedExcept(c, ns)
		expectNoError(err)

		uuid = string(util.NewUUID())

		expectNoError(resetMetrics(c))
		expectNoError(os.Mkdir(fmt.Sprintf(testContext.OutputDir+"/%s", uuid), 0777))

		Logf("Listing nodes for easy debugging:\n")
		for _, node := range nodes.Items {
			var internalIP, externalIP string
			for _, address := range node.Status.Addresses {
				if address.Type == api.NodeInternalIP {
					internalIP = address.Address
				}
				if address.Type == api.NodeExternalIP {
					externalIP = address.Address
				}
			}
			Logf("Name: %v, clusterIP: %v, externalIP: %v", node.ObjectMeta.Name, internalIP, externalIP)
		}
	})

	type Density struct {
		// Controls if e2e latency tests should be run (they are slow)
		runLatencyTest bool
		podsPerNode    int
		// Controls how often the apiserver is polled for pods
		interval time.Duration
	}

	densityTests := []Density{
		// TODO: Expose runLatencyTest as ginkgo flag.
		{podsPerNode: 3, runLatencyTest: false, interval: 10 * time.Second},
		{podsPerNode: 30, runLatencyTest: true, interval: 10 * time.Second},
		// More than 30 pods per node is outside our v1.0 goals.
		// We might want to enable those tests in the future.
		{podsPerNode: 50, runLatencyTest: false, interval: 10 * time.Second},
		{podsPerNode: 100, runLatencyTest: false, interval: 1 * time.Second},
	}

	for _, testArg := range densityTests {
		name := fmt.Sprintf("should allow starting %d pods per node", testArg.podsPerNode)
		if testArg.podsPerNode == 30 {
			name = "[Performance] " + name
			framework.addonResourceConstraints = density30AddonResourceVerifier()
		}
		itArg := testArg
		It(name, func() {
			totalPods := itArg.podsPerNode * nodeCount
			RCName = "density" + strconv.Itoa(totalPods) + "-" + uuid
			fileHndl, err := os.Create(fmt.Sprintf(testContext.OutputDir+"/%s/pod_states.csv", uuid))
			expectNoError(err)
			defer fileHndl.Close()
			config := RCConfig{Client: c,
				Image:                "gcr.io/google_containers/pause:2.0",
				Name:                 RCName,
				Namespace:            ns,
				PollInterval:         itArg.interval,
				PodStatusFile:        fileHndl,
				Replicas:             totalPods,
				MaxContainerFailures: &MaxContainerFailures,
			}

			// Create a listener for events.
			events := make([](*api.Event), 0)
			_, controller := controllerframework.NewInformer(
				&cache.ListWatch{
					ListFunc: func(options api.ListOptions) (runtime.Object, error) {
						return c.Events(ns).List(options)
					},
					WatchFunc: func(options api.ListOptions) (watch.Interface, error) {
						return c.Events(ns).Watch(options)
					},
				},
				&api.Event{},
				0,
				controllerframework.ResourceEventHandlerFuncs{
					AddFunc: func(obj interface{}) {
						events = append(events, obj.(*api.Event))
					},
				},
			)
			stop := make(chan struct{})
			go controller.Run(stop)

			// Start the replication controller.
			startTime := time.Now()
			expectNoError(RunRC(config))
			e2eStartupTime := time.Now().Sub(startTime)
			Logf("E2E startup time for %d pods: %v", totalPods, e2eStartupTime)

			By("Waiting for all events to be recorded")
			last := -1
			current := len(events)
			timeout := 10 * time.Minute
			for start := time.Now(); last < current && time.Since(start) < timeout; time.Sleep(10 * time.Second) {
				last = current
				current = len(events)
			}
			close(stop)

			if current != last {
				Logf("Warning: Not all events were recorded after waiting %.2f minutes", timeout.Minutes())
			}
			Logf("Found %d events", current)

			// Tune the threshold for allowed failures.
			badEvents := BadEvents(events)
			Expect(badEvents).NotTo(BeNumerically(">", int(math.Floor(0.01*float64(totalPods)))))

			if itArg.runLatencyTest {
				Logf("Schedling additional Pods to measure startup latencies")

				createTimes := make(map[string]unversioned.Time, 0)
				nodes := make(map[string]string, 0)
				scheduleTimes := make(map[string]unversioned.Time, 0)
				runTimes := make(map[string]unversioned.Time, 0)
				watchTimes := make(map[string]unversioned.Time, 0)

				var mutex sync.Mutex
				checkPod := func(p *api.Pod) {
					mutex.Lock()
					defer mutex.Unlock()
					defer GinkgoRecover()

					if p.Status.Phase == api.PodRunning {
						if _, found := watchTimes[p.Name]; !found {
							watchTimes[p.Name] = unversioned.Now()
							createTimes[p.Name] = p.CreationTimestamp
							nodes[p.Name] = p.Spec.NodeName
							var startTime unversioned.Time
							for _, cs := range p.Status.ContainerStatuses {
								if cs.State.Running != nil {
									if startTime.Before(cs.State.Running.StartedAt) {
										startTime = cs.State.Running.StartedAt
									}
								}
							}
							if startTime != unversioned.NewTime(time.Time{}) {
								runTimes[p.Name] = startTime
							} else {
								Failf("Pod %v is reported to be running, but none of its containers is", p.Name)
							}
						}
					}
				}

				additionalPodsPrefix = "density-latency-pod-" + string(util.NewUUID())
				_, controller := controllerframework.NewInformer(
					&cache.ListWatch{
						ListFunc: func(options api.ListOptions) (runtime.Object, error) {
							options.LabelSelector = labels.SelectorFromSet(labels.Set{"name": additionalPodsPrefix})
							return c.Pods(ns).List(options)
						},
						WatchFunc: func(options api.ListOptions) (watch.Interface, error) {
							options.LabelSelector = labels.SelectorFromSet(labels.Set{"name": additionalPodsPrefix})
							return c.Pods(ns).Watch(options)
						},
					},
					&api.Pod{},
					0,
					controllerframework.ResourceEventHandlerFuncs{
						AddFunc: func(obj interface{}) {
							p, ok := obj.(*api.Pod)
							Expect(ok).To(Equal(true))
							go checkPod(p)
						},
						UpdateFunc: func(oldObj, newObj interface{}) {
							p, ok := newObj.(*api.Pod)
							Expect(ok).To(Equal(true))
							go checkPod(p)
						},
					},
				)

				stopCh := make(chan struct{})
				go controller.Run(stopCh)

				// Create some additional pods with throughput ~5 pods/sec.
				var wg sync.WaitGroup
				wg.Add(nodeCount)
				podLabels := map[string]string{
					"name": additionalPodsPrefix,
				}
				for i := 1; i <= nodeCount; i++ {
					name := additionalPodsPrefix + "-" + strconv.Itoa(i)
					go createRunningPod(&wg, c, name, ns, "gcr.io/google_containers/pause:2.0", podLabels)
					time.Sleep(200 * time.Millisecond)
				}
				wg.Wait()

				Logf("Waiting for all Pods begin observed by the watch...")
				for start := time.Now(); len(watchTimes) < nodeCount; time.Sleep(10 * time.Second) {
					if time.Since(start) < timeout {
						Failf("Timeout reached waiting for all Pods being observed by the watch.")
					}
				}
				close(stopCh)

				selector := fields.Set{
					"involvedObject.kind":      "Pod",
					"involvedObject.namespace": ns,
					"source":                   api.DefaultSchedulerName,
				}.AsSelector()
				options := api.ListOptions{FieldSelector: selector}
				schedEvents, err := c.Events(ns).List(options)
				expectNoError(err)
				for k := range createTimes {
					for _, event := range schedEvents.Items {
						if event.InvolvedObject.Name == k {
							scheduleTimes[k] = event.FirstTimestamp
							break
						}
					}
				}

				scheduleLag := make([]podLatencyData, 0)
				startupLag := make([]podLatencyData, 0)
				watchLag := make([]podLatencyData, 0)
				schedToWatchLag := make([]podLatencyData, 0)
				e2eLag := make([]podLatencyData, 0)

				for name, create := range createTimes {
					sched, ok := scheduleTimes[name]
					Expect(ok).To(Equal(true))
					run, ok := runTimes[name]
					Expect(ok).To(Equal(true))
					watch, ok := watchTimes[name]
					Expect(ok).To(Equal(true))
					node, ok := nodes[name]
					Expect(ok).To(Equal(true))

					scheduleLag = append(scheduleLag, podLatencyData{name, node, sched.Time.Sub(create.Time)})
					startupLag = append(startupLag, podLatencyData{name, node, run.Time.Sub(sched.Time)})
					watchLag = append(watchLag, podLatencyData{name, node, watch.Time.Sub(run.Time)})
					schedToWatchLag = append(schedToWatchLag, podLatencyData{name, node, watch.Time.Sub(sched.Time)})
					e2eLag = append(e2eLag, podLatencyData{name, node, watch.Time.Sub(create.Time)})
				}

				sort.Sort(latencySlice(scheduleLag))
				sort.Sort(latencySlice(startupLag))
				sort.Sort(latencySlice(watchLag))
				sort.Sort(latencySlice(schedToWatchLag))
				sort.Sort(latencySlice(e2eLag))

				printLatencies(scheduleLag, "worst schedule latencies")
				printLatencies(startupLag, "worst run-after-schedule latencies")
				printLatencies(watchLag, "worst watch latencies")
				printLatencies(schedToWatchLag, "worst scheduled-to-end total latencies")
				printLatencies(e2eLag, "worst e2e total latencies")

				// Test whether e2e pod startup time is acceptable.
				podStartupLatency := PodStartupLatency{Latency: extractLatencyMetrics(e2eLag)}
				expectNoError(VerifyPodStartupLatency(podStartupLatency))

				// Log suspicious latency metrics/docker errors from all nodes that had slow startup times
				for _, l := range startupLag {
					if l.Latency > NodeStartupThreshold {
						HighLatencyKubeletOperations(c, 1*time.Second, l.Node)
					}
				}

				Logf("Approx throughput: %v pods/min",
					float64(nodeCount)/(e2eLag[len(e2eLag)-1].Latency.Minutes()))
			}
		})
	}
})

func createRunningPod(wg *sync.WaitGroup, c *client.Client, name, ns, image string, labels map[string]string) {
	defer GinkgoRecover()
	defer wg.Done()
	pod := &api.Pod{
		TypeMeta: unversioned.TypeMeta{
			Kind: "Pod",
		},
		ObjectMeta: api.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Spec: api.PodSpec{
			Containers: []api.Container{
				{
					Name:  name,
					Image: image,
				},
			},
			DNSPolicy: api.DNSDefault,
		},
	}
	_, err := c.Pods(ns).Create(pod)
	expectNoError(err)
	expectNoError(waitForPodRunningInNamespace(c, name, ns))
}

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
	"path/filepath"
	"time"

	"k8s.io/kubernetes/pkg/api"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/labels"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	dnsReadyTimeout = time.Minute
)

const queryDnsPythonTemplate string = `
import socket
try:
	socket.gethostbyname('%s')
	print 'ok'
except:
	print 'err'`

var _ = Describe("[Example] ClusterDns", func() {
	framework := NewFramework("cluster-dns")

	var c *client.Client
	BeforeEach(func() {
		c = framework.Client
	})

	It("should create pod that uses dns [Conformance]", func() {
		mkpath := func(file string) string {
			return filepath.Join(testContext.RepoRoot, "examples/cluster-dns", file)
		}

		// contrary to the example, this test does not use contexts, for simplicity
		// namespaces are passed directly.
		// Also, for simplicity, we don't use yamls with namespaces, but we
		// create testing namespaces instead.

		backendRcYaml := mkpath("dns-backend-rc.yaml")
		backendRcName := "dns-backend"
		backendSvcYaml := mkpath("dns-backend-service.yaml")
		backendSvcName := "dns-backend"
		backendPodName := "dns-backend"
		frontendPodYaml := mkpath("dns-frontend-pod.yaml")
		frontendPodName := "dns-frontend"
		frontendPodContainerName := "dns-frontend"

		podOutput := "Hello World!"

		// we need two namespaces anyway, so let's forget about
		// the one created in BeforeEach and create two new ones.
		namespaces := []*api.Namespace{nil, nil}
		for i := range namespaces {
			var err error
			namespaces[i], err = createTestingNS(fmt.Sprintf("dnsexample%d", i), c, nil)
			if testContext.DeleteNamespace {
				if namespaces[i] != nil {
					defer deleteNS(c, namespaces[i].Name, 5*time.Minute /* namespace deletion timeout */)
				}
				Expect(err).NotTo(HaveOccurred())
			} else {
				Logf("Found DeleteNamespace=false, skipping namespace deletion!")
			}
		}

		for _, ns := range namespaces {
			runKubectlOrDie("create", "-f", backendRcYaml, getNsCmdFlag(ns))
		}

		for _, ns := range namespaces {
			runKubectlOrDie("create", "-f", backendSvcYaml, getNsCmdFlag(ns))
		}

		// wait for objects
		for _, ns := range namespaces {
			waitForRCPodsRunning(c, ns.Name, backendRcName)
			waitForService(c, ns.Name, backendSvcName, true, poll, serviceStartTimeout)
		}
		// it is not enough that pods are running because they may be set to running, but
		// the application itself may have not been initialized. Just query the application.
		for _, ns := range namespaces {
			label := labels.SelectorFromSet(labels.Set(map[string]string{"name": backendRcName}))
			options := api.ListOptions{LabelSelector: label}
			pods, err := c.Pods(ns.Name).List(options)
			Expect(err).NotTo(HaveOccurred())
			err = podsResponding(c, ns.Name, backendPodName, false, pods)
			Expect(err).NotTo(HaveOccurred(), "waiting for all pods to respond")
			Logf("found %d backend pods responding in namespace %s", len(pods.Items), ns.Name)

			err = serviceResponding(c, ns.Name, backendSvcName)
			Expect(err).NotTo(HaveOccurred(), "waiting for the service to respond")
		}

		// Now another tricky part:
		// It may happen that the service name is not yet in DNS.
		// So if we start our pod, it will fail. We must make sure
		// the name is already resolvable. So let's try to query DNS from
		// the pod we have, until we find our service name.
		// This complicated code may be removed if the pod itself retried after
		// dns error or timeout.
		// This code is probably unnecessary, but let's stay on the safe side.
		label := labels.SelectorFromSet(labels.Set(map[string]string{"name": backendPodName}))
		options := api.ListOptions{LabelSelector: label}
		pods, err := c.Pods(namespaces[0].Name).List(options)

		if err != nil || pods == nil || len(pods.Items) == 0 {
			Failf("no running pods found")
		}
		podName := pods.Items[0].Name

		queryDns := fmt.Sprintf(queryDnsPythonTemplate, backendSvcName+"."+namespaces[0].Name)
		_, err = lookForStringInPodExec(namespaces[0].Name, podName, []string{"python", "-c", queryDns}, "ok", dnsReadyTimeout)
		Expect(err).NotTo(HaveOccurred(), "waiting for output from pod exec")

		updatedPodYaml := prepareResourceWithReplacedString(frontendPodYaml, "dns-backend.development.cluster.local", fmt.Sprintf("dns-backend.%s.cluster.local", namespaces[0].Name))

		// create a pod in each namespace
		for _, ns := range namespaces {
			newKubectlCommand("create", "-f", "-", getNsCmdFlag(ns)).withStdinData(updatedPodYaml).execOrDie()
		}

		// wait until the pods have been scheduler, i.e. are not Pending anymore. Remember
		// that we cannot wait for the pods to be running because our pods terminate by themselves.
		for _, ns := range namespaces {
			err := waitForPodNotPending(c, ns.Name, frontendPodName)
			expectNoError(err)
		}

		// wait for pods to print their result
		for _, ns := range namespaces {
			_, err := lookForStringInLog(ns.Name, frontendPodName, frontendPodContainerName, podOutput, podStartTimeout)
			Expect(err).NotTo(HaveOccurred())
		}
	})
})

func getNsCmdFlag(ns *api.Namespace) string {
	return fmt.Sprintf("--namespace=%v", ns.Name)
}

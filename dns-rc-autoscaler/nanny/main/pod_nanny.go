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

package main

import (
	"errors"
	"log"
	"os"
	"time"

	flag "github.com/spf13/pflag"

	"k8s.io/contrib/dns-rc-autoscaler/nanny"

	client "k8s.io/kubernetes/pkg/client/clientset_generated/release_1_3"
	"k8s.io/kubernetes/pkg/client/restclient"
)

var (
	// Flags to identify the container to nanny.
	podNamespace = flag.String("namespace", os.Getenv("MY_POD_NAMESPACE"), "The namespace of the ward. This defaults to the nanny pod's own namespace.")
	podName      = flag.String("pod", os.Getenv("MY_POD_NAME"), "The name of the pod to watch. This defaults to the nanny's own pod.")
	configFile   = flag.String("paramsFile", "", "Params file for the node scaling parameters")
	verbose      = flag.Bool("verbose", false, "Turn on verbose logging to stdout")
	// Flags to control runtime behavior.
	pollPeriod = time.Millisecond * time.Duration(*flag.Int("poll-period", 10000, "The time, in milliseconds, to poll the dependent container."))
)

func sanityCheckParametersAndEnvironment() error {

	var errorsFound bool
	if *configFile == "" {
		errorsFound = true
		log.Printf("-paramsFile parameter cannot be empty\n")
	}

	if _, err := nanny.ParseScalerParamsFile(*configFile); err != nil {
		errorsFound = true
		log.Printf("Could not parse scaler params file (%s)\n", err)
	}

	// Log all sanity check errors before returning a single error string
	if errorsFound {
		return errors.New("Failed to validate all input parameters")
	}
	return nil
}

func main() {
	// First log our starting config, and then set up.
	log.Printf("Invoked by %v\n", os.Args)
	flag.Parse()
	// Perform further validation of flags.
	if err := sanityCheckParametersAndEnvironment(); err != nil {
		log.Fatal(err)
	}
	// Set up work objects.
	config, err := restclient.InClusterConfig()
	if err != nil {
		log.Fatal(err)
	}
	clientset, err := client.NewForConfig(config)
	if err != nil {
		log.Fatal(err)
	}
	k8s := nanny.NewKubernetesClient(*podNamespace, *podName, clientset)
	log.Printf("Looking for parent/owner of pod %s/%s\n", *podNamespace, *podName)
	rc, err := k8s.GetParentRc(*podNamespace, *podName)
	// We cannot proceed if this pod does not have a parent object that is an RC.
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Watching replication controller: %s from pod: %s/%s\n", rc, *podNamespace, *podName)
	scaler := nanny.Scaler{ConfigFile: *configFile, Verbose: *verbose}
	// Begin nannying.
	nanny.PollAPIServer(k8s, scaler, pollPeriod, *verbose)
}

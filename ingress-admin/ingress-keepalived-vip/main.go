/*
Copyright 2016 The Kubernetes Authors.

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
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/client-go/1.5/kubernetes"
	"k8s.io/client-go/1.5/pkg/util/wait"
	"k8s.io/client-go/1.5/rest"

	"github.com/golang/glog"
	"github.com/spf13/pflag"
)

var (
	flags        = pflag.NewFlagSet("", pflag.ContinueOnError)
	resyncPeriod = flags.Duration("resync-period", 60*time.Second, `keepalived config file resync period`)
)

func init() {
	flag.Set("logtostderr", "true")
	flag.Parse()
	go wait.Until(glog.Flush, 10*time.Second, wait.NeverStop)
}

const (
	keepalivedTmpl = "keepalived.tmpl"
	keepalivedCfg  = "/etc/keepalived/keepalived.conf"

	IngressVIPAnnotationKey           = "ingress.alpha.k8s.io/ingress-vip"
	IngressVIPAllicationAnnotationKey = "ingress.alpha.k8s.io/ingress-vip-allocation"
)

func init() {
	flag.Set("logtostderr", "true")
}

func main() {
	flags.AddGoFlagSet(flag.CommandLine)
	flags.Parse(os.Args)

	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		glog.Fatal(err)
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		glog.Fatal(err)
	}

	namespace := os.Getenv("POD_NAMESPACE")
	if namespace == "" {
		glog.Fatalf("missing POD_NAMESPACE environment variable")
	}
	podName := os.Getenv("POD_NAME")
	if podName == "" {
		glog.Fatalf("missing POD_NAME environment variable")
	}
	serviceName := os.Getenv("SERVICE_NAME")
	if serviceName == "" {
		glog.Fatalf("missing SERVICE_NAME environment variable")
	}

	keepaivedController, err := newKeepalivedController(clientset, namespace, serviceName, podName)
	if err != nil {
		glog.Fatalf("can not create keepalive controller due to: %v", err)
	}

	go handleSigterm(keepaivedController)

	keepaivedController.Run(*resyncPeriod, wait.NeverStop)

	return
}

func handleSigterm(c *keepalivedController) {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGTERM)
	<-signalChan
	glog.Infof("Received SIGTERM, shutting down")

	exitCode := 0
	if err := c.Stop(); err != nil {
		glog.Infof("Error during shutdown %v", err)
		exitCode = 1
	}

	glog.Infof("Exiting with %v", exitCode)
	os.Exit(exitCode)
}

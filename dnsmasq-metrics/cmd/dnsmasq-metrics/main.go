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
	"github.com/golang/glog"
	"github.com/spf13/pflag"
	"k8s.io/kubernetes/pkg/util/flag"
	"k8s.io/kubernetes/pkg/util/logs"
	"k8s.io/kubernetes/pkg/version/verflag"

	"k8s.io/contrib/dnsmasq-metrics/pkg/server"
	"k8s.io/contrib/dnsmasq-metrics/pkg/version"
)

func main() {
	options := server.NewOptions()
	configureFlags(options, pflag.CommandLine)
	flag.InitFlags()

	logs.InitLogs()
	defer logs.FlushLogs()

	glog.Infof("dnsmasq-metrics v%s", version.VERSION)

	verflag.PrintAndExitIfRequested()

	server := server.NewServer()
	server.Run(options)
}

func configureFlags(opt *server.Options, flagSet *pflag.FlagSet) {
	flagSet.StringVar(
		&opt.DnsMasqAddr, "dnsmasq-addr", opt.DnsMasqAddr,
		"address that the dnsmasq server is listening on")
	flagSet.IntVar(
		&opt.DnsMasqPort, "dnsmasq-port", opt.DnsMasqPort,
		"port that the dnsmasq server is listening on")
	flagSet.IntVar(
		&opt.DnsMasqPollIntervalMs, "dnsmasq-poll-interval-ms", opt.DnsMasqPollIntervalMs,
		"interval with which to poll dnsmasq for stats")

	flagSet.StringVar(
		&opt.PrometheusAddr, "prometheus-addr", opt.PrometheusAddr,
		"http addr to bind metrics server to")
	flagSet.IntVar(
		&opt.PrometheusPort, "prometheus-port", opt.PrometheusPort,
		"http port to use to export prometheus metrics")
	flagSet.StringVar(
		&opt.PrometheusPath, "prometheus-path", opt.PrometheusPath,
		"http path used to export metrics")
	flagSet.StringVar(
		&opt.PrometheusNamespace, "prometheus-namespace", opt.PrometheusNamespace,
		"prometheus metric namespace")
	flagSet.StringVar(
		&opt.PrometheusSubsystem, "prometheus-subsystem", opt.PrometheusSubsystem,
		"prometheus metric subsystem")
}

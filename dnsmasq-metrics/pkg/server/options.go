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

package server

import "github.com/spf13/pflag"

// Options for the daemon
type Options struct {
	DnsMasqPort           int
	DnsMasqAddr           string
	DnsMasqPollIntervalMs int

	PrometheusAddr      string
	PrometheusPort      int
	PrometheusPath      string
	PrometheusNamespace string
	PrometheusSubsystem string
}

// NewOptions creates a new options struct.
func NewOptions() *Options {
	// Defaults should be set by ConfigureFlags() below.
	return &Options{}
}

// ConfigureFlags adds command line options for the daemon
func (opt *Options) ConfigureFlags(flagSet *pflag.FlagSet) {
	flagSet.StringVar(
		&opt.DnsMasqAddr, "dnsmasq-addr", "127.0.0.1",
		"address that the dnsmasq server is listening on")
	flagSet.IntVar(
		&opt.DnsMasqPort, "dnsmasq-port", 53,
		"port that the dnsmasq server is listening on")
	flagSet.IntVar(
		&opt.DnsMasqPollIntervalMs, "dnsmasq-poll-interval-ms", 5000,
		"interval with which to poll dnsmasq for stats")

	flagSet.StringVar(
		&opt.PrometheusAddr, "prometheus-addr", "0.0.0.0",
		"http addr to bind metrics server to")
	flagSet.IntVar(
		&opt.PrometheusPort, "prometheus-port", 10054,
		"http port to use to export prometheus metrics")
	flagSet.StringVar(
		&opt.PrometheusPath, "prometheus-path", "/metrics",
		"http path used to export metrics")
	flagSet.StringVar(
		&opt.PrometheusNamespace, "prometheus-namespace", "dnsmasq",
		"prometheus metric namespace")
	flagSet.StringVar(
		&opt.PrometheusSubsystem, "prometheus-subsystem", "cache",
		"prometheus metric subsystem")
}

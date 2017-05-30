/*
Copyright 2017 The Kubernetes Authors.

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
	"strconv"
	"time"

	"github.com/golang/glog"
	dto "github.com/prometheus/client_model/go"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	v3 "google.golang.org/api/monitoring/v3"

	"k8s.io/contrib/prometheus-to-sd/config"
	"k8s.io/contrib/prometheus-to-sd/flags"
	"k8s.io/contrib/prometheus-to-sd/translator"
)

var (
	host       = flag.String("target-host", "localhost", "The monitored component's hostname. DEPRECATED: Use --source instead.")
	port       = flag.Uint("target-port", 80, "The monitored component's port. DEPRECATED: Use --source instead.")
	component  = flag.String("component", "", "The monitored target's name. DEPRECATED: Use --source instead.")
	resolution = flag.Duration("metrics-resolution", 60*time.Second,
		"The resolution at which prometheus-to-sd will scrape the component for metrics.")
	metricsPrefix = flag.String("stackdriver-prefix", "container.googleapis.com/master",
		"Prefix that is appended to every metric.")
	whitelisted = flag.String("whitelisted-metrics", "",
		"Comma-separated list of whitelisted metrics. If empty all metrics will be exported. DEPRECATED: Use --source instead.")
	autoWhitelistMetrics = flag.Bool("auto-whitelist-metrics", false,
		"If component has no whitelisted metrics, prometheus-to-sd will fetch them from Stackdriver.")
	metricDescriptorsResolution = flag.Duration("metric-descriptors-resolution", 10*time.Minute,
		"The resolution at which prometheus-to-sd will scrape metric descriptors from Stackdriver.")
	apioverride = flag.String("api-override", "",
		"The stackdriver API endpoint to override the default one used (which is prod).")
	source = flags.Uris{}
)

func main() {
	flag.Set("logtostderr", "true")
	flag.Var(&source, "source", "source(s) to watch in [component-name]:http://host:port?whitelisted=a,b,c format")

	defer glog.Flush()
	flag.Parse()

	sourceConfigs := extractSourceConfigsFromFlags()

	gceConf, err := config.GetGceConfig(*metricsPrefix)
	if err != nil {
		glog.Fatalf("Failed to get GCE config: %v", err)
	}
	glog.Infof("GCE config: %+v", gceConf)

	client := oauth2.NewClient(oauth2.NoContext, google.ComputeTokenSource(""))
	stackdriverService, err := v3.New(client)
	if *apioverride != "" {
		stackdriverService.BasePath = *apioverride
	}
	if err != nil {
		glog.Fatalf("Failed to create Stackdriver client: %v", err)
	}
	glog.V(4).Infof("Successfully created Stackdriver client")

	if len(sourceConfigs) == 0 {
		glog.Fatalf("No sources defined. Please specify at least one --source flag.")
	}

	for _, sourceConfig := range sourceConfigs {
		glog.V(4).Infof("Starting goroutine for %+v", sourceConfig)

		// Pass sourceConfig as a parameter to avoid using the last sourceConfig by all goroutines.
		go readAndPushDataToStackdriver(stackdriverService, gceConf, sourceConfig)
	}

	// As worker goroutines work forever, block main thread as well.
	<-make(chan int)
}

func extractSourceConfigsFromFlags() []config.SourceConfig {
	var sourceConfigs []config.SourceConfig
	for _, c := range source {
		if sourceConfig, err := config.ParseSourceConfig(c); err != nil {
			glog.Fatalf("Error while parsing source config flag %v: %v", c, err)
		} else {
			sourceConfigs = append(sourceConfigs, *sourceConfig)
		}
	}

	if len(source) == 0 && *component != "" {
		glog.Warningf("--component, --host, --port and --whitelisted flags are deprecated. Please use --source instead.")
		portStr := strconv.FormatUint(uint64(*port), 10)

		if sourceConfig, err := config.NewSourceConfig(*component, *host, portStr, *whitelisted); err != nil {
			glog.Fatalf("Error while parsing --component flag: %v", err)
		} else {
			glog.Infof("Created a new source instance from --component flag: %+v", sourceConfig)
			sourceConfigs = append(sourceConfigs, *sourceConfig)
		}
	}
	return sourceConfigs
}

func readAndPushDataToStackdriver(stackdriverService *v3.Service, gceConf *config.GceConfig, sourceConfig config.SourceConfig) {
	glog.Infof("Running prometheus-to-sd, monitored target is %s %v:%v", sourceConfig.Component, sourceConfig.Host, sourceConfig.Port)

	signal := time.After(0)
	useWhitelistedMetricsAutodiscovery := *autoWhitelistMetrics && len(sourceConfig.Whitelisted) == 0

	for range time.Tick(*resolution) {
		glog.V(4).Infof("Scraping metrics of component %v", sourceConfig.Component)
		var metricDescriptors map[string]*v3.MetricDescriptor
		select {
		case <-signal:
			glog.V(4).Infof("Updating metrics cache for component %v", sourceConfig.Component)
			var err error
			metricDescriptors, err = translator.GetMetricDescriptors(stackdriverService, gceConf, sourceConfig.Component)
			if err != nil {
				glog.Warningf("Error while fetching metric descriptors for %v: %v", sourceConfig.Component, err)
			}
			if useWhitelistedMetricsAutodiscovery {
				updateWhitelistedMetrics(&sourceConfig, metricDescriptors)
			}
			signal = time.After(*metricDescriptorsResolution)
		default:
		}
		if useWhitelistedMetricsAutodiscovery && len(sourceConfig.Whitelisted) == 0 {
			glog.V(4).Infof("Skipping %v component as there are no metric to expose.", sourceConfig.Component)
			continue
		}

		metrics, err := translator.GetPrometheusMetrics(sourceConfig.Host, sourceConfig.Port)
		if err != nil {
			glog.Warningf("Error while getting Prometheus metrics %v", err)
			continue
		}
		if metricDescriptors != nil {
			updateMetricDescriptorsDescription(stackdriverService, gceConf, metricDescriptors, metrics)
		}
		ts := translator.TranslatePrometheusToStackdriver(gceConf, sourceConfig.Component, metrics, sourceConfig.Whitelisted)
		translator.SendToStackdriver(stackdriverService, gceConf, ts)
	}
}

func updateWhitelistedMetrics(sourceConfig *config.SourceConfig, metricDescriptors map[string]*v3.MetricDescriptor) {
	sourceConfig.Whitelisted = nil
	for metricName := range metricDescriptors {
		sourceConfig.Whitelisted = append(sourceConfig.Whitelisted, metricName)
	}
}

func updateMetricDescriptorsDescription(stackdriverService *v3.Service,
	config *config.GceConfig,
	descriptors map[string]*v3.MetricDescriptor,
	metrics map[string]*dto.MetricFamily) {
	for _, metricFamily := range metrics {
		metricDescriptor, ok := descriptors[metricFamily.GetName()]
		if !ok || metricDescriptor.Description != metricFamily.GetHelp() {
			translator.CreateMetricDescriptor(stackdriverService, config, *component, metricFamily)
		}
	}
}

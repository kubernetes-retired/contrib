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
	"time"

	"github.com/golang/glog"
	"golang.org/x/net/context"
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
	podId  = flag.String("pod-id", "machine",
		"Name of the pod in which monitored component is running.")
	namespaceId = flag.String("namespace-id", "",
		"Namespace name of the pod in which monitored component is running.")
)

func main() {
	flag.Set("logtostderr", "true")
	flag.Var(&source, "source", "source(s) to watch in [component-name]:http://host:port?whitelisted=a,b,c format")

	defer glog.Flush()
	flag.Parse()

	sourceConfigs := config.SourceConfigsFromFlags(source, component, host, port, whitelisted)

	gceConf, err := config.GetGceConfig(*metricsPrefix)
	podConfig := &config.PodConfig{
		PodId:       *podId,
		NamespaceId: *namespaceId,
	}
	if err != nil {
		glog.Fatalf("Failed to get GCE config: %v", err)
	}
	glog.Infof("GCE config: %+v", gceConf)

	client := oauth2.NewClient(context.Background(), google.ComputeTokenSource(""))
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
		go readAndPushDataToStackdriver(stackdriverService, gceConf, podConfig, sourceConfig)
	}

	// As worker goroutines work forever, block main thread as well.
	<-make(chan int)
}

func readAndPushDataToStackdriver(stackdriverService *v3.Service, gceConf *config.GceConfig, podConfig *config.PodConfig, sourceConfig config.SourceConfig) {
	glog.Infof("Running prometheus-to-sd, monitored target is %s %v:%v", sourceConfig.Component, sourceConfig.Host, sourceConfig.Port)
	metricDescriptorCache := translator.NewMetricDescriptorCache(stackdriverService, gceConf, sourceConfig.Component)
	signal := time.After(0)
	useWhitelistedMetricsAutodiscovery := *autoWhitelistMetrics && len(sourceConfig.Whitelisted) == 0

	for range time.Tick(*resolution) {
		metricDescriptorCache.MarkStale()
		glog.V(4).Infof("Scraping metrics of component %v", sourceConfig.Component)
		select {
		case <-signal:
			glog.V(4).Infof("Updating metrics cache for component %v", sourceConfig.Component)
			metricDescriptorCache.Refresh()
			if useWhitelistedMetricsAutodiscovery {
				sourceConfig.UpdateWhitelistedMetrics(metricDescriptorCache.GetMetricNames())
			}
			signal = time.After(*metricDescriptorsResolution)
		default:
		}
		if useWhitelistedMetricsAutodiscovery && len(sourceConfig.Whitelisted) == 0 {
			glog.V(4).Infof("Skipping %v component as there are no metric to expose.", sourceConfig.Component)
			continue
		}
		metrics, err := translator.GetPrometheusMetrics(sourceConfig.Host, sourceConfig.Port)
		commonConfig := &config.CommonConfig{
			GceConfig:     gceConf,
			PodConfig:     podConfig,
			ComponentName: sourceConfig.Component,
		}
		if err != nil {
			glog.Warningf("Error while getting Prometheus metrics %v", err)
			continue
		}
		metricDescriptorCache.UpdateMetricDescriptors(metrics)
		ts := translator.TranslatePrometheusToStackdriver(commonConfig, metrics, sourceConfig.Whitelisted)
		translator.SendToStackdriver(stackdriverService, gceConf, ts)
	}
}

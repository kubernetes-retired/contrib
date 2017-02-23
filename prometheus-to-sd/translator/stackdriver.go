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

package translator

import (
	"fmt"

	"github.com/golang/glog"
	v3 "google.golang.org/api/monitoring/v3"

	"k8s.io/contrib/prometheus-to-sd/config"
)

func SendToStackdriver(service *v3.Service, config *config.GceConfig, ts []*v3.TimeSeries) {
	if len(ts) == 0 {
		glog.Warningf("No metrics to send to Stackdriver")
		return
	}

	req := &v3.CreateTimeSeriesRequest{TimeSeries: ts}
	proj := fmt.Sprintf("projects/%s", config.Project)

	_, err := service.Projects.TimeSeries.Create(proj, req).Do()
	if err != nil {
		glog.Errorf("Error while sending request to Stackdriver %v", err)
	} else {
		glog.V(4).Infof("Successfully sent %v timeserieses to Stackdriver", len(req.TimeSeries))
	}
}

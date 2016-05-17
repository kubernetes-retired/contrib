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

package problemdetector

import (
	"github.com/golang/glog"

	kubeutil "k8s.io/kubernetes/pkg/util"

	"k8s.io/contrib/node-problem-detector/pkg/condition"
	"k8s.io/contrib/node-problem-detector/pkg/kernelmonitor"
	"k8s.io/contrib/node-problem-detector/pkg/problemclient"
	"k8s.io/contrib/node-problem-detector/pkg/util"
)

// ProblemDetector collects statuses from all problem daemons and update the node condition and send node event.
type ProblemDetector interface {
	Run() error
}

type problemDetector struct {
	client           problemclient.Client
	conditionManager condition.ConditionManager
	// TODO(random-liu): Use slices of problem daemons if multiple monitors are needed in the future
	monitor kernelmonitor.KernelMonitor
}

// NewProblemDetector creates the problem detector. Currently we just directly passed in the problem daemons, but
// in the future we may want to let the problem daemons register themselves.
func NewProblemDetector(monitor kernelmonitor.KernelMonitor) ProblemDetector {
	client := problemclient.NewClientOrDie()
	return &problemDetector{
		client:           client,
		conditionManager: condition.NewConditionManager(client, kubeutil.RealClock{}),
		monitor:          monitor,
	}
}

// Run starts the problem detector.
func (p *problemDetector) Run() error {
	p.conditionManager.Start()
	ch, err := p.monitor.Start()
	if err != nil {
		return err
	}
	glog.Info("Problem detector started")
	for {
		select {
		case status, ok := <-ch:
			if !ok {
				glog.Errorf("Monitor stopped unexpectedly")
				break
			}
			if status.Event != nil {
				p.client.Eventf(util.ConvertToAPIEventType(status.Event.Severity), status.Source, status.Event.Reason, status.Event.Message)
			}
			p.conditionManager.UpdateCondition(status.Condition)
		}
	}
}

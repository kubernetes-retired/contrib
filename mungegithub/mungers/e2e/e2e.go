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
	"sync"

	"k8s.io/contrib/mungegithub/mungers/jenkins"

	"github.com/golang/glog"
)

// BuildInfo tells the build ID and the build success
type BuildInfo struct {
	Status   string
	ID       string
	Failures []string `json:"Failures,omitempty"`
}

// E2ETester is the object which will contact a jenkins instance and get
// information about recent jobs
type E2ETester struct {
	Builders map[string]*builderInfo

	sync.Mutex
	BuildStatus map[string]BuildInfo // protect by mutex
}
type builderInfo struct {
	jenkins.BuilderConfig
	builder jenkins.Builder
}

func (e *E2ETester) locked(f func()) {
	e.Lock()
	defer e.Unlock()
	f()
}

// SetBuilders configure the builders that we will watch for build results
func (e *E2ETester) LoadBuilders(configs []*jenkins.BuilderConfig) error {
	e.Builders = make(map[string]*builderInfo)
	for _, config := range configs {
		builder, err := jenkins.NewFederatedBuilder(config)
		if err != nil {
			return fmt.Errorf("error building federated builder %q: %v", config.Name, err)
		}

		b := &builderInfo{BuilderConfig: *config}
		b.builder = builder

		e.Builders[b.Name] = b
	}

	return nil
}

// GetBuildStatus returns the build status. This map is a copy and is thus safe
// for the caller to use in any way.
func (e *E2ETester) GetBuildStatus() map[string]BuildInfo {
	e.Lock()
	defer e.Unlock()
	out := map[string]BuildInfo{}
	for k, v := range e.BuildStatus {
		out[k] = v
	}
	return out
}

func (e *E2ETester) setBuildStatus(build, status string, id string, failures []string) {
	e.Lock()
	defer e.Unlock()
	e.BuildStatus[build] = BuildInfo{Status: status, ID: id, Failures: failures}
}

// Stable is called to make sure all of the jenkins jobs are stable
func (e *E2ETester) Stable() bool {
	// Test if the build is stable in Jenkins

	allStable := true
	for key, builder := range e.Builders {
		glog.V(2).Infof("Checking build stability for %s", key)
		job, err := builder.builder.GetLastCompletedBuild()
		if err != nil {
			glog.Errorf("Error checking build %s : %v", key, err)
			e.setBuildStatus(key, "Error checking: "+err.Error(), "0", nil)
			allStable = false
			continue
		}
		if job == nil {
			glog.Warningf("Builder does not have LastCompletedBuild: %q", key)
			e.setBuildStatus(key, "No last-build found", "0", nil)
		}

		failures := job.Failures()
		if job.IsStable() {
			e.setBuildStatus(key, "Stable", job.BuildID, failures)
		} else {
			e.setBuildStatus(key, "Not Stable", job.BuildID, failures)
			if builder.Gating {
				allStable = false
			}
		}
	}
	return allStable
}

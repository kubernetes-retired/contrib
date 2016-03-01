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
	"strings"
	"sync"

	"k8s.io/contrib/mungegithub/mungers/jenkins"

	"github.com/golang/glog"
)

// BuildInfo tells the build ID and the build success
type BuildInfo struct {
	Status string
	ID     string
	URL    string
	Gating bool
}

// E2ETester is the object which will contact a jenkins instance and get
// information about recent jobs
type E2ETester struct {
	Builders map[string]*BuilderInfo

	sync.Mutex
	BuildStatus map[string]BuildInfo // protect by mutex
}

type BuilderInfo struct {
	name    string
	builder jenkins.Builder
	gating  bool
}

func (e *E2ETester) locked(f func()) {
	e.Lock()
	defer e.Unlock()
	f()
}

// Configure the builders that we will poll
func (e *E2ETester) SetBuilders(jenkinsHost string, builders []string) error {
	e.Builders = make(map[string]*BuilderInfo)
	jenkinsClient := &jenkins.JenkinsClient{Host: jenkinsHost}
	for _, spec := range builders {
		b := &BuilderInfo{}
		b.gating = true

		if strings.Contains(spec, "=") {
			tokens := strings.SplitN(spec, "=", 2)
			if len(tokens) != 2 {
				return fmt.Errorf("cannot parse federated builder spec: %q", spec)
			}
			name := tokens[0]
			path := tokens[1]

			builder, err := jenkins.NewFederatedBuilder(name, path)
			if err != nil {
				return fmt.Errorf("error building federated builder (%q): %v", spec, err)
			}

			b.name = name
			b.builder = builder

			// For the moment, we assume that all federated builders are non-gating
			b.gating = false
		} else {
			b.name = spec
			b.builder = &jenkins.JenkinsBuilder{
				Client:  jenkinsClient,
				JobName: spec,
			}
		}
		e.Builders[b.name] = b
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

func (e *E2ETester) setBuildStatus(build string, info *BuildInfo) {
	e.Lock()
	defer e.Unlock()
	e.BuildStatus[build] = *info
}

// Stable is called to make sure all of the jenkins jobs are stable
func (e *E2ETester) Stable() bool {
	// Test if the build is stable in Jenkins

	allStable := true
	for key, builder := range e.Builders {
		glog.V(2).Infof("Checking build stability for %s", key)
		job, err := builder.builder.GetLastCompletedBuild()
		info := &BuildInfo{Gating: builder.gating}
		if err != nil {
			glog.Errorf("Error checking build %s : %v", key, err)
			info.Status = "Error checking: " + err.Error()
			info.ID = "0"
			e.setBuildStatus(key, info)
			allStable = false
			continue
		}
		info.ID = job.BuildID
		info.URL = job.URL
		if job.IsStable() {
			info.Status = "Stable"
		} else {
			info.Status = "Not Stable"
			if builder.gating {
				allStable = false
			}
		}
		e.setBuildStatus(key, info)
	}
	return allStable
}

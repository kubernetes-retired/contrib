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

package config

import (
	"fmt"
	"strconv"
	"strings"

	gceurl "k8s.io/contrib/cluster-autoscaler/utils/gce_url"
	kube_api "k8s.io/kubernetes/pkg/api"
)

// InstanceConfig contains instance configuration details.
type InstanceConfig struct {
	Project string
	Zone    string
	Name    string
}

// InstanceConfigFromProviderId creates InstanceConfig object
// from provider id which must be in format:
// gce://<project-id>/<zone>/<name>
// TODO(piosz): add better check whether the id is correct
func InstanceConfigFromProviderId(id string) (*InstanceConfig, error) {
	splitted := strings.Split(id[6:], "/")
	if len(splitted) != 3 {
		return nil, fmt.Errorf("Wrong id: expected format gce://<project-id>/<zone>/<name>, got %v", id)
	}
	return &InstanceConfig{
		Project: splitted[0],
		Zone:    splitted[1],
		Name:    splitted[2],
	}, nil
}

// ScalingConfig contains managed instance group configuration details.
type ScalingConfig struct {
	MinSize int
	MaxSize int
	Project string // Unused on AWS
	Zone    string
	Name    string
}

// Url builds GCE url for the MIG.
func (scalingconfig *ScalingConfig) Url() string {
	return gceurl.GenerateMigUrl(scalingconfig.Project, scalingconfig.Zone, scalingconfig.Name)
}

// Node returns a template/dummy node for the mig.
func (scalingconfig *ScalingConfig) Node() *kube_api.Node {
	//TODO(fgrzadkowski): Implement this.
	return nil
}

// ScalingConfigFlag is an array of MIG configuration details. Working as a multi-value flag.
type ScalingConfigFlag []ScalingConfig

// String returns string representation of the MIG.
func (scalingconfigflag *ScalingConfigFlag) String() string {
	configs := make([]string, len(*scalingconfigflag))
	for _, scalingconfig := range *scalingconfigflag {
		configs = append(configs, fmt.Sprintf("%d:%d:%s:%s", scalingconfig.MinSize, scalingconfig.MaxSize, scalingconfig.Zone, scalingconfig.Name))
	}
	return "[" + strings.Join(configs, " ") + "]"
}

// Set adds a new configuration.
func (scalingconfigflag *ScalingConfigFlag) Set(value string) error {
	tokens := strings.SplitN(value, ":", 3)
	if len(tokens) != 3 {
		return fmt.Errorf("wrong nodes configuration: %s", value)
	}
	scalingconfig := ScalingConfig{}
	if size, err := strconv.Atoi(tokens[0]); err == nil {
		if size <= 0 {
			return fmt.Errorf("min size must be >= 1")
		}
		scalingconfig.MinSize = size
	} else {
		return fmt.Errorf("failed to set min size: %s, expected integer", tokens[0])
	}

	if size, err := strconv.Atoi(tokens[1]); err == nil {
		if size < scalingconfig.MinSize {
			return fmt.Errorf("max size must be greater or equal to min size")
		}
		scalingconfig.MaxSize = size
	} else {
		return fmt.Errorf("failed to set max size: %s, expected integer", tokens[1])
	}

	var err error
	// TODO: this is a bit messy on AWS, we're currently forced to fill in a GCE url with AWS ASG details
	if scalingconfig.Project, scalingconfig.Zone, scalingconfig.Name, err = gceurl.ParseMigUrl(tokens[2]); err != nil {
		return fmt.Errorf("failed to parse mig url: %s got error: %v", tokens[2], err)
	}

	*scalingconfigflag = append(*scalingconfigflag, scalingconfig)
	return nil
}

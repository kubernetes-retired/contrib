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

package firewalls

import (
	"github.com/golang/glog"
	compute "google.golang.org/api/compute/v1"
	"k8s.io/contrib/ingress/controllers/gce/utils"
)

// Src range from which the GCE L7 performs health checks.
const l7SrcRange = "130.211.0.0/22"

// Firewalls manages firewall rules.
type FirewallRules struct {
	cloud Firewall
	namer utils.Namer
}

// NewFirewallPool creates a new firewall rule manager.
// cloud: the cloud object implementing Firewall.
// namer: cluster namer.
func NewFirewallPool(cloud Firewall, namer utils.Namer) SingleFirewallPool {
	return &FirewallRules{cloud: cloud, namer: namer}
}

func (fr *FirewallRules) Sync(nodePorts []int64, defaultBackendNodePort int64, nodeNames []string) error {
	if len(nodePorts) == 0 {
		// Delete the firewall rule if there are no Service NodePorts.
		// Note that this will also cutoff the deault backend, but we can't
		// leave the firewall rule because the controller could get killed
		// at anytime and we'd end up leaking the rule.
		return fr.Shutdown()
	}
	nodePorts = append(nodePorts, defaultBackendNodePort)
	// Firewall rule prefix must match that inserted by the gce library.
	suffix := fr.namer.FrSuffix()
	// TODO: Fix upstream gce cloudprovider lib so GET also takes the suffix
	// instead of the whole name.
	name := fr.namer.FrName(suffix)
	rule, _ := fr.cloud.GetFirewall(name)
	if rule == nil {
		glog.Infof("Creating global l7 firewall rule %v", name)
		return fr.cloud.CreateFirewall(suffix, "GCE L7 firewall rule", l7SrcRange, nodePorts, nodeNames)
	}
	glog.V(3).Infof("Firewall rule %v already exists, verifying for nodeports %v", name, nodePorts)
	return fr.cloud.UpdateFirewall(suffix, "GCE L7 firewall rule", l7SrcRange, nodePorts, nodeNames)
}

func (fr *FirewallRules) Shutdown() error {
	return fr.cloud.DeleteFirewall(fr.namer.FrSuffix())
}

// GetFirewall just returns the firewall object corresponding to the given name.
// TODO: Currently only used in testing. Modify so we don't leak compute
// objects out of this interface by returning just the (src, ports, error).
func (fr *FirewallRules) GetFirewall(name string) (*compute.Firewall, error) {
	return fr.cloud.GetFirewall(name)
}

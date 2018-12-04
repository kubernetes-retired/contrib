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
	"testing"
)

func TestParseNsSvcLVS(t *testing.T) {
	testcases := map[string]struct {
		Input         string
		Namespace     string
		Service       string
		ForwardMethod string
		Subnet        int
		Gateway       string
		VlanId        int
		ExpectedKo    bool
	}{
		"just service name":          {"echoheaders", "", "", "", 0, "", 0, true},
		"missing namespace":          {"echoheaders:NAT", "", "", "", 0, "", 0, true},
		"default forward method":     {"default/echoheaders", "default", "echoheaders", "NAT", 0, "", 0, false},
		"with forward method":        {"default/echoheaders:NAT", "default", "echoheaders", "NAT", 0, "", 0, false},
		"DR as forward method":       {"default/echoheaders:DR", "default", "echoheaders", "DR", 0, "", 0, false},
		"invalid forward method":     {"default/echoheaders:AJAX", "", "", "", 0, "", 0, true},
		"with subnet and default fw": {"default/echoheaders::24", "default", "echoheaders", "NAT", 24, "", 0, false},
		"with subnet and gateway":    {"default/echoheaders:DR:24:10.0.0.1", "default", "echoheaders", "DR", 24, "10.0.0.1", 0, false},
		"with subnet and vlan only":  {"default/echoheaders::24::10", "default", "echoheaders", "NAT", 24, "", 10, false},
		"with subnet, gw and vlan":   {"default/echoheaders::24:10.0.0.1:10", "default", "echoheaders", "NAT", 24, "10.0.0.1", 10, false},
	}

	for k, tc := range testcases {
		ns, svc, lvs, subnet, gateway, vlanId, err := parseNsSvcLbVlan(tc.Input)

		if tc.ExpectedKo && err == nil {
			t.Errorf("%s: expected an error but valid information returned: %v ", k, tc.Input)
		}

		if tc.Namespace != ns {
			t.Errorf("%s: expected %v but returned %v - input %v", k, tc.Namespace, ns, tc.Input)
		}

		if tc.Service != svc {
			t.Errorf("%s: expected %v but returned %v - input %v", k, tc.Service, svc, tc.Input)
		}

		if tc.ForwardMethod != lvs {
			t.Errorf("%s: expected %v but returned %v - input %v", k, tc.ForwardMethod, lvs, tc.Input)
		}

		if tc.Subnet != subnet {
			t.Errorf("%s: expected %v but returned %v - input %v", k, tc.Subnet, lvs, tc.Input)
		}

		if tc.Gateway != gateway {
			t.Errorf("%s: expected %v but returned %v - input %v", k, tc.Gateway, lvs, tc.Input)
		}

		if tc.VlanId != vlanId {
			t.Errorf("%s: expected %v but returned %v - input %v", k, tc.VlanId, lvs, tc.Input)
		}
	}
}

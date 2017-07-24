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
		ExpectedOk    bool
	}{
		"just service name":      {"echoheaders", "", "", "", true},
		"missing namespace":      {"echoheaders:NAT", "", "", "", true},
		"default forward method": {"default/echoheaders", "default", "echoheaders", "NAT", false},
		"with forward method":    {"default/echoheaders:NAT", "default", "echoheaders", "NAT", false},
		"DR as forward method":   {"default/echoheaders:DR", "default", "echoheaders", "DR", false},
		"invalid forward method": {"default/echoheaders:AJAX", "", "", "", true},
	}

	for k, tc := range testcases {
		ns, svc, lvs, err := parseNsSvcLVS(tc.Input)

		if tc.ExpectedOk && err == nil {
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
	}
}

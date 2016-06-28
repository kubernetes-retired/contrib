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

package utils

import (
	"testing"
)

func TestGetPathToJenkinsGoogleBucket(t *testing.T) {
	table := []struct {
		bucket string
		dir    string
		job    string
		build  int
		expect string
	}{
		{
			bucket: "kubernetes-jenkins",
			dir:    "logs",
			job:    "kubernetes-gce-e2e",
			build:  1458,
			expect: "/kubernetes-jenkins/logs/kubernetes-gce-e2e/1458/",
		},
	}

	for _, tt := range table {
		u := NewUtils(tt.bucket, tt.dir)
		out := u.GetPathToJenkinsGoogleBucket(tt.job, tt.build)
		if out != tt.expect {
			t.Errorf("Expected %v but got %v", tt.expect, out)
		}
	}
}

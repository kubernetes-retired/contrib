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

package gce

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildMig(t *testing.T) {
	_, err := buildMig("a", nil)
	assert.Error(t, err)
	_, err = buildMig("a:b:c", nil)
	assert.Error(t, err)
	_, err = buildMig("1:2:x", nil)
	assert.Error(t, err)
	_, err = buildMig("1:2:", nil)
	assert.Error(t, err)

	mig, err := buildMig("111:222:https://content.googleapis.com/compute/v1/projects/test-project/zones/test-zone/instanceGroups/test-name", nil)
	assert.NoError(t, err)
	assert.Equal(t, 111, mig.MinSize())
	assert.Equal(t, 222, mig.MaxSize())
	assert.Equal(t, "test-zone", mig.Zone)
	assert.Equal(t, "test-name", mig.Name)

	mig, err = buildMig("111:222:https://content.googleapis.com/compute/v1/projects/test-project/zones/test-zone/instanceGroups/test-name:1.5", nil)
	assert.NoError(t, err)
	assert.Equal(t, 111, mig.MinSize())
	assert.Equal(t, 222, mig.MaxSize())
	assert.Equal(t, "test-zone", mig.Zone)
	assert.Equal(t, "test-name", mig.Name)
	assert.Equal(t, 1.5, mig.NodeCost())
}

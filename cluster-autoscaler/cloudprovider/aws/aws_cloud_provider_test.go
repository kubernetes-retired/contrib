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

package aws

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildAsg(t *testing.T) {
	_, err := buildAsg("a", nil)
	assert.Error(t, err)
	_, err = buildAsg("a:b:c", nil)
	assert.Error(t, err)
	_, err = buildAsg("1:2:x", nil)
	assert.Error(t, err)
	_, err = buildAsg("1:2:", nil)
	assert.Error(t, err)

	mig, err := buildAsg("111:222:test-name", nil)
	assert.NoError(t, err)
	assert.Equal(t, 111, mig.MinSize())
	assert.Equal(t, 222, mig.MaxSize())
	assert.Equal(t, "test-zone", asg.Zone)
	assert.Equal(t, "test-name", asg.Name)
}

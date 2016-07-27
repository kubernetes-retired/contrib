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

var testAwsManager *AwsManager = &AwsManager{
	asgs:     make([]*asgInformation, 0),
	service:  nil,
	asgCache: make(map[AwsRef]*Asg),
}

func testProvider(t *testing.T) *AwsCloudProvider {
	m := testAwsManager
	provider, err := BuildAwsCloudProvider(m, nil)
	assert.NoError(t, err)
	return provider
}

func TestBuildAwsCloudProvider(t *testing.T) {
	m := testAwsManager
	_, err := BuildAwsCloudProvider(m, []string{"bad spec"})
	assert.Error(t, err)

	_, err = BuildAwsCloudProvider(m, nil)
	assert.NoError(t, err)
}

func TestAddNodeGroup(t *testing.T) {
	provider := testProvider(t)
	err := provider.addNodeGroup("bad spec")
	assert.Error(t, err)
	assert.Equal(t, len(provider.asgs), 0)

	err = provider.addNodeGroup("1:5:test-asg")
	assert.NoError(t, err)
	assert.Equal(t, len(provider.asgs), 1)
}

func TestName(t *testing.T) {
	provider := testProvider(t)
	assert.Equal(t, provider.Name(), "aws")
}

func TestNodeGroups(t *testing.T) {
	provider := testProvider(t)
	assert.Equal(t, len(provider.NodeGroups()), 0)
	err := provider.addNodeGroup("1:5:test-asg")
	assert.NoError(t, err)
	assert.Equal(t, len(provider.NodeGroups()), 1)
}

// TODO: NodeGroupForNode

func TestAwsRefFromProviderId(t *testing.T) {
	_, err := AwsRefFromProviderId("aws123")
	assert.Error(t, err)
	_, err = AwsRefFromProviderId("aws://test-az/test-instance-id")
	assert.Error(t, err)

	awsRef, err := AwsRefFromProviderId("aws:///us-east-1a/i-260942b3")
	assert.NoError(t, err)
	assert.Equal(t, awsRef, &AwsRef{Name: "i-260942b3"})
}

func TestMaxSize(t *testing.T) {
	provider := testProvider(t)
	err := provider.addNodeGroup("1:5:test-asg")
	assert.NoError(t, err)
	assert.Equal(t, len(provider.asgs), 1)
	assert.Equal(t, provider.asgs[0].MaxSize(), 5)
}

func TestMinSize(t *testing.T) {
	provider := testProvider(t)
	err := provider.addNodeGroup("1:5:test-asg")
	assert.NoError(t, err)
	assert.Equal(t, len(provider.asgs), 1)
	assert.Equal(t, provider.asgs[0].MinSize(), 1)
}

// TODO: Mock aws api response
// func TestTargetSize(t *testing.T) {
// 	provider := testProvider(t)
// 	err := provider.addNodeGroup("1:5:test-asg")
// 	assert.NoError(t, err)
// 	assert.Equal(t, len(provider.asgs), 1)
// 	targetSize, err := provider.asgs[0].TargetSize()
// 	assert.Equal(t, targetSize, 1)
// 	assert.NoError(t, err)
// }

// TODO: IncreaseSize

// TODO: Belongs

// TODO: DeleteNodes

func TestId(t *testing.T) {
	provider := testProvider(t)
	err := provider.addNodeGroup("1:5:test-asg")
	assert.NoError(t, err)
	assert.Equal(t, len(provider.asgs), 1)
	assert.Equal(t, provider.asgs[0].Id(), "test-asg")
}

func TestDebug(t *testing.T) {
	asg := Asg{
		awsManager: testAwsManager,
		minSize:    5,
		maxSize:    55,
	}
	asg.Name = "test-asg"
	assert.Equal(t, asg.Debug(), "test-asg (5:55)")
}

func TestBuildAsg(t *testing.T) {
	_, err := buildAsg("a", nil)
	assert.Error(t, err)
	_, err = buildAsg("a:b:c", nil)
	assert.Error(t, err)
	_, err = buildAsg("1:", nil)
	assert.Error(t, err)
	_, err = buildAsg("1:2:", nil)
	assert.Error(t, err)

	asg, err := buildAsg("111:222:test-name", nil)
	assert.NoError(t, err)
	assert.Equal(t, 111, asg.MinSize())
	assert.Equal(t, 222, asg.MaxSize())
	assert.Equal(t, "test-name", asg.Name)
}

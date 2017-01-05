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

package clusterstate

import (
	"testing"
	"time"

	"k8s.io/contrib/cluster-autoscaler/cloudprovider/test"
	. "k8s.io/contrib/cluster-autoscaler/utils/test"
	apiv1 "k8s.io/kubernetes/pkg/api/v1"
	metav1 "k8s.io/kubernetes/pkg/apis/meta/v1"

	"github.com/stretchr/testify/assert"
)

func TestOKWithScaleUp(t *testing.T) {
	now := time.Now()

	ng1_1 := BuildTestNode("ng1-1", 1000, 1000)
	SetNodeReadyState(ng1_1, true, now.Add(-time.Minute))
	ng2_1 := BuildTestNode("ng2-1", 1000, 1000)
	SetNodeReadyState(ng2_1, true, now.Add(-time.Minute))

	provider := testprovider.NewTestCloudProvider(nil, nil)
	provider.AddNodeGroup("ng1", 1, 10, 5)
	provider.AddNodeGroup("ng2", 1, 10, 1)

	provider.AddNode("ng1", ng1_1)
	provider.AddNode("ng2", ng2_1)
	assert.NotNil(t, provider)

	clusterstate := NewClusterStateRegistry(provider, ClusterStateRegistryConfig{
		MaxTotalUnreadyPercentage: 10,
		OkTotalUnreadyCount:       1,
	})
	clusterstate.RegisterScaleUp(&ScaleUpRequest{
		NodeGroupName:   "ng1",
		Increase:        4,
		Time:            now,
		ExpectedAddTime: now.Add(time.Minute),
	})
	err := clusterstate.UpdateNodes([]*apiv1.Node{ng1_1, ng2_1}, now)
	assert.NoError(t, err)
	assert.True(t, clusterstate.IsClusterHealthy(now))
}

func TestOKOneUnreadyNode(t *testing.T) {
	now := time.Now()

	ng1_1 := BuildTestNode("ng1-1", 1000, 1000)
	SetNodeReadyState(ng1_1, true, now.Add(-time.Minute))
	ng2_1 := BuildTestNode("ng2-1", 1000, 1000)
	SetNodeReadyState(ng2_1, false, now.Add(-time.Minute))

	provider := testprovider.NewTestCloudProvider(nil, nil)
	provider.AddNodeGroup("ng1", 1, 10, 1)
	provider.AddNodeGroup("ng2", 1, 10, 1)
	provider.AddNode("ng1", ng1_1)
	provider.AddNode("ng2", ng2_1)
	assert.NotNil(t, provider)

	clusterstate := NewClusterStateRegistry(provider, ClusterStateRegistryConfig{
		MaxTotalUnreadyPercentage: 10,
		OkTotalUnreadyCount:       1,
	})
	err := clusterstate.UpdateNodes([]*apiv1.Node{ng1_1, ng2_1}, now)
	assert.NoError(t, err)
	assert.True(t, clusterstate.IsClusterHealthy(now))
	assert.True(t, clusterstate.IsNodeGroupHealthy("ng1"))
}

func TestMissingNodes(t *testing.T) {
	now := time.Now()

	ng1_1 := BuildTestNode("ng1-1", 1000, 1000)
	SetNodeReadyState(ng1_1, true, now.Add(-time.Minute))
	ng2_1 := BuildTestNode("ng2-1", 1000, 1000)
	SetNodeReadyState(ng2_1, true, now.Add(-time.Minute))

	provider := testprovider.NewTestCloudProvider(nil, nil)
	provider.AddNodeGroup("ng1", 1, 10, 5)
	provider.AddNodeGroup("ng2", 1, 10, 1)

	provider.AddNode("ng1", ng1_1)
	provider.AddNode("ng2", ng2_1)
	assert.NotNil(t, provider)
	clusterstate := NewClusterStateRegistry(provider, ClusterStateRegistryConfig{
		MaxTotalUnreadyPercentage: 10,
		OkTotalUnreadyCount:       1,
	})
	err := clusterstate.UpdateNodes([]*apiv1.Node{ng1_1, ng2_1}, now)
	assert.NoError(t, err)
	assert.True(t, clusterstate.IsClusterHealthy(now))
	assert.False(t, clusterstate.IsNodeGroupHealthy("ng1"))
}

func TestToManyUnready(t *testing.T) {
	now := time.Now()

	ng1_1 := BuildTestNode("ng1-1", 1000, 1000)
	SetNodeReadyState(ng1_1, false, now.Add(-time.Minute))
	ng2_1 := BuildTestNode("ng2-1", 1000, 1000)
	SetNodeReadyState(ng2_1, false, now.Add(-time.Minute))

	provider := testprovider.NewTestCloudProvider(nil, nil)
	provider.AddNodeGroup("ng1", 1, 10, 1)
	provider.AddNodeGroup("ng2", 1, 10, 1)
	provider.AddNode("ng1", ng1_1)
	provider.AddNode("ng2", ng2_1)

	assert.NotNil(t, provider)
	clusterstate := NewClusterStateRegistry(provider, ClusterStateRegistryConfig{
		MaxTotalUnreadyPercentage: 10,
		OkTotalUnreadyCount:       1,
	})
	err := clusterstate.UpdateNodes([]*apiv1.Node{ng1_1, ng2_1}, now)
	assert.NoError(t, err)
	assert.False(t, clusterstate.IsClusterHealthy(now))
	assert.True(t, clusterstate.IsNodeGroupHealthy("ng1"))
}

func TestExpiredScaleUp(t *testing.T) {
	now := time.Now()

	ng1_1 := BuildTestNode("ng1-1", 1000, 1000)
	SetNodeReadyState(ng1_1, true, now.Add(-time.Minute))

	provider := testprovider.NewTestCloudProvider(nil, nil)
	provider.AddNodeGroup("ng1", 1, 10, 5)
	provider.AddNode("ng1", ng1_1)
	assert.NotNil(t, provider)

	clusterstate := NewClusterStateRegistry(provider, ClusterStateRegistryConfig{
		MaxTotalUnreadyPercentage: 10,
		OkTotalUnreadyCount:       1,
	})
	clusterstate.RegisterScaleUp(&ScaleUpRequest{
		NodeGroupName:   "ng1",
		Increase:        4,
		Time:            now.Add(-3 * time.Minute),
		ExpectedAddTime: now.Add(-1 * time.Minute),
	})
	err := clusterstate.UpdateNodes([]*apiv1.Node{ng1_1}, now)
	assert.NoError(t, err)
	assert.True(t, clusterstate.IsClusterHealthy(now))
	assert.False(t, clusterstate.IsNodeGroupHealthy("ng1"))
}

func TestRegisterScaleDown(t *testing.T) {
	ng1_1 := BuildTestNode("ng1-1", 1000, 1000)
	provider := testprovider.NewTestCloudProvider(nil, nil)
	provider.AddNodeGroup("ng1", 1, 10, 1)
	provider.AddNode("ng1", ng1_1)
	assert.NotNil(t, provider)

	clusterstate := NewClusterStateRegistry(provider, ClusterStateRegistryConfig{
		MaxTotalUnreadyPercentage: 10,
		OkTotalUnreadyCount:       1,
	})

	now := time.Now()

	clusterstate.RegisterScaleDown(&ScaleDownRequest{
		NodeGroupName:      "ng1",
		NodeName:           "ng1-1",
		ExpectedDeleteTime: now.Add(time.Minute),
		Time:               now,
	})
	assert.Equal(t, 1, len(clusterstate.scaleDownRequests))
	clusterstate.cleanUp(now.Add(5 * time.Minute))
	assert.Equal(t, 0, len(clusterstate.scaleDownRequests))
}

func TestUpcomingNodes(t *testing.T) {
	provider := testprovider.NewTestCloudProvider(nil, nil)
	now := time.Now()

	// 6 nodes are expected to come.
	ng1_1 := BuildTestNode("ng1-1", 1000, 1000)
	SetNodeReadyState(ng1_1, true, now.Add(-time.Minute))
	provider.AddNodeGroup("ng1", 1, 10, 7)
	provider.AddNode("ng1", ng1_1)

	// One node is expected to come. One node is unready for the long time
	// but this should not make any differnece.
	ng2_1 := BuildTestNode("ng2-1", 1000, 1000)
	SetNodeReadyState(ng2_1, false, now.Add(-time.Minute))
	provider.AddNodeGroup("ng2", 1, 10, 2)
	provider.AddNode("ng2", ng2_1)

	// Two nodes are expected to come. One is just being started for the first time,
	// the other one is not there yet.
	ng3_1 := BuildTestNode("ng3-1", 1000, 1000)
	SetNodeReadyState(ng3_1, false, now.Add(-time.Minute))
	ng3_1.CreationTimestamp = metav1.Time{Time: now.Add(-time.Minute)}
	provider.AddNodeGroup("ng3", 1, 10, 2)
	provider.AddNode("ng3", ng3_1)

	// Nothing should be added here.
	ng4_1 := BuildTestNode("ng4-1", 1000, 1000)
	SetNodeReadyState(ng4_1, false, now.Add(-time.Minute))
	provider.AddNodeGroup("ng4", 1, 10, 1)
	provider.AddNode("ng4", ng4_1)

	assert.NotNil(t, provider)
	clusterstate := NewClusterStateRegistry(provider, ClusterStateRegistryConfig{
		MaxTotalUnreadyPercentage: 10,
		OkTotalUnreadyCount:       1,
	})
	err := clusterstate.UpdateNodes([]*apiv1.Node{ng1_1, ng2_1, ng3_1, ng4_1}, now)
	assert.NoError(t, err)

	upcomingNodes := clusterstate.GetUpcomingNodes()
	assert.Equal(t, 6, upcomingNodes["ng1"])
	assert.Equal(t, 1, upcomingNodes["ng2"])
	assert.Equal(t, 2, upcomingNodes["ng3"])
	assert.NotContains(t, upcomingNodes, "ng4")
}

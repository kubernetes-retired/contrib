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

package aws

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiv1 "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/client/clientset_generated/clientset/fake"
)

type AutoScalingMock struct {
	mock.Mock
}

type EC2Mock struct {
	mock.Mock
}

func (e *EC2Mock) DescribeInstances(input *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
	return &ec2.DescribeInstancesOutput{
		Reservations: []*ec2.Reservation{
			{Instances: []*ec2.Instance{
				{
					InstanceId: aws.String("test-instance-id"),
					Tags: []*ec2.Tag{{
						Key:   aws.String("aws:autoscaling:groupName"),
						Value: aws.String("test-asg"),
					},
					},
				},
			},
			},
		},
	}, nil
}

func (a *AutoScalingMock) DescribeAutoScalingGroups(input *autoscaling.DescribeAutoScalingGroupsInput) (*autoscaling.DescribeAutoScalingGroupsOutput, error) {
	args := a.Called(input)
	return args.Get(0).(*autoscaling.DescribeAutoScalingGroupsOutput), nil
}

func (a *AutoScalingMock) SetDesiredCapacity(input *autoscaling.SetDesiredCapacityInput) (*autoscaling.SetDesiredCapacityOutput, error) {
	args := a.Called(input)
	return args.Get(0).(*autoscaling.SetDesiredCapacityOutput), nil
}

func (a *AutoScalingMock) TerminateInstanceInAutoScalingGroup(input *autoscaling.TerminateInstanceInAutoScalingGroupInput) (*autoscaling.TerminateInstanceInAutoScalingGroupOutput, error) {
	args := a.Called(input)
	return args.Get(0).(*autoscaling.TerminateInstanceInAutoScalingGroupOutput), nil
}

var testAwsManager = &AwsManager{
	asgs:               make([]*asgInformation, 0),
	autoscalingService: &AutoScalingMock{},
	ec2Service:         &EC2Mock{},
	asgCache:           make(map[AwsRef]*Asg),
}

func testProvider(t *testing.T, m *AwsManager) *AwsCloudProvider {
	interval, _ := time.ParseDuration("10s")
	provider, err := BuildAwsCloudProvider(m, nil, nil, false, interval)
	assert.NoError(t, err)
	return provider
}

func TestBuildAwsCloudProvider(t *testing.T) {
	interval, _ := time.ParseDuration("10s")
	m := testAwsManager
	_, err := BuildAwsCloudProvider(m, []string{"bad spec"}, nil, false, interval)
	assert.Error(t, err)

	_, err = BuildAwsCloudProvider(m, nil, nil, false, interval)
	assert.NoError(t, err)
}

func TestAddNodeGroup(t *testing.T) {
	provider := testProvider(t, testAwsManager)
	err := provider.addNodeGroup("bad spec")
	assert.Error(t, err)
	assert.Equal(t, len(provider.asgs), 0)

	err = provider.addNodeGroup("1:5:test-asg")
	assert.NoError(t, err)
	assert.Equal(t, len(provider.asgs), 1)
}

func TestName(t *testing.T) {
	provider := testProvider(t, testAwsManager)
	assert.Equal(t, provider.Name(), "aws")
}

func TestNodeGroups(t *testing.T) {
	provider := testProvider(t, testAwsManager)
	assert.Equal(t, len(provider.NodeGroups()), 0)
	err := provider.addNodeGroup("1:5:test-asg")
	assert.NoError(t, err)
	assert.Equal(t, len(provider.NodeGroups()), 1)
}

func TestNodeGroupForNode(t *testing.T) {
	node := &apiv1.Node{
		Spec: apiv1.NodeSpec{
			ProviderID: "aws:///us-east-1a/test-instance-id",
		},
	}
	service := &AutoScalingMock{}
	ec2service := &EC2Mock{}
	m := &AwsManager{
		asgs:               make([]*asgInformation, 0),
		autoscalingService: service,
		ec2Service:         ec2service,
		asgCache:           make(map[AwsRef]*Asg),
	}
	name := "test-asg"
	var maxRecord int64 = 1
	service.On("DescribeAutoScalingGroups", &autoscaling.DescribeAutoScalingGroupsInput{AutoScalingGroupNames: []*string{&name}, MaxRecords: &maxRecord}).
		Return(&autoscaling.DescribeAutoScalingGroupsOutput{
			AutoScalingGroups: []*autoscaling.Group{
				{
					AutoScalingGroupName: aws.String("test-asg"),
					MinSize:              aws.Int64(1),
					MaxSize:              aws.Int64(10),
					DesiredCapacity:      aws.Int64(2),
					Instances: []*autoscaling.Instance{
						{
							InstanceId: aws.String("test-instance-id"),
						},
						{
							InstanceId: aws.String("second-test-instance-id"),
						},
					},
				},
			},
		})

	provider := testProvider(t, m)

	err := provider.addNodeGroup("1:5:test-asg")
	assert.NoError(t, err)
	group, err := provider.NodeGroupForNode(node)

	assert.NoError(t, err)
	assert.Equal(t, group.Id(), "test-asg")
	assert.Equal(t, group.MinSize(), 1)
	assert.Equal(t, group.MaxSize(), 5)

	// test node in cluster that is not in a group managed by cluster autoscaler
	nodeNotInGroup := &apiv1.Node{
		Spec: apiv1.NodeSpec{
			ProviderID: "aws:///us-east-1a/test-instance-id-not-in-group",
		},
	}

	group, err = provider.NodeGroupForNode(nodeNotInGroup)
	assert.NoError(t, err)
	assert.Nil(t, group)
}

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
	provider := testProvider(t, testAwsManager)
	err := provider.addNodeGroup("1:5:test-asg")
	assert.NoError(t, err)
	assert.Equal(t, len(provider.asgs), 1)
	assert.Equal(t, provider.asgs[0].MaxSize(), 5)
}

func TestMinSize(t *testing.T) {
	provider := testProvider(t, testAwsManager)
	err := provider.addNodeGroup("1:5:test-asg")
	assert.NoError(t, err)
	assert.Equal(t, len(provider.asgs), 1)
	assert.Equal(t, provider.asgs[0].MinSize(), 1)
}

func TestTargetSize(t *testing.T) {

	service := &AutoScalingMock{}
	ec2service := &EC2Mock{}
	m := &AwsManager{
		asgs:               make([]*asgInformation, 0),
		autoscalingService: service,
		ec2Service:         ec2service,
		asgCache:           make(map[AwsRef]*Asg),
	}
	name := "test-asg"
	var maxRecord int64 = 1
	service.On("DescribeAutoScalingGroups", &autoscaling.DescribeAutoScalingGroupsInput{AutoScalingGroupNames: []*string{&name}, MaxRecords: &maxRecord}).
		Return(&autoscaling.DescribeAutoScalingGroupsOutput{
			AutoScalingGroups: []*autoscaling.Group{
				{
					AutoScalingGroupName: aws.String("test-asg"),
					MinSize:              aws.Int64(1),
					MaxSize:              aws.Int64(10),
					DesiredCapacity:      aws.Int64(2),
					Instances: []*autoscaling.Instance{
						{
							InstanceId: aws.String("test-instance-id"),
						},
						{
							InstanceId: aws.String("second-test-instance-id"),
						},
					},
				},
			},
		})

	provider := testProvider(t, m)

	err := provider.addNodeGroup("1:5:test-asg")
	assert.NoError(t, err)
	targetSize, err := provider.asgs[0].TargetSize()
	assert.Equal(t, targetSize, 2)
	assert.NoError(t, err)
}

func TestIncreaseSize(t *testing.T) {
	service := &AutoScalingMock{}
	ec2service := &EC2Mock{}
	m := &AwsManager{
		asgs:               make([]*asgInformation, 0),
		autoscalingService: service,
		ec2Service:         ec2service,
		asgCache:           make(map[AwsRef]*Asg),
	}
	name := "test-asg"
	var maxRecord int64 = 1
	service.On("DescribeAutoScalingGroups", &autoscaling.DescribeAutoScalingGroupsInput{AutoScalingGroupNames: []*string{&name}, MaxRecords: &maxRecord}).
		Return(&autoscaling.DescribeAutoScalingGroupsOutput{
			AutoScalingGroups: []*autoscaling.Group{
				{
					AutoScalingGroupName: aws.String("test-asg"),
					MinSize:              aws.Int64(1),
					MaxSize:              aws.Int64(10),
					DesiredCapacity:      aws.Int64(2),
					Instances: []*autoscaling.Instance{
						{
							InstanceId: aws.String("test-instance-id"),
						},
						{
							InstanceId: aws.String("second-test-instance-id"),
						},
					},
				},
			},
		})

	provider := testProvider(t, m)

	err := provider.addNodeGroup("1:5:test-asg")
	assert.NoError(t, err)
	assert.Equal(t, len(provider.asgs), 1)

	service.On("SetDesiredCapacity", &autoscaling.SetDesiredCapacityInput{
		AutoScalingGroupName: aws.String(provider.asgs[0].Name),
		DesiredCapacity:      aws.Int64(3),
		HonorCooldown:        aws.Bool(false),
	}).Return(&autoscaling.SetDesiredCapacityOutput{})

	err = provider.asgs[0].IncreaseSize(1)
	assert.NoError(t, err)
	service.AssertNumberOfCalls(t, "SetDesiredCapacity", 1)
}

func TestBelongs(t *testing.T) {
	service := &AutoScalingMock{}
	ec2service := &EC2Mock{}
	m := &AwsManager{
		asgs:               make([]*asgInformation, 0),
		autoscalingService: service,
		ec2Service:         ec2service,
		asgCache:           make(map[AwsRef]*Asg),
	}
	name := "test-asg"
	var maxRecord int64 = 1
	service.On("DescribeAutoScalingGroups", &autoscaling.DescribeAutoScalingGroupsInput{AutoScalingGroupNames: []*string{&name}, MaxRecords: &maxRecord}).
		Return(&autoscaling.DescribeAutoScalingGroupsOutput{
			AutoScalingGroups: []*autoscaling.Group{
				{
					AutoScalingGroupName: aws.String("test-asg"),
					MinSize:              aws.Int64(1),
					MaxSize:              aws.Int64(10),
					DesiredCapacity:      aws.Int64(2),
					Instances: []*autoscaling.Instance{
						{
							InstanceId: aws.String("test-instance-id"),
						},
						{
							InstanceId: aws.String("second-test-instance-id"),
						},
					},
				},
			},
		})

	provider := testProvider(t, m)

	err := provider.addNodeGroup("1:5:test-asg")
	assert.NoError(t, err)

	invalidNode := &apiv1.Node{
		Spec: apiv1.NodeSpec{
			ProviderID: "aws:///us-east-1a/invalid-instance-id",
		},
	}
	_, err = provider.asgs[0].Belongs(invalidNode)
	assert.Error(t, err)

	validNode := &apiv1.Node{
		Spec: apiv1.NodeSpec{
			ProviderID: "aws:///us-east-1a/test-instance-id",
		},
	}
	belongs, err := provider.asgs[0].Belongs(validNode)
	assert.Equal(t, belongs, true)
	assert.NoError(t, err)
}

func TestDeleteNodes(t *testing.T) {
	service := &AutoScalingMock{}
	ec2service := &EC2Mock{}
	m := &AwsManager{
		asgs:               make([]*asgInformation, 0),
		autoscalingService: service,
		ec2Service:         ec2service,
		asgCache:           make(map[AwsRef]*Asg),
	}
	name := "test-asg"
	var maxRecord int64 = 1
	service.On("DescribeAutoScalingGroups", &autoscaling.DescribeAutoScalingGroupsInput{AutoScalingGroupNames: []*string{&name}, MaxRecords: &maxRecord}).
		Return(&autoscaling.DescribeAutoScalingGroupsOutput{
			AutoScalingGroups: []*autoscaling.Group{
				{
					AutoScalingGroupName: aws.String("test-asg"),
					MinSize:              aws.Int64(10),
					MaxSize:              aws.Int64(100),
					DesiredCapacity:      aws.Int64(2),
					Instances: []*autoscaling.Instance{
						{
							InstanceId: aws.String("test-instance-id"),
						},
						{
							InstanceId: aws.String("second-test-instance-id"),
						},
					},
				},
			},
		})

	service.On("TerminateInstanceInAutoScalingGroup", &autoscaling.TerminateInstanceInAutoScalingGroupInput{
		InstanceId:                     aws.String("test-instance-id"),
		ShouldDecrementDesiredCapacity: aws.Bool(true),
	}).Return(&autoscaling.TerminateInstanceInAutoScalingGroupOutput{
		Activity: &autoscaling.Activity{Description: aws.String("Deleted instance")},
	})

	provider := testProvider(t, m)

	err := provider.addNodeGroup("1:5:test-asg")
	assert.NoError(t, err)

	node := &apiv1.Node{
		Spec: apiv1.NodeSpec{
			ProviderID: "aws:///us-east-1a/test-instance-id",
		},
	}
	err = provider.asgs[0].DeleteNodes([]*apiv1.Node{node})
	assert.NoError(t, err)
	service.AssertNumberOfCalls(t, "TerminateInstanceInAutoScalingGroup", 1)
}

func TestId(t *testing.T) {
	provider := testProvider(t, testAwsManager)
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

func TestAutoDiscoverASG(t *testing.T) {
	service := &AutoScalingMock{}
	ec2service := &EC2Mock{}
	m := &AwsManager{
		asgs:               make([]*asgInformation, 0),
		autoscalingService: service,
		ec2Service:         ec2service,
		asgCache:           make(map[AwsRef]*Asg),
	}
	name := "test-asg"
	service.On("DescribeAutoScalingGroups", &autoscaling.DescribeAutoScalingGroupsInput{AutoScalingGroupNames: []*string{&name}}).
		Return(&autoscaling.DescribeAutoScalingGroupsOutput{
			AutoScalingGroups: []*autoscaling.Group{
				{
					AutoScalingGroupName: aws.String("test-asg"),
					MinSize:              aws.Int64(1),
					MaxSize:              aws.Int64(10),
					DesiredCapacity:      aws.Int64(2),
					Instances: []*autoscaling.Instance{
						{
							InstanceId: aws.String("test-instance-id"),
						},
						{
							InstanceId: aws.String("second-test-instance-id"),
						},
					},
				},
			},
		})

	provider := testProvider(t, m)

	node := "aws:///us-east-1a/test-instance-i"
	nl := []*string{&node}

	err := AutoDiscoverNodeGroup(provider, nl)
	assert.NoError(t, err)
	assert.Equal(t, 1, provider.asgs[0].MinSize())
	assert.Equal(t, 10, provider.asgs[0].MaxSize())
	assert.Equal(t, "test-asg", provider.asgs[0].Name)
}

func TestAutoDiscoverUpdatedASG(t *testing.T) {

	service := &AutoScalingMock{}
	ec2service := &EC2Mock{}
	m := &AwsManager{
		asgs:               make([]*asgInformation, 0),
		autoscalingService: service,
		ec2Service:         ec2service,
		asgCache:           make(map[AwsRef]*Asg),
	}
	nodeName := "aws:///us-east-1a/test-instance-i"
	asgName := "test-asg"
	var prevMax int64 = 10

	service.On("DescribeAutoScalingGroups", &autoscaling.DescribeAutoScalingGroupsInput{AutoScalingGroupNames: []*string{&asgName}}).
		Return(&autoscaling.DescribeAutoScalingGroupsOutput{
			AutoScalingGroups: []*autoscaling.Group{
				{
					AutoScalingGroupName: aws.String(asgName),
					MinSize:              aws.Int64(1),
					MaxSize:              aws.Int64(prevMax),
					DesiredCapacity:      aws.Int64(2),
					Instances: []*autoscaling.Instance{
						{
							InstanceId: aws.String("test-instance-id"),
						},
						{
							InstanceId: aws.String("second-test-instance-id"),
						},
					},
				},
			},
		}).Once()

	node1 := apiv1.Node{ObjectMeta: meta.ObjectMeta{Name: nodeName}}
	c := fake.NewSimpleClientset(&apiv1.NodeList{Items: []apiv1.Node{node1}})

	_, err := BuildAwsCloudProvider(m, nil, c, true, 1*time.Second)

	assert.NoError(t, err)

	key := "private-dns-name"
	ec2service.On("DescribeInstances", &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name:   &key,
				Values: []*string{&nodeName},
			},
		},
	}).Return(ec2.DescribeInstancesOutput{
		Reservations: []*ec2.Reservation{
			{Instances: []*ec2.Instance{
				{
					InstanceId: aws.String("test-instance-id"),
					Tags: []*ec2.Tag{{
						Key:   aws.String("aws:autoscaling:groupName"),
						Value: aws.String(asgName),
					},
					},
				},
			},
			},
		},
	})
	service.On("DescribeAutoScalingGroups", &autoscaling.DescribeAutoScalingGroupsInput{AutoScalingGroupNames: []*string{&asgName}}).
		Return(&autoscaling.DescribeAutoScalingGroupsOutput{
			AutoScalingGroups: []*autoscaling.Group{
				{
					AutoScalingGroupName: aws.String(asgName),
					MinSize:              aws.Int64(1),
					MaxSize:              aws.Int64(100),
					DesiredCapacity:      aws.Int64(2),
					Instances: []*autoscaling.Instance{
						{
							InstanceId: aws.String("test-instance-id"),
						},
						{
							InstanceId: aws.String("second-test-instance-id"),
						},
					},
				},
			},
		})
	//give the goroutine the time to read the new configuration
	time.Sleep(4 * time.Second)
	assert.NotEqual(t, int(prevMax), m.asgs[0].config.maxSize)
}

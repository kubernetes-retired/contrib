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
	"fmt"
	"sync"
	"time"

	"k8s.io/contrib/cluster-autoscaler/config"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/util/wait"
)

const (
	operationWaitTimeout  = 5 * time.Second
	operationPollInterval = 100 * time.Millisecond
)

// GceManager is handles gce communication and data caching.
type AwsManager struct {
	configs     []*config.ScalingConfig
	service     *autoscaling.AutoScaling
	configCache map[config.InstanceConfig]*config.ScalingConfig
	cacheMutex  sync.Mutex
}

// CreateAwsManager constructs gceManager object.
func CreateAwsManager(configs []*config.ScalingConfig) (*AwsManager, error) {
	service := autoscaling.New(session.New())
	manager := &AwsManager{
		configs:     configs,
		service:     service,
		configCache: map[config.InstanceConfig]*config.ScalingConfig{},
	}

	go wait.Forever(func() { manager.regenerateCacheIgnoreError() }, time.Hour)

	return manager, nil
}

// GetMigSize gets ASG size.
func (m *AwsManager) GetScalingGroupSize(sgConf *config.ScalingConfig) (int64, error) {
	params := &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []*string{aws.String(sgConf.Name)},
		MaxRecords:            aws.Int64(1),
	}
	resp, err := m.service.DescribeAutoScalingGroups(params)

	if err != nil {
		return -1, err
	}

	// TODO: check for nil pointers
	asg := *resp.AutoScalingGroups[0]
	return *asg.DesiredCapacity, nil
}

// SetMigSize sets ASG size.
func (m *AwsManager) SetScalingGroupSize(sgConf *config.ScalingConfig, size int64) error {
	params := &autoscaling.SetDesiredCapacityInput{
		AutoScalingGroupName: aws.String(sgConf.Name),
		DesiredCapacity:      aws.Int64(size),
		HonorCooldown:        aws.Bool(false),
	}
	// TODO implement waitForOp as it is on GCE
	// TODO do something with the response
	_, err := m.service.SetDesiredCapacity(params)

	if err != nil {
		return err
	}
	return nil
}

// func (m *AwsManager) waitForOp(operation *gce.Operation, project string, zone string) error {
// 	for start := time.Now(); time.Since(start) < operationWaitTimeout; time.Sleep(operationPollInterval) {
// 		glog.V(4).Infof("Waiting for operation %s %s %s", project, zone, operation.Name)
// 		if op, err := m.service.ZoneOperations.Get(project, zone, operation.Name).Do(); err == nil {
// 			glog.V(4).Infof("Operation %s %s %s status: %s", project, zone, operation.Name, op.Status)
// 			if op.Status == "DONE" {
// 				return nil
// 			}
// 		} else {
// 			glog.Warningf("Error while getting operation %s on %s: %v", operation.Name, operation.TargetLink, err)
// 		}
// 	}
// 	return fmt.Errorf("Timeout while waiting for operation %s on %s to complete.", operation.Name, operation.TargetLink)
// }

// DeleteInstances deletes the given instances. All instances must be controlled by the same ASG.
func (m *AwsManager) DeleteInstances(instances []*config.InstanceConfig) error {
	if len(instances) == 0 {
		return nil
	}
	commonAsg, err := m.GetScalingGroupForInstance(instances[0])
	if err != nil {
		return err
	}
	for _, instance := range instances {
		asg, err := m.GetScalingGroupForInstance(instance)
		if err != nil {
			return err
		}
		if asg != commonAsg {
			return fmt.Errorf("Connot delete instances which don't belong to the same ASG.")
		}
	}

	for _, instance := range instances {
		params := &autoscaling.TerminateInstanceInAutoScalingGroupInput{
			InstanceId:                     aws.String(instance.Name),
			ShouldDecrementDesiredCapacity: aws.Bool(true),
		}
		// TODO: do something with this response
		_, err := m.service.TerminateInstanceInAutoScalingGroup(params)
		if err != nil {
			return err
		}
	}

	return nil
}

// GetMigForInstance returns ScalingConfig of the given Instance
func (m *AwsManager) GetScalingGroupForInstance(instance *config.InstanceConfig) (*config.ScalingConfig, error) {
	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()
	if config, found := m.configCache[*instance]; found {
		return config, nil
	}
	if err := m.regenerateCache(); err != nil {
		return nil, fmt.Errorf("Error while looking for ASG for instance %+v, error: %v", *instance, err)
	}
	if config, found := m.configCache[*instance]; found {
		return config, nil
	}
	return nil, fmt.Errorf("Instance %+v does not belong to any known ASG", *instance)
}

func (m *AwsManager) regenerateCacheIgnoreError() {
	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()
	if err := m.regenerateCache(); err != nil {
		glog.Errorf("Error while regenerating Mig cache: %v", err)
	}
}

func (m *AwsManager) regenerateCache() error {
	newCache := map[config.InstanceConfig]*config.ScalingConfig{}

	for _, asg := range m.configs {
		glog.V(4).Infof("Regenerating ASG information for %s", asg.Name)
		params := &autoscaling.DescribeAutoScalingGroupsInput{
			AutoScalingGroupNames: []*string{aws.String(asg.Name)},
			MaxRecords:            aws.Int64(1),
		}
		groups, err := m.service.DescribeAutoScalingGroups(params)
		if err != nil {
			glog.V(4).Infof("Failed ASG info request for %s: %v", asg.Name, err)
			return err
		}
		// TODO: check for nil pointers
		group := *groups.AutoScalingGroups[0]

		for _, instance := range group.Instances {
			// TODO fewer queries
			params := &autoscaling.DescribeAutoScalingInstancesInput{
				InstanceIds: []*string{
					aws.String(*instance.InstanceId),
				},
				MaxRecords: aws.Int64(1),
			}
			resp, err := m.service.DescribeAutoScalingInstances(params)

			if err != nil {
				return err
			}
			details := *resp.AutoScalingInstances[0]
			newCache[config.InstanceConfig{Zone: *details.AvailabilityZone, Name: *instance.InstanceId}] = asg
		}
	}

	m.configCache = newCache
	return nil
}

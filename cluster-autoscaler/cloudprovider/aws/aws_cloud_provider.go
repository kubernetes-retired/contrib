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
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	aws_util "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/glog"
	"k8s.io/contrib/cluster-autoscaler/cloudprovider"
	apiv1 "k8s.io/kubernetes/pkg/api/v1"
	kube_client "k8s.io/kubernetes/pkg/client/clientset_generated/release_1_5"
)

// AwsCloudProvider implements CloudProvider interface.
type AwsCloudProvider struct {
	awsManager *AwsManager
	asgMutex   sync.Mutex
	asgs       []*Asg
}

// BuildAwsCloudProvider builds CloudProvider implementation for AWS.
func BuildAwsCloudProvider(awsManager *AwsManager, specs []string, kubeClient kube_client.Interface, autoDiscoverNodeGroup bool, scanInterval time.Duration) (*AwsCloudProvider, error) {
	aws := &AwsCloudProvider{
		awsManager: awsManager,
		asgs:       make([]*Asg, 0),
	}
	if autoDiscoverNodeGroup {
		go func() {
			for {
				select {
				case <-time.After(scanInterval):
					{
						selector := "master!=true"
						listAll := apiv1.ListOptions{LabelSelector: selector}
						nl, err := kubeClient.Core().Nodes().List(listAll)
						if err != nil {
							glog.Errorf("err: %s\n", err)
						}

						var nodesList []*string

						for _, n := range nl.Items {
							glog.V(4).Infof("Considering node: %s", n.Name)
							nodesList = append(nodesList, &n.Name)
						}
						AutoDiscoverNodeGroup(aws, nodesList)
					}
				}
			}
		}()
	} else {
		for _, spec := range specs {
			if err := aws.addNodeGroup(spec); err != nil {
				return nil, err
			}
		}
	}
	return aws, nil
}

//AutoDiscoverNodeGroup will auto update the autoscaler group fetching the information directly from the AWS API.
func AutoDiscoverNodeGroup(aws *AwsCloudProvider, nodesList []*string) error {
	params := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws_util.String("private-dns-name"),
				Values: nodesList,
			},
		},
	}
	instances, err := aws.awsManager.ec2Service.DescribeInstances(params)
	if err != nil {
		glog.Errorf("Error describing EC2 instances: %s\n", err)
		return err
	}

	//make a "uniq" on the names so that we don't consider autoscaling groups twice
	asgs := make(map[string]bool, 0)
	names := make([]*string, 0)
	for idx := range instances.Reservations {
		for _, inst := range instances.Reservations[idx].Instances {
			for _, tag := range inst.Tags {
				if *(tag.Key) == "aws:autoscaling:groupName" {
					if _, ok := asgs[*(tag.Value)]; !ok {
						names = append(names, tag.Value)
					}
					asgs[*(tag.Value)] = true
				}
			}
		}
	}

	request := &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: names,
	}
	asgGroups, err := aws.awsManager.autoscalingService.DescribeAutoScalingGroups(request)
	if err != nil {
		glog.Errorf("Error describing autoscaling groups: %s\n", err)
		return err
	}

	aws.asgMutex.Lock()
	defer aws.asgMutex.Unlock()
	aws.asgs = aws.asgs[:0]
	aws.awsManager.ClearAsgs()
	for _, group := range asgGroups.AutoScalingGroups {
		glog.V(4).Infof("Adding ASG: %s, with min: %d, max %d\n", *group.AutoScalingGroupName, *group.MinSize, *group.MaxSize)
		asg := Asg{
			awsManager: aws.awsManager,
			AwsRef:     AwsRef{Name: *group.AutoScalingGroupName},
			maxSize:    int(*group.MaxSize),
			minSize:    int(*group.MinSize),
		}
		aws.asgs = append(aws.asgs, &asg)
		aws.awsManager.RegisterAsg(&asg)
	}

	return nil
}

// addNodeGroup adds node group defined in string spec. Format:
// minNodes:maxNodes:asgName
func (aws *AwsCloudProvider) addNodeGroup(spec string) error {
	asg, err := buildAsg(spec, aws.awsManager)
	if err != nil {
		return err
	}
	aws.asgMutex.Lock()
	defer aws.asgMutex.Unlock()
	aws.asgs = append(aws.asgs, asg)
	aws.awsManager.RegisterAsg(asg)
	return nil
}

// Name returns name of the cloud provider.
func (aws *AwsCloudProvider) Name() string {
	return "aws"
}

// NodeGroups returns all node groups configured for this cloud provider.
func (aws *AwsCloudProvider) NodeGroups() []cloudprovider.NodeGroup {
	result := make([]cloudprovider.NodeGroup, 0, len(aws.asgs))
	aws.asgMutex.Lock()
	defer aws.asgMutex.Unlock()
	for _, asg := range aws.asgs {
		result = append(result, asg)
	}
	return result
}

// NodeGroupForNode returns the node group for the given node.
func (aws *AwsCloudProvider) NodeGroupForNode(node *apiv1.Node) (cloudprovider.NodeGroup, error) {
	ref, err := AwsRefFromProviderId(node.Spec.ProviderID)
	if err != nil {
		return nil, err
	}
	asg, err := aws.awsManager.GetAsgForInstance(ref)
	return asg, err
}

// AwsRef contains a reference to some entity in AWS/GKE world.
type AwsRef struct {
	Name string
}

// AwsRefFromProviderId creates InstanceConfig object from provider id which
// must be in format: aws:///zone/name
func AwsRefFromProviderId(id string) (*AwsRef, error) {
	validIdRegex := regexp.MustCompile(`^aws\:\/\/\/[-0-9a-z]*\/[-0-9a-z]*$`)
	if validIdRegex.FindStringSubmatch(id) == nil {
		return nil, fmt.Errorf("Wrong id: expected format aws:///<zone>/<name>, got %v", id)
	}
	splitted := strings.Split(id[7:], "/")
	return &AwsRef{
		Name: splitted[1],
	}, nil
}

// Asg implements NodeGroup interfrace.
type Asg struct {
	AwsRef

	awsManager *AwsManager

	minSize int
	maxSize int
}

// MaxSize returns maximum size of the node group.
func (asg *Asg) MaxSize() int {
	return asg.maxSize
}

// MinSize returns minimum size of the node group.
func (asg *Asg) MinSize() int {
	return asg.minSize
}

// TargetSize returns the current TARGET size of the node group. It is possible that the
// number is different from the number of nodes registered in Kuberentes.
func (asg *Asg) TargetSize() (int, error) {
	size, err := asg.awsManager.GetAsgSize(asg)
	return int(size), err
}

// IncreaseSize increases Asg size
func (asg *Asg) IncreaseSize(delta int) error {
	if delta <= 0 {
		return fmt.Errorf("size increase must be positive")
	}
	size, err := asg.awsManager.GetAsgSize(asg)
	if err != nil {
		return err
	}
	if int(size)+delta > asg.MaxSize() {
		return fmt.Errorf("size increase too large - desired:%d max:%d", int(size)+delta, asg.MaxSize())
	}
	return asg.awsManager.SetAsgSize(asg, size+int64(delta))
}

// Belongs returns true if the given node belongs to the NodeGroup.
func (asg *Asg) Belongs(node *apiv1.Node) (bool, error) {
	ref, err := AwsRefFromProviderId(node.Spec.ProviderID)
	if err != nil {
		return false, err
	}
	targetAsg, err := asg.awsManager.GetAsgForInstance(ref)
	if err != nil {
		return false, err
	}
	if targetAsg == nil {
		return false, fmt.Errorf("%s doesn't belong to a known asg", node.Name)
	}
	if targetAsg.Id() != asg.Id() {
		return false, nil
	}
	return true, nil
}

// DeleteNodes deletes the nodes from the group.
func (asg *Asg) DeleteNodes(nodes []*apiv1.Node) error {
	size, err := asg.awsManager.GetAsgSize(asg)
	if err != nil {
		return err
	}
	if int(size) <= asg.MinSize() {
		return fmt.Errorf("min size reached, nodes will not be deleted")
	}
	refs := make([]*AwsRef, 0, len(nodes))
	for _, node := range nodes {
		belongs, err := asg.Belongs(node)
		if err != nil {
			return err
		}
		if belongs != true {
			return fmt.Errorf("%s belongs to a different asg than %s", node.Name, asg.Id())
		}
		awsref, err := AwsRefFromProviderId(node.Spec.ProviderID)
		if err != nil {
			return err
		}
		refs = append(refs, awsref)
	}
	return asg.awsManager.DeleteInstances(refs)
}

// Id returns asg id.
func (asg *Asg) Id() string {
	return asg.Name
}

// Debug returns a debug string for the Asg.
func (asg *Asg) Debug() string {
	return fmt.Sprintf("%s (%d:%d)", asg.Id(), asg.MinSize(), asg.MaxSize())
}

func buildAsg(value string, awsManager *AwsManager) (*Asg, error) {
	tokens := strings.SplitN(value, ":", 3)
	if len(tokens) != 3 {
		return nil, fmt.Errorf("wrong nodes configuration: %s", value)
	}

	asg := Asg{
		awsManager: awsManager,
	}
	if size, err := strconv.Atoi(tokens[0]); err == nil {
		if size <= 0 {
			return nil, fmt.Errorf("min size must be >= 1")
		}
		asg.minSize = size
	} else {
		return nil, fmt.Errorf("failed to set min size: %s, expected integer", tokens[0])
	}

	if size, err := strconv.Atoi(tokens[1]); err == nil {
		if size < asg.minSize {
			return nil, fmt.Errorf("max size must be greater or equal to min size")
		}
		asg.maxSize = size
	} else {
		return nil, fmt.Errorf("failed to set max size: %s, expected integer", tokens[1])
	}

	if tokens[2] == "" {
		return nil, fmt.Errorf("asg name must not be blank: got error: %v", tokens[2])
	}

	asg.Name = tokens[2]
	return &asg, nil
}

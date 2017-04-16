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
	"strings"

	"errors"
	"k8s.io/contrib/cluster-autoscaler/cloudprovider"
	"k8s.io/contrib/cluster-autoscaler/config/dynamic"
	apiv1 "k8s.io/kubernetes/pkg/api/v1"
)

// awsCloudProvider implements CloudProvider interface.
type awsCloudProvider struct {
	awsManager *AwsManager
	asgs       []*Asg
}

// autoDiscoveringProvider implements CloudProvider interface.
type autoDiscoveringProvider struct {
	*awsCloudProvider
}

// BuildAwsCloudProvider builds CloudProvider implementation for AWS.
func BuildAwsCloudProvider(awsManager *AwsManager, discoveryOpts cloudprovider.NodeGroupDiscoveryOptions) (cloudprovider.CloudProvider, error) {
	if len(discoveryOpts.NodeGroupSpecs) > 0 {
		return buildStaticallyDiscoveringProvider(awsManager, discoveryOpts.NodeGroupSpecs)
	}
	if discoveryOpts.NodeGroupAutoDiscoverySpec != "" {
		return buildAutoDiscoveringProvider(awsManager, discoveryOpts.NodeGroupAutoDiscoverySpec)
	}
	return nil, errors.New("Failed to build an aws cloud provider: Either node group specs or node group auto discovery spec must be specified")
}

func buildAutoDiscoveringProvider(awsManager *AwsManager, spec string) (*autoDiscoveringProvider, error) {
	tokens := strings.Split(spec, ":")
	if len(tokens) != 2 {
		return nil, fmt.Errorf("Invalid node group auto discovery spec specified: %s", spec)
	}
	discoverer := tokens[0]
	if discoverer != "asg" {
		return nil, fmt.Errorf("Unsupported discoverer specified: %s", discoverer)
	}
	param := tokens[1]
	paramTokens := strings.Split(param, "=")
	parameterKey := paramTokens[0]
	if parameterKey != "tag" {
		return nil, fmt.Errorf("Unsupported parameter key specified for discoverer \"%s\": %s", discoverer, parameterKey)
	}
	tag := paramTokens[1]
	if tag == "" {
		return nil, errors.New("Invalid ASG tag for auto discovery specified: ASG tag must not be empty")
	}
	underlying := &awsCloudProvider{
		awsManager: awsManager,
		asgs:       make([]*Asg, 0),
	}
	asgs, err := awsManager.getAutoscalingGroupsByTag(tag)
	if err != nil {
		return nil, fmt.Errorf("Failed to get ASGs: %v", err)
	}

	aws := &autoDiscoveringProvider{
		awsCloudProvider: underlying,
	}
	for _, asg := range asgs {
		aws.addAsg(buildAsg(aws.awsManager, int(*asg.MinSize), int(*asg.MaxSize), *asg.AutoScalingGroupName))
	}

	return aws, nil
}

func buildStaticallyDiscoveringProvider(awsManager *AwsManager, specs []string) (*awsCloudProvider, error) {
	aws := &awsCloudProvider{
		awsManager: awsManager,
		asgs:       make([]*Asg, 0),
	}
	for _, spec := range specs {
		if err := aws.addNodeGroup(spec); err != nil {
			return nil, err
		}
	}
	return aws, nil
}

// addNodeGroup adds node group defined in string spec. Format:
// minNodes:maxNodes:asgName
func (aws *awsCloudProvider) addNodeGroup(spec string) error {
	asg, err := buildAsgFromSpec(spec, aws.awsManager)
	if err != nil {
		return err
	}
	aws.addAsg(asg)
	return nil
}

// addAsg adds and registers an asg to this cloud provider
func (aws *awsCloudProvider) addAsg(asg *Asg) {
	aws.asgs = append(aws.asgs, asg)
	aws.awsManager.RegisterAsg(asg)
}

// Name returns name of the cloud provider.
func (aws *awsCloudProvider) Name() string {
	return "aws"
}

// NodeGroups returns all node groups configured for this cloud provider.
func (aws *awsCloudProvider) NodeGroups() []cloudprovider.NodeGroup {
	result := make([]cloudprovider.NodeGroup, 0, len(aws.asgs))
	for _, asg := range aws.asgs {
		result = append(result, asg)
	}
	return result
}

// NodeGroupForNode returns the node group for the given node.
func (aws *awsCloudProvider) NodeGroupForNode(node *apiv1.Node) (cloudprovider.NodeGroup, error) {
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

// DecreaseTargetSize decreases the target size of the node group. This function
// doesn't permit to delete any existing node and can be used only to reduce the
// request for new nodes that have not been yet fulfilled. Delta should be negative.
// It is assumed that cloud provider will not delete the existing nodes if the size
// when there is an option to just decrease the target.
func (asg *Asg) DecreaseTargetSize(delta int) error {
	if delta >= 0 {
		return fmt.Errorf("size decrease size must be negative")
	}
	size, err := asg.awsManager.GetAsgSize(asg)
	if err != nil {
		return err
	}
	nodes, err := asg.awsManager.GetAsgNodes(asg)
	if err != nil {
		return err
	}
	if int(size)+delta < len(nodes) {
		return fmt.Errorf("attempt to delete existing nodes targetSize:%d delta:%d existingNodes: %d",
			size, delta, len(nodes))
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

// Nodes returns a list of all nodes that belong to this node group.
func (asg *Asg) Nodes() ([]string, error) {
	return asg.awsManager.GetAsgNodes(asg)
}

func buildAsgFromSpec(value string, awsManager *AwsManager) (*Asg, error) {
	spec, err := dynamic.SpecFromString(value)

	if err != nil {
		return nil, fmt.Errorf("failed to parse node group spec: %v", err)
	}

	asg := buildAsg(awsManager, spec.MinSize, spec.MaxSize, spec.Name)

	return asg, nil
}

func buildAsg(awsManager *AwsManager, minSize int, maxSize int, name string) *Asg {
	return &Asg{
		awsManager: awsManager,
		minSize:    minSize,
		maxSize:    maxSize,
		AwsRef: AwsRef{
			Name: name,
		},
	}
}

package openstack_heat

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"k8s.io/contrib/cluster-autoscaler/cloudprovider"
	kube_api "k8s.io/kubernetes/pkg/api"
)

type OpenstackHeatProvider struct {
	heatManager    *HeatManager
	resourceGroups []*HeatResourceGroup
}

func BuildOpenstackHeatCloudProvider(heatManager *HeatManager, specs []string) (*OpenstackHeatProvider, error) {
	provider := &OpenstackHeatProvider{
		heatManager:    heatManager,
		resourceGroups: make([]*HeatResourceGroup, 0),
	}
	for _, spec := range specs {
		if err := provider.addResourceGroup(spec); err != nil {
			return nil, fmt.Errorf("could not register resource group with spec %s: %v", spec, err)
		}
	}
	return provider, nil
}

func (provider *OpenstackHeatProvider) addResourceGroup(spec string) error {
	rscGroup, err := provider.buildGroupFromSpec(spec)
	if err != nil {
		return fmt.Errorf("could not parse spec for node group: %v", err)
	}

	err = provider.heatManager.RegisterResourceGroup(rscGroup)
	if err != nil {
		return err
	}

	provider.resourceGroups = append(provider.resourceGroups, rscGroup)

	return nil
}

// the spec is min:max:resource-group:stack-name:size-param
func (provider *OpenstackHeatProvider) buildGroupFromSpec(spec string) (*HeatResourceGroup, error) {
	tokens := strings.SplitN(spec, ":", 5)
	rscGroup := &HeatResourceGroup{heatManager: provider.heatManager}

	if len(tokens) != 5 {
		return nil, errors.New("group spec is not complete, should contain: min:max:resource-group:stack-name:size-param")
	}

	minSize, err := strconv.Atoi(tokens[0])
	if err != nil {
		return nil, fmt.Errorf("failed to set min size: %s, expected integer", tokens[0])
	}
	if minSize <= 0 {
		return nil, errors.New("min size must be >= 1")
	}

	maxSize, err := strconv.Atoi(tokens[1])
	if err != nil {
		return nil, fmt.Errorf("failed to set max size: %s, expected integer", tokens[1])
	}
	if maxSize < rscGroup.minSize {
		return nil, errors.New("max size must be greater or equal to min size")
	}

	rscGroup.minSize = minSize
	rscGroup.maxSize = maxSize
	rscGroup.name = tokens[2]
	rscGroup.stackName = tokens[3]
	rscGroup.sizeParamName = tokens[4]
	return rscGroup, nil

}

func (provider *OpenstackHeatProvider) Name() string {
	return "openstack-heat"
}

func (provider *OpenstackHeatProvider) NodeGroups() []cloudprovider.NodeGroup {
	result := make([]cloudprovider.NodeGroup, 0, len(provider.resourceGroups))
	for _, resourceGrp := range provider.resourceGroups {
		result = append(result, resourceGrp)
	}
	return result
}

func (provider *OpenstackHeatProvider) NodeGroupForNode(node *kube_api.Node) (cloudprovider.NodeGroup, error) {
	id, err := extractInstanceID(node.Spec.ProviderID)
	if err != nil {
		return nil, err
	}
	resourceGroup, err := provider.heatManager.GetGroupForInstance(id)
	return resourceGroup, err
}

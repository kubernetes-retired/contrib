package openstack_heat

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gophercloud/gophercloud/openstack/orchestration/v1/stackresources"

	kube_api "k8s.io/kubernetes/pkg/api"
	provider_openstack "k8s.io/kubernetes/pkg/cloudprovider/providers/openstack"
)

type HeatResourceGroup struct {
	heatManager   *HeatManager
	heatResource  *stackresources.Resource
	name          string
	stackName     string
	stackID       string
	sizeParamName string
	minSize       int
	maxSize       int
}

func (resourceGrp *HeatResourceGroup) MaxSize() int {
	return resourceGrp.maxSize
}

func (resourceGrp *HeatResourceGroup) MinSize() int {
	return resourceGrp.minSize
}

func (resourceGrp *HeatResourceGroup) TargetSize() (int, error) {
	size, err := resourceGrp.heatManager.GetResourceGroupSize(resourceGrp)
	return int(size), err
}

func (resourceGrp *HeatResourceGroup) IncreaseSize(delta int) error {
	if delta <= 0 {
		return errors.New("size increase must be positive")
	}
	size, err := resourceGrp.heatManager.GetResourceGroupSize(resourceGrp)
	if err != nil {
		return err
	}
	if int(size)+delta > resourceGrp.MaxSize() {
		return fmt.Errorf("size increase too large - desired:%d max:%d", int(size)+delta, resourceGrp.MaxSize())
	}
	return resourceGrp.heatManager.SetResourceGroupSize(resourceGrp, size+int64(delta))
}

func (resourceGrp *HeatResourceGroup) DeleteNodes(nodes []*kube_api.Node) error {
	size, err := resourceGrp.heatManager.GetResourceGroupSize(resourceGrp)
	if err != nil {
		return fmt.Errorf("error when deleting nodes, retrieving size of group %s failed: %v", resourceGrp.name, err)
	}
	if int(size) <= resourceGrp.MinSize() {
		return errors.New("min size reached, nodes will not be deleted")
	}
	toBeDeleted := make([]string, 0)
	for _, node := range nodes {
		belongs, err := resourceGrp.Contains(node)
		if err != nil {
			return fmt.Errorf("failed to check membership of node %s in group %s: %v", node.Name, resourceGrp.name, err)
		}
		if !belongs {
			return fmt.Errorf("%s belongs to a different mig than %s", node.Name, resourceGrp.Id())
		}
		instanceID, err := extractInstanceID(node.Spec.ProviderID)
		if err != nil {
			return fmt.Errorf("node %s's cloud provider ID is malformed: %v", node.Name, err)
		}
		toBeDeleted = append(toBeDeleted, instanceID)
	}
	return resourceGrp.heatManager.DeleteInstances(toBeDeleted)
}

func (resourceGrp *HeatResourceGroup) Contains(node *kube_api.Node) (bool, error) {
	instanceID, err := extractInstanceID(node.Spec.ProviderID)
	if err != nil {
		return false, err
	}
	targetResourceGrp, err := resourceGrp.heatManager.GetGroupForInstance(instanceID)
	if err != nil {
		return false, err
	}
	if targetResourceGrp == nil {
		return false, fmt.Errorf("%s doesn't belong to a known resource group", node.Name)
	}
	// TODO: use this when I enable IDs for the resource groups
	if targetResourceGrp.Id() != targetResourceGrp.Id() {
		return false, nil
	}
	return true, nil
}

func (resourceGrp *HeatResourceGroup) Id() string {
	// TODO: generate the openstack URL for this resource group and use it as ID
	return resourceGrp.name
}

func (resourceGrp *HeatResourceGroup) Debug() string {
	return fmt.Sprintf("%s (%d:%d)", resourceGrp.Id(), resourceGrp.MinSize(), resourceGrp.MaxSize())
}

func extractInstanceID(providerID string) (string, error) {
	offset := len(provider_openstack.ProviderName + ":///")
	splitted := strings.Split(providerID[offset:], "/")
	if len(splitted) != 1 {
		return "", fmt.Errorf("node provider ID %s should be of the form 'openstack://<instance id>', but is not", providerID)
	}
	return splitted[0], nil
}

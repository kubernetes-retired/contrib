package azure

import (
	"fmt"
	"strconv"
	"strings"

	"k8s.io/contrib/cluster-autoscaler/cloudprovider"
	kube_api "k8s.io/kubernetes/pkg/api"
)

// AwsCloudProvider implements CloudProvider interface.
type AzureCloudProvider struct {
	azureManager *AzureManager
	scaleSets    []*ScaleSet
}

func BuildAzureCloudProvider(azureManager *AzureManager, specs []string) (*AzureCloudProvider, error) {
	azure := &AzureCloudProvider{
		azureManager: azureManager,
	}
	for _, spec := range specs {
		if err := azure.addNodeGroup(spec); err != nil {
			return nil, err
		}
	}

	return azure, nil
}

// addNodeGroup adds node group defined in string spec. Format:
// minNodes:maxNodes:scaleSetName
func (azure *AzureCloudProvider) addNodeGroup(spec string) error {
	scaleSet, err := buildScaleSet(spec, azure.azureManager)
	if err != nil {
		return err
	}
	azure.scaleSets = append(azure.scaleSets, scaleSet)
	azure.azureManager.RegisterScaleSet(scaleSet)
	return nil
}

// Name returns name of the cloud provider.
func (azure *AzureCloudProvider) Name() string {
	return "azure"
}

// NodeGroups returns all node groups configured for this cloud provider.
func (azure *AzureCloudProvider) NodeGroups() []cloudprovider.NodeGroup {
	result := make([]cloudprovider.NodeGroup, 0, len(azure.scaleSets))
	for _, scaleSet := range azure.scaleSets {
		result = append(result, scaleSet)
	}
	return result
}

// NodeGroupForNode returns the node group for the given node.
func (azure *AzureCloudProvider) NodeGroupForNode(node *kube_api.Node) (cloudprovider.NodeGroup, error) {
	fmt.Printf("Searching for node group for the node: %s, %s\n", node.Spec.ExternalID, node.Spec.ProviderID)
	ref := &AzureRef{
		Subscription:  azure.azureManager.subscription,
		ResourceGroup: azure.azureManager.resourceGroupName,
		Name:          node.Spec.ProviderID,
	}

	scaleSet, err := azure.azureManager.GetScaleSetForInstance(ref)

	return scaleSet, err
}

// AzureRef contains a reference to some entity in Azure world.
type AzureRef struct {
	Subscription  string
	ResourceGroup string
	Name          string
}

func (m *AzureRef) GetKey() string {
	return m.Name
}

// AzureRefFromProviderId creates InstanceConfig object from provider id which
// must be in format: azure:///resourceGroupName/name
func AzureRefFromProviderId(id string) (*AzureRef, error) {
	splitted := strings.Split(id[9:], "/")
	if len(splitted) != 8 {
		panic("Wrong id: expected format azure:///subscriptions/<subscriptionId>/resourceGroups/<resourceGroup>/providers/Microsoft.Compute/virtualMachines/<instance-name>, got " + id)
		return nil, fmt.Errorf("Wrong id: expected format azure:///subscriptions/<subscriptionId>/resourceGroups/<resourceGroup>/providers/Microsoft.Compute/virtualMachines/<instance-name>, got %v", id)
	}
	return &AzureRef{
		Subscription:  splitted[1],
		ResourceGroup: splitted[3],
		Name:          splitted[len(splitted)-1],
	}, nil
}

// ScaleSet implements NodeGroup interface.
type ScaleSet struct {
	AzureRef

	azureManager *AzureManager
	minSize      int
	maxSize      int
}

// MaxSize returns maximum size of the node group.
func (scaleSet *ScaleSet) MinSize() int {
	return scaleSet.minSize
}

// MinSize returns minimum size of the node group.
func (scaleSet *ScaleSet) MaxSize() int {
	return scaleSet.maxSize
}

// TargetSize returns the current TARGET size of the node group. It is possible that the
// number is different from the number of nodes registered in Kuberentes.
func (asg *ScaleSet) TargetSize() (int, error) {
	size, err := asg.azureManager.GetScaleSetSize(asg)
	return int(size), err
}

// IncreaseSize increases Asg size
func (asg *ScaleSet) IncreaseSize(delta int) error {
	if delta <= 0 {
		return fmt.Errorf("size increase must be positive")
	}
	size, err := asg.azureManager.GetScaleSetSize(asg)
	if err != nil {
		return err
	}
	if int(size)+delta > asg.MaxSize() {
		return fmt.Errorf("size increase too large - desired:%d max:%d", int(size)+delta, asg.MaxSize())
	}
	return asg.azureManager.SetScaleSetSize(asg, size+int64(delta))
}

// Belongs returns true if the given node belongs to the NodeGroup.
func (scaleSet *ScaleSet) Belongs(node *kube_api.Node) (bool, error) {
	fmt.Printf("Check if node belongs to this scale set: scaleset:%v, node:%v\n", scaleSet, node)

	ref := &AzureRef{
		Subscription:  scaleSet.Subscription,
		ResourceGroup: scaleSet.ResourceGroup,
		Name:          node.Spec.ProviderID,
	}

	targetAsg, err := scaleSet.azureManager.GetScaleSetForInstance(ref)
	if err != nil {
		return false, err
	}
	if targetAsg == nil {
		return false, fmt.Errorf("%s doesn't belong to a known scale set", node.Name)
	}
	if targetAsg.Id() != scaleSet.Id() {
		return false, nil
	}
	return true, nil
}

// DeleteNodes deletes the nodes from the group.
func (scaleSet *ScaleSet) DeleteNodes(nodes []*kube_api.Node) error {
	fmt.Printf("Delete nodes requested: %v\n", nodes)
	size, err := scaleSet.azureManager.GetScaleSetSize(scaleSet)
	if err != nil {
		return err
	}
	if int(size) <= scaleSet.MinSize() {
		return fmt.Errorf("min size reached, nodes will not be deleted")
	}
	refs := make([]*AzureRef, 0, len(nodes))
	for _, node := range nodes {
		belongs, err := scaleSet.Belongs(node)
		if err != nil {
			return err
		}
		if belongs != true {
			return fmt.Errorf("%s belongs to a different asg than %s", node.Name, scaleSet.Id())
		}
		azureRef := &AzureRef{
			Subscription:  scaleSet.Subscription,
			ResourceGroup: scaleSet.ResourceGroup,
			Name:          node.Spec.ProviderID,
		}
		refs = append(refs, azureRef)
	}
	return scaleSet.azureManager.DeleteInstances(refs)
}

// Id returns ScaleSet id.
func (scaleSet *ScaleSet) Id() string {
	return scaleSet.Name
}

// Debug returns a debug string for the Scale Set.
func (asg *ScaleSet) Debug() string {
	return fmt.Sprintf("%s (%d:%d)", asg.Id(), asg.MinSize(), asg.MaxSize())
}

// Create ScaleSet from provided spec.
// spec is in the following format: min-size:max-size:scale-set-name.
func buildScaleSet(spec string, azureManager *AzureManager) (*ScaleSet, error) {
	tokens := strings.SplitN(spec, ":", 3)
	if len(tokens) != 3 {
		return nil, fmt.Errorf("wrong nodes configuration: %s", spec)
	}

	scaleSet := ScaleSet{
		azureManager: azureManager,
	}
	if size, err := strconv.Atoi(tokens[0]); err == nil {
		if size <= 0 {
			return nil, fmt.Errorf("min size must be >= 1")
		}
		scaleSet.minSize = size
	} else {
		return nil, fmt.Errorf("failed to set min size: %s, expected integer", tokens[0])
	}

	if size, err := strconv.Atoi(tokens[1]); err == nil {
		if size < scaleSet.minSize {
			return nil, fmt.Errorf("max size must be greater or equal to min size")
		}
		scaleSet.maxSize = size
	} else {
		return nil, fmt.Errorf("failed to set max size: %s, expected integer", tokens[1])
	}

	if tokens[2] == "" {
		return nil, fmt.Errorf("scale set name must not be blank: %s got error: %v", tokens[2])
	}

	scaleSet.Name = tokens[2]
	return &scaleSet, nil
}

package azure

import (
	"fmt"
	"testing"

	"github.com/Azure/azure-sdk-for-go/arm/compute"
	"github.com/Azure/go-autorest/autorest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	kube_api "k8s.io/kubernetes/pkg/api"
	"net/http"
)

// Mock for VirtualMachineScaleSetsClient
type VirtualMachineScaleSetsClientMock struct {
	mock.Mock
}

func (client *VirtualMachineScaleSetsClientMock) Get(resourceGroupName string,
	vmScaleSetName string) (result compute.VirtualMachineScaleSet, err error) {
	fmt.Printf("Called VirtualMachineScaleSetsClientMock.Get(%s,%s)\n", resourceGroupName, vmScaleSetName)
	capacity := int64(2)
	return compute.VirtualMachineScaleSet{

		Name: &vmScaleSetName,
		Sku: &compute.Sku{
			Capacity: &capacity,
		},
	}, nil
}

func (client *VirtualMachineScaleSetsClientMock) CreateOrUpdate(
	resourceGroupName string, name string, parameters compute.VirtualMachineScaleSet, cancel <-chan struct{}) (result autorest.Response, err error) {
	fmt.Printf("Called VirtualMachineScaleSetsClientMock.CreateOrUpdate(%s,%s)\n", resourceGroupName, name)
	return autorest.Response{}, nil
}

func (client *VirtualMachineScaleSetsClientMock) DeleteInstances(resourceGroupName string, vmScaleSetName string,
	vmInstanceIDs compute.VirtualMachineScaleSetVMInstanceRequiredIDs, cancel <-chan struct{}) (result autorest.Response, err error) {

	args := client.Called(resourceGroupName, vmScaleSetName, vmInstanceIDs, cancel)
	return args.Get(0).(autorest.Response), args.Error(1)
}

// Mock for VirtualMachineScaleSetVMsClient
type VirtualMachineScaleSetVMsClientMock struct {
	mock.Mock
}

func (m *VirtualMachineScaleSetVMsClientMock) List(resourceGroupName string, virtualMachineScaleSetName string, filter string, selectParameter string, expand string) (result compute.VirtualMachineScaleSetVMListResult, err error) {
	value := make([]compute.VirtualMachineScaleSetVM, 1)
	vmInstanceId := "test-instance-id"
	value[0] = compute.VirtualMachineScaleSetVM{
		InstanceID: &vmInstanceId,
	}

	return compute.VirtualMachineScaleSetVMListResult{
		Value: &value,
	}, nil

}

var testAzureManager = &AzureManager{
	scaleSets:        make([]*scaleSetInformation, 0),
	scaleSetClient:   &VirtualMachineScaleSetsClientMock{},
	scaleSetVmClient: &VirtualMachineScaleSetVMsClientMock{},
	scaleSetCache:    make(map[AzureRef]*ScaleSet),
}

func testProvider(t *testing.T, m *AzureManager) *AzureCloudProvider {
	provider, err := BuildAzureCloudProvider(m, nil)
	assert.NoError(t, err)
	return provider
}

func TestBuildAwsCloudProvider(t *testing.T) {
	m := testAzureManager
	_, err := BuildAzureCloudProvider(m, []string{"bad spec"})
	assert.Error(t, err)

	_, err = BuildAzureCloudProvider(m, nil)
	assert.NoError(t, err)
}

func TestAddNodeGroup(t *testing.T) {
	provider := testProvider(t, testAzureManager)
	err := provider.addNodeGroup("bad spec")
	assert.Error(t, err)
	assert.Equal(t, len(provider.scaleSets), 0)

	err = provider.addNodeGroup("1:5:test-asg")
	assert.NoError(t, err)
	assert.Equal(t, len(provider.scaleSets), 1)
}

func TestName(t *testing.T) {
	provider := testProvider(t, testAzureManager)
	assert.Equal(t, provider.Name(), "azure")
}

func TestNodeGroups(t *testing.T) {
	provider := testProvider(t, testAzureManager)
	assert.Equal(t, len(provider.NodeGroups()), 0)
	err := provider.addNodeGroup("1:5:test-asg")
	assert.NoError(t, err)
	assert.Equal(t, len(provider.NodeGroups()), 1)
}

func TestNodeGroupForNode(t *testing.T) {
	node := &kube_api.Node{
		Spec: kube_api.NodeSpec{
			ProviderID: "azure:///test-resource-group/test-instance-id",
		},
	}

	scaleSetVmClient := VirtualMachineScaleSetVMsClientMock{}

	var testAzureManager = &AzureManager{
		scaleSets:        make([]*scaleSetInformation, 0),
		scaleSetClient:   &VirtualMachineScaleSetsClientMock{},
		scaleSetVmClient: &scaleSetVmClient,
		scaleSetCache:    make(map[AzureRef]*ScaleSet),
	}

	provider := testProvider(t, testAzureManager)
	err := provider.addNodeGroup("1:5:test-asg")
	assert.NoError(t, err)

	assert.Equal(t, len(provider.scaleSets), 1)

	group, err := provider.NodeGroupForNode(node)

	assert.NoError(t, err)

	assert.Equal(t, group.Id(), "test-asg")
	assert.Equal(t, group.MinSize(), 1)
	assert.Equal(t, group.MaxSize(), 5)

	// test node in cluster that is not in a group managed by cluster autoscaler
	nodeNotInGroup := &kube_api.Node{
		Spec: kube_api.NodeSpec{
			ProviderID: "azure://test-resource-group/test-instance-id-not-in-group",
		},
	}

	group, err = provider.NodeGroupForNode(nodeNotInGroup)
	assert.NoError(t, err)
	assert.Nil(t, group)

}

func TestAwsRefFromProviderId(t *testing.T) {
	_, err := AzureRefFromProviderId("azure:///123")
	assert.Error(t, err)
	_, err = AzureRefFromProviderId("azure://test/rg/test-instance-id")
	assert.Error(t, err)

	awsRef, err := AzureRefFromProviderId("azure:///test-rg/i-260942b3")
	assert.NoError(t, err)
	assert.Equal(t, awsRef, &AzureRef{
		Name: "i-260942b3",
	})
}

func TestMaxSize(t *testing.T) {
	provider := testProvider(t, testAzureManager)
	err := provider.addNodeGroup("1:5:test-asg")
	assert.NoError(t, err)
	assert.Equal(t, len(provider.scaleSets), 1)
	assert.Equal(t, provider.scaleSets[0].MaxSize(), 5)
}

func TestMinSize(t *testing.T) {
	provider := testProvider(t, testAzureManager)
	err := provider.addNodeGroup("1:5:test-asg")
	assert.NoError(t, err)
	assert.Equal(t, len(provider.scaleSets), 1)
	assert.Equal(t, provider.scaleSets[0].MinSize(), 1)
}

func TestTargetSize(t *testing.T) {
	provider := testProvider(t, testAzureManager)
	err := provider.addNodeGroup("1:5:test-asg")
	assert.NoError(t, err)
	targetSize, err := provider.scaleSets[0].TargetSize()
	assert.Equal(t, targetSize, 2)
	assert.NoError(t, err)
}

func TestIncreaseSize(t *testing.T) {

	var testAzureManager = &AzureManager{

		scaleSets:        make([]*scaleSetInformation, 0),
		scaleSetClient:   &VirtualMachineScaleSetsClientMock{},
		scaleSetVmClient: &VirtualMachineScaleSetVMsClientMock{},
		scaleSetCache:    make(map[AzureRef]*ScaleSet),
	}

	provider := testProvider(t, testAzureManager)

	err := provider.addNodeGroup("1:5:test-asg")
	assert.NoError(t, err)
	assert.Equal(t, len(provider.scaleSets), 1)

	err = provider.scaleSets[0].IncreaseSize(1)
	assert.NoError(t, err)

}

func TestBelongs(t *testing.T) {

	var testAzureManager = &AzureManager{

		scaleSets:        make([]*scaleSetInformation, 0),
		scaleSetClient:   &VirtualMachineScaleSetsClientMock{},
		scaleSetVmClient: &VirtualMachineScaleSetVMsClientMock{},
		scaleSetCache:    make(map[AzureRef]*ScaleSet),
	}

	provider := testProvider(t, testAzureManager)
	err := provider.addNodeGroup("1:5:test-asg")
	assert.NoError(t, err)

	invalidNode := &kube_api.Node{
		Spec: kube_api.NodeSpec{
			ProviderID: "azure://eastus-1/invalid-instance-id",
		},
	}

	_, err = provider.scaleSets[0].Belongs(invalidNode)
	assert.Error(t, err)

	validNode := &kube_api.Node{
		Spec: kube_api.NodeSpec{
			ProviderID: "azure://eastus-1/test-instance-id",
		},
	}
	belongs, err := provider.scaleSets[0].Belongs(validNode)
	assert.Equal(t, belongs, true)
	assert.NoError(t, err)

}

func TestDeleteNodes(t *testing.T) {
	scaleSetClient := &VirtualMachineScaleSetsClientMock{}
	m := &AzureManager{
		scaleSets:        make([]*scaleSetInformation, 0),
		scaleSetClient:   scaleSetClient,
		scaleSetVmClient: &VirtualMachineScaleSetVMsClientMock{},
		scaleSetCache:    make(map[AzureRef]*ScaleSet),
	}

	//(resourceGroupName string, vmScaleSetName string,
	// vmInstanceIDs compute.VirtualMachineScaleSetVMInstanceRequiredIDs, cancel <-chan struct{})
	// (result autorest.Response, err error)
	//cancel := make(<-chan struct{})
	instanceIds := make([]string, 1)
	instanceIds[0] = "test-instance-id"

	//requiredIds := compute.VirtualMachineScaleSetVMInstanceRequiredIDs{
	//	InstanceIds: &instanceIds,
	//}
	response := autorest.Response{
		Response: &http.Response{
			Status: "OK",
		},
	}
	scaleSetClient.On("DeleteInstances", "", "test-asg", mock.Anything, mock.Anything).Return(response, nil)

	provider := testProvider(t, m)
	err := provider.addNodeGroup("1:5:test-asg")
	assert.NoError(t, err)

	node := &kube_api.Node{
		Spec: kube_api.NodeSpec{
			ProviderID: "azure://eastus-1/test-instance-id",
		},
	}
	err = provider.scaleSets[0].DeleteNodes([]*kube_api.Node{node})
	assert.NoError(t, err)
	scaleSetClient.AssertNumberOfCalls(t, "DeleteInstances", 1)
}

func TestId(t *testing.T) {
	provider := testProvider(t, testAzureManager)
	err := provider.addNodeGroup("1:5:test-asg")
	assert.NoError(t, err)
	assert.Equal(t, len(provider.scaleSets), 1)
	assert.Equal(t, provider.scaleSets[0].Id(), "test-asg")
}

func TestDebug(t *testing.T) {
	asg := ScaleSet{
		azureManager: testAzureManager,
		minSize:      5,
		maxSize:      55,
	}
	asg.Name = "test-scale-set"
	assert.Equal(t, asg.Debug(), "test-scale-set (5:55)")
}

func TestBuildAsg(t *testing.T) {
	_, err := buildScaleSet("a", nil)
	assert.Error(t, err)
	_, err = buildScaleSet("a:b:c", nil)
	assert.Error(t, err)
	_, err = buildScaleSet("1:", nil)
	assert.Error(t, err)
	_, err = buildScaleSet("1:2:", nil)
	assert.Error(t, err)

	_, err = buildScaleSet("-1:2:", nil)
	assert.Error(t, err)

	_, err = buildScaleSet("5:3:", nil)
	assert.Error(t, err)

	_, err = buildScaleSet("5:ddd:test-name", nil)
	assert.Error(t, err)

	asg, err := buildScaleSet("111:222:test-name", nil)
	assert.NoError(t, err)
	assert.Equal(t, 111, asg.MinSize())
	assert.Equal(t, 222, asg.MaxSize())
	assert.Equal(t, "test-name", asg.Name)
}

package azure

import (
	"io"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/arm/compute"
	"github.com/golang/glog"
	"gopkg.in/gcfg.v1"

	"fmt"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure"
	"k8s.io/kubernetes/pkg/util/wait"
	"log"
	"net/http"
	"os"
)

type scaleSetInformation struct {
	config   *ScaleSet
	basename string
}

type scaleSetClient interface {
	Get(resourceGroupName string, vmScaleSetName string) (result compute.VirtualMachineScaleSet, err error)
	CreateOrUpdate(resourceGroupName string, name string, parameters compute.VirtualMachineScaleSet, cancel <-chan struct{}) (result autorest.Response, err error)
	DeleteInstances(resourceGroupName string, vmScaleSetName string, vmInstanceIDs compute.VirtualMachineScaleSetVMInstanceRequiredIDs, cancel <-chan struct{}) (result autorest.Response, err error)
}

type scaleSetVMClient interface {
	List(resourceGroupName string, virtualMachineScaleSetName string, filter string, selectParameter string, expand string) (result compute.VirtualMachineScaleSetVMListResult, err error)
}

// AzureManager handles Azure communication and data caching.
type AzureManager struct {
	resourceGroupName string
	subscription      string
	scaleSetClient    scaleSetClient
	scaleSetVmClient  scaleSetVMClient

	scaleSets     []*scaleSetInformation
	scaleSetCache map[AzureRef]*ScaleSet

	cacheMutex sync.Mutex
}

// Config holds the configuration parsed from the --cloud-config flag
type Config struct {
	Cloud                      string `json:"cloud" yaml:"cloud"`
	TenantID                   string `json:"tenantId" yaml:"tenantId"`
	SubscriptionID             string `json:"subscriptionId" yaml:"subscriptionId"`
	ResourceGroup              string `json:"resourceGroup" yaml:"resourceGroup"`
	Location                   string `json:"location" yaml:"location"`
	VnetName                   string `json:"vnetName" yaml:"vnetName"`
	SubnetName                 string `json:"subnetName" yaml:"subnetName"`
	SecurityGroupName          string `json:"securityGroupName" yaml:"securityGroupName"`
	RouteTableName             string `json:"routeTableName" yaml:"routeTableName"`
	PrimaryAvailabilitySetName string `json:"primaryAvailabilitySetName" yaml:"primaryAvailabilitySetName"`

	AADClientID     string `json:"aadClientId" yaml:"aadClientId"`
	AADClientSecret string `json:"aadClientSecret" yaml:"aadClientSecret"`
	AADTenantID     string `json:"aadTenantId" yaml:"aadTenantId"`
}

// CreateAzureManager creates Azure Manager object to work with Azure.
func CreateAzureManager(configReader io.Reader) (*AzureManager, error) {
	subscriptionId := string("")
	resourceGroup := string("")
	tenantId := string("")
	clientId := string("")
	clientSecret := string("")
	var scaleSetAPI scaleSetClient = nil
	var scaleSetVmAPI scaleSetVMClient = nil
	if configReader != nil {
		var cfg Config
		if err := gcfg.ReadInto(&cfg, configReader); err != nil {
			glog.Errorf("Couldn't read config: %v", err)
			return nil, err
		}
		subscriptionId = cfg.SubscriptionID
		resourceGroup = cfg.ResourceGroup
		tenantId = cfg.AADTenantID
		clientId = cfg.AADClientID
		clientSecret = cfg.AADClientSecret

	} else {
		subscriptionId = os.Getenv("ARM_SUBSCRIPTION_ID")
		resourceGroup = os.Getenv("ARM_RESOURCE_GROUP")
		tenantId = os.Getenv("ARM_TENANT_ID")
		clientId = os.Getenv("ARM_CLIENT_ID")
		clientSecret = os.Getenv("ARM_CLIENT_SECRET")
	}

	if resourceGroup == "" {
		panic("Resource group not found")
	}

	if subscriptionId == "" {
		panic("Subscription ID not found")
	}

	if tenantId == "" {
		panic("Tenant ID not found.")
	}

	if clientId == "" {
		panic("ARM Client  ID not found")
	}

	if clientSecret == "" {
		panic("ARM Client Secret not found.")
	}

	glog.Infof("read configuration: %v", subscriptionId)

	spt, err := NewServicePrincipalTokenFromCredentials(tenantId, clientId, clientSecret, azure.PublicCloud.ServiceManagementEndpoint)
	if err != nil {
		panic(err)
	}

	scaleSetAPI = compute.NewVirtualMachineScaleSetsClient(subscriptionId)
	scaleSetsClient := scaleSetAPI.(compute.VirtualMachineScaleSetsClient)
	scaleSetsClient.Authorizer = spt

	scaleSetsClient.Sender = autorest.CreateSender(
		autorest.WithLogging(log.New(os.Stdout, "sdk-example: ", log.LstdFlags)))

	scaleSetsClient.RequestInspector = withInspection()
	scaleSetsClient.ResponseInspector = byInspecting()

	glog.Infof("Created scale set client with authorizer: %v", scaleSetsClient)

	scaleSetVmAPI = compute.NewVirtualMachineScaleSetVMsClient(subscriptionId)
	scaleSetVMsClient := scaleSetVmAPI.(compute.VirtualMachineScaleSetVMsClient)
	scaleSetVMsClient.Authorizer = spt

	scaleSetVMsClient.RequestInspector = withInspection()
	scaleSetVMsClient.ResponseInspector = byInspecting()

	glog.Infof("Created scale set vm client with authorizer: %v", scaleSetVMsClient)

	// Create Availability Sets Azure Client.
	manager := &AzureManager{
		subscription:      subscriptionId,
		resourceGroupName: resourceGroup,
		scaleSetClient:    scaleSetsClient,
		scaleSetVmClient:  scaleSetVMsClient,
		scaleSets:         make([]*scaleSetInformation, 0),
		scaleSetCache:     make(map[AzureRef]*ScaleSet),
	}

	go wait.Forever(func() {
		manager.cacheMutex.Lock()
		defer manager.cacheMutex.Unlock()
		if err := manager.regenerateCache(); err != nil {
			glog.Errorf("Error while regenerating AS cache: %v", err)
		}
	}, time.Hour)

	return manager, nil
}

// NewServicePrincipalTokenFromCredentials creates a new ServicePrincipalToken using values of the
// passed credentials map.
func NewServicePrincipalTokenFromCredentials(tenantId string, clientId string, clientSecret string, scope string) (*azure.ServicePrincipalToken, error) {
	oauthConfig, err := azure.PublicCloud.OAuthConfigForTenant(tenantId)
	if err != nil {
		panic(err)
	}
	return azure.NewServicePrincipalToken(*oauthConfig, clientId, clientSecret, scope)
}

func withInspection() autorest.PrepareDecorator {
	return func(p autorest.Preparer) autorest.Preparer {
		return autorest.PreparerFunc(func(r *http.Request) (*http.Request, error) {
			glog.Infof("Inspecting Request: %s %s\n", r.Method, r.URL)
			return p.Prepare(r)
		})
	}
}

func byInspecting() autorest.RespondDecorator {
	return func(r autorest.Responder) autorest.Responder {
		return autorest.ResponderFunc(func(resp *http.Response) error {
			glog.Infof("Inspecting Response: %s for %s %s\n", resp.Status, resp.Request.Method, resp.Request.URL)
			return r.Respond(resp)
		})
	}
}

// RegisterAvailabilitySet registers scale set in Azure Manager.
func (m *AzureManager) RegisterScaleSet(scaleSet *ScaleSet) {
	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()

	m.scaleSets = append(m.scaleSets,
		&scaleSetInformation{
			config:   scaleSet,
			basename: scaleSet.Name,
		})

}

// GetScaleSetSize gets Scale Set size.
func (m *AzureManager) GetScaleSetSize(asConfig *ScaleSet) (int64, error) {
	set, err := m.scaleSetClient.Get(m.resourceGroupName, asConfig.Name)
	if err != nil {
		return -1, err
	}
	return *set.Sku.Capacity, nil
}

// SetScaleSetSize sets ScaleSet size.
// TODO(uthark) it may worth to do PATCH request here.
func (m *AzureManager) SetScaleSetSize(asConfig *ScaleSet, size int64) error {
	op, err := m.scaleSetClient.Get(m.resourceGroupName, asConfig.Name)
	if err != nil {
		return err
	}
	op.Sku.Capacity = &size

	cancel := make(chan struct{})
	_, err = m.scaleSetClient.CreateOrUpdate(m.resourceGroupName, asConfig.Name, op, cancel)

	if err != nil {
		return err
	}
	return nil
}

// GetScaleSetForInstance returns ScaleSetConfig of the given Instance
func (m *AzureManager) GetScaleSetForInstance(instance *AzureRef) (*ScaleSet, error) {

	if m.resourceGroupName == "" {
		m.resourceGroupName = instance.ResourceGroup
	}

	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()
	if config, found := m.scaleSetCache[*instance]; found {
		return config, nil
	}
	fmt.Printf("Cache: %v, key: %v\n", m.scaleSetCache, instance)
	if err := m.regenerateCache(); err != nil {
		return nil, fmt.Errorf("Error while looking for ScaleSet for instance %+v, error: %v", *instance, err)
	}
	fmt.Printf("Cache after regeneration: %v, key: %v\n", m.scaleSetCache, *instance)
	if config, found := m.scaleSetCache[*instance]; found {
		return config, nil
	}
	// instance does not belong to any configured Scale Set
	return nil, nil
}

// DeleteInstances deletes the given instances. All instances must be controlled by the same ASG.
func (m *AzureManager) DeleteInstances(instances []*AzureRef) error {
	if len(instances) == 0 {
		return nil
	}
	commonAsg, err := m.GetScaleSetForInstance(instances[0])
	if err != nil {
		return err
	}
	for _, instance := range instances {
		asg, err := m.GetScaleSetForInstance(instance)
		if err != nil {
			return err
		}
		if asg != commonAsg {
			return fmt.Errorf("Cannot delete instances which don't belong to the same Scale Set.")
		}
	}

	instanceIds := make([]string, len(instances))
	for i, instance := range instances {
		instanceIds[i] = instance.Name
	}
	requiredIds := &compute.VirtualMachineScaleSetVMInstanceRequiredIDs{
		InstanceIds: &instanceIds,
	}
	cancel := make(chan struct{})
	resp, err := m.scaleSetClient.DeleteInstances(m.resourceGroupName, commonAsg.Name, *requiredIds, cancel)
	if err != nil {
		return err
	}

	glog.V(4).Infof(resp.Status)

	return nil
}

func (m *AzureManager) regenerateCache() error {
	newCache := make(map[AzureRef]*ScaleSet)

	fmt.Printf("Iterating over scaleSets: %d, %v\n", len(m.scaleSets), m.scaleSets)
	for _, sset := range m.scaleSets {

		glog.V(4).Infof("Regenerating Scale Set information for %s", sset.config.Name)

		scaleSet, err := m.scaleSetClient.Get(m.resourceGroupName, sset.config.Name)
		if err != nil {
			glog.V(4).Infof("Failed AS info request for %s: %v", sset.config.Name, err)
			return err
		}
		fmt.Printf("Got scaleSet: %s, %s, '%s' \n", m.resourceGroupName, sset.basename, *scaleSet.Name)
		sset.basename = *scaleSet.Name

		fmt.Printf("Calling list\n")
		result, err := m.scaleSetVmClient.List(m.resourceGroupName, sset.basename, "", "", "")

		fmt.Printf("Called list: %v, %v\n", result, err)

		if err != nil {
			glog.V(4).Infof("Failed AS info request for %s: %v", sset.config.Name, err)
			return err
		}

		for _, instance := range *result.Value {
			ref := AzureRef{
				Subscription:  m.subscription,
				ResourceGroup: m.resourceGroupName,
				Name:          *instance.InstanceID,
			}
			newCache[ref] = sset.config
		}
	}

	m.scaleSetCache = newCache
	return nil
}

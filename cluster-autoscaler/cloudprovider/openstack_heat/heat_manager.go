package openstack_heat

import (
	"io"
	"strconv"
	"time"

	"os"

	"fmt"

	"strings"

	"errors"

	"sync"

	"github.com/golang/glog"
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/orchestration/v1/stackresources"
	"github.com/gophercloud/gophercloud/openstack/orchestration/v1/stacks"
	"gopkg.in/gcfg.v1"
	"k8s.io/kubernetes/pkg/util/wait"
)

const (
	resourceGroupType    = "OS::Heat::ResourceGroup"
	autoScalingGroupType = "OS::Heat::AutoScalingGroup"
	novaServerType       = "OS::Nova::Server"

	stackUpdateComplete = "UPDATE_COMPLETE"
)

type HeatManager struct {
	groups      []*HeatResourceGroup
	heatClient  *gophercloud.ServiceClient
	groupsCache map[string]InstanceEntry
	cacheMutex  sync.Mutex
}

type InstanceEntry struct {
	group           *HeatResourceGroup
	parentResource  string
	parentStackName string
	parentStackID   string
}

// CreateHeatManager creates a heat manager that can work with OpenStack
// resource groups and auto scaling groups
func CreateHeatManager(configReader io.Reader) (*HeatManager, error) {
	var opts *gophercloud.AuthOptions
	if configReader != nil {
		var cfg Config
		if err := gcfg.ReadInto(&cfg, configReader); err != nil {
			return nil, fmt.Errorf("couldn't read openstack config: %v", err)
		}
		opts = cfg.toAuthOptions()

	} else {
		if envOpts, err := openstack.AuthOptionsFromEnv(); err == nil {
			opts = &envOpts
		}
		return nil, errors.New("could not detect keystone authentication options from environment")
	}
	providerClient, err := openstack.AuthenticatedClient(*opts)
	if err != nil {
		return nil, fmt.Errorf("could not initialize keystone client: %v", err)
	}
	heatClient, err := openstack.NewOrchestrationV1(providerClient, gophercloud.EndpointOpts{
		Region: os.Getenv("OS_REGION_NAME"),
	})
	if err != nil {
		return nil, fmt.Errorf("could not initialize heat client: %v", err)
	}
	manager := &HeatManager{
		groups:      make([]*HeatResourceGroup, 0),
		heatClient:  heatClient,
		groupsCache: make(map[string]InstanceEntry),
	}
	go wait.Forever(func() {
		manager.cacheMutex.Lock()
		defer manager.cacheMutex.Unlock()
		manager.regenerateCache()
	}, time.Hour)

	return manager, nil
}

// RegisterResourceGroup registers a resource group in Heat Manager.
func (m *HeatManager) RegisterResourceGroup(grp *HeatResourceGroup) error {
	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()

	stackID, err := m.findStackIDByName(grp.stackName)
	if err != nil {
		return fmt.Errorf("cannot register resource group %s : %v", grp.name, err)
	}
	grp.stackID = stackID

	groupResource, err := m.getResourceForGroup(grp)
	if err != nil {
		return fmt.Errorf("resource group %s is not a resource in stack %s : %v",
			grp.name, grp.stackName, err)
	}
	grp.heatResource = groupResource

	m.groups = append(m.groups, grp)

	return nil
}

// GetResourceGroupSize gets the current size of the group.
func (m *HeatManager) GetResourceGroupSize(grp *HeatResourceGroup) (int64, error) {
	grpStack, err := m.getStack(grp.stackName)
	if err != nil {
		return -1, err
	}

	sizeString, ok := grpStack.Parameters[grp.sizeParamName]
	if !ok {
		return -1, err
	}
	size, err := strconv.Atoi(sizeString)
	if err != nil {
		return -1, err
	}
	return int64(size), nil
}

// SetResourceGroupSize sets the instances count in a ResourceGroup by updating a
// predefined Heat stack parameter (specified by the user)
func (m *HeatManager) SetResourceGroupSize(grp *HeatResourceGroup, size int64) error {
	updateURL := m.heatClient.ServiceURL("stacks", grp.stackName, grp.stackID)
	body := map[string]map[string]string{
		"parameters": map[string]string{
			grp.sizeParamName: strconv.FormatInt(size, 10),
		},
	}
	opts := &gophercloud.RequestOpts{
		OkCodes: []int{202},
	}
	_, err := m.heatClient.Patch(updateURL, body, nil, opts)
	if err != nil {
		return fmt.Errorf("could not update stack parameters for stack %s: %v", grp.stackName, err)
	}

	// wait until the update is finished
	wait.PollInfinite(5*time.Second, func() (bool, error) {
		stack, err := m.getStack(grp.stackName)
		if err != nil {
			return false, fmt.Errorf("cannot retrieve stack %s after update: %v", grp.stackName, err)
		}
		if stack.Status == stackUpdateComplete {
			return true, nil
		}
		return false, nil
	})

	return nil
}

// GetGroupForInstance retrieves the resource group that contains
// a given OpenStack instance
func (m *HeatManager) GetGroupForInstance(instanceID string) (*HeatResourceGroup, error) {
	m.cacheMutex.Lock()
	defer m.cacheMutex.Unlock()
	if entry, ok := m.groupsCache[instanceID]; ok {
		return entry.group, nil
	}

	m.regenerateCache()
	if entry, ok := m.groupsCache[instanceID]; ok {
		return entry.group, nil
	}

	glog.Warningf("Instance %s does not belong to any managed resorce group", instanceID)
	return nil, nil
}

// DeleteInstances deletes the specified instances from the
// OpenStack resource group
func (m *HeatManager) DeleteInstances(instanceIDs []string) error {
	commonGrp, err := m.GetGroupForInstance(instanceIDs[0])
	if err != nil {
		return fmt.Errorf("no resource group found to delete instances from: %v", err)
	}
	currentSize, err := commonGrp.TargetSize()
	if err != nil {
		return fmt.Errorf("cannot retrieve group size for %s prior to node deletion: %v", commonGrp.name, err)
	}

	deleted := 0
	for _, id := range instanceIDs {
		entry, ok := m.groupsCache[id]
		if !ok || entry.group.Id() != commonGrp.Id() {
			glog.Warning("instance %s will not be deleted; it is not managed for resource group %s", id,
				commonGrp.name)
			continue
		}

		// mark resource containing instance for deletion
		updateURL := m.heatClient.ServiceURL(
			"stacks",
			entry.parentStackName,
			entry.parentStackID,
			"resources",
			entry.parentResource)

		body := map[string]string{"mark_unhealthy": "true"}
		opts := &gophercloud.RequestOpts{
			OkCodes: []int{200},
		}
		_, err := m.heatClient.Patch(updateURL, body, nil, opts)
		if err != nil {
			return fmt.Errorf("could not delete resource for instance %s: %v", id, err)
		}
		deleted = deleted + 1
	}

	// update the resource group size, marked instances will be deleted
	return m.SetResourceGroupSize(commonGrp, int64(currentSize-deleted))
}

func (m *HeatManager) getStack(stackName string) (*stacks.RetrievedStack, error) {
	stackID, err := m.findStackIDByName(stackName)
	if err != nil {
		return nil, fmt.Errorf("error when retrieving the stack ID for %s : %v", stackName, err)
	}
	stack, err := stacks.Get(m.heatClient, stackName, stackID).Extract()
	if err != nil {
		return nil, fmt.Errorf("error when retrieving stack with name %s and ID %s : %v",
			stackName, stackID, err)
	}
	return stack, nil
}

func (m *HeatManager) findStackIDByName(stackName string) (string, error) {
	pages, err := stacks.List(m.heatClient, stacks.ListOpts{Name: stackName}).AllPages()
	if err != nil {
		return "", fmt.Errorf("error retrieving stack %s [%v]", stackName, err)
	}
	foundStacks, err := stacks.ExtractStacks(pages)
	if err != nil {
		return "", fmt.Errorf("error extracting stack information for stack %s : %v", stackName, err)
	}
	if len(foundStacks) == 0 {
		return "", fmt.Errorf("stack %s not found", stackName)
	}
	return foundStacks[0].ID, nil
}

func (m *HeatManager) regenerateCache() {
	m.groupsCache = make(map[string]InstanceEntry)
	for _, resourceGroup := range m.groups {
		glog.V(4).Infof("Regenerating resource group information for %s", resourceGroup.stackName)
		err := m.refreshResourceGroupNodes(resourceGroup)
		if err != nil {
			glog.Warningf("could not retrieve nodes for group %s : %v", resourceGroup.name, err)
		}
	}
}

func (m *HeatManager) getResourceForGroup(grp *HeatResourceGroup) (*stackresources.Resource, error) {
	foundGroup, err := stackresources.Get(m.heatClient, grp.stackName, grp.stackID,
		grp.name).Extract()
	if err != nil {
		return nil, err
	}

	if foundGroup.Type != resourceGroupType && foundGroup.Type != autoScalingGroupType {
		return nil, fmt.Errorf("the resource %s in stack %s is not a ResourceGroup or AutoscalingGroup",
			grp.name, grp.stackName)
	}

	return foundGroup, nil

}

func (m *HeatManager) refreshResourceGroupNodes(grp *HeatResourceGroup) error {
	nestedStackName, nestedStackID, err := m.getNestedStack(grp.heatResource)
	if err != nil {
		return err
	}
	resources, err := m.getResources(nestedStackName, nestedStackID)
	if err != nil {
		return fmt.Errorf("failed to fetch nested resources for group %s: %v", grp.name, err)
	}

	for _, resource := range resources {
		instanceID, parent, err := m.extractServerDetails(&resource)
		if err != nil {
			return fmt.Errorf("resource group %s does not contain a Nova::Server resource. "+
				"Note that only 1 level of nesting is currently supported: %v", grp.name, err)
		}
		glog.Infof("Managing openstack instance with id %s in group %s\n", instanceID, grp.name)
		m.groupsCache[instanceID] = InstanceEntry{group: grp, parentResource: parent,
			parentStackName: nestedStackName, parentStackID: nestedStackID}
	}
	return nil
}

func (m *HeatManager) getNestedStack(resource *stackresources.Resource) (string, string, error) {
	url := ""
	for _, link := range resource.Links {
		if link.Rel == "nested" {
			url = link.Href
		}
	}
	if url == "" {
		return "", "", fmt.Errorf("resource %s has no nested resources", resource.Name)
	}
	return m.getStackDetailsFromURL(url)
}

func (m *HeatManager) getResources(stackName, stackID string) ([]stackresources.Resource, error) {
	resources := make([]stackresources.Resource, 0)
	// get all nested resources
	resourcesPage, err := stackresources.List(m.heatClient, stackName, stackID, stackresources.ListOpts{}).AllPages()
	if err != nil {
		return resources, err
	}
	resources, err = stackresources.ExtractResources(resourcesPage)
	if err != nil {
		return resources, err
	}
	return resources, nil
}

func (m *HeatManager) extractServerDetails(resource *stackresources.Resource) (string, string, error) {
	instanceID := ""
	parent := ""

	if resource.Type == novaServerType {
		// in this case the resource is a direct Nova::Server
		instanceID = resource.PhysicalID
		parent = resource.Name
	} else if strings.HasSuffix(resource.Type, ".yaml") {
		// search for a direct Nova::Server child of the resource
		nestedStackName, nestedStackID, err := m.getNestedStack(resource)
		if err != nil {
			return "", "", fmt.Errorf("failed to fetch nested resources for resource %s: %v\n", resource.PhysicalID, err)
		}
		resources, err := m.getResources(nestedStackName, nestedStackID)
		if err != nil {
			return "", "", fmt.Errorf("failed to fetch nested resources for resource %s: %v\n", resource.PhysicalID, err)
		}
		for _, nested := range resources {
			if nested.Type == novaServerType {
				instanceID = nested.PhysicalID
				parent = resource.Name
			}
		}
	}

	if instanceID == "" || parent == "" {
		return "", "", fmt.Errorf("resource %s does not contain any Nova::Server nested resources as direct children",
			resource.PhysicalID)
	}
	return instanceID, parent, nil
}

// returns stackName, stackID
func (m *HeatManager) getStackDetailsFromURL(url string) (string, string, error) {
	tokens := strings.Split(url, "/")
	length := len(tokens)
	if length < 3 || tokens[length-3] != "stacks" {
		return "", "", fmt.Errorf("resource group nested stack URL is malformed: %s", url)
	}
	return tokens[length-2], tokens[length-1], nil
}

type Config struct {
	Global struct {
		AuthUrl    string `gcfg:"auth-url"`
		Username   string `gcfg:"username"`
		UserId     string `gcfg:"user-id"`
		Password   string `gcfg:"password"`
		ApiKey     string `gcfg:"api-key"`
		TenantId   string `gcfg:"tenant-id"`
		TenantName string `gcfg:"tenant-name"`
		DomainId   string `gcfg:"domain-id"`
		DomainName string `gcfg:"domain-name"`
		Region     string `gcfg:"region"`
	}
}

func (cfg *Config) toAuthOptions() *gophercloud.AuthOptions {
	return &gophercloud.AuthOptions{
		IdentityEndpoint: cfg.Global.AuthUrl,
		Username:         cfg.Global.Username,
		UserID:           cfg.Global.UserId,
		Password:         cfg.Global.Password,
		TenantID:         cfg.Global.TenantId,
		TenantName:       cfg.Global.TenantName,
		DomainID:         cfg.Global.DomainId,
		DomainName:       cfg.Global.DomainName,

		// Persistent service, so we need to be able to renew tokens.
		AllowReauth: true,
	}
}

package lbaasv2

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/rackspace/gophercloud"
	openstack_lib "github.com/rackspace/gophercloud/openstack"
	"github.com/rackspace/gophercloud/openstack/networking/v2/extensions/lbaas_v2/listeners"
	"github.com/rackspace/gophercloud/openstack/networking/v2/extensions/lbaas_v2/loadbalancers"
	"github.com/rackspace/gophercloud/openstack/networking/v2/extensions/lbaas_v2/monitors"
	"github.com/rackspace/gophercloud/openstack/networking/v2/extensions/lbaas_v2/pools"
	"github.com/rackspace/gophercloud/openstack/networking/v2/subnets"
	"github.com/rackspace/gophercloud/pagination"
	"k8s.io/contrib/loadbalancer/loadbalancer/backend"
	"k8s.io/contrib/loadbalancer/loadbalancer/utils"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/util/wait"
)

const (
	LOADBALANCER = "loadbalancer"
	LISTENER     = "listener"
	POOL         = "pool"
	MONITOR      = "monitor"
)

var empty struct{}

// LBaaSController Controller to manage LBaaS resources
type LBaaSController struct {
	kubeClient          *unversioned.Client
	watchNamespace      string
	configMapLabelKey   string
	configMapLabelValue string
	compute             *gophercloud.ServiceClient
	network             *gophercloud.ServiceClient
	subnetID            string
}

func init() {
	backend.Register("openstack-lbaasv2", NewLBaaSController)
}

// NewLBaaSController creates a LBaaS controller
func NewLBaaSController(kubeClient *unversioned.Client, watchNamespace string, conf map[string]string, configLabelKey, configLabelValue string) (backend.BackendController, error) {
	authOptions := gophercloud.AuthOptions{
		IdentityEndpoint: os.Getenv("OS_AUTH_URL"),
		Username:         os.Getenv("OS_USERNAME"),
		Password:         os.Getenv("OS_PASSWORD"),
		TenantName:       os.Getenv("OS_TENANT_NAME"),
		// Persistent service, so we need to be able to renew tokens.
		AllowReauth: true,
	}

	openstackClient, err := openstack_lib.AuthenticatedClient(authOptions)
	if err != nil {
		glog.Fatalf("Failed to retrieve openstack client. %v", err)
	}

	compute, err := openstack_lib.NewComputeV2(openstackClient, gophercloud.EndpointOpts{
		Region: os.Getenv("OS_REGION_NAME"),
	})
	if err != nil {
		glog.Fatalf("Failed to find compute endpoint: %v", err)
	}

	network, err := openstack_lib.NewNetworkV2(openstackClient, gophercloud.EndpointOpts{
		Region: os.Getenv("OS_REGION_NAME"),
	})
	if err != nil {
		glog.Fatalf("Failed to find network endpoint: %v", err)
	}

	lbaasControl := LBaaSController{
		kubeClient:          kubeClient,
		watchNamespace:      watchNamespace,
		configMapLabelKey:   configLabelKey,
		configMapLabelValue: configLabelValue,
		compute:             compute,
		network:             network,
		subnetID:            os.Getenv("OS_SUBNET_ID"),
	}

	// check if subnet-id exists
	_, err = subnets.Get(network, lbaasControl.subnetID).Extract()
	if err != nil {
		glog.Infof("Error checking subnet-id: %v %v", lbaasControl.subnetID, err)
		if strings.Contains(err.Error(), "404") {
			//subnetID not found
			return nil, err
		}
	}

	// sync deleted nodes with loadbalancer pool members
	nodeIPs, _ := lbaasControl.getReadyNodeIPs()
	currentNodes := make(map[string]struct{})
	for _, ip := range nodeIPs {
		currentNodes[ip] = empty
	}

	configMapNodePortMap := utils.GetLBConfigMapNodePortMap(kubeClient, watchNamespace, configLabelKey, configLabelValue)
	for configmapName := range configMapNodePortMap {
		poolName := getResourceName(POOL, configmapName)
		poolID, err := lbaasControl.getPoolIDFromName(poolName)
		if err != nil {
			glog.Errorf("Could not get pool %v. %v", poolName, err)
		}

		allPages, err := pools.ListAssociateMembers(network, poolID, pools.MemberListOpts{}).AllPages()
		if err != nil {
			glog.Errorf("Error listing pool members %v", err.Error)
			if strings.Contains(err.Error(), "404") {
				break
			}
		}
		memberList, _ := pools.ExtractMembers(allPages)

		// check if member IP exist in the Ready node IP list
		for _, member := range memberList {
			if _, ok := currentNodes[member.Address]; !ok {
				// delete the member
				err = pools.DeleteMember(network, poolID, member.ID).ExtractErr()
				if err != nil {
					glog.Errorf("Could not delete member for pool %v. memberID: %v", poolName, member.ID)
					continue
				}
			}
		}
	}

	return &lbaasControl, nil
}

// Name returns the name of the backend controller
func (lbaas *LBaaSController) Name() string {
	return "LBaaSController"
}

// GetBindIP returns the IP used by users to access their apps
func (lbaas *LBaaSController) GetBindIP(name string) string {
	// Find loadbalancer by name
	lbName := getResourceName(LOADBALANCER, name)
	opts := loadbalancers.ListOpts{Name: lbName}
	pager := loadbalancers.List(lbaas.network, opts)
	var bindIP string
	lbErr := pager.EachPage(func(page pagination.Page) (bool, error) {
		loadbalancerList, err := loadbalancers.ExtractLoadbalancers(page)
		if err != nil {
			return false, err
		}

		if len(loadbalancerList) == 0 {
			err = fmt.Errorf("Load balancer with name %v not found.", lbName)
			return false, err
		}

		if len(loadbalancerList) > 1 {
			err = fmt.Errorf("More than one loadbalancer with name %v found.", lbName)
			return false, err
		}
		bindIP = loadbalancerList[0].VipAddress
		return true, nil
	})

	if lbErr != nil {
		glog.Errorf("Could not get list of loadbalancer. %v.", lbErr)
	}

	return bindIP
}

// check if Loadbalancer already exists with the correct resources
func (lbaas *LBaaSController) checkLoadbalancerExist(cmName string, servicePort *api.ServicePort, listenerBindPort string) (bool, error) {
	expectedBindPort, _ := strconv.Atoi(listenerBindPort)
	expectedPoolProtocol := string(servicePort.Protocol)
	expectedNodePort := servicePort.NodePort
	expectedLbName := getResourceName(LOADBALANCER, cmName)
	expectedPoolName := getResourceName(POOL, cmName)
	expectedListenerName := getResourceName(LISTENER, cmName)
	opts := loadbalancers.ListOpts{Name: expectedLbName}

	//get all pages and check if the  loadbalancer doesnt exists
	allPages, err := loadbalancers.List(lbaas.network, opts).AllPages()
	if err != nil {
		glog.Errorf("Error getting list of Loadbalancers")
		return false, err
	}

	// extract the loadbalancer list
	loadbalancerList, err := loadbalancers.ExtractLoadbalancers(allPages)
	if err != nil {
		return false, err
	}
	if len(loadbalancerList) == 0 {
		glog.Infof("No Load balancer found")
		return false, nil
	}
	if len(loadbalancerList) > 1 {
		err = fmt.Errorf("More than one loadbalancer with name %v found.", expectedLbName)
		return false, err
	}

	lbFound := loadbalancerList[0] // assuming there is only one loadbalancer with this name
	if lbFound.Name == expectedLbName {
		// check if loadbalancer listener exists
		if lbFound.VipSubnetID != lbaas.subnetID {
			return false, nil
		}
		listenerList := lbFound.Listeners
		if len(listenerList) != 0 {
			listener, _ := listeners.Get(lbaas.network, listenerList[0].ID).Extract() // assuming there is only one listener

			if listener.Name == expectedListenerName {
				if listener.ProtocolPort != expectedBindPort {
					glog.Infof("Listener port %v is different from expected port %v", listener.ProtocolPort, expectedBindPort)
					return false, nil
				}
				// check if loadbalancer listener pool exists
				poolID := listener.DefaultPoolID
				if poolID != "" {
					pool, _ := pools.Get(lbaas.network, poolID).Extract()
					if pool.Name == expectedPoolName {
						if pool.Protocol != expectedPoolProtocol {
							glog.Infof("Pool protocol %v is different from expected protocol %v", pool.Protocol, expectedPoolProtocol)
							return false, nil
						}
					}
					//check if service hasn't changed by comparing the member nodePort
					allPages, _ := pools.ListAssociateMembers(lbaas.network, poolID, pools.MemberListOpts{}).AllPages()
					memberList, _ := pools.ExtractMembers(allPages)
					poolMember0 := memberList[0]
					if expectedNodePort != int32(poolMember0.ProtocolPort) {
						return false, nil
					}
					if pool.MonitorID != "" {
						return true, nil
					}
				}
			}
		}
	}

	return false, nil
}

// HandleConfigMapCreate creates a new lbaas loadbalancer resource
func (lbaas *LBaaSController) HandleConfigMapCreate(configMap *api.ConfigMap) {
	name := configMap.Namespace + "-" + configMap.Name
	config := configMap.Data
	serviceName := config["target-service-name"]
	namespace := config["namespace"]
	serviceObj, err := lbaas.kubeClient.Services(namespace).Get(serviceName)
	if err != nil {
		glog.Errorf("Error getting service object %v/%v. %v", namespace, serviceName, err)
		return
	}

	servicePort, err := utils.GetServicePort(serviceObj, config["target-port-name"])
	if err != nil {
		glog.Errorf("Error while getting the service port %v", err)
		return
	}

	if servicePort.NodePort == 0 {
		glog.Errorf("NodePort is needed for loadbalancer")
		return
	}

	// check if the loadbalancer already exists
	lbExists, err := lbaas.checkLoadbalancerExist(name, servicePort, config["bind-port"])
	if err != nil {
		glog.Errorf("Couldn't check the existence of %v loadbalancer", getResourceName(LOADBALANCER, name))
	}
	if lbExists {
		glog.Infof("Loadbalancer %v already exists", getResourceName(LOADBALANCER, name))
		return
	}
	// Delete current load balancer for this service if it exist
	lbaas.HandleConfigMapDelete(name)

	lbName := getResourceName(LOADBALANCER, name)
	lb, err := loadbalancers.Create(lbaas.network, loadbalancers.CreateOpts{
		Name:         lbName,
		AdminStateUp: loadbalancers.Up,
		VipSubnetID:  lbaas.subnetID,
	}).Extract()
	if err != nil {
		glog.Errorf("Could not create loadbalancer %v. %v", lbName, err)
		return
	}
	glog.Infof("Created loadbalancer %v. ID: %v", lbName, lb.ID)

	// Wait for load balancer resource to be ACTIVE state
	lbaas.waitLoadbalancerReady(lb.ID)

	// Create a listener resource for the loadbalancer
	listenerName := getResourceName(LISTENER, name)
	bindPort, _ := strconv.Atoi(config["bind-port"])
	listener, err := listeners.Create(lbaas.network, listeners.CreateOpts{
		Protocol:       listeners.Protocol(servicePort.Protocol),
		Name:           listenerName,
		LoadbalancerID: lb.ID,
		AdminStateUp:   listeners.Up,
		ProtocolPort:   bindPort,
	}).Extract()
	if err != nil {
		glog.Errorf("Could not create listener %v. %v", listenerName, err)
		defer lbaas.deleteLBaaSResource(lb.ID, LOADBALANCER, lb.ID)
		return
	}
	glog.Infof("Created listener %v. ID: %v", listenerName, listener.ID)

	// Wait for load balancer resource to be ACTIVE state
	lbaas.waitLoadbalancerReady(lb.ID)

	// Create a pool resource for the listener
	poolName := getResourceName(POOL, name)
	pool, err := pools.Create(lbaas.network, pools.CreateOpts{
		LBMethod:   pools.LBMethodRoundRobin,
		Protocol:   pools.Protocol(servicePort.Protocol),
		Name:       poolName,
		ListenerID: listener.ID,
	}).Extract()
	if err != nil {
		glog.Errorf("Could not create pool %v. %v", poolName, err)
		defer lbaas.deleteLBaaSResource(lb.ID, LOADBALANCER, lb.ID)
		defer lbaas.deleteLBaaSResource(lb.ID, LISTENER, listener.ID)
		return
	}
	glog.Infof("Created pool %v. ID: %v", poolName, pool.ID)
	// Wait for load balancer resource to be ACTIVE state
	lbaas.waitLoadbalancerReady(lb.ID)

	// Associate nodes to the pool
	nodes, _ := lbaas.getReadyNodeIPs()
	for _, ip := range nodes {
		member, err := pools.CreateAssociateMember(lbaas.network, pool.ID, pools.MemberCreateOpts{
			SubnetID:     lbaas.subnetID,
			Address:      ip,
			ProtocolPort: int(servicePort.NodePort),
		}).ExtractMember()
		if err != nil {
			glog.Errorf("Could not create member for %v. %v", ip, err)
			defer lbaas.deleteLBaaSResource(lb.ID, LOADBALANCER, lb.ID)
			defer lbaas.deleteLBaaSResource(lb.ID, LISTENER, listener.ID)
			defer lbaas.deleteLBaaSResource(lb.ID, POOL, pool.ID)
			return
		}
		glog.Infof("Created member for %v. ID: %v", ip, member.ID)
		// Wait for load balancer resource to be ACTIVE state
		lbaas.waitLoadbalancerReady(lb.ID)
	}

	// Create health monitor for the pool
	monitor, err := monitors.Create(lbaas.network, monitors.CreateOpts{
		Type:       string(servicePort.Protocol),
		PoolID:     pool.ID,
		Delay:      20,
		Timeout:    10,
		MaxRetries: 5,
	}).Extract()
	if err != nil {
		glog.Errorf("Could not create health monitor for pool %v. %v", poolName, err)
		defer lbaas.deleteLBaaSResource(lb.ID, LOADBALANCER, lb.ID)
		defer lbaas.deleteLBaaSResource(lb.ID, LISTENER, listener.ID)
		defer lbaas.deleteLBaaSResource(lb.ID, POOL, pool.ID)
		return
	}
	glog.Infof("Created health monitor for pool %v. ID: %v", poolName, monitor.ID)
	// Wait for load balancer resource to be ACTIVE state
	lbaas.waitLoadbalancerReady(lb.ID)
}

// HandleConfigMapDelete delete the lbaas loadbalancer resource
func (lbaas *LBaaSController) HandleConfigMapDelete(name string) {
	// Find loadbalancer by name
	lbName := getResourceName(LOADBALANCER, name)
	opts := loadbalancers.ListOpts{Name: lbName}
	pager := loadbalancers.List(lbaas.network, opts)
	lbErr := pager.EachPage(func(page pagination.Page) (bool, error) {
		loadbalancerList, err := loadbalancers.ExtractLoadbalancers(page)
		if err != nil {
			return false, err
		}

		if len(loadbalancerList) == 0 {
			err = fmt.Errorf("Load balancer with name %v not found.", lbName)
			return false, err
		}

		if len(loadbalancerList) > 1 {
			err = fmt.Errorf("More than one loadbalancer with name %v found.", lbName)
			return false, err
		}

		lbID := loadbalancerList[0].ID // assuming there is only one loadbalancer with this name
		listenerList := loadbalancerList[0].Listeners
		if len(listenerList) != 0 {
			listener, _ := listeners.Get(lbaas.network, listenerList[0].ID).Extract() // assuming there is only one listener
			poolID := listener.DefaultPoolID
			if poolID != "" {
				pool, _ := pools.Get(lbaas.network, poolID).Extract()
				if pool.MonitorID != "" {
					lbaas.deleteLBaaSResource(lbID, MONITOR, pool.MonitorID)
				}
				lbaas.deleteLBaaSResource(lbID, POOL, poolID)
			}
			lbaas.deleteLBaaSResource(lbID, LISTENER, listener.ID)
		}
		lbaas.deleteLBaaSResource(lbID, LOADBALANCER, lbID)
		return true, nil
	})

	if lbErr != nil {
		glog.Errorf("Could not get list of loadbalancer. %v.", lbErr)
	}

}

// HandleNodeCreate creates new member for this node in every loadbalancer pool
func (lbaas *LBaaSController) HandleNodeCreate(node *api.Node) {

	ip, err := utils.GetNodeHostIP(*node)
	if err != nil {
		glog.Errorf("Error getting IP for node %v", node.Name)
		return
	}
	configMapNodePortMap := utils.GetLBConfigMapNodePortMap(lbaas.kubeClient, lbaas.watchNamespace, lbaas.configMapLabelKey, lbaas.configMapLabelValue)

	for configmapName, nodePort := range configMapNodePortMap {
		poolName := getResourceName(POOL, configmapName)
		poolID, err := lbaas.getPoolIDFromName(poolName)
		if err != nil {
			glog.Errorf("Could not get pool %v. %v", poolName, err)
			continue
		}
		memberID, err := lbaas.createMemberResource(poolID, *ip, nodePort)
		if err != nil {
			glog.Errorf("Could not create member for pool %v. IP: %v. Port: %v", poolName, *ip, nodePort)
			continue
		}
		glog.Infof("Created member for %v. ID: %v", *ip, memberID)
	}
}

// HandleNodeDelete deletes member for this node
func (lbaas *LBaaSController) HandleNodeDelete(node *api.Node) {
	ip, err := utils.GetNodeHostIP(*node)
	if err != nil {
		glog.Errorf("Error getting IP for node %v", node.Name)
		return
	}
	configMapNodePortMap := utils.GetLBConfigMapNodePortMap(lbaas.kubeClient, lbaas.watchNamespace, lbaas.configMapLabelKey, lbaas.configMapLabelValue)

	for configmapName := range configMapNodePortMap {
		poolName := getResourceName(POOL, configmapName)
		poolID, err := lbaas.getPoolIDFromName(poolName)
		if err != nil {
			glog.Errorf("Could not get pool for %v. %v", poolName, err)
			continue
		}
		memberID, err := lbaas.getMemberIDFromIP(poolID, *ip)
		if err != nil {
			glog.Errorf("Could not get member for pool %v. IP: %v.", poolName, *ip)
			continue
		}
		err = pools.DeleteMember(lbaas.network, poolID, memberID).ExtractErr()
		if err != nil {
			glog.Errorf("Could not get member for pool %v. memberID: %v", poolName, memberID)
			continue
		}
		glog.Infof("Deleted member for pool %v with IP: %v. ID: %v", poolName, *ip, memberID)
	}
}

// HandleNodeUpdate updates IP of the member for this node if it exists. If it doesnt, it will create a new member
func (lbaas *LBaaSController) HandleNodeUpdate(oldNode *api.Node, curNode *api.Node) {
	newIP, err := utils.GetNodeHostIP(*curNode)
	if err != nil {
		glog.Errorf("Error getting IP for node %v", curNode.Name)
		return
	}
	oldIP, err := utils.GetNodeHostIP(*oldNode)
	if err != nil {
		glog.Errorf("Error getting IP for node %v", oldNode.Name)
		return
	}

	configMapNodePortMap := utils.GetLBConfigMapNodePortMap(lbaas.kubeClient, lbaas.watchNamespace, lbaas.configMapLabelKey, lbaas.configMapLabelValue)

	for configmapName, nodePort := range configMapNodePortMap {
		poolName := getResourceName(POOL, configmapName)
		poolID, err := lbaas.getPoolIDFromName(poolName)
		if err != nil {
			glog.Errorf("Could not get pool for %v. %v", poolName, err)
			continue
		}
		memberID, err := lbaas.getMemberIDFromIP(poolID, *oldIP)
		if err != nil {
			glog.Warningf("Could not get member for pool %v. IP: %v. Creating...", poolName, *oldIP)
		} else {
			// delete the existing pool member with old IP
			err = pools.DeleteMember(lbaas.network, poolID, memberID).ExtractErr()
			if err != nil {
				glog.Errorf("Could not get member for pool %v. memberID: %v", poolName, memberID)
				continue
			}
			glog.Infof("Deleted member for pool %v with IP: %v. ID: %v", poolName, *oldIP, memberID)
		}

		// create the pool member again to update with new IP
		memberID, err = lbaas.createMemberResource(poolID, *newIP, nodePort)
		if err != nil {
			glog.Errorf("Could not create member for pool %v. IP: %v. Port: %v", poolName, *newIP, nodePort)
			continue
		}
		glog.Infof("Created member for %v. ID: %v", *newIP, memberID)
	}
}

func getResourceName(resourceType string, names ...string) string {
	return strings.Join(names, "-") + "-" + resourceType
}

func (lbaas *LBaaSController) getPoolIDFromName(poolName string) (string, error) {
	pager := pools.List(lbaas.network, pools.ListOpts{Name: poolName})
	var poolID string
	poolErr := pager.EachPage(func(page pagination.Page) (bool, error) {
		poolList, err := pools.ExtractPools(page)
		if err != nil {
			return false, err
		}

		if len(poolList) == 0 {
			err = fmt.Errorf("Pool with name %v not found.", poolName)
			return false, err
		}

		if len(poolList) > 1 {
			err = fmt.Errorf("More than one pool with name %v found. %v", poolName, poolList)
			return false, err
		}
		poolID = poolList[0].ID
		return true, nil
	})

	return poolID, poolErr
}

func (lbaas *LBaaSController) createMemberResource(poolID string, ip string, nodePort int) (string, error) {
	member, err := pools.CreateAssociateMember(lbaas.network, poolID, pools.MemberCreateOpts{
		SubnetID:     lbaas.subnetID,
		Address:      ip,
		ProtocolPort: nodePort,
	}).ExtractMember()
	if err != nil {
		return "", fmt.Errorf("Could not create member for %v. %v", ip, err)
	}
	return member.ID, nil
}

func (lbaas *LBaaSController) getMemberIDFromIP(poolID string, ip string) (string, error) {
	var memberID string
	pager := pools.ListAssociateMembers(lbaas.network, poolID, pools.MemberListOpts{Address: ip})
	memberErr := pager.EachPage(func(page pagination.Page) (bool, error) {
		membersList, err := pools.ExtractMembers(page)
		// There should only be one member that has this IP.
		if err != nil {
			return false, err
		}

		if len(membersList) == 0 {
			err = fmt.Errorf("Member with IP %v not found.", ip)
			return false, err
		}

		if len(membersList) > 1 {
			err = fmt.Errorf("More than one member with IP %v found.", ip)
			return false, err
		}
		memberID = membersList[0].ID
		return true, nil
	})
	return memberID, memberErr
}

func (lbaas *LBaaSController) waitLoadbalancerReady(lbID string) {
	// Wait for load balancer resource to be ACTIVE state
	err := wait.PollImmediate(2*time.Second, 5*time.Minute, func() (done bool, err error) {
		lb, err := loadbalancers.Get(lbaas.network, lbID).Extract()
		if err != nil {
			return true, err
		}

		if lb.ProvisioningStatus == "ACTIVE" || lb.ProvisioningStatus == "ERROR" {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		glog.Errorf("Loadbalancer %v resource did not go ACTIVE. %v", lbID, err)
		return
	}
}

func (lbaas *LBaaSController) deleteLBaaSResource(lbID string, resourceType string, resourceID string) {
	glog.Errorf("Deleting %v %v.", resourceType, resourceID)
	var err error
	switch {
	case resourceType == LOADBALANCER:
		err = loadbalancers.Delete(lbaas.network, resourceID).Err
	case resourceType == LISTENER:
		err = listeners.Delete(lbaas.network, resourceID).Err
	case resourceType == POOL:
		err = pools.Delete(lbaas.network, resourceID).Err
	case resourceType == MONITOR:
		err = monitors.Delete(lbaas.network, resourceID).Err
	}
	if err != nil {
		glog.Errorf("Could not delete %v %v. %v", resourceType, resourceID, err)
		return
	}
	if resourceType != LOADBALANCER {
		// Wait for load balancer resource to be ACTIVE state
		lbaas.waitLoadbalancerReady(lbID)
	}
	glog.Infof("%v %v Deleted", resourceType, resourceID)
}

// getReadyNodeNames returns names of schedulable, ready nodes from the node lister.
func (lbaas *LBaaSController) getReadyNodeIPs() ([]string, error) {
	nodeIPs := []string{}
	nodes, err := lbaas.kubeClient.Nodes().List(api.ListOptions{})
	if err != nil {
		return nodeIPs, err
	}
	nodesReady := utils.Filter(nodes, utils.NodeReady)
	for _, n := range nodesReady {
		if n.Spec.Unschedulable {
			continue
		}
		ip, err := utils.GetNodeHostIP(n)
		if err != nil {
			glog.Errorf("Error getting node IP for %v. %v", n.Name, err)
			continue
		}
		nodeIPs = append(nodeIPs, *ip)
	}
	return nodeIPs, nil
}

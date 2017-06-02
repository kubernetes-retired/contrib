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

package lbaasv2

import (
	"fmt"
	"os"
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
	loadBalancerResource = "loadbalancer"
	listenerResource     = "listener"
	poolResource         = "pool"
	monitorResource      = "monitor"
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

	poolNodePortMap := utils.GetPoolNodePortMap(kubeClient, watchNamespace, configLabelKey, configLabelValue)
	for poolName := range poolNodePortMap {
		poolID, err := lbaasControl.getPoolIDFromName(poolName)
		if err != nil {
			glog.Errorf("Could not get pool %v. %v", poolName, err)
		}

		allMemberPages, err := pools.ListAssociateMembers(network, poolID, pools.MemberListOpts{}).AllPages()
		if err != nil {
			if strings.Contains(err.Error(), "404") {
				// no pool members found
				continue
			}
			glog.Errorf("Error listing pool members %v", err.Error())
		}
		memberList, _ := pools.ExtractMembers(allMemberPages)

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
func (lbaas *LBaaSController) GetBindIP(name string) (string, error) {
	// Find loadbalancer by name
	lbName := utils.GetResourceName(loadBalancerResource, name)
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
		return "", lbErr
	}

	return bindIP, nil
}

// check if Loadbalancer already exists with the correct resources
func (lbaas *LBaaSController) checkLoadbalancerExist(cmName string, serviceObj *api.Service) (bool, error) {

	expectedLbName := utils.GetResourceName(loadBalancerResource, cmName)
	opts := loadbalancers.ListOpts{Name: expectedLbName}

	//get all pages and check if the loadbalancer exist
	allPages, err := loadbalancers.List(lbaas.network, opts).AllPages()
	if err != nil {
		glog.Errorf("Error getting list of Loadbalancers")
		return false, err
	}

	loadbalancerList, err := loadbalancers.ExtractLoadbalancers(allPages)
	if err != nil {
		return false, err
	}
	if len(loadbalancerList) == 0 {
		return false, nil
	}
	if len(loadbalancerList) > 1 {
		err = fmt.Errorf("More than one loadbalancer with name %v found.", expectedLbName)
		return false, err
	}

	lbFound := loadbalancerList[0] // assuming there is only one loadbalancer with this name
	listenerMap := make(map[string]*listeners.Listener)
	if lbFound.Name == expectedLbName {
		// check if loadbalancer listener exist
		if lbFound.VipSubnetID != lbaas.subnetID {
			return false, nil
		}
		listenerList := lbFound.Listeners
		if len(listenerList) != 0 {
			for _, lsnr := range listenerList {
				listener, _ := listeners.Get(lbaas.network, lsnr.ID).Extract()
				listenerMap[listener.Name] = listener
			}
		}
	}

	// check for pool and members of respective listeners
	for _, port := range serviceObj.Spec.Ports {
		servicePortName := port.Name
		expectedListenerName := utils.GetResourceName(listenerResource, cmName, servicePortName)
		expectedBindPort := int(port.Port)
		expectedPoolProtocol := string(port.Protocol)
		expectedNodePort := port.NodePort
		expectedPoolName := utils.GetResourceName(poolResource, cmName, servicePortName)

		if listenerObj, ok := listenerMap[expectedListenerName]; ok {
			if listenerObj.ProtocolPort != expectedBindPort {
				return false, nil
			}

			poolID := listenerObj.DefaultPoolID
			if poolID != "" {
				pool, _ := pools.Get(lbaas.network, poolID).Extract()
				if pool.Name == expectedPoolName {
					if pool.Protocol != expectedPoolProtocol {
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
				if pool.MonitorID == "" {
					return false, nil
				}
			}
		} else {
			return false, nil
		}
	}
	return true, nil
}

// HandleConfigMapCreate creates a new lbaas loadbalancer resource
func (lbaas *LBaaSController) HandleConfigMapCreate(configMap *api.ConfigMap) error {
	name := configMap.Namespace + "-" + configMap.Name
	config := configMap.Data
	serviceName := config["target-service-name"]
	namespace := config["namespace"]
	serviceObj, err := lbaas.kubeClient.Services(namespace).Get(serviceName)
	if err != nil {
		err = fmt.Errorf("Error getting service object %v/%v. %v", namespace, serviceName, err)
		return err
	}

	// check if the loadbalancer already exists
	lbExists, err := lbaas.checkLoadbalancerExist(name, serviceObj)
	if err != nil {
		glog.Errorf("Error checking %v loadbalancer existence: %v", utils.GetResourceName(loadBalancerResource, name), err)
	}
	if lbExists {
		glog.Infof("Loadbalancer %v already exist", utils.GetResourceName(loadBalancerResource, name))
		return nil
	}
	// Delete current load balancer for this service if it exist
	lbaas.HandleConfigMapDelete(configMap)

	lbName := utils.GetResourceName(loadBalancerResource, name)
	lb, err := loadbalancers.Create(lbaas.network, loadbalancers.CreateOpts{
		Name:         lbName,
		AdminStateUp: loadbalancers.Up,
		VipSubnetID:  lbaas.subnetID,
	}).Extract()
	if err != nil {
		err = fmt.Errorf("Could not create loadbalancer %v. %v", lbName, err)
		return err
	}
	glog.Infof("Created loadbalancer %v. ID: %v", lbName, lb.ID)

	// Wait for load balancer resource to be ACTIVE state
	lbaas.waitLoadbalancerReady(lb.ID)

	for _, port := range serviceObj.Spec.Ports {
		servicePortName := port.Name
		if port.NodePort == 0 {
			err = fmt.Errorf("NodePort is needed for loadbalancer")
			return err
		}

		// Create a listener resource for the loadbalancer
		listenerName := utils.GetResourceName(listenerResource, name, servicePortName)
		bindPort := int(port.Port)
		listener, err := listeners.Create(lbaas.network, listeners.CreateOpts{
			Protocol:       listeners.Protocol(port.Protocol),
			Name:           listenerName,
			LoadbalancerID: lb.ID,
			AdminStateUp:   listeners.Up,
			ProtocolPort:   bindPort,
		}).Extract()
		if err != nil {
			err = fmt.Errorf("Could not create listener %v. %v", listenerName, err)
			defer lbaas.deleteLBaaSResource(lb.ID, loadBalancerResource, lb.ID)
			return err
		}
		glog.Infof("Created listener %v. ID: %v", listenerName, listener.ID)

		// Wait for load balancer resource to be ACTIVE state
		lbaas.waitLoadbalancerReady(lb.ID)

		// Create a pool resource for the listener
		poolName := utils.GetResourceName(poolResource, name, servicePortName)
		pool, err := pools.Create(lbaas.network, pools.CreateOpts{
			LBMethod:   pools.LBMethodRoundRobin,
			Protocol:   pools.Protocol(port.Protocol),
			Name:       poolName,
			ListenerID: listener.ID,
		}).Extract()
		if err != nil {
			err = fmt.Errorf("Could not create pool %v. %v", poolName, err)
			defer lbaas.deleteLBaaSResource(lb.ID, loadBalancerResource, lb.ID)
			defer lbaas.deleteLBaaSResource(lb.ID, listenerResource, listener.ID)
			return err
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
				ProtocolPort: int(port.NodePort),
			}).ExtractMember()
			if err != nil {
				err = fmt.Errorf("Could not create member for %v. %v", ip, err)
				defer lbaas.deleteLBaaSResource(lb.ID, loadBalancerResource, lb.ID)
				defer lbaas.deleteLBaaSResource(lb.ID, listenerResource, listener.ID)
				defer lbaas.deleteLBaaSResource(lb.ID, poolResource, pool.ID)
				return err
			}
			glog.Infof("Created member for %v. ID: %v", ip, member.ID)
			// Wait for load balancer resource to be ACTIVE state
			lbaas.waitLoadbalancerReady(lb.ID)
		}

		// Create health monitor for the pool
		monitor, err := monitors.Create(lbaas.network, monitors.CreateOpts{
			Type:       string(port.Protocol),
			PoolID:     pool.ID,
			Delay:      20,
			Timeout:    10,
			MaxRetries: 5,
		}).Extract()
		if err != nil {
			err = fmt.Errorf("Could not create health monitor for pool %v. %v", poolName, err)
			defer lbaas.deleteLBaaSResource(lb.ID, loadBalancerResource, lb.ID)
			defer lbaas.deleteLBaaSResource(lb.ID, listenerResource, listener.ID)
			defer lbaas.deleteLBaaSResource(lb.ID, poolResource, pool.ID)
			return err
		}
		glog.Infof("Created health monitor for pool %v. ID: %v", poolName, monitor.ID)
		// Wait for load balancer resource to be ACTIVE state
		lbaas.waitLoadbalancerReady(lb.ID)
	}
	return nil
}

// HandleConfigMapDelete delete the lbaas loadbalancer resource
func (lbaas *LBaaSController) HandleConfigMapDelete(configMap *api.ConfigMap) {
	name := configMap.Namespace + "-" + configMap.Name
	// Find loadbalancer by name
	lbName := utils.GetResourceName(loadBalancerResource, name)
	opts := loadbalancers.ListOpts{Name: lbName}

	//get all pages and check if the  loadbalancer doesnt exists
	allPages, err := loadbalancers.List(lbaas.network, opts).AllPages()
	if err != nil {
		glog.Errorf("Error getting list of Loadbalancers: %v", err)
		return
	}
	// extract the loadbalancer list
	loadbalancerList, err := loadbalancers.ExtractLoadbalancers(allPages)
	if err != nil {
		glog.Errorf("Error getting list of Loadbalancers: %v", err)
		return
	}
	if len(loadbalancerList) == 0 {
		glog.Infof("No Load balancer found to delete")
		return
	}
	if len(loadbalancerList) > 1 {
		glog.Errorf("More than one loadbalancer with name %v found.", lbName)
		return
	}

	lbID := loadbalancerList[0].ID // assuming there is only one loadbalancer with this name
	listenerList := loadbalancerList[0].Listeners
	if len(listenerList) != 0 {
		for _, listener := range listenerList {
			listenerObj, _ := listeners.Get(lbaas.network, listener.ID).Extract()
			poolID := listenerObj.DefaultPoolID

			if poolID != "" {
				pool, _ := pools.Get(lbaas.network, poolID).Extract()
				if pool.MonitorID != "" {
					lbaas.deleteLBaaSResource(lbID, monitorResource, pool.MonitorID)
				}
				lbaas.deleteLBaaSResource(lbID, poolResource, poolID)
			}
			lbaas.deleteLBaaSResource(lbID, listenerResource, listenerObj.ID)
		}
		lbaas.deleteLBaaSResource(lbID, loadBalancerResource, lbID)
	}
}

// HandleNodeCreate creates new member for this node in every loadbalancer pool
func (lbaas *LBaaSController) HandleNodeCreate(node *api.Node) {

	ip, err := utils.GetNodeHostIP(*node)
	if err != nil {
		glog.Errorf("Error getting IP for node %v", node.Name)
		return
	}
	poolNodePortMap := utils.GetPoolNodePortMap(lbaas.kubeClient, lbaas.watchNamespace, lbaas.configMapLabelKey, lbaas.configMapLabelValue)
	for poolName, nodePorts := range poolNodePortMap {
		poolID, err := lbaas.getPoolIDFromName(poolName)
		if err != nil {
			glog.Errorf("Could not get pool %v. %v", poolName, err)
			continue
		}
		memberID, err := lbaas.createMemberResource(poolID, *ip, nodePorts)
		if err != nil {
			glog.Errorf("Could not create member for pool %v. IP: %v. Port: %v", poolName, *ip, nodePorts)
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
	poolNodePortMap := utils.GetPoolNodePortMap(lbaas.kubeClient, lbaas.watchNamespace, lbaas.configMapLabelKey, lbaas.configMapLabelValue)
	for poolName := range poolNodePortMap {
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

	poolNodePortMap := utils.GetPoolNodePortMap(lbaas.kubeClient, lbaas.watchNamespace, lbaas.configMapLabelKey, lbaas.configMapLabelValue)
	for poolName, nodePorts := range poolNodePortMap {
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
		memberID, err = lbaas.createMemberResource(poolID, *newIP, nodePorts)
		if err != nil {
			glog.Errorf("Could not create member for pool %v. IP: %v. Port: %v", poolName, *newIP, nodePorts)
			continue
		}
		glog.Infof("Created member for %v. ID: %v", *newIP, memberID)
	}
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

func (lbaas *LBaaSController) waitLoadbalancerDeleted(lbID string) {
	// Wait for load balancer resource to get deleted
	err := wait.PollImmediate(2*time.Second, 5*time.Minute, func() (done bool, err error) {
		lb, _ := loadbalancers.Get(lbaas.network, lbID).Extract()
		if lb != nil {
			return false, nil
		}
		return true, nil

	})
	if err != nil {
		glog.Errorf("Loadbalancer %v resource did not delete. %v", lbID, err)
		return
	}
}

func (lbaas *LBaaSController) deleteLBaaSResource(lbID string, resourceType string, resourceID string) {
	glog.Errorf("Deleting %v %v.", resourceType, resourceID)
	var err error
	switch {
	case resourceType == loadBalancerResource:
		err = loadbalancers.Delete(lbaas.network, resourceID).Err
	case resourceType == listenerResource:
		err = listeners.Delete(lbaas.network, resourceID).Err
	case resourceType == poolResource:
		err = pools.Delete(lbaas.network, resourceID).Err
	case resourceType == monitorResource:
		err = monitors.Delete(lbaas.network, resourceID).Err
	}
	if err != nil {
		glog.Errorf("Could not delete %v %v. %v", resourceType, resourceID, err)
		return
	}
	if resourceType != loadBalancerResource {
		// Wait for load balancer resource to be ACTIVE state
		lbaas.waitLoadbalancerReady(lbID)
	} else {
		// Wait for load balancer resource to get deleted
		lbaas.waitLoadbalancerDeleted(lbID)
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

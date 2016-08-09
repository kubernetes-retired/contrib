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

package daemon

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"sync"

	"github.com/golang/glog"
	"k8s.io/contrib/loadbalancer/loadbalancer/backend"
	"k8s.io/contrib/loadbalancer/loadbalancer/controllers"
	"k8s.io/contrib/loadbalancer/loadbalancer/utils"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/unversioned"
)

const (
	configLabelKey       = "loadbalancer"
	configLabelValue     = "daemon"
	defaultConfigMapName = "daemon-configmap"
)

var configMapMutex sync.Mutex
var empty struct{}

// LoadbalancerDaemonController Controller to communicate with loadbalancer-daemon controllers
type LoadbalancerDaemonController struct {
	configMapName string
	kubeClient    *unversioned.Client
	namespace     string
	ipManager     *controllers.IPManager
}

func init() {
	backend.Register("loadbalancer-daemon", NewLoadbalancerDaemonController)
}

// NewLoadbalancerDaemonController creates a loadbalancer-daemon controller
func NewLoadbalancerDaemonController(kubeClient *unversioned.Client, watchNamespace string, conf map[string]string, configLabelKey, configLabelValue string) (backend.BackendController, error) {
	cmName := os.Getenv("LOADBALANCER_CONFIGMAP")
	if cmName == "" {
		cmName = defaultConfigMapName
	}

	ns := os.Getenv("POD_NAMESPACE")
	if ns == "" {
		ns = api.NamespaceDefault
	}

	ipMgr := controllers.NewIPManager(kubeClient, ns, watchNamespace, configLabelKey, configLabelValue)
	if ipMgr == nil {
		glog.Fatalln("NewIPManager returned nil")
	}

	lbControl := LoadbalancerDaemonController{
		configMapName: cmName,
		kubeClient:    kubeClient,
		namespace:     ns,
		ipManager:     ipMgr,
	}

	// sync daemon configmap data with user configmaps
	olddaemonCm := lbControl.getDaemonConfigMap()
	daemonCm := lbControl.getDaemonConfigMap()
	daemonData := daemonCm.Data

	if len(daemonData) != 0 {
		userCms := utils.GetUserConfigMaps(kubeClient, configLabelKey, configLabelValue, watchNamespace)
		cmGroup := utils.GetConfigMapGroups(daemonData)

		// delete daemon group if user configmap doesnt exist
		deleteCm := make(map[string]struct{})
		for k := range cmGroup {
			if _, ok := userCms[k]; !ok {
				deleteCm[k] = empty
			}
		}
		updatedDaemonCm := utils.DeleteConfigMapGroups(daemonData, deleteCm)

		//update Daemon Configmap if its changed
		if !reflect.DeepEqual(olddaemonCm.Data, updatedDaemonCm) {
			_, err := lbControl.kubeClient.ConfigMaps(lbControl.namespace).Update(daemonCm)
			if err != nil {
				glog.Infof("Error updating daemon configmap %v: %v", daemonCm.Name, err)
			}
		}
	}
	return &lbControl, nil
}

// Name returns the name of the backend controller
func (lbControl *LoadbalancerDaemonController) Name() string {
	return "loadbalancer-daemon"
}

// GetBindIP returns the IP used by users to access their apps
func (lbControl *LoadbalancerDaemonController) GetBindIP(name string) (string, error) {
	daemonCM := lbControl.getDaemonConfigMap()
	daemonData := daemonCM.Data
	return daemonData[name+".bind-ip"], nil
}

// HandleConfigMapCreate a new loadbalancer resource
func (lbControl *LoadbalancerDaemonController) HandleConfigMapCreate(configMap *api.ConfigMap) error {

	// Block execution until the ip config map gets updated
	configMapMutex.Lock()
	defer configMapMutex.Unlock()
	name := configMap.Namespace + "-" + configMap.Name
	glog.Infof("Adding group %v to daemon configmap", name)

	daemonCM := lbControl.getDaemonConfigMap()
	daemonData := daemonCM.Data
	cmData := configMap.Data
	namespace := cmData["namespace"]
	serviceName := cmData["target-service-name"]
	serviceObj, err := lbControl.kubeClient.Services(namespace).Get(serviceName)
	if err != nil {
		err = fmt.Errorf("Error getting service object %v/%v. %v", namespace, serviceName, err)
		return err
	}

	//generate Virtual IP
	bindIP, err := lbControl.ipManager.GenerateVirtualIP(configMap)
	if err != nil {
		err = fmt.Errorf("Error generating Virtual IP - %v", err)
		return err
	}

	servicePorts := serviceObj.Spec.Ports
	if len(servicePorts) == 0 {
		err = fmt.Errorf("Could not find any port from service %v.", serviceName)
		return err
	}
	daemonData[name+".namespace"] = namespace
	daemonData[name+".bind-ip"] = bindIP
	daemonData[name+".target-service-name"] = serviceName
	daemonData[name+".target-ip"] = serviceObj.Spec.ClusterIP
	for i, port := range servicePorts {
		daemonData[name+".port"+strconv.Itoa(i)] = strconv.Itoa(int(port.Port))
	}

	_, err = lbControl.kubeClient.ConfigMaps(lbControl.namespace).Update(daemonCM)
	if err != nil {
		glog.Infof("Error updating daemon configmap %v: %v", daemonCM.Name, err)
	}
	return nil
}

// HandleConfigMapDelete delete the group from the configmap that represents a loadbalancer
func (lbControl *LoadbalancerDaemonController) HandleConfigMapDelete(configMap *api.ConfigMap) {
	// Block execution until the ip config map gets updated
	configMapMutex.Lock()
	name := configMap.Namespace + "-" + configMap.Name
	glog.Infof("Deleting group %v from daemon configmap", name)
	daemonCM := lbControl.getDaemonConfigMap()
	daemonData := daemonCM.Data
	delete(daemonData, name+".namespace")
	delete(daemonData, name+".bind-ip")
	delete(daemonData, name+".target-service-name")
	delete(daemonData, name+".target-ip")
	delete(daemonData, name+".target-port")

	i := 0
	for _, exist := daemonData[name+".port"+strconv.Itoa(i)]; exist; {
		delete(daemonData, name+".port"+strconv.Itoa(i))
		i++
		_, exist = daemonData[name+".port"+strconv.Itoa(i)]
	}
	_, err := lbControl.kubeClient.ConfigMaps(lbControl.namespace).Update(daemonCM)
	configMapMutex.Unlock()
	if err != nil {
		glog.Infof("Error updating daemon configmap %v: %v", daemonCM.Name, err)
	}
	// delete configmap from ipConfigMap
	err = lbControl.ipManager.DeleteVirtualIP(name)
	if err != nil {
		glog.Errorf("Error deleting Virtual IP - %v", err)
	}
}

// HandleNodeCreate creates new member for this node in every loadbalancer pool
func (lbControl *LoadbalancerDaemonController) HandleNodeCreate(node *api.Node) {
	glog.Infof("NO operation for add node")
}

// HandleNodeDelete deletes member for this node
func (lbControl *LoadbalancerDaemonController) HandleNodeDelete(node *api.Node) {
	glog.Infof("NO operation for delete node")
}

// HandleNodeUpdate update IP of the member for this node if it exists. If it doesnt, it will create a new member
func (lbControl *LoadbalancerDaemonController) HandleNodeUpdate(oldNode *api.Node, curNode *api.Node) {
	glog.Infof("NO operation for update node")
}

// getDaemonConfigMap get the configmap to be consumed by the daemon, or creates it if it doesnt exist
func (lbControl *LoadbalancerDaemonController) getDaemonConfigMap() *api.ConfigMap {
	cmClient := lbControl.kubeClient.ConfigMaps(lbControl.namespace)
	cm, err := cmClient.Get(lbControl.configMapName)
	if err != nil {
		glog.Infof("ConfigMap %v does not exist. Creating...", lbControl.configMapName)
		labels := make(map[string]string)
		labels[configLabelKey] = configLabelValue
		configMapRequest := &api.ConfigMap{
			ObjectMeta: api.ObjectMeta{
				Name:      lbControl.configMapName,
				Namespace: lbControl.namespace,
				Labels:    labels,
			},
		}
		cm, err = cmClient.Create(configMapRequest)
		if err != nil {
			glog.Infof("Error creating configmap %v", err)
		}
	}
	return cm
}

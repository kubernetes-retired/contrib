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

package controllers

import (
	"bytes"
	"errors"
	"net"
	"sync"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/labels"
)

const (
	ipConfigMapName = "ip-manager-configmap"
)

var ErrIPRangeExhausted = errors.New("Exhausted given Virtual IP range")

var ipConfigMutex sync.Mutex
var empty struct{}

type IPManager struct {
	configMapName string
	namespace     string
	userNamespace string
	ipRange       ipRange
	kubeClient    *unversioned.Client
}

type ipRange struct {
	startIP string
	endIP   string
}

func NewIPManager(kubeClient *unversioned.Client, startIP, endIP, ipCmNamespace, userNamespace, configLabelKey, configLabelValue string) *IPManager {

	ipRange := ipRange{
		startIP: startIP,
		endIP:   endIP,
	}
	ipManager := IPManager{
		configMapName: ipConfigMapName,
		namespace:     ipCmNamespace,
		userNamespace: userNamespace,
		ipRange:       ipRange,
		kubeClient:    kubeClient,
	}

	// check if VIP has changed between failover
	ipCm := ipManager.getConfigMap()
	ipCmData := ipCm.Data
	if len(ipCmData) != 0 {
		for k := range ipCmData {
			if !ipManager.checkInIPRange(k) {
				delete(ipCmData, k)
			}
		}
	}

	err := ipManager.updateIPConfigMap(ipCm)
	if err != nil {
		glog.Errorf("Error updating configmap %v: %v", ipCm.Name, err)
		return nil
	}

	// sync deleted configmaps with ip configmap
	ipMgrCm := ipManager.getConfigMap()
	ipMgrCmData := ipMgrCm.Data
	if len(ipMgrCmData) != 0 {
		var opts api.ListOptions
		opts.LabelSelector = labels.Set{configLabelKey: configLabelValue}.AsSelector()
		cms, err := kubeClient.ConfigMaps(userNamespace).List(opts)
		if err != nil {
			glog.Infof("Error syncing ipconfigmap %v", err)
		}

		cmList := cms.Items
		currentCms := make(map[string]struct{})
		for _, cm := range cmList {
			name := cm.Namespace + "-" + cm.Name
			currentCms[name] = empty
		}

		//update ipconfigmap if user configmap got deleted between reloads
		for k, v := range ipMgrCmData {
			if _, ok := currentCms[v]; !ok {
				delete(ipMgrCmData, k)
			}
		}

		err = ipManager.updateIPConfigMap(ipMgrCm)
		if err != nil {
			glog.Infof("Error updating ip configmap %v: %v", ipMgrCm.Name, err)
			return nil
		}
	}

	return &ipManager
}

func (ipManager *IPManager) checkConfigMap(cmName string) (bool, string) {
	cm := ipManager.getConfigMap()
	cmData := cm.Data
	for k, v := range cmData {
		if v == cmName {
			return true, k
		}
	}
	return false, ""
}

func (ipManager *IPManager) GenerateVirtualIP(configMap *api.ConfigMap) (string, error) {

	// Block execution until the ip config map gets updated with the new virtual IP
	ipConfigMutex.Lock()
	defer ipConfigMutex.Unlock()

	//check if the user configmap entry already exists in ip configmap
	cmName := configMap.Namespace + "-" + configMap.Name
	if ok, vip := ipManager.checkConfigMap(cmName); ok {
		return vip, nil
	}

	virtualIP, err := ipManager.getFreeVirtualIP()
	if err != nil {
		return "", err
	}

	//update ipConfigMap to add new configMap entry
	ipConfigMap := ipManager.getConfigMap()
	ipConfigMapData := ipConfigMap.Data
	name := configMap.Namespace + "-" + configMap.Name
	ipConfigMapData[virtualIP] = name

	err = ipManager.updateIPConfigMap(ipConfigMap)
	if err != nil {
		glog.Infof("Error updating ip configmap %v: %v", ipConfigMap.Name, err)
		return "", err
	}

	return virtualIP, nil
}

func (ipManager *IPManager) DeleteVirtualIP(name string) error {
	ipConfigMap := ipManager.getConfigMap()
	ipConfigMapData := ipConfigMap.Data

	//delete the configMap entry
	for k, v := range ipConfigMapData {
		if v == name {
			delete(ipConfigMapData, k)
			break
		}
	}

	err := ipManager.updateIPConfigMap(ipConfigMap)
	if err != nil {
		glog.Infof("Error updating ip configmap %v: %v", ipConfigMap.Name, err)
		return err
	}
	return nil
}

//gets the ip configmap or creates if it doesn't exist
func (ipManager *IPManager) getConfigMap() *api.ConfigMap {
	cmClient := ipManager.kubeClient.ConfigMaps(ipManager.namespace)
	cm, err := cmClient.Get(ipManager.configMapName)
	if err != nil {
		glog.Infof("ConfigMap %v does not exist. Creating...", ipManager.configMapName)
		configMapRequest := &api.ConfigMap{
			ObjectMeta: api.ObjectMeta{
				Name:      ipManager.configMapName,
				Namespace: ipManager.namespace,
			},
		}
		cm, err = cmClient.Create(configMapRequest)
		if err != nil {
			glog.Infof("Error creating configmap %v", err)
		}
	}
	return cm
}

//generate virtual IP in the given range
func (ipManager *IPManager) getFreeVirtualIP() (string, error) {
	startIPV4 := net.ParseIP(ipManager.ipRange.startIP).To4()
	endIPV4 := net.ParseIP(ipManager.ipRange.endIP).To4()
	temp := startIPV4
	ipConfigMap := ipManager.getConfigMap()
	ipConfigMapData := ipConfigMap.Data

	//check if the start IP is allocated
	if _, ok := ipConfigMapData[ipManager.ipRange.startIP]; !ok {
		return ipManager.ipRange.startIP, nil
	}

	for bytes.Compare(startIPV4, endIPV4) != 0 {
		for i := 3; i >= 0; i-- {
			if temp[i] == 255 {
				temp[i-1]++
			}
		}
		startIPV4[3]++

		if _, ok := ipConfigMapData[temp.String()]; !ok {
			return temp.String(), nil
		}
	}
	return "", ErrIPRangeExhausted
}

// checks if IP is in the given range
func (ipManager *IPManager) checkInIPRange(ip string) bool {
	trial := net.ParseIP(ip)
	startIP := net.ParseIP(ipManager.ipRange.startIP)
	endIP := net.ParseIP(ipManager.ipRange.endIP)

	if trial.To4() == nil {
		glog.Infof("%v is not an IPv4 address\n", trial)
		return false
	}
	if bytes.Compare(trial, startIP) >= 0 && bytes.Compare(trial, endIP) <= 0 {
		return true
	}
	return false
}

// update ip configmap
func (ipManager *IPManager) updateIPConfigMap(configMap *api.ConfigMap) error {
	_, err := ipManager.kubeClient.ConfigMaps(ipManager.namespace).Update(configMap)
	if err != nil {
		return err
	}
	return nil
}

package daemon

import (
	"os"
	"strconv"

	"github.com/golang/glog"
	"k8s.io/contrib/loadbalancer/loadbalancer/backend"
	"k8s.io/contrib/loadbalancer/loadbalancer/utils"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/unversioned"
)

const (
	configLabelKey       = "loadbalancer"
	configLabelValue     = "daemon"
	defaultConfigMapName = "daemon-configmap"
)

// LoadbalancerDaemonController Controller to communicate with loadbalancer-daemon controllers
type LoadbalancerDaemonController struct {
	configMapName string
	kubeClient    *unversioned.Client
	namespace     string
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
	lbControl := LoadbalancerDaemonController{
		configMapName: cmName,
		kubeClient:    kubeClient,
		namespace:     ns,
	}
	return &lbControl, nil
}

// Name returns the name of the backend controller
func (lbControl *LoadbalancerDaemonController) Name() string {
	return "loadbalancer-daemon"
}

// HandleConfigMapCreate a new loadbalancer resource
func (lbControl *LoadbalancerDaemonController) HandleConfigMapCreate(configMap *api.ConfigMap) {
	name := configMap.Namespace + "-" + configMap.Name
	glog.Infof("Adding group %v to daemon configmap", name)

	daemonCM := lbControl.getDaemonConfigMap()
	daemonData := daemonCM.Data
	cmData := configMap.Data
	namespace := cmData["namespace"]
	serviceName := cmData["target-service-name"]
	serviceObj, err := lbControl.kubeClient.Services(namespace).Get(serviceName)
	if err != nil {
		glog.Errorf("Error getting service object %v/%v. %v", namespace, serviceName, err)
		return
	}

	servicePort, err := utils.GetServicePort(serviceObj, cmData["target-port-name"])
	if err != nil {
		glog.Errorf("Error while getting the service port %v", err)
		return
	}

	daemonData[name+".namespace"] = namespace
	daemonData[name+".bind-ip"] = cmData["bind-ip"] // TODO abitha's IP feature
	daemonData[name+".bind-port"] = cmData["bind-port"]
	daemonData[name+".target-service-name"] = serviceName
	daemonData[name+".target-ip"] = serviceObj.Spec.ClusterIP
	daemonData[name+".target-port"] = strconv.Itoa(int(servicePort.Port))

	_, err = lbControl.kubeClient.ConfigMaps(lbControl.namespace).Update(daemonCM)
	if err != nil {
		glog.Infof("Error updating daemon configmap %v: %v", daemonCM.Name, err)
	}

}

// HandleConfigMapDelete the lbaas loadbalancer resource
func (lbControl *LoadbalancerDaemonController) HandleConfigMapDelete(name string) {
	glog.Infof("Deleting group %v from daemon configmap", name)
	daemonCM := lbControl.getDaemonConfigMap()
	daemonData := daemonCM.Data
	delete(daemonData, name+".namespace")
	delete(daemonData, name+".bind-ip")
	delete(daemonData, name+".bind-port")
	delete(daemonData, name+".target-service-name")
	delete(daemonData, name+".target-ip")
	delete(daemonData, name+".target-port")

	_, err := lbControl.kubeClient.ConfigMaps(lbControl.namespace).Update(daemonCM)
	if err != nil {
		glog.Infof("Error updating daemon configmap %v: %v", daemonCM.Name, err)
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

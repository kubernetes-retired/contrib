/*
Copyright 2016 The Kubernetes Authors.

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

package controller

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/util/workqueue"

	"k8s.io/client-go/1.5/dynamic"
	"k8s.io/client-go/1.5/kubernetes"
	"k8s.io/client-go/1.5/pkg/api"
	"k8s.io/client-go/1.5/pkg/api/unversioned"
	"k8s.io/client-go/1.5/pkg/api/v1"
	"k8s.io/client-go/1.5/pkg/runtime"
	"k8s.io/client-go/1.5/pkg/util/wait"
	"k8s.io/client-go/1.5/pkg/watch"
	"k8s.io/client-go/1.5/tools/cache"

	tpapi "k8s.io/contrib/ingress-admin/loadbalancer-controller/api"
	"k8s.io/contrib/ingress-admin/loadbalancer-controller/loadbalancerprovider"
)

var keyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc

const (
	updateLoadBalancerClaimRetryCount = 3
	updateLoadBalancerClaimInterval   = 10 * time.Millisecond
)

var (
	lbcresource = &unversioned.APIResource{Name: "loadbalancerclaims", Kind: "loadbalancerclaim", Namespaced: true}
)

type ProvisionController struct {
	clientset     *kubernetes.Clientset
	dynamicClient *dynamic.Client

	claimController *cache.Controller
	claimStore      cache.Store

	pluginMgr loadbalancerprovider.LoadBalancerPluginMgr

	queue       *workqueue.Type
	syncHandler func(key string) error
}

func NewProvisionController(clientset *kubernetes.Clientset, dynamicClient *dynamic.Client, pluginMgr loadbalancerprovider.LoadBalancerPluginMgr) *ProvisionController {
	pc := &ProvisionController{
		clientset:     clientset,
		dynamicClient: dynamicClient,
		pluginMgr:     pluginMgr,
		queue:         workqueue.New(),
	}

	pc.claimStore, pc.claimController = cache.NewInformer(
		&cache.ListWatch{
			ListFunc: func(options api.ListOptions) (runtime.Object, error) {
				return pc.dynamicClient.Resource(lbcresource, v1.NamespaceAll).List(&api.ListOptions{})
			},
			WatchFunc: func(options api.ListOptions) (watch.Interface, error) {
				return pc.dynamicClient.Resource(lbcresource, v1.NamespaceAll).Watch(&api.ListOptions{})
			},
		},
		&runtime.Unstructured{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: pc.enqueueClaim,
			UpdateFunc: func(old, cur interface{}) {
				pc.enqueueClaim(cur)
			},
			DeleteFunc: pc.enqueueClaim,
		},
	)
	pc.syncHandler = pc.syncClaim
	return pc
}

func (pc *ProvisionController) enqueueClaim(obj interface{}) {
	item := *obj.(*runtime.Unstructured)
	if !isProvisioningNeeded(item.GetAnnotations()) {
		glog.Infof("provision is not needed since annotation is %v", item.GetAnnotations())
		return
	}
	key, err := keyFunc(obj)
	if err != nil {
		glog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	glog.Infof("add %v into work queue", key)
	pc.queue.Add(key)
}

func (pc *ProvisionController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	go pc.claimController.Run(stopCh)
	for i := 0; i < workers; i++ {
		go wait.Until(pc.worker, time.Second, stopCh)
	}
	<-stopCh
	glog.Infof("Shutting down loadbalancer-provision controller")
	pc.queue.ShutDown()
}

func (pc *ProvisionController) worker() {
	for {
		func() {
			key, quit := pc.queue.Get()
			if quit {
				return
			}
			defer pc.queue.Done(key)
			if err := pc.syncHandler(key.(string)); err != nil {
				glog.Errorf("failed to sync %v due to %v", key, err)
				pc.queue.Add(key)
			}
		}()
	}
}

func (pc *ProvisionController) syncClaim(key string) error {
	obj, exists, err := pc.claimStore.GetByKey(key)
	if !exists {
		glog.Errorf("loadbalancerclaim has been deleted %v", key)
		return nil
	}
	if err != nil {
		glog.Errorf("Unable to get obj from local store due to: %v", err)
		return err
	}

	claim, err := tpapi.ToLoadbalancerClaim(obj.(*runtime.Unstructured))
	if err != nil {
		glog.Errorf("Unable to convert obj to runtime.Unstructured due to: %v", err)
		return err
	}

	if !isProvisioningNeeded(claim.GetAnnotations()) {
		glog.Infof("provision is not needed for %v", claim.Name)
		return nil
	}

	lbName, provisionErr := pc.provosion(claim)
	if provisionErr != nil {
		glog.Errorf("failed to provision %v due to %v", key, provisionErr)
	}

	return pc.updateLoadBalancerClaimStatus(claim, lbName, provisionErr)
}

func (pc *ProvisionController) provosion(claim *tpapi.LoadBalancerClaim) (string, error) {
	plugin, err := pc.pluginMgr.FindPluginBySpec(claim)
	if err != nil {
		return "", err
	}

	resourceList, err := getResourceList(claim.Annotations)
	if err != nil {
		return "", err
	}

	provisioner := plugin.NewProvisioner(loadbalancerprovider.LoadBalancerOptions{
		Resources: v1.ResourceRequirements{
			Requests: *resourceList,
			Limits:   *resourceList,
		},
		LoadBalancerName: generateLoadBalancerName(claim),
		LoadBalancerVIP:  claim.Annotations[IngressParameterVIPKey],
	})

	return provisioner.Provision(pc.clientset, pc.dynamicClient)
}

func (pc *ProvisionController) updateLoadBalancerClaimStatus(claim *tpapi.LoadBalancerClaim, lbName string, provisionErr error) error {
	for i := 0; i < updateLoadBalancerClaimRetryCount; i++ {
		if err := func() error {
			unstructed, err := pc.dynamicClient.Resource(lbcresource, claim.Namespace).Get(claim.Name)
			if err != nil {
				return err
			}

			claimNew, err := tpapi.ToLoadbalancerClaim(unstructed)
			if err != nil {
				return err
			}

			if provisionErr != nil {
				claimNew.Annotations[ingressProvisioningRequiredAnnotationKey] = ingressProvisioningFailedAnnotationValue
				claimNew.Status.Phase = tpapi.LoadBalancerClaimFailed
				claimNew.Status.Message = provisionErr.Error()
			} else {
				claimNew.Spec.LoadBalancerName = lbName
				claimNew.Annotations[ingressProvisioningRequiredAnnotationKey] = ingressProvisioningCompletedAnnotationValue
				claimNew.Status.Phase = tpapi.LoadBalancerClaimBound
				claimNew.Status.Message = "Provision Succeeded"
			}

			unstructedNew, err := claimNew.ToUnstructured()
			if err != nil {
				return err
			}

			if _, err := pc.dynamicClient.Resource(lbcresource, claim.Namespace).Update(unstructedNew); err != nil {
				return err
			}

			return nil
		}(); err != nil {
			glog.Errorf("filed to update loadbalancer due to: %v", err)
			time.Sleep(updateLoadBalancerClaimInterval)
		}

		return nil
	}

	return fmt.Errorf("Failed to update loadbalancer claim status")
}

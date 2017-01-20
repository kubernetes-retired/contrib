/*
Copyright 2014 The Kubernetes Authors.

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

// Package factory can set up a scheduler. This code is here instead of
// plugin/cmd/scheduler for both testability and reuse.
package factory

import (
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
	genericapirequest "k8s.io/apiserver/pkg/request"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	"k8s.io/kubernetes/pkg/controller/informers"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/plugin/pkg/scheduler"
	"k8s.io/kubernetes/plugin/pkg/scheduler/algorithm"
	"k8s.io/kubernetes/plugin/pkg/scheduler/algorithm/predicates"
	schedulerapi "k8s.io/kubernetes/plugin/pkg/scheduler/api"
	"k8s.io/kubernetes/plugin/pkg/scheduler/api/validation"

	"github.com/golang/glog"
	extensions "k8s.io/kubernetes/pkg/apis/extensions/v1beta1"
	"k8s.io/kubernetes/plugin/pkg/scheduler/schedulercache"
	"k8s.io/kubernetes/plugin/pkg/scheduler/util"
)

const (
	SchedulerAnnotationKey = "scheduler.alpha.kubernetes.io/name"
	initialGetBackoff      = 100 * time.Millisecond
	maximalGetBackoff      = time.Minute
)

// ConfigFactory is the default implementation of the scheduler.Configurator interface.
// TODO make this private if possible, so that only its interface is externally used.
type ConfigFactory struct {
	client clientset.Interface
	// queue for pods that need scheduling
	podQueue *cache.FIFO
	// a means to list all known scheduled pods.
	scheduledPodLister *cache.StoreToPodLister
	// a means to list all known scheduled pods and pods assumed to have been scheduled.
	podLister algorithm.PodLister
	// a means to list all nodes
	nodeLister *cache.StoreToNodeLister
	// a means to list all PersistentVolumes
	pVLister *cache.StoreToPVFetcher
	// a means to list all PersistentVolumeClaims
	pVCLister *cache.StoreToPersistentVolumeClaimLister
	// a means to list all services
	serviceLister *cache.StoreToServiceLister
	// a means to list all controllers
	controllerLister *cache.StoreToReplicationControllerLister
	// a means to list all replicasets
	replicaSetLister *cache.StoreToReplicaSetLister

	// Close this to stop all reflectors
	StopEverything chan struct{}

	informerFactory       informers.SharedInformerFactory
	scheduledPodPopulator cache.Controller
	nodePopulator         cache.Controller
	pvPopulator           cache.Controller
	pvcPopulator          cache.Controller
	servicePopulator      cache.Controller
	controllerPopulator   cache.Controller

	schedulerCache schedulercache.Cache

	// SchedulerName of a scheduler is used to select which pods will be
	// processed by this scheduler, based on pods's annotation key:
	// 'scheduler.alpha.kubernetes.io/name'
	schedulerName string

	// RequiredDuringScheduling affinity is not symmetric, but there is an implicit PreferredDuringScheduling affinity rule
	// corresponding to every RequiredDuringScheduling affinity rule.
	// HardPodAffinitySymmetricWeight represents the weight of implicit PreferredDuringScheduling affinity rule, in the range 0-100.
	hardPodAffinitySymmetricWeight int

	// Indicate the "all topologies" set for empty topologyKey when it's used for PreferredDuringScheduling pod anti-affinity.
	failureDomains []string

	// Equivalence class cache
	equivalencePodCache *scheduler.EquivalenceCache
}

// NewConfigFactory initializes the default implementation of a Configurator To encourage eventual privatization of the struct type, we only
// return the interface.
func NewConfigFactory(client clientset.Interface, schedulerName string, hardPodAffinitySymmetricWeight int, failureDomains string) scheduler.Configurator {
	stopEverything := make(chan struct{})
	schedulerCache := schedulercache.New(30*time.Second, stopEverything)

	// TODO: pass this in as an argument...
	informerFactory := informers.NewSharedInformerFactory(client, nil, 0)
	pvcInformer := informerFactory.PersistentVolumeClaims()

	c := &ConfigFactory{
		client:             client,
		podQueue:           cache.NewFIFO(cache.MetaNamespaceKeyFunc),
		scheduledPodLister: &cache.StoreToPodLister{},
		informerFactory:    informerFactory,
		// Only nodes in the "Ready" condition with status == "True" are schedulable
		nodeLister:                     &cache.StoreToNodeLister{},
		pVLister:                       &cache.StoreToPVFetcher{Store: cache.NewStore(cache.MetaNamespaceKeyFunc)},
		pVCLister:                      pvcInformer.Lister(),
		pvcPopulator:                   pvcInformer.Informer().GetController(),
		serviceLister:                  &cache.StoreToServiceLister{Indexer: cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})},
		controllerLister:               &cache.StoreToReplicationControllerLister{Indexer: cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})},
		replicaSetLister:               &cache.StoreToReplicaSetLister{Indexer: cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})},
		schedulerCache:                 schedulerCache,
		StopEverything:                 stopEverything,
		schedulerName:                  schedulerName,
		hardPodAffinitySymmetricWeight: hardPodAffinitySymmetricWeight,
		failureDomains:                 strings.Split(failureDomains, ","),
	}

	c.podLister = schedulerCache

	// On add/delete to the scheduled pods, remove from the assumed pods.
	// We construct this here instead of in CreateFromKeys because
	// ScheduledPodLister is something we provide to plug in functions that
	// they may need to call.
	c.scheduledPodLister.Indexer, c.scheduledPodPopulator = cache.NewIndexerInformer(
		c.createAssignedNonTerminatedPodLW(),
		&v1.Pod{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    c.addPodToCache,
			UpdateFunc: c.updatePodInCache,
			DeleteFunc: c.deletePodFromCache,
		},
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)

	c.nodeLister.Store, c.nodePopulator = cache.NewInformer(
		c.createNodeLW(),
		&v1.Node{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    c.addNodeToCache,
			UpdateFunc: c.updateNodeInCache,
			DeleteFunc: c.deleteNodeFromCache,
		},
	)

	// TODO(harryz) need to fill all the handlers here and below for equivalence cache
	c.pVLister.Store, c.pvPopulator = cache.NewInformer(
		c.createPersistentVolumeLW(),
		&v1.PersistentVolume{},
		0,
		cache.ResourceEventHandlerFuncs{},
	)

	c.serviceLister.Indexer, c.servicePopulator = cache.NewIndexerInformer(
		c.createServiceLW(),
		&v1.Service{},
		0,
		cache.ResourceEventHandlerFuncs{},
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)

	c.controllerLister.Indexer, c.controllerPopulator = cache.NewIndexerInformer(
		c.createControllerLW(),
		&v1.ReplicationController{},
		0,
		cache.ResourceEventHandlerFuncs{},
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)

	return c
}

// GetNodeStore provides the cache to the nodes, mostly internal use, but may also be called by mock-tests.
func (c *ConfigFactory) GetNodeStore() cache.Store {
	return c.nodeLister.Store
}

func (c *ConfigFactory) GetHardPodAffinitySymmetricWeight() int {
	return c.hardPodAffinitySymmetricWeight
}

func (c *ConfigFactory) GetFailureDomains() []string {
	return c.failureDomains
}

func (f *ConfigFactory) GetSchedulerName() string {
	return f.schedulerName
}

// GetClient provides a kubernetes client, mostly internal use, but may also be called by mock-tests.
func (f *ConfigFactory) GetClient() clientset.Interface {
	return f.client
}

// GetScheduledPodListerIndexer provides a pod lister, mostly internal use, but may also be called by mock-tests.
func (c *ConfigFactory) GetScheduledPodListerIndexer() cache.Indexer {
	return c.scheduledPodLister.Indexer
}

// TODO(harryz) need to update all the handlers here and below for equivalence cache
func (c *ConfigFactory) addPodToCache(obj interface{}) {
	pod, ok := obj.(*v1.Pod)
	if !ok {
		glog.Errorf("cannot convert to *v1.Pod: %v", obj)
		return
	}

	if err := c.schedulerCache.AddPod(pod); err != nil {
		glog.Errorf("scheduler cache AddPod failed: %v", err)
	}
}

func (c *ConfigFactory) updatePodInCache(oldObj, newObj interface{}) {
	oldPod, ok := oldObj.(*v1.Pod)
	if !ok {
		glog.Errorf("cannot convert oldObj to *v1.Pod: %v", oldObj)
		return
	}
	newPod, ok := newObj.(*v1.Pod)
	if !ok {
		glog.Errorf("cannot convert newObj to *v1.Pod: %v", newObj)
		return
	}

	if err := c.schedulerCache.UpdatePod(oldPod, newPod); err != nil {
		glog.Errorf("scheduler cache UpdatePod failed: %v", err)
	}
}

func (c *ConfigFactory) deletePodFromCache(obj interface{}) {
	var pod *v1.Pod
	switch t := obj.(type) {
	case *v1.Pod:
		pod = t
	case cache.DeletedFinalStateUnknown:
		var ok bool
		pod, ok = t.Obj.(*v1.Pod)
		if !ok {
			glog.Errorf("cannot convert to *v1.Pod: %v", t.Obj)
			return
		}
	default:
		glog.Errorf("cannot convert to *v1.Pod: %v", t)
		return
	}
	if err := c.schedulerCache.RemovePod(pod); err != nil {
		glog.Errorf("scheduler cache RemovePod failed: %v", err)
	}
}

func (c *ConfigFactory) addNodeToCache(obj interface{}) {
	node, ok := obj.(*v1.Node)
	if !ok {
		glog.Errorf("cannot convert to *v1.Node: %v", obj)
		return
	}

	if err := c.schedulerCache.AddNode(node); err != nil {
		glog.Errorf("scheduler cache AddNode failed: %v", err)
	}
}

func (c *ConfigFactory) updateNodeInCache(oldObj, newObj interface{}) {
	oldNode, ok := oldObj.(*v1.Node)
	if !ok {
		glog.Errorf("cannot convert oldObj to *v1.Node: %v", oldObj)
		return
	}
	newNode, ok := newObj.(*v1.Node)
	if !ok {
		glog.Errorf("cannot convert newObj to *v1.Node: %v", newObj)
		return
	}

	if err := c.schedulerCache.UpdateNode(oldNode, newNode); err != nil {
		glog.Errorf("scheduler cache UpdateNode failed: %v", err)
	}
}

func (c *ConfigFactory) deleteNodeFromCache(obj interface{}) {
	var node *v1.Node
	switch t := obj.(type) {
	case *v1.Node:
		node = t
	case cache.DeletedFinalStateUnknown:
		var ok bool
		node, ok = t.Obj.(*v1.Node)
		if !ok {
			glog.Errorf("cannot convert to *v1.Node: %v", t.Obj)
			return
		}
	default:
		glog.Errorf("cannot convert to *v1.Node: %v", t)
		return
	}
	if err := c.schedulerCache.RemoveNode(node); err != nil {
		glog.Errorf("scheduler cache RemoveNode failed: %v", err)
	}
}

// Create creates a scheduler with the default algorithm provider.
func (f *ConfigFactory) Create() (*scheduler.Config, error) {
	return f.CreateFromProvider(DefaultProvider)
}

// Creates a scheduler from the name of a registered algorithm provider.
func (f *ConfigFactory) CreateFromProvider(providerName string) (*scheduler.Config, error) {
	glog.V(2).Infof("Creating scheduler from algorithm provider '%v'", providerName)
	provider, err := GetAlgorithmProvider(providerName)
	if err != nil {
		return nil, err
	}

	return f.CreateFromKeys(provider.FitPredicateKeys, provider.PriorityFunctionKeys, []algorithm.SchedulerExtender{})
}

// Creates a scheduler from the configuration file
func (f *ConfigFactory) CreateFromConfig(policy schedulerapi.Policy) (*scheduler.Config, error) {
	glog.V(2).Infof("Creating scheduler from configuration: %v", policy)

	// validate the policy configuration
	if err := validation.ValidatePolicy(policy); err != nil {
		return nil, err
	}

	predicateKeys := sets.NewString()
	for _, predicate := range policy.Predicates {
		glog.V(2).Infof("Registering predicate: %s", predicate.Name)
		predicateKeys.Insert(RegisterCustomFitPredicate(predicate))
	}

	priorityKeys := sets.NewString()
	for _, priority := range policy.Priorities {
		glog.V(2).Infof("Registering priority: %s", priority.Name)
		priorityKeys.Insert(RegisterCustomPriorityFunction(priority))
	}

	extenders := make([]algorithm.SchedulerExtender, 0)
	if len(policy.ExtenderConfigs) != 0 {
		for ii := range policy.ExtenderConfigs {
			glog.V(2).Infof("Creating extender with config %+v", policy.ExtenderConfigs[ii])
			if extender, err := scheduler.NewHTTPExtender(&policy.ExtenderConfigs[ii], policy.APIVersion); err != nil {
				return nil, err
			} else {
				extenders = append(extenders, extender)
			}
		}
	}
	return f.CreateFromKeys(predicateKeys, priorityKeys, extenders)
}

// Creates a scheduler from a set of registered fit predicate keys and priority keys.
func (f *ConfigFactory) CreateFromKeys(predicateKeys, priorityKeys sets.String, extenders []algorithm.SchedulerExtender) (*scheduler.Config, error) {
	glog.V(2).Infof("Creating scheduler with fit predicates '%v' and priority functions '%v", predicateKeys, priorityKeys)

	if f.GetHardPodAffinitySymmetricWeight() < 0 || f.GetHardPodAffinitySymmetricWeight() > 100 {
		return nil, fmt.Errorf("invalid hardPodAffinitySymmetricWeight: %d, must be in the range 0-100", f.GetHardPodAffinitySymmetricWeight())
	}

	predicateFuncs, err := f.GetPredicates(predicateKeys)
	if err != nil {
		return nil, err
	}

	priorityConfigs, err := f.GetPriorityFunctionConfigs(priorityKeys)
	if err != nil {
		return nil, err
	}

	priorityMetaProducer, err := f.GetPriorityMetadataProducer()
	if err != nil {
		return nil, err
	}

	predicateMetaProducer, err := f.GetPredicateMetadataProducer()
	if err != nil {
		return nil, err
	}

	f.Run()
	algo := scheduler.NewGenericScheduler(f.schedulerCache, predicateFuncs, predicateMetaProducer, priorityConfigs, priorityMetaProducer, extenders)
	podBackoff := util.CreateDefaultPodBackoff()
	return &scheduler.Config{
		SchedulerCache: f.schedulerCache,
		// The scheduler only needs to consider schedulable nodes.
		NodeLister:          f.nodeLister.NodeCondition(getNodeConditionPredicate()),
		Algorithm:           algo,
		Binder:              &binder{f.client},
		PodConditionUpdater: &podConditionUpdater{f.client},
		NextPod: func() *v1.Pod {
			return f.getNextPod()
		},
		Error:          f.MakeDefaultErrorFunc(podBackoff, f.podQueue),
		StopEverything: f.StopEverything,
	}, nil
}

func (f *ConfigFactory) GetPriorityFunctionConfigs(priorityKeys sets.String) ([]algorithm.PriorityConfig, error) {
	pluginArgs, err := f.getPluginArgs()
	if err != nil {
		return nil, err
	}

	return getPriorityFunctionConfigs(priorityKeys, *pluginArgs)
}

func (f *ConfigFactory) GetPriorityMetadataProducer() (algorithm.MetadataProducer, error) {
	pluginArgs, err := f.getPluginArgs()
	if err != nil {
		return nil, err
	}

	return getPriorityMetadataProducer(*pluginArgs)
}

func (f *ConfigFactory) GetPredicateMetadataProducer() (algorithm.MetadataProducer, error) {
	pluginArgs, err := f.getPluginArgs()
	if err != nil {
		return nil, err
	}
	return getPredicateMetadataProducer(*pluginArgs)
}

func (f *ConfigFactory) GetPredicates(predicateKeys sets.String) (map[string]algorithm.FitPredicate, error) {
	pluginArgs, err := f.getPluginArgs()
	if err != nil {
		return nil, err
	}

	return getFitPredicateFunctions(predicateKeys, *pluginArgs)
}

func (f *ConfigFactory) getPluginArgs() (*PluginFactoryArgs, error) {
	for _, failureDomain := range f.failureDomains {
		if errs := utilvalidation.IsQualifiedName(failureDomain); len(errs) != 0 {
			return nil, fmt.Errorf("invalid failure domain: %q: %s", failureDomain, strings.Join(errs, ";"))
		}
	}

	return &PluginFactoryArgs{
		PodLister:        f.podLister,
		ServiceLister:    f.serviceLister,
		ControllerLister: f.controllerLister,
		ReplicaSetLister: f.replicaSetLister,
		// All fit predicates only need to consider schedulable nodes.
		NodeLister: f.nodeLister.NodeCondition(getNodeConditionPredicate()),
		NodeInfo:   &predicates.CachedNodeInfo{StoreToNodeLister: f.nodeLister},
		PVInfo:     f.pVLister,
		PVCInfo:    &predicates.CachedPersistentVolumeClaimInfo{StoreToPersistentVolumeClaimLister: f.pVCLister},
		HardPodAffinitySymmetricWeight: f.hardPodAffinitySymmetricWeight,
		FailureDomains:                 sets.NewString(f.failureDomains...).List(),
	}, nil
}

func (f *ConfigFactory) Run() {
	// Watch and queue pods that need scheduling.
	cache.NewReflector(f.createUnassignedNonTerminatedPodLW(), &v1.Pod{}, f.podQueue, 0).RunUntil(f.StopEverything)

	// Begin populating scheduled pods.
	go f.scheduledPodPopulator.Run(f.StopEverything)

	// Begin populating nodes.
	go f.nodePopulator.Run(f.StopEverything)

	// Begin populating pv & pvc
	go f.pvPopulator.Run(f.StopEverything)
	go f.pvcPopulator.Run(f.StopEverything)

	// Begin populating services
	go f.servicePopulator.Run(f.StopEverything)

	// Begin populating controllers
	go f.controllerPopulator.Run(f.StopEverything)

	// start informers...
	f.informerFactory.Start(f.StopEverything)

	// Watch and cache all ReplicaSet objects. Scheduler needs to find all pods
	// created by the same services or ReplicationControllers/ReplicaSets, so that it can spread them correctly.
	// Cache this locally.
	cache.NewReflector(f.createReplicaSetLW(), &extensions.ReplicaSet{}, f.replicaSetLister.Indexer, 0).RunUntil(f.StopEverything)
}

func (f *ConfigFactory) getNextPod() *v1.Pod {
	for {
		pod := cache.Pop(f.podQueue).(*v1.Pod)
		if f.ResponsibleForPod(pod) {
			glog.V(4).Infof("About to try and schedule pod %v", pod.Name)
			return pod
		}
	}
}

func (f *ConfigFactory) ResponsibleForPod(pod *v1.Pod) bool {
	if f.schedulerName == v1.DefaultSchedulerName {
		return pod.Annotations[SchedulerAnnotationKey] == f.schedulerName || pod.Annotations[SchedulerAnnotationKey] == ""
	} else {
		return pod.Annotations[SchedulerAnnotationKey] == f.schedulerName
	}
}

func getNodeConditionPredicate() cache.NodeConditionPredicate {
	return func(node *v1.Node) bool {
		for i := range node.Status.Conditions {
			cond := &node.Status.Conditions[i]
			// We consider the node for scheduling only when its:
			// - NodeReady condition status is ConditionTrue,
			// - NodeOutOfDisk condition status is ConditionFalse,
			// - NodeNetworkUnavailable condition status is ConditionFalse.
			if cond.Type == v1.NodeReady && cond.Status != v1.ConditionTrue {
				glog.V(4).Infof("Ignoring node %v with %v condition status %v", node.Name, cond.Type, cond.Status)
				return false
			} else if cond.Type == v1.NodeOutOfDisk && cond.Status != v1.ConditionFalse {
				glog.V(4).Infof("Ignoring node %v with %v condition status %v", node.Name, cond.Type, cond.Status)
				return false
			} else if cond.Type == v1.NodeNetworkUnavailable && cond.Status != v1.ConditionFalse {
				glog.V(4).Infof("Ignoring node %v with %v condition status %v", node.Name, cond.Type, cond.Status)
				return false
			}
		}
		// Ignore nodes that are marked unschedulable
		if node.Spec.Unschedulable {
			glog.V(4).Infof("Ignoring node %v since it is unschedulable", node.Name)
			return false
		}
		return true
	}
}

// Returns a cache.ListWatch that finds all pods that need to be
// scheduled.
func (factory *ConfigFactory) createUnassignedNonTerminatedPodLW() *cache.ListWatch {
	selector := fields.ParseSelectorOrDie("spec.nodeName==" + "" + ",status.phase!=" + string(v1.PodSucceeded) + ",status.phase!=" + string(v1.PodFailed))
	return cache.NewListWatchFromClient(factory.client.Core().RESTClient(), "pods", v1.NamespaceAll, selector)
}

// Returns a cache.ListWatch that finds all pods that are
// already scheduled.
// TODO: return a ListerWatcher interface instead?
func (factory *ConfigFactory) createAssignedNonTerminatedPodLW() *cache.ListWatch {
	selector := fields.ParseSelectorOrDie("spec.nodeName!=" + "" + ",status.phase!=" + string(v1.PodSucceeded) + ",status.phase!=" + string(v1.PodFailed))
	return cache.NewListWatchFromClient(factory.client.Core().RESTClient(), "pods", v1.NamespaceAll, selector)
}

// createNodeLW returns a cache.ListWatch that gets all changes to nodes.
func (factory *ConfigFactory) createNodeLW() *cache.ListWatch {
	// all nodes are considered to ensure that the scheduler cache has access to all nodes for lookups
	// the NodeCondition is used to filter out the nodes that are not ready or unschedulable
	// the filtered list is used as the super set of nodes to consider for scheduling
	return cache.NewListWatchFromClient(factory.client.Core().RESTClient(), "nodes", v1.NamespaceAll, fields.ParseSelectorOrDie(""))
}

// createPersistentVolumeLW returns a cache.ListWatch that gets all changes to persistentVolumes.
func (factory *ConfigFactory) createPersistentVolumeLW() *cache.ListWatch {
	return cache.NewListWatchFromClient(factory.client.Core().RESTClient(), "persistentVolumes", v1.NamespaceAll, fields.ParseSelectorOrDie(""))
}

// createPersistentVolumeClaimLW returns a cache.ListWatch that gets all changes to persistentVolumeClaims.
func (factory *ConfigFactory) createPersistentVolumeClaimLW() *cache.ListWatch {
	return cache.NewListWatchFromClient(factory.client.Core().RESTClient(), "persistentVolumeClaims", v1.NamespaceAll, fields.ParseSelectorOrDie(""))
}

// Returns a cache.ListWatch that gets all changes to services.
func (factory *ConfigFactory) createServiceLW() *cache.ListWatch {
	return cache.NewListWatchFromClient(factory.client.Core().RESTClient(), "services", v1.NamespaceAll, fields.ParseSelectorOrDie(""))
}

// Returns a cache.ListWatch that gets all changes to controllers.
func (factory *ConfigFactory) createControllerLW() *cache.ListWatch {
	return cache.NewListWatchFromClient(factory.client.Core().RESTClient(), "replicationControllers", v1.NamespaceAll, fields.ParseSelectorOrDie(""))
}

// Returns a cache.ListWatch that gets all changes to replicasets.
func (factory *ConfigFactory) createReplicaSetLW() *cache.ListWatch {
	return cache.NewListWatchFromClient(factory.client.Extensions().RESTClient(), "replicasets", v1.NamespaceAll, fields.ParseSelectorOrDie(""))
}

func (factory *ConfigFactory) MakeDefaultErrorFunc(backoff *util.PodBackoff, podQueue *cache.FIFO) func(pod *v1.Pod, err error) {
	return func(pod *v1.Pod, err error) {
		if err == scheduler.ErrNoNodesAvailable {
			glog.V(4).Infof("Unable to schedule %v %v: no nodes are registered to the cluster; waiting", pod.Namespace, pod.Name)
		} else {
			glog.Errorf("Error scheduling %v %v: %v; retrying", pod.Namespace, pod.Name, err)
		}
		backoff.Gc()
		// Retry asynchronously.
		// Note that this is extremely rudimentary and we need a more real error handling path.
		go func() {
			defer runtime.HandleCrash()
			podID := types.NamespacedName{
				Namespace: pod.Namespace,
				Name:      pod.Name,
			}

			entry := backoff.GetEntry(podID)
			if !entry.TryWait(backoff.MaxDuration()) {
				glog.Warningf("Request for pod %v already in flight, abandoning", podID)
				return
			}
			// Get the pod again; it may have changed/been scheduled already.
			getBackoff := initialGetBackoff
			for {
				pod, err := factory.client.Core().Pods(podID.Namespace).Get(podID.Name, metav1.GetOptions{})
				if err == nil {
					if len(pod.Spec.NodeName) == 0 {
						podQueue.AddIfNotPresent(pod)
					}
					break
				}
				if errors.IsNotFound(err) {
					glog.Warningf("A pod %v no longer exists", podID)
					return
				}
				glog.Errorf("Error getting pod %v for retry: %v; retrying...", podID, err)
				if getBackoff = getBackoff * 2; getBackoff > maximalGetBackoff {
					getBackoff = maximalGetBackoff
				}
				time.Sleep(getBackoff)
			}
		}()
	}
}

// nodeEnumerator allows a cache.Poller to enumerate items in an v1.NodeList
type nodeEnumerator struct {
	*v1.NodeList
}

// Len returns the number of items in the node list.
func (ne *nodeEnumerator) Len() int {
	if ne.NodeList == nil {
		return 0
	}
	return len(ne.Items)
}

// Get returns the item (and ID) with the particular index.
func (ne *nodeEnumerator) Get(index int) interface{} {
	return &ne.Items[index]
}

type binder struct {
	Client clientset.Interface
}

// Bind just does a POST binding RPC.
func (b *binder) Bind(binding *v1.Binding) error {
	glog.V(3).Infof("Attempting to bind %v to %v", binding.Name, binding.Target.Name)
	ctx := genericapirequest.WithNamespace(genericapirequest.NewContext(), binding.Namespace)
	return b.Client.Core().RESTClient().Post().Namespace(genericapirequest.NamespaceValue(ctx)).Resource("bindings").Body(binding).Do().Error()
	// TODO: use Pods interface for binding once clusters are upgraded
	// return b.Pods(binding.Namespace).Bind(binding)
}

type podConditionUpdater struct {
	Client clientset.Interface
}

func (p *podConditionUpdater) Update(pod *v1.Pod, condition *v1.PodCondition) error {
	glog.V(2).Infof("Updating pod condition for %s/%s to (%s==%s)", pod.Namespace, pod.Name, condition.Type, condition.Status)
	if v1.UpdatePodCondition(&pod.Status, condition) {
		_, err := p.Client.Core().Pods(pod.Namespace).UpdateStatus(pod)
		return err
	}
	return nil
}

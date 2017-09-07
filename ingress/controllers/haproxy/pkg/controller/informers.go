/*
Copyright 2015 The Kubernetes Authors.

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
	"reflect"

	log "github.com/golang/glog"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"
)

// setIngressInformer creates an ingress informer (store and controller)
func (lbc *LoadBalancerController) setIngressInformer() {
	log.Info("setting up Ingress informer")

	ingressHandler := framework.ResourceEventHandlerFuncs{
		AddFunc: func(o interface{}) {
			ing := o.(*extensions.Ingress)
			if lbc.ingressFilterChecksNeeded && !ingAnnotations(ing.ObjectMeta.Annotations).filterClass(lbc.ingressFilter) {
				log.Infof("ignoring add for ingress '%v' based on class annotation filtering", ing.Name)
				return
			}
			lbc.recorder.Eventf(ing, api.EventTypeNormal, "create", fmt.Sprintf("%s/%s", ing.Namespace, ing.Name))
			lbc.ingQueue.enqueue(o)
			lbc.syncQueue.enqueue(o)
		},
		DeleteFunc: func(o interface{}) {
			ing := o.(*extensions.Ingress)
			if lbc.ingressFilterChecksNeeded && !ingAnnotations(ing.ObjectMeta.Annotations).filterClass(lbc.ingressFilter) {
				log.Infof("ignoring delete for ingress %v based on class annotation filtering", ing.Name)
				return
			}
			lbc.recorder.Eventf(ing, api.EventTypeNormal, "delete", fmt.Sprintf("%s/%s", ing.Namespace, ing.Name))
			lbc.syncQueue.enqueue(o)
		},
		UpdateFunc: func(old, cur interface{}) {
			ing := cur.(*extensions.Ingress)
			if lbc.ingressFilterChecksNeeded && !ingAnnotations(ing.ObjectMeta.Annotations).filterClass(lbc.ingressFilter) {
				log.Infof("ignoring update for ingress %v based on class annotation filtering", ing.Name)
				return
			}
			if !reflect.DeepEqual(old, cur) {
				lbc.recorder.Eventf(ing, api.EventTypeNormal, "update", fmt.Sprintf("%s/%s", ing.Namespace, ing.Name))
				lbc.ingQueue.enqueue(cur)
				lbc.syncQueue.enqueue(cur)
			}
		},
	}

	listFunc := func(lo api.ListOptions) (runtime.Object, error) {
		log.V(2).Info("listing ingresses")
		return lbc.client.Extensions().Ingress(lbc.namespace).List(lo)
	}

	watchFunc := func(lo api.ListOptions) (watch.Interface, error) {
		log.V(2).Info("watching ingresses")
		return lbc.client.Extensions().Ingress(lbc.namespace).Watch(lo)
	}

	lw := &cache.ListWatch{
		ListFunc:  listFunc,
		WatchFunc: watchFunc,
	}

	lbc.ingressStore, lbc.ingController = framework.NewInformer(lw, &extensions.Ingress{}, lbc.resyncPeriod, ingressHandler)
}

// setServiceInformer creates a service informer (store and controller)
func (lbc *LoadBalancerController) setServiceInformer() {
	log.Info("setting up service informer")

	serviceHandler := framework.ResourceEventHandlerFuncs{
		AddFunc: func(o interface{}) {
			svc := o.(*api.Service)
			lbc.recorder.Eventf(svc, api.EventTypeNormal, "create", fmt.Sprintf("%s/%s", svc.Namespace, svc.Name))
			lbc.syncQueue.enqueue(svc)
		},
		DeleteFunc: func(o interface{}) {
			svc := o.(*api.Service)
			lbc.recorder.Eventf(svc, api.EventTypeNormal, "delete", fmt.Sprintf("%s/%s", svc.Namespace, svc.Name))
			lbc.syncQueue.enqueue(svc)
		},
		UpdateFunc: func(old, cur interface{}) {
			if !reflect.DeepEqual(old, cur) {
				curSvc := cur.(*api.Service)
				lbc.recorder.Eventf(curSvc, api.EventTypeNormal, "update", fmt.Sprintf("%s/%s", curSvc.Namespace, curSvc.Name))
				lbc.syncQueue.enqueue(cur)
			}
		},
	}

	// TODO check how kube proxy is listening to services
	// https://github.com/kubernetes/kubernetes/blob/f2ddd60eb9e7e9e29f7a105a9a8fa020042e8e52/pkg/proxy/config/api.go#L28
	lw := cache.NewListWatchFromClient(lbc.client, "services", api.NamespaceAll, fields.Everything())

	lbc.svcLister.Store, lbc.svcController = framework.NewInformer(lw, &api.Service{}, lbc.resyncPeriod, serviceHandler)
}

// setEndPointInformer creates an endpoint Informer (store and controller)
func (lbc *LoadBalancerController) setEndPointInformer() {
	log.Info("setting up endpoint informer")

	endpointHandler := framework.ResourceEventHandlerFuncs{
		AddFunc: func(o interface{}) {
			endp := o.(*api.Endpoints)
			lbc.recorder.Eventf(endp, api.EventTypeNormal, "create", fmt.Sprintf("%s/%s", endp.Namespace, endp.Name))
			lbc.syncQueue.enqueue(endp)
		},
		DeleteFunc: func(o interface{}) {
			endp := o.(*api.Endpoints)
			lbc.recorder.Eventf(endp, api.EventTypeNormal, "delete", fmt.Sprintf("%s/%s", endp.Namespace, endp.Name))
			lbc.syncQueue.enqueue(endp)
		},
		UpdateFunc: func(old, cur interface{}) {

			if !reflect.DeepEqual(old, cur) {
				endp := cur.(*api.Endpoints)
				lbc.recorder.Eventf(endp, api.EventTypeNormal, "update", fmt.Sprintf("%s/%s", endp.Namespace, endp.Name))
				lbc.syncQueue.enqueue(cur)
			}
		},
	}

	listFunc := func(lo api.ListOptions) (runtime.Object, error) {
		log.V(2).Info("listing endpoints")
		return lbc.client.Endpoints(lbc.namespace).List(lo)
	}

	watchFunc := func(lo api.ListOptions) (watch.Interface, error) {
		log.V(2).Info("watching endpoints")
		return lbc.client.Endpoints(lbc.namespace).Watch(lo)
	}

	lw := &cache.ListWatch{
		ListFunc:  listFunc,
		WatchFunc: watchFunc,
	}

	lbc.endpLister.Store, lbc.endpController = framework.NewInformer(lw, &api.Endpoints{}, lbc.resyncPeriod, endpointHandler)
}

// setSecretInformer creates a secret Informer (store and controller)
func (lbc *LoadBalancerController) setSecretInformer() {
	log.Info("setting up secret informer")

	secretHandler := framework.ResourceEventHandlerFuncs{
		AddFunc: func(o interface{}) {
			secr := o.(*api.Secret)
			lbc.recorder.Eventf(secr, api.EventTypeNormal, "create", fmt.Sprintf("%s/%s", secr.Namespace, secr.Name))
			lbc.syncQueue.enqueue(o)
		},
		DeleteFunc: func(o interface{}) {
			secr := o.(*api.Secret)
			lbc.recorder.Eventf(secr, api.EventTypeNormal, "delete", fmt.Sprintf("%s/%s", secr.Namespace, secr.Name))
			lbc.syncQueue.enqueue(o)
		},
		UpdateFunc: func(old, cur interface{}) {
			if !reflect.DeepEqual(old, cur) {
				secr := cur.(*api.Secret)
				lbc.recorder.Eventf(secr, api.EventTypeNormal, "update", fmt.Sprintf("%s/%s", secr.Namespace, secr.Name))
				lbc.syncQueue.enqueue(cur)
			}
		},
	}

	listFunc := func(lo api.ListOptions) (runtime.Object, error) {
		log.V(2).Info("listing secrets")
		return lbc.client.Secrets(lbc.namespace).List(lo)
	}

	watchFunc := func(lo api.ListOptions) (watch.Interface, error) {
		log.V(2).Info("watching secrets")
		return lbc.client.Secrets(lbc.namespace).Watch(lo)
	}

	lw := &cache.ListWatch{
		ListFunc:  listFunc,
		WatchFunc: watchFunc,
	}

	lbc.secretStore, lbc.secretController = framework.NewInformer(lw, &api.Secret{}, lbc.resyncPeriod, secretHandler)
}

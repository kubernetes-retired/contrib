package cache

import "k8s.io/kubernetes/pkg/client/cache"

// StoreToIngressLister makes a Store that lists Ingress.
type StoreToIngressLister struct {
	cache.Store
}

// StoreToSecretsLister makes a Store that lists Secrets.
type StoreToSecretsLister struct {
	cache.Store
}

// StoreToConfigmapLister makes a Store that lists Configmap.
type StoreToConfigmapLister struct {
	cache.Store
}

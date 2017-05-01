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

	"k8s.io/kubernetes/pkg/api"
	apierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/cache"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	unversionedcore "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/unversioned"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/serviceaccount"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/golang/glog"
)

// ControllerClientBuilder allow syou to get clients and configs for controllers
type ControllerClientBuilder interface {
	Config(name string) (*restclient.Config, error)
	Client(name string) (clientset.Interface, error)
	ClientOrDie(name string) clientset.Interface
}

// SimpleControllerClientBuilder returns a fixed client with different user agents
type SimpleControllerClientBuilder struct {
	// ClientConfig is a skeleton config to clone and use as the basis for each controller client
	ClientConfig *restclient.Config
}

func (b SimpleControllerClientBuilder) Config(name string) (*restclient.Config, error) {
	clientConfig := *b.ClientConfig
	return &clientConfig, nil
}

func (b SimpleControllerClientBuilder) Client(name string) (clientset.Interface, error) {
	clientConfig, err := b.Config(name)
	if err != nil {
		return nil, err
	}
	return clientset.NewForConfig(restclient.AddUserAgent(clientConfig, name))
}

func (b SimpleControllerClientBuilder) ClientOrDie(name string) clientset.Interface {
	client, err := b.Client(name)
	if err != nil {
		glog.Fatal(err)
	}
	return client
}

// SAControllerClientBuilder is a ControllerClientBuilder that returns clients identifying as
// service accounts
type SAControllerClientBuilder struct {
	// ClientConfig is a skeleton config to clone and use as the basis for each controller client
	ClientConfig *restclient.Config

	// CoreClient is used to provision service accounts if needed and watch for their associated tokens
	// to construct a controller client
	CoreClient unversionedcore.CoreInterface

	// Namespace is the namespace used to host the service accounts that will back the
	// controllers.  It must be highly privileged namespace which normal users cannot inspect.
	Namespace string
}

// config returns a complete clientConfig for constructing clients.  This is separate in anticipation of composition
// which means that not all clientsets are known here
func (b SAControllerClientBuilder) Config(name string) (*restclient.Config, error) {
	clientConfig := restclient.AnonymousClientConfig(b.ClientConfig)

	// we need the SA UID to find a matching SA token
	sa, err := b.CoreClient.ServiceAccounts(b.Namespace).Get(name)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	} else if apierrors.IsNotFound(err) {
		// check to see if the namespace exists.  If it isn't a NotFound, just try to create the SA.
		// It'll probably fail, but perhaps that will have a better message.
		if _, err := b.CoreClient.Namespaces().Get(b.Namespace); apierrors.IsNotFound(err) {
			_, err = b.CoreClient.Namespaces().Create(&api.Namespace{ObjectMeta: api.ObjectMeta{Name: b.Namespace}})
			if err != nil && !apierrors.IsAlreadyExists(err) {
				return nil, err
			}
		}

		sa, err = b.CoreClient.ServiceAccounts(b.Namespace).Create(
			&api.ServiceAccount{ObjectMeta: api.ObjectMeta{Namespace: b.Namespace, Name: name}})
		if err != nil {
			return nil, err
		}
	}

	lw := &cache.ListWatch{
		ListFunc: func(options api.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fields.SelectorFromSet(map[string]string{api.SecretTypeField: string(api.SecretTypeServiceAccountToken)})
			return b.CoreClient.Secrets(b.Namespace).List(options)
		},
		WatchFunc: func(options api.ListOptions) (watch.Interface, error) {
			options.FieldSelector = fields.SelectorFromSet(map[string]string{api.SecretTypeField: string(api.SecretTypeServiceAccountToken)})
			return b.CoreClient.Secrets(b.Namespace).Watch(options)
		},
	}
	_, err = watch.ListWatchUntil(30*time.Second, lw,
		func(event watch.Event) (bool, error) {
			switch event.Type {
			case watch.Deleted:
				return false, nil
			case watch.Error:
				return false, fmt.Errorf("error watching")

			case watch.Added, watch.Modified:
				secret := event.Object.(*api.Secret)
				if !serviceaccount.IsServiceAccountToken(secret, sa) ||
					len(secret.Data[api.ServiceAccountTokenKey]) == 0 {
					return false, nil
				}
				// TODO maybe verify the token is valid
				clientConfig.BearerToken = string(secret.Data[api.ServiceAccountTokenKey])
				restclient.AddUserAgent(clientConfig, serviceaccount.MakeUsername(b.Namespace, name))
				return true, nil

			default:
				return false, fmt.Errorf("unexpected event type: %v", event.Type)
			}
		})
	if err != nil {
		return nil, fmt.Errorf("unable to get token for service account: %v", err)
	}

	return clientConfig, nil
}

func (b SAControllerClientBuilder) Client(name string) (clientset.Interface, error) {
	clientConfig, err := b.Config(name)
	if err != nil {
		return nil, err
	}
	return clientset.NewForConfig(clientConfig)
}

func (b SAControllerClientBuilder) ClientOrDie(name string) clientset.Interface {
	client, err := b.Client(name)
	if err != nil {
		glog.Fatal(err)
	}
	return client
}

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

// Package webhook implements the authorizer.Authorizer interface using HTTP webhooks.
package webhook

import (
	"encoding/json"
	"errors"
	"time"

	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/authorization/v1beta1"
	"k8s.io/kubernetes/pkg/auth/authorizer"
	"k8s.io/kubernetes/pkg/util/cache"
	"k8s.io/kubernetes/plugin/pkg/webhook"

	_ "k8s.io/kubernetes/pkg/apis/authorization/install"
)

var (
	groupVersions = []unversioned.GroupVersion{v1beta1.SchemeGroupVersion}
)

// Ensure Webhook implements the authorizer.Authorizer interface.
var _ authorizer.Authorizer = (*WebhookAuthorizer)(nil)

type WebhookAuthorizer struct {
	*webhook.GenericWebhook
	responseCache   *cache.LRUExpireCache
	authorizedTTL   time.Duration
	unauthorizedTTL time.Duration
}

// New creates a new WebhookAuthorizer from the provided kubeconfig file.
//
// The config's cluster field is used to refer to the remote service, user refers to the returned authorizer.
//
//     # clusters refers to the remote service.
//     clusters:
//     - name: name-of-remote-authz-service
//       cluster:
//         certificate-authority: /path/to/ca.pem      # CA for verifying the remote service.
//         server: https://authz.example.com/authorize # URL of remote service to query. Must use 'https'.
//
//     # users refers to the API server's webhook configuration.
//     users:
//     - name: name-of-api-server
//       user:
//         client-certificate: /path/to/cert.pem # cert for the webhook plugin to use
//         client-key: /path/to/key.pem          # key matching the cert
//
// For additional HTTP configuration, refer to the kubeconfig documentation
// http://kubernetes.io/v1.1/docs/user-guide/kubeconfig-file.html.
func New(kubeConfigFile string, authorizedTTL, unauthorizedTTL time.Duration) (*WebhookAuthorizer, error) {
	gw, err := webhook.NewGenericWebhook(kubeConfigFile, groupVersions)
	if err != nil {
		return nil, err
	}
	return &WebhookAuthorizer{gw, cache.NewLRUExpireCache(1024), authorizedTTL, unauthorizedTTL}, nil
}

// Authorize makes a REST request to the remote service describing the attempted action as a JSON
// serialized api.authorization.v1beta1.SubjectAccessReview object. An example request body is
// provided bellow.
//
//     {
//       "apiVersion": "authorization.k8s.io/v1beta1",
//       "kind": "SubjectAccessReview",
//       "spec": {
//         "resourceAttributes": {
//           "namespace": "kittensandponies",
//           "verb": "GET",
//           "group": "group3",
//           "resource": "pods"
//         },
//         "user": "jane",
//         "group": [
//           "group1",
//           "group2"
//         ]
//       }
//     }
//
// The remote service is expected to fill the SubjectAccessReviewStatus field to either allow or
// disallow access. A permissive response would return:
//
//     {
//       "apiVersion": "authorization.k8s.io/v1beta1",
//       "kind": "SubjectAccessReview",
//       "status": {
//         "allowed": true
//       }
//     }
//
// To disallow access, the remote service would return:
//
//     {
//       "apiVersion": "authorization.k8s.io/v1beta1",
//       "kind": "SubjectAccessReview",
//       "status": {
//         "allowed": false,
//         "reason": "user does not have read access to the namespace"
//       }
//     }
//
func (w *WebhookAuthorizer) Authorize(attr authorizer.Attributes) (err error) {
	r := &v1beta1.SubjectAccessReview{
		Spec: v1beta1.SubjectAccessReviewSpec{
			User:   attr.GetUserName(),
			Groups: attr.GetGroups(),
		},
	}
	if attr.IsResourceRequest() {
		r.Spec.ResourceAttributes = &v1beta1.ResourceAttributes{
			Namespace:   attr.GetNamespace(),
			Verb:        attr.GetVerb(),
			Group:       attr.GetAPIGroup(),
			Version:     attr.GetAPIVersion(),
			Resource:    attr.GetResource(),
			Subresource: attr.GetSubresource(),
			Name:        attr.GetName(),
		}
	} else {
		r.Spec.NonResourceAttributes = &v1beta1.NonResourceAttributes{
			Path: attr.GetPath(),
			Verb: attr.GetVerb(),
		}
	}
	key, err := json.Marshal(r.Spec)
	if err != nil {
		return err
	}
	if entry, ok := w.responseCache.Get(string(key)); ok {
		r.Status = entry.(v1beta1.SubjectAccessReviewStatus)
	} else {
		result := w.RestClient.Post().Body(r).Do()
		if err := result.Error(); err != nil {
			return err
		}
		if err := result.Into(r); err != nil {
			return err
		}
		go func() {
			if r.Status.Allowed {
				w.responseCache.Add(string(key), r.Status, w.authorizedTTL)
			} else {
				w.responseCache.Add(string(key), r.Status, w.unauthorizedTTL)
			}
		}()
	}
	if r.Status.Allowed {
		return nil
	}
	if r.Status.Reason != "" {
		return errors.New(r.Status.Reason)
	}
	return errors.New("unauthorized")
}

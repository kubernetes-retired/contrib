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

// Package rbac implements the authorizer.Authorizer interface using roles base access control.
package rbac

import (
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/rbac"
	"k8s.io/kubernetes/pkg/apis/rbac/validation"
	"k8s.io/kubernetes/pkg/auth/authorizer"
	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/registry/clusterrole"
	"k8s.io/kubernetes/pkg/registry/clusterrolebinding"
	"k8s.io/kubernetes/pkg/registry/role"
	"k8s.io/kubernetes/pkg/registry/rolebinding"
)

type RBACAuthorizer struct {
	superUser string

	authorizationRuleResolver validation.AuthorizationRuleResolver
}

func (r *RBACAuthorizer) Authorize(attr authorizer.Attributes) error {
	if r.superUser != "" && attr.GetUserName() == r.superUser {
		return nil
	}

	userInfo := &user.DefaultInfo{
		Name:   attr.GetUserName(),
		Groups: attr.GetGroups(),
	}

	ctx := api.WithNamespace(api.WithUser(api.NewContext(), userInfo), attr.GetNamespace())

	// Frame the authorization request as a privilege escalation check.
	var requestedRule rbac.PolicyRule
	if attr.IsResourceRequest() {
		requestedRule = rbac.PolicyRule{
			Verbs:         []string{attr.GetVerb()},
			APIGroups:     []string{attr.GetAPIGroup()}, // TODO(ericchiang): add api version here too?
			Resources:     []string{attr.GetResource()},
			ResourceNames: []string{attr.GetName()},
		}
	} else {
		requestedRule = rbac.PolicyRule{
			NonResourceURLs: []string{attr.GetPath()},
		}
	}

	return validation.ConfirmNoEscalation(ctx, r.authorizationRuleResolver, []rbac.PolicyRule{requestedRule})
}

func New(roleRegistry role.Registry, roleBindingRegistry rolebinding.Registry, clusterRoleRegistry clusterrole.Registry, clusterRoleBindingRegistry clusterrolebinding.Registry, superUser string) (*RBACAuthorizer, error) {
	authorizer := &RBACAuthorizer{
		superUser: superUser,
		authorizationRuleResolver: validation.NewDefaultRuleResolver(
			roleRegistry,
			roleBindingRegistry,
			clusterRoleRegistry,
			clusterRoleBindingRegistry,
		),
	}
	return authorizer, nil
}

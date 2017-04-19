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

package validation

import (
	"errors"
	"hash/fnv"
	"io"
	"reflect"
	"sort"
	"testing"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/rbac"
	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/util/diff"
)

func newMockRuleResolver(r *staticRoles) AuthorizationRuleResolver {
	return NewDefaultRuleResolver(r, r, r, r)
}

type staticRoles struct {
	roles               []rbac.Role
	roleBindings        []rbac.RoleBinding
	clusterRoles        []rbac.ClusterRole
	clusterRoleBindings []rbac.ClusterRoleBinding
}

func (r *staticRoles) GetRole(ctx api.Context, id string) (*rbac.Role, error) {
	namespace, ok := api.NamespaceFrom(ctx)
	if !ok || namespace == "" {
		return nil, errors.New("must provide namespace when getting role")
	}
	for _, role := range r.roles {
		if role.Namespace == namespace && role.Name == id {
			return &role, nil
		}
	}
	return nil, errors.New("role not found")
}

func (r *staticRoles) GetClusterRole(ctx api.Context, id string) (*rbac.ClusterRole, error) {
	namespace, ok := api.NamespaceFrom(ctx)
	if ok && namespace != "" {
		return nil, errors.New("cannot provide namespace when getting cluster role")
	}
	for _, clusterRole := range r.clusterRoles {
		if clusterRole.Namespace == namespace && clusterRole.Name == id {
			return &clusterRole, nil
		}
	}
	return nil, errors.New("role not found")
}

func (r *staticRoles) ListRoleBindings(ctx api.Context, options *api.ListOptions) (*rbac.RoleBindingList, error) {
	namespace, ok := api.NamespaceFrom(ctx)
	if !ok || namespace == "" {
		return nil, errors.New("must provide namespace when listing role bindings")
	}

	roleBindingList := new(rbac.RoleBindingList)
	for _, roleBinding := range r.roleBindings {
		if roleBinding.Namespace != namespace {
			continue
		}
		// TODO(ericchiang): need to implement label selectors?
		roleBindingList.Items = append(roleBindingList.Items, roleBinding)
	}
	return roleBindingList, nil
}

func (r *staticRoles) ListClusterRoleBindings(ctx api.Context, options *api.ListOptions) (*rbac.ClusterRoleBindingList, error) {
	namespace, ok := api.NamespaceFrom(ctx)
	if ok && namespace != "" {
		return nil, errors.New("cannot list cluster role bindings from within a namespace")
	}
	clusterRoleBindings := new(rbac.ClusterRoleBindingList)
	clusterRoleBindings.Items = make([]rbac.ClusterRoleBinding, len(r.clusterRoleBindings))
	copy(clusterRoleBindings.Items, r.clusterRoleBindings)
	return clusterRoleBindings, nil
}

// compute a hash of a policy rule so we can sort in a deterministic order
func hashOf(p rbac.PolicyRule) string {
	hash := fnv.New32()
	writeStrings := func(slis ...[]string) {
		for _, sli := range slis {
			for _, s := range sli {
				io.WriteString(hash, s)
			}
		}
	}
	writeStrings(p.Verbs, p.APIGroups, p.Resources, p.ResourceNames, p.NonResourceURLs)
	return string(hash.Sum(nil))
}

// byHash sorts a set of policy rules by a hash of its fields
type byHash []rbac.PolicyRule

func (b byHash) Len() int           { return len(b) }
func (b byHash) Less(i, j int) bool { return hashOf(b[i]) < hashOf(b[j]) }
func (b byHash) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }

func TestDefaultRuleResolver(t *testing.T) {
	ruleReadPods := rbac.PolicyRule{
		Verbs:     []string{"GET", "WATCH"},
		APIGroups: []string{"v1"},
		Resources: []string{"pods"},
	}
	ruleReadServices := rbac.PolicyRule{
		Verbs:     []string{"GET", "WATCH"},
		APIGroups: []string{"v1"},
		Resources: []string{"services"},
	}
	ruleWriteNodes := rbac.PolicyRule{
		Verbs:     []string{"PUT", "CREATE", "UPDATE"},
		APIGroups: []string{"v1"},
		Resources: []string{"nodes"},
	}
	ruleAdmin := rbac.PolicyRule{
		Verbs:     []string{"*"},
		APIGroups: []string{"*"},
		Resources: []string{"*"},
	}

	staticRoles1 := staticRoles{
		roles: []rbac.Role{
			{
				ObjectMeta: api.ObjectMeta{Namespace: "namespace1", Name: "readthings"},
				Rules:      []rbac.PolicyRule{ruleReadPods, ruleReadServices},
			},
		},
		clusterRoles: []rbac.ClusterRole{
			{
				ObjectMeta: api.ObjectMeta{Name: "cluster-admin"},
				Rules:      []rbac.PolicyRule{ruleAdmin},
			},
			{
				ObjectMeta: api.ObjectMeta{Name: "write-nodes"},
				Rules:      []rbac.PolicyRule{ruleWriteNodes},
			},
		},
		roleBindings: []rbac.RoleBinding{
			{
				ObjectMeta: api.ObjectMeta{Namespace: "namespace1"},
				Subjects: []rbac.Subject{
					{Kind: rbac.UserKind, Name: "foobar"},
					{Kind: rbac.GroupKind, Name: "group1"},
				},
				RoleRef: api.ObjectReference{Kind: "Role", Namespace: "namespace1", Name: "readthings"},
			},
		},
		clusterRoleBindings: []rbac.ClusterRoleBinding{
			{
				Subjects: []rbac.Subject{
					{Kind: rbac.UserKind, Name: "admin"},
					{Kind: rbac.GroupKind, Name: "admin"},
				},
				RoleRef: api.ObjectReference{Kind: "ClusterRole", Name: "cluster-admin"},
			},
		},
	}

	tests := []struct {
		staticRoles

		// For a given context, what are the rules that apply?
		ctx            api.Context
		effectiveRules []rbac.PolicyRule
	}{
		{
			staticRoles: staticRoles1,
			ctx: api.WithNamespace(
				api.WithUser(api.NewContext(), &user.DefaultInfo{Name: "foobar"}), "namespace1",
			),
			effectiveRules: []rbac.PolicyRule{ruleReadPods, ruleReadServices},
		},
		{
			staticRoles: staticRoles1,
			ctx: api.WithNamespace(
				// Same as above but diffrerent namespace. Should return no rules.
				api.WithUser(api.NewContext(), &user.DefaultInfo{Name: "foobar"}), "namespace2",
			),
			effectiveRules: []rbac.PolicyRule{},
		},
		{
			staticRoles: staticRoles1,
			// GetEffectivePolicyRules only returns the policies for the namespace, not the master namespace.
			ctx: api.WithNamespace(
				api.WithUser(api.NewContext(), &user.DefaultInfo{
					Name: "foobar", Groups: []string{"admin"},
				}), "namespace1",
			),
			effectiveRules: []rbac.PolicyRule{ruleReadPods, ruleReadServices},
		},
		{
			staticRoles: staticRoles1,
			// Same as above but without a namespace. Only cluster rules should apply.
			ctx: api.WithUser(api.NewContext(), &user.DefaultInfo{
				Name: "foobar", Groups: []string{"admin"},
			}),
			effectiveRules: []rbac.PolicyRule{ruleAdmin},
		},
		{
			staticRoles:    staticRoles1,
			ctx:            api.WithUser(api.NewContext(), &user.DefaultInfo{}),
			effectiveRules: []rbac.PolicyRule{},
		},
	}

	for i, tc := range tests {
		ruleResolver := newMockRuleResolver(&tc.staticRoles)
		rules, err := ruleResolver.GetEffectivePolicyRules(tc.ctx)
		if err != nil {
			t.Errorf("case %d: GetEffectivePolicyRules(context)=%v", i, err)
			continue
		}

		// Sort for deep equals
		sort.Sort(byHash(rules))
		sort.Sort(byHash(tc.effectiveRules))

		if !reflect.DeepEqual(rules, tc.effectiveRules) {
			ruleDiff := diff.ObjectDiff(rules, tc.effectiveRules)
			t.Errorf("case %d: %s", i, ruleDiff)
		}
	}
}

func TestAppliesTo(t *testing.T) {
	tests := []struct {
		subjects  []rbac.Subject
		ctx       api.Context
		appliesTo bool
		testCase  string
	}{
		{
			subjects: []rbac.Subject{
				{Kind: rbac.UserKind, Name: "foobar"},
			},
			ctx:       api.WithUser(api.NewContext(), &user.DefaultInfo{Name: "foobar"}),
			appliesTo: true,
			testCase:  "single subject that matches username",
		},
		{
			subjects: []rbac.Subject{
				{Kind: rbac.UserKind, Name: "barfoo"},
				{Kind: rbac.UserKind, Name: "foobar"},
			},
			ctx:       api.WithUser(api.NewContext(), &user.DefaultInfo{Name: "foobar"}),
			appliesTo: true,
			testCase:  "multiple subjects, one that matches username",
		},
		{
			subjects: []rbac.Subject{
				{Kind: rbac.UserKind, Name: "barfoo"},
				{Kind: rbac.UserKind, Name: "foobar"},
			},
			ctx:       api.WithUser(api.NewContext(), &user.DefaultInfo{Name: "zimzam"}),
			appliesTo: false,
			testCase:  "multiple subjects, none that match username",
		},
		{
			subjects: []rbac.Subject{
				{Kind: rbac.UserKind, Name: "barfoo"},
				{Kind: rbac.GroupKind, Name: "foobar"},
			},
			ctx:       api.WithUser(api.NewContext(), &user.DefaultInfo{Name: "zimzam", Groups: []string{"foobar"}}),
			appliesTo: true,
			testCase:  "multiple subjects, one that match group",
		},
		{
			subjects: []rbac.Subject{
				{Kind: rbac.UserKind, Name: "barfoo"},
				{Kind: rbac.GroupKind, Name: "foobar"},
			},
			ctx: api.WithNamespace(
				api.WithUser(api.NewContext(), &user.DefaultInfo{Name: "zimzam", Groups: []string{"foobar"}}),
				"namespace1",
			),
			appliesTo: true,
			testCase:  "multiple subjects, one that match group, should ignore namespace",
		},
		{
			subjects: []rbac.Subject{
				{Kind: rbac.UserKind, Name: "barfoo"},
				{Kind: rbac.GroupKind, Name: "foobar"},
				{Kind: rbac.ServiceAccountKind, Name: "kube-system", Namespace: "default"},
			},
			ctx: api.WithNamespace(
				api.WithUser(api.NewContext(), &user.DefaultInfo{Name: "system:serviceaccount:kube-system:default"}),
				"default",
			),
			appliesTo: true,
			testCase:  "multiple subjects with a service account that matches",
		},
		{
			subjects: []rbac.Subject{
				{Kind: rbac.UserKind, Name: "*"},
			},
			ctx: api.WithNamespace(
				api.WithUser(api.NewContext(), &user.DefaultInfo{Name: "foobar"}),
				"default",
			),
			appliesTo: true,
			testCase:  "multiple subjects with a service account that matches",
		},
	}

	for _, tc := range tests {
		got, err := appliesTo(tc.ctx, tc.subjects)
		if err != nil {
			t.Errorf("case %q %v", tc.testCase, err)
			continue
		}
		if got != tc.appliesTo {
			t.Errorf("case %q want appliesTo=%t, got appliesTo=%t", tc.testCase, tc.appliesTo, got)
		}
	}
}

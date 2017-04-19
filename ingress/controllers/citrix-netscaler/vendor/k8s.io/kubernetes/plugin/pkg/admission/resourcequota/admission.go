/*
Copyright 2014 The Kubernetes Authors All rights reserved.

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

package resourcequota

import (
	"io"
	"sort"
	"strings"
	"time"

	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	"k8s.io/kubernetes/pkg/admission"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/quota"
	"k8s.io/kubernetes/pkg/quota/install"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
)

func init() {
	admission.RegisterPlugin("ResourceQuota",
		func(client clientset.Interface, config io.Reader) (admission.Interface, error) {
			registry := install.NewRegistry(client)
			// TODO: expose a stop channel in admission factory
			return NewResourceQuota(client, registry, 5, make(chan struct{}))
		})
}

// quotaAdmission implements an admission controller that can enforce quota constraints
type quotaAdmission struct {
	*admission.Handler

	evaluator *quotaEvaluator
}

type liveLookupEntry struct {
	expiry time.Time
	items  []*api.ResourceQuota
}

// NewResourceQuota configures an admission controller that can enforce quota constraints
// using the provided registry.  The registry must have the capability to handle group/kinds that
// are persisted by the server this admission controller is intercepting
func NewResourceQuota(client clientset.Interface, registry quota.Registry, numEvaluators int, stopCh <-chan struct{}) (admission.Interface, error) {
	evaluator, err := newQuotaEvaluator(client, registry)
	if err != nil {
		return nil, err
	}

	defer utilruntime.HandleCrash()
	go evaluator.Run(numEvaluators, stopCh)

	return &quotaAdmission{
		Handler:   admission.NewHandler(admission.Create, admission.Update),
		evaluator: evaluator,
	}, nil
}

// Admit makes admission decisions while enforcing quota
func (q *quotaAdmission) Admit(a admission.Attributes) (err error) {
	// ignore all operations that correspond to sub-resource actions
	if a.GetSubresource() != "" {
		return nil
	}

	// if we do not know how to evaluate use for this kind, just ignore
	evaluators := q.evaluator.registry.Evaluators()
	evaluator, found := evaluators[a.GetKind().GroupKind()]
	if !found {
		return nil
	}

	// for this kind, check if the operation could mutate any quota resources
	// if no resources tracked by quota are impacted, then just return
	op := a.GetOperation()
	operationResources := evaluator.OperationResources(op)
	if len(operationResources) == 0 {
		return nil
	}

	return q.evaluator.evaluate(a)
}

// prettyPrint formats a resource list for usage in errors
// it outputs resources sorted in increasing order
func prettyPrint(item api.ResourceList) string {
	parts := []string{}
	keys := []string{}
	for key := range item {
		keys = append(keys, string(key))
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := item[api.ResourceName(key)]
		constraint := key + "=" + value.String()
		parts = append(parts, constraint)
	}
	return strings.Join(parts, ",")
}

// hasUsageStats returns true if for each hard constraint there is a value for its current usage
func hasUsageStats(resourceQuota *api.ResourceQuota) bool {
	for resourceName := range resourceQuota.Status.Hard {
		if _, found := resourceQuota.Status.Used[resourceName]; !found {
			return false
		}
	}
	return true
}

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

package admission

import (
	"fmt"
	"io"
	"strings"

	"github.com/golang/glog"

	admission "k8s.io/kubernetes/pkg/admission"
	api "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/client/cache"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/runtime"
	psp "k8s.io/kubernetes/pkg/security/podsecuritypolicy"
	psputil "k8s.io/kubernetes/pkg/security/podsecuritypolicy/util"
	sc "k8s.io/kubernetes/pkg/securitycontext"
	"k8s.io/kubernetes/pkg/serviceaccount"
	"k8s.io/kubernetes/pkg/util/validation/field"
	"k8s.io/kubernetes/pkg/watch"
)

const (
	PluginName = "PodSecurityPolicy"
)

func init() {
	admission.RegisterPlugin(PluginName, func(client clientset.Interface, config io.Reader) (admission.Interface, error) {
		plugin := NewPlugin(client, psp.NewSimpleStrategyFactory(), getMatchingPolicies, false)
		plugin.Run()
		return plugin, nil
	})
}

// PSPMatchFn allows plugging in how PSPs are matched against user information.
type PSPMatchFn func(store cache.Store, user user.Info, sa user.Info) ([]*extensions.PodSecurityPolicy, error)

// podSecurityPolicyPlugin holds state for and implements the admission plugin.
type podSecurityPolicyPlugin struct {
	*admission.Handler
	client           clientset.Interface
	strategyFactory  psp.StrategyFactory
	pspMatcher       PSPMatchFn
	failOnNoPolicies bool

	reflector *cache.Reflector
	stopChan  chan struct{}
	store     cache.Store
}

var _ admission.Interface = &podSecurityPolicyPlugin{}

// NewPlugin creates a new PSP admission plugin.
func NewPlugin(kclient clientset.Interface, strategyFactory psp.StrategyFactory, pspMatcher PSPMatchFn, failOnNoPolicies bool) *podSecurityPolicyPlugin {
	store := cache.NewStore(cache.MetaNamespaceKeyFunc)
	reflector := cache.NewReflector(
		&cache.ListWatch{
			ListFunc: func(options api.ListOptions) (runtime.Object, error) {
				return kclient.Extensions().PodSecurityPolicies().List(options)
			},
			WatchFunc: func(options api.ListOptions) (watch.Interface, error) {
				return kclient.Extensions().PodSecurityPolicies().Watch(options)
			},
		},
		&extensions.PodSecurityPolicy{},
		store,
		0,
	)

	return &podSecurityPolicyPlugin{
		Handler:          admission.NewHandler(admission.Create, admission.Update),
		client:           kclient,
		strategyFactory:  strategyFactory,
		pspMatcher:       pspMatcher,
		failOnNoPolicies: failOnNoPolicies,

		store:     store,
		reflector: reflector,
	}
}

func (a *podSecurityPolicyPlugin) Run() {
	if a.stopChan == nil {
		a.stopChan = make(chan struct{})
	}
	a.reflector.RunUntil(a.stopChan)
}
func (a *podSecurityPolicyPlugin) Stop() {
	if a.stopChan != nil {
		close(a.stopChan)
		a.stopChan = nil
	}
}

// Admit determines if the pod should be admitted based on the requested security context
// and the available PSPs.
//
// 1.  Find available PSPs.
// 2.  Create the providers, includes setting pre-allocated values if necessary.
// 3.  Try to generate and validate a PSP with providers.  If we find one then admit the pod
//     with the validated PSP.  If we don't find any reject the pod and give all errors from the
//     failed attempts.
func (c *podSecurityPolicyPlugin) Admit(a admission.Attributes) error {
	if a.GetResource().GroupResource() != api.Resource("pods") {
		return nil
	}

	if len(a.GetSubresource()) != 0 {
		return nil
	}

	pod, ok := a.GetObject().(*api.Pod)
	// if we can't convert then we don't handle this object so just return
	if !ok {
		return nil
	}

	// get all constraints that are usable by the user
	glog.V(4).Infof("getting pod security policies for pod %s (generate: %s)", pod.Name, pod.GenerateName)
	var saInfo user.Info
	if len(pod.Spec.ServiceAccountName) > 0 {
		saInfo = serviceaccount.UserInfo(a.GetNamespace(), pod.Spec.ServiceAccountName, "")
	}

	matchedPolicies, err := c.pspMatcher(c.store, a.GetUserInfo(), saInfo)
	if err != nil {
		return admission.NewForbidden(a, err)
	}

	// if we have no policies and want to succeed then return.  Otherwise we'll end up with no
	// providers and fail with "unable to validate against any pod security policy" below.
	if len(matchedPolicies) == 0 && !c.failOnNoPolicies {
		return nil
	}

	providers, errs := c.createProvidersFromPolicies(matchedPolicies, pod.Namespace)
	logProviders(pod, providers, errs)

	if len(providers) == 0 {
		return admission.NewForbidden(a, fmt.Errorf("no providers available to validate pod request"))
	}

	// all containers in a single pod must validate under a single provider or we will reject the request
	validationErrs := field.ErrorList{}
	for _, provider := range providers {
		if errs := assignSecurityContext(provider, pod, field.NewPath(fmt.Sprintf("provider %s: ", provider.GetPSPName()))); len(errs) > 0 {
			validationErrs = append(validationErrs, errs...)
			continue
		}

		// the entire pod validated, annotate and accept the pod
		glog.V(4).Infof("pod %s (generate: %s) validated against provider %s", pod.Name, pod.GenerateName, provider.GetPSPName())
		if pod.ObjectMeta.Annotations == nil {
			pod.ObjectMeta.Annotations = map[string]string{}
		}
		pod.ObjectMeta.Annotations[psputil.ValidatedPSPAnnotation] = provider.GetPSPName()
		return nil
	}

	// we didn't validate against any provider, reject the pod and give the errors for each attempt
	glog.V(4).Infof("unable to validate pod %s (generate: %s) against any pod security policy: %v", pod.Name, pod.GenerateName, validationErrs)
	return admission.NewForbidden(a, fmt.Errorf("unable to validate against any pod security policy: %v", validationErrs))
}

// assignSecurityContext creates a security context for each container in the pod
// and validates that the sc falls within the psp constraints.  All containers must validate against
// the same psp or is not considered valid.
func assignSecurityContext(provider psp.Provider, pod *api.Pod, fldPath *field.Path) field.ErrorList {
	generatedSCs := make([]*api.SecurityContext, len(pod.Spec.Containers))
	var generatedInitSCs []*api.SecurityContext

	errs := field.ErrorList{}

	psc, err := provider.CreatePodSecurityContext(pod)
	if err != nil {
		errs = append(errs, field.Invalid(field.NewPath("spec", "securityContext"), pod.Spec.SecurityContext, err.Error()))
	}

	// save the original PSC and validate the generated PSC.  Leave the generated PSC
	// set for container generation/validation.  We will reset to original post container
	// validation.
	originalPSC := pod.Spec.SecurityContext
	pod.Spec.SecurityContext = psc
	errs = append(errs, provider.ValidatePodSecurityContext(pod, field.NewPath("spec", "securityContext"))...)

	// Note: this is not changing the original container, we will set container SCs later so long
	// as all containers validated under the same PSP.
	for i, containerCopy := range pod.Spec.InitContainers {
		// We will determine the effective security context for the container and validate against that
		// since that is how the sc provider will eventually apply settings in the runtime.
		// This results in an SC that is based on the Pod's PSC with the set fields from the container
		// overriding pod level settings.
		containerCopy.SecurityContext = sc.DetermineEffectiveSecurityContext(pod, &containerCopy)

		sc, err := provider.CreateContainerSecurityContext(pod, &containerCopy)
		if err != nil {
			errs = append(errs, field.Invalid(field.NewPath("spec", "initContainers").Index(i).Child("securityContext"), "", err.Error()))
			continue
		}
		generatedInitSCs = append(generatedInitSCs, sc)

		containerCopy.SecurityContext = sc
		errs = append(errs, provider.ValidateContainerSecurityContext(pod, &containerCopy, field.NewPath("spec", "initContainers").Index(i).Child("securityContext"))...)
	}

	// Note: this is not changing the original container, we will set container SCs later so long
	// as all containers validated under the same PSP.
	for i, containerCopy := range pod.Spec.Containers {
		// We will determine the effective security context for the container and validate against that
		// since that is how the sc provider will eventually apply settings in the runtime.
		// This results in an SC that is based on the Pod's PSC with the set fields from the container
		// overriding pod level settings.
		containerCopy.SecurityContext = sc.DetermineEffectiveSecurityContext(pod, &containerCopy)

		sc, err := provider.CreateContainerSecurityContext(pod, &containerCopy)
		if err != nil {
			errs = append(errs, field.Invalid(field.NewPath("spec", "containers").Index(i).Child("securityContext"), "", err.Error()))
			continue
		}
		generatedSCs[i] = sc

		containerCopy.SecurityContext = sc
		errs = append(errs, provider.ValidateContainerSecurityContext(pod, &containerCopy, field.NewPath("spec", "containers").Index(i).Child("securityContext"))...)
	}

	if len(errs) > 0 {
		// ensure psc is not mutated if there are errors
		pod.Spec.SecurityContext = originalPSC
		return errs
	}

	// if we've reached this code then we've generated and validated an SC for every container in the
	// pod so let's apply what we generated.  Note: the psc is already applied.
	for i, sc := range generatedInitSCs {
		pod.Spec.InitContainers[i].SecurityContext = sc
	}
	for i, sc := range generatedSCs {
		pod.Spec.Containers[i].SecurityContext = sc
	}
	return nil
}

// createProvidersFromPolicies creates providers from the constraints supplied.
func (c *podSecurityPolicyPlugin) createProvidersFromPolicies(psps []*extensions.PodSecurityPolicy, namespace string) ([]psp.Provider, []error) {
	var (
		// collected providers
		providers []psp.Provider
		// collected errors to return
		errs []error
	)

	for _, constraint := range psps {
		provider, err := psp.NewSimpleProvider(constraint, namespace, c.strategyFactory)
		if err != nil {
			errs = append(errs, fmt.Errorf("error creating provider for PSP %s: %v", constraint.Name, err))
			continue
		}
		providers = append(providers, provider)
	}
	return providers, errs
}

// getMatchingPolicies returns policies from the store.  For now this returns everything
// in the future it can filter based on UserInfo and permissions.
func getMatchingPolicies(store cache.Store, user user.Info, sa user.Info) ([]*extensions.PodSecurityPolicy, error) {
	matchedPolicies := make([]*extensions.PodSecurityPolicy, 0)

	for _, c := range store.List() {
		constraint, ok := c.(*extensions.PodSecurityPolicy)
		if !ok {
			return nil, errors.NewInternalError(fmt.Errorf("error converting object from store to a pod security policy: %v", c))
		}
		matchedPolicies = append(matchedPolicies, constraint)
	}

	return matchedPolicies, nil
}

// logProviders logs what providers were found for the pod as well as any errors that were encountered
// while creating providers.
func logProviders(pod *api.Pod, providers []psp.Provider, providerCreationErrs []error) {
	names := make([]string, len(providers))
	for i, p := range providers {
		names[i] = p.GetPSPName()
	}
	glog.V(4).Infof("validating pod %s (generate: %s) against providers %s", pod.Name, pod.GenerateName, strings.Join(names, ","))

	for _, err := range providerCreationErrs {
		glog.V(4).Infof("provider creation error: %v", err)
	}
}

/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	unversionedvalidation "k8s.io/kubernetes/pkg/api/unversioned/validation"
	apivalidation "k8s.io/kubernetes/pkg/api/validation"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/labels"
	psputil "k8s.io/kubernetes/pkg/security/podsecuritypolicy/util"
	"k8s.io/kubernetes/pkg/util/intstr"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/validation"
	"k8s.io/kubernetes/pkg/util/validation/field"
)

func ValidateThirdPartyResourceUpdate(update, old *extensions.ThirdPartyResource) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&update.ObjectMeta, &old.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateThirdPartyResource(update)...)
	return allErrs
}

func ValidateThirdPartyResourceName(name string, prefix bool) (bool, string) {
	return apivalidation.NameIsDNSSubdomain(name, prefix)
}

func ValidateThirdPartyResource(obj *extensions.ThirdPartyResource) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&obj.ObjectMeta, false, ValidateThirdPartyResourceName, field.NewPath("metadata"))...)

	versions := sets.String{}
	for ix := range obj.Versions {
		version := &obj.Versions[ix]
		if len(version.Name) == 0 {
			allErrs = append(allErrs, field.Invalid(field.NewPath("versions").Index(ix).Child("name"), version, "must not be empty"))
		}
		if versions.Has(version.Name) {
			allErrs = append(allErrs, field.Duplicate(field.NewPath("versions").Index(ix).Child("name"), version))
		}
		versions.Insert(version.Name)
	}
	return allErrs
}

// ValidateDaemonSet tests if required fields in the DaemonSet are set.
func ValidateDaemonSet(ds *extensions.DaemonSet) field.ErrorList {
	allErrs := apivalidation.ValidateObjectMeta(&ds.ObjectMeta, true, ValidateDaemonSetName, field.NewPath("metadata"))
	allErrs = append(allErrs, ValidateDaemonSetSpec(&ds.Spec, field.NewPath("spec"))...)
	return allErrs
}

// ValidateDaemonSetUpdate tests if required fields in the DaemonSet are set.
func ValidateDaemonSetUpdate(ds, oldDS *extensions.DaemonSet) field.ErrorList {
	allErrs := apivalidation.ValidateObjectMetaUpdate(&ds.ObjectMeta, &oldDS.ObjectMeta, field.NewPath("metadata"))
	allErrs = append(allErrs, ValidateDaemonSetSpec(&ds.Spec, field.NewPath("spec"))...)
	return allErrs
}

// validateDaemonSetStatus validates a DaemonSetStatus
func validateDaemonSetStatus(status *extensions.DaemonSetStatus, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(status.CurrentNumberScheduled), fldPath.Child("currentNumberScheduled"))...)
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(status.NumberMisscheduled), fldPath.Child("numberMisscheduled"))...)
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(status.DesiredNumberScheduled), fldPath.Child("desiredNumberScheduled"))...)
	return allErrs
}

// ValidateDaemonSetStatus validates tests if required fields in the DaemonSet Status section
func ValidateDaemonSetStatusUpdate(ds, oldDS *extensions.DaemonSet) field.ErrorList {
	allErrs := apivalidation.ValidateObjectMetaUpdate(&ds.ObjectMeta, &oldDS.ObjectMeta, field.NewPath("metadata"))
	allErrs = append(allErrs, validateDaemonSetStatus(&ds.Status, field.NewPath("status"))...)
	return allErrs
}

// ValidateDaemonSetSpec tests if required fields in the DaemonSetSpec are set.
func ValidateDaemonSetSpec(spec *extensions.DaemonSetSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, unversionedvalidation.ValidateLabelSelector(spec.Selector, fldPath.Child("selector"))...)

	selector, err := unversioned.LabelSelectorAsSelector(spec.Selector)
	if err == nil && !selector.Matches(labels.Set(spec.Template.Labels)) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("template", "metadata", "labels"), spec.Template.Labels, "`selector` does not match template `labels`"))
	}
	if spec.Selector != nil && len(spec.Selector.MatchLabels)+len(spec.Selector.MatchExpressions) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("selector"), spec.Selector, "empty selector is not valid for daemonset."))
	}

	allErrs = append(allErrs, apivalidation.ValidatePodTemplateSpec(&spec.Template, fldPath.Child("template"))...)
	// Daemons typically run on more than one node, so mark Read-Write persistent disks as invalid.
	allErrs = append(allErrs, apivalidation.ValidateReadOnlyPersistentDisks(spec.Template.Spec.Volumes, fldPath.Child("template", "spec", "volumes"))...)
	// RestartPolicy has already been first-order validated as per ValidatePodTemplateSpec().
	if spec.Template.Spec.RestartPolicy != api.RestartPolicyAlways {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("template", "spec", "restartPolicy"), spec.Template.Spec.RestartPolicy, []string{string(api.RestartPolicyAlways)}))
	}
	return allErrs
}

// ValidateDaemonSetName can be used to check whether the given daemon set name is valid.
// Prefix indicates this name will be used as part of generation, in which case
// trailing dashes are allowed.
func ValidateDaemonSetName(name string, prefix bool) (bool, string) {
	return apivalidation.NameIsDNSSubdomain(name, prefix)
}

// Validates that the given name can be used as a deployment name.
func ValidateDeploymentName(name string, prefix bool) (bool, string) {
	return apivalidation.NameIsDNSSubdomain(name, prefix)
}

func ValidatePositiveIntOrPercent(intOrPercent intstr.IntOrString, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if intOrPercent.Type == intstr.String {
		if !validation.IsValidPercent(intOrPercent.StrVal) {
			allErrs = append(allErrs, field.Invalid(fldPath, intOrPercent, "must be an integer or percentage (e.g '5%')"))
		}
	} else if intOrPercent.Type == intstr.Int {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(intOrPercent.IntValue()), fldPath)...)
	}
	return allErrs
}

func getPercentValue(intOrStringValue intstr.IntOrString) (int, bool) {
	if intOrStringValue.Type != intstr.String || !validation.IsValidPercent(intOrStringValue.StrVal) {
		return 0, false
	}
	value, _ := strconv.Atoi(intOrStringValue.StrVal[:len(intOrStringValue.StrVal)-1])
	return value, true
}

func getIntOrPercentValue(intOrStringValue intstr.IntOrString) int {
	value, isPercent := getPercentValue(intOrStringValue)
	if isPercent {
		return value
	}
	return intOrStringValue.IntValue()
}

func IsNotMoreThan100Percent(intOrStringValue intstr.IntOrString, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	value, isPercent := getPercentValue(intOrStringValue)
	if !isPercent || value <= 100 {
		return nil
	}
	allErrs = append(allErrs, field.Invalid(fldPath, intOrStringValue, "must not be greater than 100%"))
	return allErrs
}

func ValidateRollingUpdateDeployment(rollingUpdate *extensions.RollingUpdateDeployment, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, ValidatePositiveIntOrPercent(rollingUpdate.MaxUnavailable, fldPath.Child("maxUnavailable"))...)
	allErrs = append(allErrs, ValidatePositiveIntOrPercent(rollingUpdate.MaxSurge, fldPath.Child("maxSurge"))...)
	if getIntOrPercentValue(rollingUpdate.MaxUnavailable) == 0 && getIntOrPercentValue(rollingUpdate.MaxSurge) == 0 {
		// Both MaxSurge and MaxUnavailable cannot be zero.
		allErrs = append(allErrs, field.Invalid(fldPath.Child("maxUnavailable"), rollingUpdate.MaxUnavailable, "may not be 0 when `maxSurge` is 0"))
	}
	// Validate that MaxUnavailable is not more than 100%.
	allErrs = append(allErrs, IsNotMoreThan100Percent(rollingUpdate.MaxUnavailable, fldPath.Child("maxUnavailable"))...)
	return allErrs
}

func ValidateDeploymentStrategy(strategy *extensions.DeploymentStrategy, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if strategy.RollingUpdate == nil {
		return allErrs
	}
	switch strategy.Type {
	case extensions.RecreateDeploymentStrategyType:
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("rollingUpdate"), "may not be specified when strategy `type` is '"+string(extensions.RecreateDeploymentStrategyType+"'")))
	case extensions.RollingUpdateDeploymentStrategyType:
		allErrs = append(allErrs, ValidateRollingUpdateDeployment(strategy.RollingUpdate, fldPath.Child("rollingUpdate"))...)
	}
	return allErrs
}

func ValidateRollback(rollback *extensions.RollbackConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	v := rollback.Revision
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(v), fldPath.Child("version"))...)
	return allErrs
}

// Validates given deployment spec.
func ValidateDeploymentSpec(spec *extensions.DeploymentSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(spec.Replicas), fldPath.Child("replicas"))...)

	if spec.Selector == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("selector"), ""))
	} else {
		allErrs = append(allErrs, unversionedvalidation.ValidateLabelSelector(spec.Selector, fldPath.Child("selector"))...)
		if len(spec.Selector.MatchLabels)+len(spec.Selector.MatchExpressions) == 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("selector"), spec.Selector, "empty selector is not valid for deployment."))
		}
	}

	selector, err := unversioned.LabelSelectorAsSelector(spec.Selector)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("selector"), spec.Selector, "invalid label selector."))
	} else {
		allErrs = append(allErrs, ValidatePodTemplateSpecForReplicaSet(&spec.Template, selector, spec.Replicas, fldPath.Child("template"))...)
	}

	allErrs = append(allErrs, ValidateDeploymentStrategy(&spec.Strategy, fldPath.Child("strategy"))...)
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(spec.MinReadySeconds), fldPath.Child("minReadySeconds"))...)
	if spec.RevisionHistoryLimit != nil {
		// zero is a valid RevisionHistoryLimit
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(*spec.RevisionHistoryLimit), fldPath.Child("revisionHistoryLimit"))...)
	}
	if spec.RollbackTo != nil {
		allErrs = append(allErrs, ValidateRollback(spec.RollbackTo, fldPath.Child("rollback"))...)
	}
	return allErrs
}

// Validates given deployment status.
func ValidateDeploymentStatus(status *extensions.DeploymentStatus, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(status.ObservedGeneration, fldPath.Child("observedGeneration"))...)
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(status.Replicas), fldPath.Child("replicas"))...)
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(status.UpdatedReplicas), fldPath.Child("updatedReplicas"))...)
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(status.AvailableReplicas), fldPath.Child("availableReplicas"))...)
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(status.UnavailableReplicas), fldPath.Child("unavailableReplicas"))...)
	return allErrs
}

func ValidateDeploymentUpdate(update, old *extensions.Deployment) field.ErrorList {
	allErrs := apivalidation.ValidateObjectMetaUpdate(&update.ObjectMeta, &old.ObjectMeta, field.NewPath("metadata"))
	allErrs = append(allErrs, ValidateDeploymentSpec(&update.Spec, field.NewPath("spec"))...)
	return allErrs
}

func ValidateDeploymentStatusUpdate(update, old *extensions.Deployment) field.ErrorList {
	allErrs := apivalidation.ValidateObjectMetaUpdate(&update.ObjectMeta, &old.ObjectMeta, field.NewPath("metadata"))
	allErrs = append(allErrs, ValidateDeploymentStatus(&update.Status, field.NewPath("status"))...)
	return allErrs
}

func ValidateDeployment(obj *extensions.Deployment) field.ErrorList {
	allErrs := apivalidation.ValidateObjectMeta(&obj.ObjectMeta, true, ValidateDeploymentName, field.NewPath("metadata"))
	allErrs = append(allErrs, ValidateDeploymentSpec(&obj.Spec, field.NewPath("spec"))...)
	return allErrs
}

func ValidateDeploymentRollback(obj *extensions.DeploymentRollback) field.ErrorList {
	allErrs := apivalidation.ValidateAnnotations(obj.UpdatedAnnotations, field.NewPath("updatedAnnotations"))
	if len(obj.Name) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("name"), "name is required"))
	}
	allErrs = append(allErrs, ValidateRollback(&obj.RollbackTo, field.NewPath("rollback"))...)
	return allErrs
}

func ValidateThirdPartyResourceDataUpdate(update, old *extensions.ThirdPartyResourceData) field.ErrorList {
	return ValidateThirdPartyResourceData(update)
}

func ValidateThirdPartyResourceData(obj *extensions.ThirdPartyResourceData) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(obj.Name) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("name"), ""))
	}
	return allErrs
}

// ValidateIngress tests if required fields in the Ingress are set.
func ValidateIngress(ingress *extensions.Ingress) field.ErrorList {
	allErrs := apivalidation.ValidateObjectMeta(&ingress.ObjectMeta, true, ValidateIngressName, field.NewPath("metadata"))
	allErrs = append(allErrs, ValidateIngressSpec(&ingress.Spec, field.NewPath("spec"))...)
	return allErrs
}

// ValidateIngressName validates that the given name can be used as an Ingress name.
func ValidateIngressName(name string, prefix bool) (bool, string) {
	return apivalidation.NameIsDNSSubdomain(name, prefix)
}

func validateIngressTLS(spec *extensions.IngressSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	// TODO: Perform a more thorough validation of spec.TLS.Hosts that takes
	// the wildcard spec from RFC 6125 into account.
	return allErrs
}

// ValidateIngressSpec tests if required fields in the IngressSpec are set.
func ValidateIngressSpec(spec *extensions.IngressSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	// TODO: Is a default backend mandatory?
	if spec.Backend != nil {
		allErrs = append(allErrs, validateIngressBackend(spec.Backend, fldPath.Child("backend"))...)
	} else if len(spec.Rules) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, spec.Rules, "either `backend` or `rules` must be specified"))
	}
	if len(spec.Rules) > 0 {
		allErrs = append(allErrs, validateIngressRules(spec.Rules, fldPath.Child("rules"))...)
	}
	if len(spec.TLS) > 0 {
		allErrs = append(allErrs, validateIngressTLS(spec, fldPath.Child("tls"))...)
	}
	return allErrs
}

// ValidateIngressUpdate tests if required fields in the Ingress are set.
func ValidateIngressUpdate(ingress, oldIngress *extensions.Ingress) field.ErrorList {
	allErrs := apivalidation.ValidateObjectMetaUpdate(&ingress.ObjectMeta, &oldIngress.ObjectMeta, field.NewPath("metadata"))
	allErrs = append(allErrs, ValidateIngressSpec(&ingress.Spec, field.NewPath("spec"))...)
	return allErrs
}

// ValidateIngressStatusUpdate tests if required fields in the Ingress are set when updating status.
func ValidateIngressStatusUpdate(ingress, oldIngress *extensions.Ingress) field.ErrorList {
	allErrs := apivalidation.ValidateObjectMetaUpdate(&ingress.ObjectMeta, &oldIngress.ObjectMeta, field.NewPath("metadata"))
	allErrs = append(allErrs, apivalidation.ValidateLoadBalancerStatus(&ingress.Status.LoadBalancer, field.NewPath("status", "loadBalancer"))...)
	return allErrs
}

func validateIngressRules(IngressRules []extensions.IngressRule, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(IngressRules) == 0 {
		return append(allErrs, field.Required(fldPath, ""))
	}
	for i, ih := range IngressRules {
		if len(ih.Host) > 0 {
			// TODO: Ports and ips are allowed in the host part of a url
			// according to RFC 3986, consider allowing them.
			if valid, errMsg := apivalidation.NameIsDNSSubdomain(ih.Host, false); !valid {
				allErrs = append(allErrs, field.Invalid(fldPath.Index(i).Child("host"), ih.Host, errMsg))
			}
			if isIP := (net.ParseIP(ih.Host) != nil); isIP {
				allErrs = append(allErrs, field.Invalid(fldPath.Index(i).Child("host"), ih.Host, "must be a DNS name, not an IP address"))
			}
		}
		allErrs = append(allErrs, validateIngressRuleValue(&ih.IngressRuleValue, fldPath.Index(0))...)
	}
	return allErrs
}

func validateIngressRuleValue(ingressRule *extensions.IngressRuleValue, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if ingressRule.HTTP != nil {
		allErrs = append(allErrs, validateHTTPIngressRuleValue(ingressRule.HTTP, fldPath.Child("http"))...)
	}
	return allErrs
}

func validateHTTPIngressRuleValue(httpIngressRuleValue *extensions.HTTPIngressRuleValue, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(httpIngressRuleValue.Paths) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("paths"), ""))
	}
	for i, rule := range httpIngressRuleValue.Paths {
		if len(rule.Path) > 0 {
			if !strings.HasPrefix(rule.Path, "/") {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("paths").Index(i).Child("path"), rule.Path, "must be an absolute path"))
			}
			// TODO: More draconian path regex validation.
			// Path must be a valid regex. This is the basic requirement.
			// In addition to this any characters not allowed in a path per
			// RFC 3986 section-3.3 cannot appear as a literal in the regex.
			// Consider the example: http://host/valid?#bar, everything after
			// the last '/' is a valid regex that matches valid#bar, which
			// isn't a valid path, because the path terminates at the first ?
			// or #. A more sophisticated form of validation would detect that
			// the user is confusing url regexes with path regexes.
			_, err := regexp.CompilePOSIX(rule.Path)
			if err != nil {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("paths").Index(i).Child("path"), rule.Path, "must be a valid regex"))
			}
		}
		allErrs = append(allErrs, validateIngressBackend(&rule.Backend, fldPath.Child("backend"))...)
	}
	return allErrs
}

// validateIngressBackend tests if a given backend is valid.
func validateIngressBackend(backend *extensions.IngressBackend, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// All backends must reference a single local service by name, and a single service port by name or number.
	if len(backend.ServiceName) == 0 {
		return append(allErrs, field.Required(fldPath.Child("serviceName"), ""))
	} else if ok, errMsg := apivalidation.ValidateServiceName(backend.ServiceName, false); !ok {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("serviceName"), backend.ServiceName, errMsg))
	}
	if backend.ServicePort.Type == intstr.String {
		if !validation.IsDNS1123Label(backend.ServicePort.StrVal) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("servicePort"), backend.ServicePort.StrVal, apivalidation.DNS1123LabelErrorMsg))
		}
		if !validation.IsValidPortName(backend.ServicePort.StrVal) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("servicePort"), backend.ServicePort.StrVal, apivalidation.PortNameErrorMsg))
		}
	} else if !validation.IsValidPortNum(backend.ServicePort.IntValue()) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("servicePort"), backend.ServicePort, apivalidation.PortRangeErrorMsg))
	}
	return allErrs
}

func ValidateScale(scale *extensions.Scale) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&scale.ObjectMeta, true, apivalidation.NameIsDNSSubdomain, field.NewPath("metadata"))...)

	if scale.Spec.Replicas < 0 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "replicas"), scale.Spec.Replicas, "must be greater than or equal to 0"))
	}

	return allErrs
}

// ValidateReplicaSetName can be used to check whether the given ReplicaSet
// name is valid.
// Prefix indicates this name will be used as part of generation, in which case
// trailing dashes are allowed.
func ValidateReplicaSetName(name string, prefix bool) (bool, string) {
	return apivalidation.NameIsDNSSubdomain(name, prefix)
}

// ValidateReplicaSet tests if required fields in the ReplicaSet are set.
func ValidateReplicaSet(rs *extensions.ReplicaSet) field.ErrorList {
	allErrs := apivalidation.ValidateObjectMeta(&rs.ObjectMeta, true, ValidateReplicaSetName, field.NewPath("metadata"))
	allErrs = append(allErrs, ValidateReplicaSetSpec(&rs.Spec, field.NewPath("spec"))...)
	return allErrs
}

// ValidateReplicaSetUpdate tests if required fields in the ReplicaSet are set.
func ValidateReplicaSetUpdate(rs, oldRs *extensions.ReplicaSet) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&rs.ObjectMeta, &oldRs.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateReplicaSetSpec(&rs.Spec, field.NewPath("spec"))...)
	return allErrs
}

// ValidateReplicaSetStatusUpdate tests if required fields in the ReplicaSet are set.
func ValidateReplicaSetStatusUpdate(rs, oldRs *extensions.ReplicaSet) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&rs.ObjectMeta, &oldRs.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(rs.Status.Replicas), field.NewPath("status", "replicas"))...)
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(rs.Status.FullyLabeledReplicas), field.NewPath("status", "fullyLabeledReplicas"))...)
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(rs.Status.ObservedGeneration), field.NewPath("status", "observedGeneration"))...)
	return allErrs
}

// ValidateReplicaSetSpec tests if required fields in the ReplicaSet spec are set.
func ValidateReplicaSetSpec(spec *extensions.ReplicaSetSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(spec.Replicas), fldPath.Child("replicas"))...)

	if spec.Selector == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("selector"), ""))
	} else {
		allErrs = append(allErrs, unversionedvalidation.ValidateLabelSelector(spec.Selector, fldPath.Child("selector"))...)
		if len(spec.Selector.MatchLabels)+len(spec.Selector.MatchExpressions) == 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("selector"), spec.Selector, "empty selector is not valid for deployment."))
		}
	}

	selector, err := unversioned.LabelSelectorAsSelector(spec.Selector)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("selector"), spec.Selector, "invalid label selector."))
	} else {
		allErrs = append(allErrs, ValidatePodTemplateSpecForReplicaSet(&spec.Template, selector, spec.Replicas, fldPath.Child("template"))...)
	}
	return allErrs
}

// Validates the given template and ensures that it is in accordance with the desired selector and replicas.
func ValidatePodTemplateSpecForReplicaSet(template *api.PodTemplateSpec, selector labels.Selector, replicas int32, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if template == nil {
		allErrs = append(allErrs, field.Required(fldPath, ""))
	} else {
		if !selector.Empty() {
			// Verify that the ReplicaSet selector matches the labels in template.
			labels := labels.Set(template.Labels)
			if !selector.Matches(labels) {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("metadata", "labels"), template.Labels, "`selector` does not match template `labels`"))
			}
		}
		allErrs = append(allErrs, apivalidation.ValidatePodTemplateSpec(template, fldPath)...)
		if replicas > 1 {
			allErrs = append(allErrs, apivalidation.ValidateReadOnlyPersistentDisks(template.Spec.Volumes, fldPath.Child("spec", "volumes"))...)
		}
		// RestartPolicy has already been first-order validated as per ValidatePodTemplateSpec().
		if template.Spec.RestartPolicy != api.RestartPolicyAlways {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("spec", "restartPolicy"), template.Spec.RestartPolicy, []string{string(api.RestartPolicyAlways)}))
		}
	}
	return allErrs
}

// ValidatePodSecurityPolicyName can be used to check whether the given
// pod security policy name is valid.
// Prefix indicates this name will be used as part of generation, in which case
// trailing dashes are allowed.
func ValidatePodSecurityPolicyName(name string, prefix bool) (bool, string) {
	return apivalidation.NameIsDNSSubdomain(name, prefix)
}

func ValidatePodSecurityPolicy(psp *extensions.PodSecurityPolicy) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&psp.ObjectMeta, false, ValidatePodSecurityPolicyName, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidatePodSecurityPolicySpec(&psp.Spec, field.NewPath("spec"))...)
	return allErrs
}

func ValidatePodSecurityPolicySpec(spec *extensions.PodSecurityPolicySpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validatePSPRunAsUser(fldPath.Child("runAsUser"), &spec.RunAsUser)...)
	allErrs = append(allErrs, validatePSPSELinux(fldPath.Child("seLinux"), &spec.SELinux)...)
	allErrs = append(allErrs, validatePSPSupplementalGroup(fldPath.Child("supplementalGroups"), &spec.SupplementalGroups)...)
	allErrs = append(allErrs, validatePSPFSGroup(fldPath.Child("fsGroup"), &spec.FSGroup)...)
	allErrs = append(allErrs, validatePodSecurityPolicyVolumes(fldPath, spec.Volumes)...)
	allErrs = append(allErrs, validatePSPCapsAgainstDrops(spec.RequiredDropCapabilities, spec.DefaultAddCapabilities, field.NewPath("defaultAddCapabilities"))...)
	allErrs = append(allErrs, validatePSPCapsAgainstDrops(spec.RequiredDropCapabilities, spec.AllowedCapabilities, field.NewPath("allowedCapabilities"))...)

	return allErrs
}

// validatePSPSELinux validates the SELinux fields of PodSecurityPolicy.
func validatePSPSELinux(fldPath *field.Path, seLinux *extensions.SELinuxStrategyOptions) field.ErrorList {
	allErrs := field.ErrorList{}

	// ensure the selinux strategy has a valid rule
	supportedSELinuxRules := sets.NewString(string(extensions.SELinuxStrategyMustRunAs),
		string(extensions.SELinuxStrategyRunAsAny))
	if !supportedSELinuxRules.Has(string(seLinux.Rule)) {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("rule"), seLinux.Rule, supportedSELinuxRules.List()))
	}

	return allErrs
}

// validatePSPRunAsUser validates the RunAsUser fields of PodSecurityPolicy.
func validatePSPRunAsUser(fldPath *field.Path, runAsUser *extensions.RunAsUserStrategyOptions) field.ErrorList {
	allErrs := field.ErrorList{}

	// ensure the user strategy has a valid rule
	supportedRunAsUserRules := sets.NewString(string(extensions.RunAsUserStrategyMustRunAs),
		string(extensions.RunAsUserStrategyMustRunAsNonRoot),
		string(extensions.RunAsUserStrategyRunAsAny))
	if !supportedRunAsUserRules.Has(string(runAsUser.Rule)) {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("rule"), runAsUser.Rule, supportedRunAsUserRules.List()))
	}

	// validate range settings
	for idx, rng := range runAsUser.Ranges {
		allErrs = append(allErrs, validateIDRanges(fldPath.Child("ranges").Index(idx), rng)...)
	}

	return allErrs
}

// validatePSPFSGroup validates the FSGroupStrategyOptions fields of the PodSecurityPolicy.
func validatePSPFSGroup(fldPath *field.Path, groupOptions *extensions.FSGroupStrategyOptions) field.ErrorList {
	allErrs := field.ErrorList{}

	supportedRules := sets.NewString(
		string(extensions.FSGroupStrategyMustRunAs),
		string(extensions.FSGroupStrategyRunAsAny),
	)
	if !supportedRules.Has(string(groupOptions.Rule)) {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("rule"), groupOptions.Rule, supportedRules.List()))
	}

	for idx, rng := range groupOptions.Ranges {
		allErrs = append(allErrs, validateIDRanges(fldPath.Child("ranges").Index(idx), rng)...)
	}
	return allErrs
}

// validatePSPSupplementalGroup validates the SupplementalGroupsStrategyOptions fields of the PodSecurityPolicy.
func validatePSPSupplementalGroup(fldPath *field.Path, groupOptions *extensions.SupplementalGroupsStrategyOptions) field.ErrorList {
	allErrs := field.ErrorList{}

	supportedRules := sets.NewString(
		string(extensions.SupplementalGroupsStrategyRunAsAny),
		string(extensions.SupplementalGroupsStrategyMustRunAs),
	)
	if !supportedRules.Has(string(groupOptions.Rule)) {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("rule"), groupOptions.Rule, supportedRules.List()))
	}

	for idx, rng := range groupOptions.Ranges {
		allErrs = append(allErrs, validateIDRanges(fldPath.Child("ranges").Index(idx), rng)...)
	}
	return allErrs
}

// validatePodSecurityPolicyVolumes validates the volume fields of PodSecurityPolicy.
func validatePodSecurityPolicyVolumes(fldPath *field.Path, volumes []extensions.FSType) field.ErrorList {
	allErrs := field.ErrorList{}
	allowed := psputil.GetAllFSTypesAsSet()
	// add in the * value since that is a pseudo type that is not included by default
	allowed.Insert(string(extensions.All))
	for _, v := range volumes {
		if !allowed.Has(string(v)) {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("volumes"), v, allowed.List()))
		}
	}

	return allErrs
}

// validateIDRanges ensures the range is valid.
func validateIDRanges(fldPath *field.Path, rng extensions.IDRange) field.ErrorList {
	allErrs := field.ErrorList{}

	// if 0 <= Min <= Max then we do not need to validate max.  It is always greater than or
	// equal to 0 and Min.
	if rng.Min < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("min"), rng.Min, "min cannot be negative"))
	}
	if rng.Max < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("max"), rng.Max, "max cannot be negative"))
	}
	if rng.Min > rng.Max {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("min"), rng.Min, "min cannot be greater than max"))
	}

	return allErrs
}

// validatePSPCapsAgainstDrops ensures an allowed cap is not listed in the required drops.
func validatePSPCapsAgainstDrops(requiredDrops []api.Capability, capsToCheck []api.Capability, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if requiredDrops == nil {
		return allErrs
	}
	for _, cap := range capsToCheck {
		if hasCap(cap, requiredDrops) {
			allErrs = append(allErrs, field.Invalid(fldPath, cap,
				fmt.Sprintf("capability is listed in %s and requiredDropCapabilities", fldPath.String())))
		}
	}
	return allErrs
}

// hasCap checks for needle in haystack.
func hasCap(needle api.Capability, haystack []api.Capability) bool {
	for _, c := range haystack {
		if needle == c {
			return true
		}
	}
	return false
}

// ValidatePodSecurityPolicyUpdate validates a PSP for updates.
func ValidatePodSecurityPolicyUpdate(old *extensions.PodSecurityPolicy, new *extensions.PodSecurityPolicy) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&old.ObjectMeta, &new.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidatePodSecurityPolicySpec(&new.Spec, field.NewPath("spec"))...)
	return allErrs
}

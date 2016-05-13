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
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	unversionedvalidation "k8s.io/kubernetes/pkg/api/unversioned/validation"
	apivalidation "k8s.io/kubernetes/pkg/api/validation"
	"k8s.io/kubernetes/pkg/apis/batch"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/util/validation/field"
)

// TODO: generalize for other controller objects that will follow the same pattern, such as ReplicaSet and DaemonSet, and
// move to new location.  Replace batch.Job with an interface.
//
// ValidateGeneratedSelector validates that the generated selector on a controller object match the controller object
// metadata, and the labels on the pod template are as generated.
func ValidateGeneratedSelector(obj *batch.Job) field.ErrorList {
	allErrs := field.ErrorList{}
	if obj.Spec.ManualSelector != nil && *obj.Spec.ManualSelector {
		return allErrs
	}

	if obj.Spec.Selector == nil {
		return allErrs // This case should already have been checked in caller.  No need for more errors.
	}

	// If somehow uid was unset then we would get "controller-uid=" as the selector
	// which is bad.
	if obj.ObjectMeta.UID == "" {
		allErrs = append(allErrs, field.Required(field.NewPath("metadata").Child("uid"), ""))
	}

	// If somehow uid was unset then we would get "controller-uid=" as the selector
	// which is bad.
	if obj.ObjectMeta.UID == "" {
		allErrs = append(allErrs, field.Required(field.NewPath("metadata").Child("uid"), ""))
	}

	// If selector generation was requested, then expected labels must be
	// present on pod template, and much match job's uid and name.  The
	// generated (not-manual) selectors/labels ensure no overlap with other
	// controllers.  The manual mode allows orphaning, adoption,
	// backward-compatibility, and experimentation with new
	// labeling/selection schemes.  Automatic selector generation should
	// have placed certain labels on the pod, but this could have failed if
	// the user added coflicting labels.  Validate that the expected
	// generated ones are there.

	allErrs = append(allErrs, apivalidation.ValidateHasLabel(obj.Spec.Template.ObjectMeta, field.NewPath("spec").Child("template").Child("metadata"), "controller-uid", string(obj.UID))...)
	allErrs = append(allErrs, apivalidation.ValidateHasLabel(obj.Spec.Template.ObjectMeta, field.NewPath("spec").Child("template").Child("metadata"), "job-name", string(obj.Name))...)
	expectedLabels := make(map[string]string)
	expectedLabels["controller-uid"] = string(obj.UID)
	expectedLabels["job-name"] = string(obj.Name)
	// Whether manually or automatically generated, the selector of the job must match the pods it will produce.
	if selector, err := unversioned.LabelSelectorAsSelector(obj.Spec.Selector); err == nil {
		if !selector.Matches(labels.Set(expectedLabels)) {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("selector"), obj.Spec.Selector, "`selector` not auto-generated"))
		}
	}

	return allErrs
}

func ValidateJob(job *batch.Job) field.ErrorList {
	// Jobs and rcs have the same name validation
	allErrs := apivalidation.ValidateObjectMeta(&job.ObjectMeta, true, apivalidation.ValidateReplicationControllerName, field.NewPath("metadata"))
	allErrs = append(allErrs, ValidateGeneratedSelector(job)...)
	allErrs = append(allErrs, ValidateJobSpec(&job.Spec, field.NewPath("spec"))...)
	return allErrs
}

func ValidateJobSpec(spec *batch.JobSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if spec.Parallelism != nil {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(*spec.Parallelism), fldPath.Child("parallelism"))...)
	}
	if spec.Completions != nil {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(*spec.Completions), fldPath.Child("completions"))...)
	}
	if spec.ActiveDeadlineSeconds != nil {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(*spec.ActiveDeadlineSeconds), fldPath.Child("activeDeadlineSeconds"))...)
	}
	if spec.Selector == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("selector"), ""))
	} else {
		allErrs = append(allErrs, unversionedvalidation.ValidateLabelSelector(spec.Selector, fldPath.Child("selector"))...)
	}

	// Whether manually or automatically generated, the selector of the job must match the pods it will produce.
	if selector, err := unversioned.LabelSelectorAsSelector(spec.Selector); err == nil {
		labels := labels.Set(spec.Template.Labels)
		if !selector.Matches(labels) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("template", "metadata", "labels"), spec.Template.Labels, "`selector` does not match template `labels`"))
		}
	}

	allErrs = append(allErrs, apivalidation.ValidatePodTemplateSpec(&spec.Template, fldPath.Child("template"))...)
	if spec.Template.Spec.RestartPolicy != api.RestartPolicyOnFailure &&
		spec.Template.Spec.RestartPolicy != api.RestartPolicyNever {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("template", "spec", "restartPolicy"),
			spec.Template.Spec.RestartPolicy, []string{string(api.RestartPolicyOnFailure), string(api.RestartPolicyNever)}))
	}
	return allErrs
}

func ValidateJobStatus(status *batch.JobStatus, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(status.Active), fldPath.Child("active"))...)
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(status.Succeeded), fldPath.Child("succeeded"))...)
	allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(status.Failed), fldPath.Child("failed"))...)
	return allErrs
}

func ValidateJobUpdate(job, oldJob *batch.Job) field.ErrorList {
	allErrs := apivalidation.ValidateObjectMetaUpdate(&oldJob.ObjectMeta, &job.ObjectMeta, field.NewPath("metadata"))
	allErrs = append(allErrs, ValidateJobSpecUpdate(job.Spec, oldJob.Spec, field.NewPath("spec"))...)
	return allErrs
}

func ValidateJobUpdateStatus(job, oldJob *batch.Job) field.ErrorList {
	allErrs := apivalidation.ValidateObjectMetaUpdate(&oldJob.ObjectMeta, &job.ObjectMeta, field.NewPath("metadata"))
	allErrs = append(allErrs, ValidateJobStatusUpdate(job.Status, oldJob.Status)...)
	return allErrs
}

func ValidateJobSpecUpdate(spec, oldSpec batch.JobSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, ValidateJobSpec(&spec, fldPath)...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(spec.Completions, oldSpec.Completions, fldPath.Child("completions"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(spec.Selector, oldSpec.Selector, fldPath.Child("selector"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(spec.Template, oldSpec.Template, fldPath.Child("template"))...)
	return allErrs
}

func ValidateJobStatusUpdate(status, oldStatus batch.JobStatus) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, ValidateJobStatus(&status, field.NewPath("status"))...)
	return allErrs
}

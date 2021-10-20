/*
Copyright The Kubernetes Authors.

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

package v1beta1

import (
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var metal3remediationlog = logf.Log.WithName("metal3remediation-resource")

func (r *Metal3Remediation) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-metal3remediation,mutating=false,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=metal3remediations,versions=v1beta1,name=validation.metal3remediation.infrastructure.cluster.x-k8s.io,matchPolicy=Equivalent,sideEffects=None,admissionReviewVersions=v1;v1beta1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-metal3remediation,mutating=true,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=metal3remediations,versions=v1beta1,name=default.metal3remediation.infrastructure.cluster.x-k8s.io,matchPolicy=Equivalent,sideEffects=None,admissionReviewVersions=v1;v1beta1

var _ webhook.Defaulter = &Metal3Remediation{}
var _ webhook.Validator = &Metal3Remediation{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Metal3Remediation) Default() {
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Metal3Remediation) ValidateCreate() error {
	return r.validate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Metal3Remediation) ValidateUpdate(old runtime.Object) error {
	return r.validate()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Metal3Remediation) ValidateDelete() error {
	metal3remediationlog.Info("validate delete", "name", r.Name)
	return nil
}

func (r *Metal3Remediation) validate() error {
	var allErrs field.ErrorList
	if r.Spec.Strategy.Timeout != nil && r.Spec.Strategy.Timeout.Seconds() < minTimeout.Seconds() {
		allErrs = append(
			allErrs,
			field.Invalid(
				field.NewPath("spec", "strategy", "timeout"),
				r.Spec.Strategy.Timeout,
				"min duration is minTimeout.Seconds()",
			),
		)
	}

	if r.Spec.Strategy.Type != RebootRemediationStrategy {
		allErrs = append(
			allErrs,
			field.Invalid(
				field.NewPath("spec", "strategy", "type"),
				r.Spec.Strategy.Type,
				"is only supported remediation strategy",
			),
		)
	}

	if r.Spec.Strategy.RetryLimit < minRetryLimit {
		allErrs = append(
			allErrs,
			field.Invalid(
				field.NewPath("spec", "strategy", "retryLimit"),
				r.Spec.Strategy.RetryLimit,
				"is minimum retrylimit",
			),
		)
	}

	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(GroupVersion.WithKind("Metal3Remediation").GroupKind(), r.Name, allErrs)
}

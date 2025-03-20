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
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// Deprecated: This method is going to be removed in a next release.
func (webhook *Metal3Remediation) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(webhook).
		WithDefaulter(webhook, admission.DefaulterRemoveUnknownOrOmitableFields).
		WithValidator(webhook).
		Complete()
}

var _ webhook.CustomDefaulter = &Metal3Remediation{}
var _ webhook.CustomValidator = &Metal3Remediation{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
// Deprecated: This method is going to be removed in a next release.
func (webhook *Metal3Remediation) Default(_ context.Context, _ runtime.Object) error {
	return nil
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
// Deprecated: This method is going to be removed in a next release.
func (webhook *Metal3Remediation) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	c, ok := obj.(*Metal3Remediation)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a Metal3Remediation but got a %T", obj))
	}
	return nil, webhook.validate(c)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
// Deprecated: This method is going to be removed in a next release.
func (webhook *Metal3Remediation) ValidateUpdate(_ context.Context, _, newObj runtime.Object) (admission.Warnings, error) {
	newM3R, ok := newObj.(*Metal3Remediation)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a Metal3Remediation but got a %T", newObj))
	}

	return nil, webhook.validate(newM3R)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
// Deprecated: This method is going to be removed in a next release.
func (webhook *Metal3Remediation) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// Deprecated: This method is going to be removed in a next release.
func (webhook *Metal3Remediation) validate(newM3R *Metal3Remediation) error {
	var allErrs field.ErrorList
	if newM3R.Spec.Strategy.Timeout != nil && newM3R.Spec.Strategy.Timeout.Seconds() < minTimeout.Seconds() {
		allErrs = append(
			allErrs,
			field.Invalid(
				field.NewPath("spec", "strategy", "timeout"),
				newM3R.Spec.Strategy.Timeout,
				"min duration is minTimeout.Seconds()",
			),
		)
	}

	if newM3R.Spec.Strategy.Type != RebootRemediationStrategy {
		allErrs = append(
			allErrs,
			field.Invalid(
				field.NewPath("spec", "strategy", "type"),
				newM3R.Spec.Strategy.Type,
				"is only supported remediation strategy",
			),
		)
	}

	if newM3R.Spec.Strategy.RetryLimit < minRetryLimit {
		allErrs = append(
			allErrs,
			field.Invalid(
				field.NewPath("spec", "strategy", "retryLimit"),
				newM3R.Spec.Strategy.RetryLimit,
				"is minimum retrylimit",
			),
		)
	}

	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(GroupVersion.WithKind("Metal3Remediation").GroupKind(), newM3R.Name, allErrs)
}

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
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var (
	// Default retry timeout is 600 seconds.
	defaultTimeout = metav1.Duration{Duration: 600 * time.Second}
	// Minimum time between remediation retries.
	minTimeout = metav1.Duration{Duration: 100 * time.Second}
	// Mininum remediation retry limit is 1.
	// Controller will try to remediate unhealhy node at least once.
	minRetryLimit = 1
)

// log is for logging in this package.
var metal3remediationtemplatelog = logf.Log.WithName("metal3remediationtemplate-resource")

func (webhook *Metal3RemediationTemplate) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(webhook).
		WithDefaulter(webhook, admission.DefaulterRemoveUnknownOrOmitableFields).
		WithValidator(webhook).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-metal3remediationtemplate,mutating=false,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=metal3remediationtemplates,versions=v1beta1,name=validation.metal3remediationtemplate.infrastructure.cluster.x-k8s.io,matchPolicy=Equivalent,sideEffects=None,admissionReviewVersions=v1;v1beta1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-metal3remediationtemplate,mutating=true,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=metal3remediationtemplates,versions=v1beta1,name=default.metal3remediationtemplate.infrastructure.cluster.x-k8s.io,matchPolicy=Equivalent,sideEffects=None,admissionReviewVersions=v1;v1beta1

var _ webhook.CustomDefaulter = &Metal3RemediationTemplate{}
var _ webhook.CustomValidator = &Metal3RemediationTemplate{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (webhook *Metal3RemediationTemplate) Default(_ context.Context, obj runtime.Object) error {
	c, ok := obj.(*Metal3RemediationTemplate)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a Metal3RemediationTemplate but got a %T", obj))
	}

	if c.Spec.Template.Spec.Strategy.Type == "" {
		c.Spec.Template.Spec.Strategy.Type = RebootRemediationStrategy
	}

	if c.Spec.Template.Spec.Strategy.Timeout == nil {
		c.Spec.Template.Spec.Strategy.Timeout = &defaultTimeout
	}

	if c.Spec.Template.Spec.Strategy.RetryLimit == 0 || c.Spec.Template.Spec.Strategy.RetryLimit < minRetryLimit {
		c.Spec.Template.Spec.Strategy.RetryLimit = minRetryLimit
	}

	return nil
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (webhook *Metal3RemediationTemplate) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	c, ok := obj.(*Metal3RemediationTemplate)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a Metal3RemediationTemplate but got a %T", obj))
	}

	metal3remediationtemplatelog.Info("validate create", "name", c.Name)

	return nil, webhook.validate(c)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (webhook *Metal3RemediationTemplate) ValidateUpdate(_ context.Context, _, newObj runtime.Object) (admission.Warnings, error) {
	c, ok := newObj.(*Metal3RemediationTemplate)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a Metal3RemediationTemplate but got a %T", newObj))
	}
	metal3remediationtemplatelog.Info("validate update", "name", c.Name)
	return nil, webhook.validate(c)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (webhook *Metal3RemediationTemplate) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (webhook *Metal3RemediationTemplate) validate(newM3RT *Metal3RemediationTemplate) error {
	var allErrs field.ErrorList
	if newM3RT.Spec.Template.Spec.Strategy.Timeout != nil && newM3RT.Spec.Template.Spec.Strategy.Timeout.Seconds() < minTimeout.Seconds() {
		allErrs = append(
			allErrs,
			field.Invalid(
				field.NewPath("spec", "template", "spec", "strategy", "timeout"),
				newM3RT.Spec.Template.Spec.Strategy.Timeout,
				"min duration is 100s",
			),
		)
	}

	if newM3RT.Spec.Template.Spec.Strategy.Type != RebootRemediationStrategy {
		allErrs = append(
			allErrs,
			field.Invalid(
				field.NewPath("spec", "template", "spec", "strategy", "type"),
				newM3RT.Spec.Template.Spec.Strategy.Type,
				"only supported remediation strategy is reboot",
			),
		)
	}

	if newM3RT.Spec.Template.Spec.Strategy.RetryLimit < minRetryLimit {
		allErrs = append(
			allErrs,
			field.Invalid(
				field.NewPath("spec", "template", "spec", "strategy", "retryLimit"),
				newM3RT.Spec.Template.Spec.Strategy.RetryLimit,
				"minimun retrylimit is 1",
			),
		)
	}

	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(GroupVersion.WithKind("Metal3Remediation").GroupKind(), newM3RT.Name, allErrs)
}

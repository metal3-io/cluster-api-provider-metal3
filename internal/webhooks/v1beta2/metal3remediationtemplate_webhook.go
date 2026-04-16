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

package webhooks

import (
	"context"
	"fmt"

	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	// Default retry timeout.
	defaultTimeout = int32(600)
	// Minimum time between remediation retries.
	minTimeout = int32(100)
	// Mininum remediation retry limit is 1.
	// Controller will try to remediate unhealhy node at least once.
	minRetryLimit = int32(1)
)

// log is for logging in this package.
var metal3remediationtemplatelog = logf.Log.WithName("metal3remediationtemplate-resource")

func (webhook *Metal3RemediationTemplate) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &infrav1.Metal3RemediationTemplate{}).
		WithValidator(webhook).
		WithDefaulter(webhook).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta2-metal3remediationtemplate,mutating=false,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=metal3remediationtemplates,versions=v1beta2,name=validation.metal3remediationtemplate.infrastructure.cluster.x-k8s.io,matchPolicy=Equivalent,sideEffects=None,admissionReviewVersions=v1;v1beta1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-infrastructure-cluster-x-k8s-io-v1beta2-metal3remediationtemplate,mutating=true,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=metal3remediationtemplates,versions=v1beta2,name=default.metal3remediationtemplate.infrastructure.cluster.x-k8s.io,matchPolicy=Equivalent,sideEffects=None,admissionReviewVersions=v1;v1beta1

// Metal3RemediationTemplate implements a validation and defaulting webhook for Metal3RemediationTemplate.
type Metal3RemediationTemplate struct{}

var _ admission.Validator[*infrav1.Metal3RemediationTemplate] = &Metal3RemediationTemplate{}
var _ admission.Defaulter[*infrav1.Metal3RemediationTemplate] = &Metal3RemediationTemplate{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (webhook *Metal3RemediationTemplate) Default(_ context.Context, m3rt *infrav1.Metal3RemediationTemplate) error {
	if m3rt == nil {
		return apierrors.NewBadRequest("expected a Metal3RemediationTemplate but got nil")
	}

	if m3rt.Spec.Template.Spec.Strategy.Type == "" {
		m3rt.Spec.Template.Spec.Strategy.Type = infrav1.RebootRemediationStrategy
	}

	if m3rt.Spec.Template.Spec.Strategy.TimeoutSeconds == nil {
		timeout := defaultTimeout
		m3rt.Spec.Template.Spec.Strategy.TimeoutSeconds = &timeout
	}

	if m3rt.Spec.Template.Spec.Strategy.RetryLimit == 0 || m3rt.Spec.Template.Spec.Strategy.RetryLimit < minRetryLimit {
		m3rt.Spec.Template.Spec.Strategy.RetryLimit = minRetryLimit
	}

	return nil
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (webhook *Metal3RemediationTemplate) ValidateCreate(_ context.Context, m3rt *infrav1.Metal3RemediationTemplate) (admission.Warnings, error) {
	if m3rt == nil {
		return nil, apierrors.NewBadRequest("expected a Metal3RemediationTemplate but got nil")
	}

	metal3remediationtemplatelog.Info("validate create", "name", m3rt.Name)

	return nil, webhook.validate(m3rt)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (webhook *Metal3RemediationTemplate) ValidateUpdate(_ context.Context, _, m3rt *infrav1.Metal3RemediationTemplate) (admission.Warnings, error) {
	if m3rt == nil {
		return nil, apierrors.NewBadRequest("expected a Metal3RemediationTemplate but got nil")
	}
	metal3remediationtemplatelog.Info("validate update", "name", m3rt.Name)
	return nil, webhook.validate(m3rt)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (webhook *Metal3RemediationTemplate) ValidateDelete(_ context.Context, _ *infrav1.Metal3RemediationTemplate) (admission.Warnings, error) {
	return nil, nil
}

func (webhook *Metal3RemediationTemplate) validate(newM3rt *infrav1.Metal3RemediationTemplate) error {
	var allErrs field.ErrorList
	if newM3rt.Spec.Template.Spec.Strategy.TimeoutSeconds != nil && *newM3rt.Spec.Template.Spec.Strategy.TimeoutSeconds < minTimeout {
		allErrs = append(
			allErrs,
			field.Invalid(
				field.NewPath("spec", "template", "spec", "strategy", "timeoutSeconds"),
				newM3rt.Spec.Template.Spec.Strategy.TimeoutSeconds,
				fmt.Sprintf("min duration is %d seconds", minTimeout),
			),
		)
	}

	if newM3rt.Spec.Template.Spec.Strategy.Type != infrav1.RebootRemediationStrategy {
		allErrs = append(
			allErrs,
			field.Invalid(
				field.NewPath("spec", "template", "spec", "strategy", "type"),
				newM3rt.Spec.Template.Spec.Strategy.Type,
				"only supported remediation strategy is reboot",
			),
		)
	}

	if newM3rt.Spec.Template.Spec.Strategy.RetryLimit < minRetryLimit {
		allErrs = append(
			allErrs,
			field.Invalid(
				field.NewPath("spec", "template", "spec", "strategy", "retryLimit"),
				newM3rt.Spec.Template.Spec.Strategy.RetryLimit,
				fmt.Sprintf("minimum retry limit is %d", minRetryLimit),
			),
		)
	}

	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(infrav1.GroupVersion.WithKind("Metal3RemediationTemplate").GroupKind(), newM3rt.Name, allErrs)
}

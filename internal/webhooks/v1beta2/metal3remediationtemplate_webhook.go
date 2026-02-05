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
	"time"

	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	defaultDuration = 600 * time.Second
	minDuration     = 100 * time.Second
)

var (
	// Default retry timeout is 600 seconds.
	defaultTimeout = metav1.Duration{Duration: defaultDuration}
	// Minimum time between remediation retries.
	minTimeout = metav1.Duration{Duration: minDuration}
	// Mininum remediation retry limit is 1.
	// Controller will try to remediate unhealhy node at least once.
	minRetryLimit = 1
)

// log is for logging in this package.
var metal3remediationtemplatelog = logf.Log.WithName("metal3remediationtemplate-resource")

func (webhook *Metal3RemediationTemplate) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&infrav1.Metal3RemediationTemplate{}).
		WithValidator(webhook).
		WithDefaulter(webhook).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta2-metal3remediationtemplate,mutating=false,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=metal3remediationtemplates,versions=v1beta2,name=validation.metal3remediationtemplate.infrastructure.cluster.x-k8s.io,matchPolicy=Equivalent,sideEffects=None,admissionReviewVersions=v1;v1beta1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-infrastructure-cluster-x-k8s-io-v1beta2-metal3remediationtemplate,mutating=true,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=metal3remediationtemplates,versions=v1beta2,name=default.metal3remediationtemplate.infrastructure.cluster.x-k8s.io,matchPolicy=Equivalent,sideEffects=None,admissionReviewVersions=v1;v1beta1

// Metal3RemediationTemplate implements a validation and defaulting webhook for Metal3RemediationTemplate.
type Metal3RemediationTemplate struct{}

var _ webhook.CustomDefaulter = &Metal3RemediationTemplate{}
var _ webhook.CustomValidator = &Metal3RemediationTemplate{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (webhook *Metal3RemediationTemplate) Default(_ context.Context, obj runtime.Object) error {
	m3rt, ok := obj.(*infrav1.Metal3RemediationTemplate)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a Metal3RemediationTemplate but got a %T", obj))
	}

	if m3rt.Spec.Template.Spec.Strategy.Type == "" {
		m3rt.Spec.Template.Spec.Strategy.Type = infrav1.RebootRemediationStrategy
	}

	if m3rt.Spec.Template.Spec.Strategy.Timeout == nil {
		m3rt.Spec.Template.Spec.Strategy.Timeout = &defaultTimeout
	}

	if m3rt.Spec.Template.Spec.Strategy.RetryLimit == 0 || m3rt.Spec.Template.Spec.Strategy.RetryLimit < minRetryLimit {
		m3rt.Spec.Template.Spec.Strategy.RetryLimit = minRetryLimit
	}

	return nil
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (webhook *Metal3RemediationTemplate) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	m3rt, ok := obj.(*infrav1.Metal3RemediationTemplate)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a Metal3RemediationTemplate but got a %T", obj))
	}

	metal3remediationtemplatelog.Info("validate create", "name", m3rt.Name)

	return nil, webhook.validate(m3rt)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (webhook *Metal3RemediationTemplate) ValidateUpdate(_ context.Context, _, newObj runtime.Object) (admission.Warnings, error) {
	c, ok := newObj.(*infrav1.Metal3RemediationTemplate)
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

func (webhook *Metal3RemediationTemplate) validate(newM3rt *infrav1.Metal3RemediationTemplate) error {
	var allErrs field.ErrorList
	if newM3rt.Spec.Template.Spec.Strategy.Timeout != nil && newM3rt.Spec.Template.Spec.Strategy.Timeout.Seconds() < minTimeout.Seconds() {
		allErrs = append(
			allErrs,
			field.Invalid(
				field.NewPath("spec", "template", "spec", "strategy", "timeout"),
				newM3rt.Spec.Template.Spec.Strategy.Timeout,
				"min duration is 100s",
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
				"minimun retrylimit is 1",
			),
		)
	}

	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(infrav1.GroupVersion.WithKind("Metal3Remediation").GroupKind(), newM3rt.Name, allErrs)
}

/*
Copyright 2020 The Kubernetes Authors.
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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (webhook *Metal3Machine) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&infrav1.Metal3Machine{}).
		WithDefaulter(webhook, admission.DefaulterRemoveUnknownOrOmitableFields).
		WithValidator(webhook).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta2-metal3machine,mutating=false,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=metal3machines,versions=v1beta2,name=validation.metal3machine.infrastructure.cluster.x-k8s.io,matchPolicy=Equivalent,sideEffects=None,admissionReviewVersions=v1;v1beta1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-infrastructure-cluster-x-k8s-io-v1beta2-metal3machine,mutating=true,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=metal3machines,versions=v1beta2,name=default.metal3machine.infrastructure.cluster.x-k8s.io,matchPolicy=Equivalent,sideEffects=None,admissionReviewVersions=v1;v1beta1

// Metal3Machine implements a validation and defaulting webhook for Metal3Machine.
type Metal3Machine struct{}

var _ webhook.CustomDefaulter = &Metal3Machine{}
var _ webhook.CustomValidator = &Metal3Machine{}

func (webhook *Metal3Machine) Default(_ context.Context, _ runtime.Object) error {
	return nil
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (webhook *Metal3Machine) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	c, ok := obj.(*infrav1.Metal3Machine)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a Metal3Machine but got a %T", obj))
	}

	return nil, webhook.validate(c)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (webhook *Metal3Machine) ValidateUpdate(_ context.Context, _, newObj runtime.Object) (admission.Warnings, error) {
	newM3M, ok := newObj.(*infrav1.Metal3Machine)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a Metal3Machine but got a %T", newObj))
	}

	return nil, webhook.validate(newM3M)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (webhook *Metal3Machine) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (webhook *Metal3Machine) validate(newM3M *infrav1.Metal3Machine) error {
	var allErrs field.ErrorList

	if newM3M.Spec.CustomDeploy == nil || newM3M.Spec.CustomDeploy.Method == "" {
		allErrs = append(allErrs, newM3M.Spec.Image.Validate(*field.NewPath("Spec", "Image"))...)
	}

	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(infrav1.GroupVersion.WithKind("Metal3Machine").GroupKind(), newM3M.Name, allErrs)
}

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
func (webhook *Metal3MachineTemplate) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(webhook).
		WithDefaulter(webhook, admission.DefaulterRemoveUnknownOrOmitableFields).
		WithValidator(webhook).
		Complete()
}

var _ webhook.CustomDefaulter = &Metal3MachineTemplate{}
var _ webhook.CustomValidator = &Metal3MachineTemplate{}

// Deprecated: This method is going to be removed in a next release.
func (webhook *Metal3MachineTemplate) Default(_ context.Context, _ runtime.Object) error {
	return nil
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
// Deprecated: This method is going to be removed in a next release.
func (webhook *Metal3MachineTemplate) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	c, ok := obj.(*Metal3MachineTemplate)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a Metal3MachineTemplate but got a %T", obj))
	}
	return nil, webhook.validate(c)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
// Deprecated: This method is going to be removed in a next release.
func (webhook *Metal3MachineTemplate) ValidateUpdate(_ context.Context, _, newObj runtime.Object) (admission.Warnings, error) {
	c, ok := newObj.(*Metal3MachineTemplate)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a Metal3MachineTemplate but got a %T", newObj))
	}
	return nil, webhook.validate(c)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (webhook *Metal3MachineTemplate) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (webhook *Metal3MachineTemplate) validate(newM3mt *Metal3MachineTemplate) error {
	var allErrs field.ErrorList

	if newM3mt.Spec.Template.Spec.CustomDeploy == nil || newM3mt.Spec.Template.Spec.CustomDeploy.Method == "" {
		allErrs = append(allErrs, newM3mt.Spec.Template.Spec.Image.Validate(*field.NewPath("Spec", "Template", "Spec", "Image"))...)
	}

	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(GroupVersion.WithKind("Metal3MachineTemplate").GroupKind(), newM3mt.Name, allErrs)
}

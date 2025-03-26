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
func (c *Metal3Machine) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(c).
		WithDefaulter(c, admission.DefaulterRemoveUnknownOrOmitableFields).
		WithValidator(c).
		Complete()
}

var _ webhook.CustomDefaulter = &Metal3Machine{}
var _ webhook.CustomValidator = &Metal3Machine{}

// Deprecated: This method is going to be removed in a next release.
func (c *Metal3Machine) Default(_ context.Context, _ runtime.Object) error {
	return nil
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
// Deprecated: This method is going to be removed in a next release.
func (c *Metal3Machine) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	m3m, ok := obj.(*Metal3Machine)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a Metal3Machine but got a %T", obj))
	}

	return nil, c.validate(m3m)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
// Deprecated: This method is going to be removed in a next release.
func (c *Metal3Machine) ValidateUpdate(_ context.Context, _, newObj runtime.Object) (admission.Warnings, error) {
	newM3M, ok := newObj.(*Metal3Machine)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a Metal3Machine but got a %T", newObj))
	}

	return nil, c.validate(newM3M)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
// Deprecated: This method is going to be removed in a next release.
func (c *Metal3Machine) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// Deprecated: This method is going to be removed in a next release.
func (c *Metal3Machine) validate(newM3M *Metal3Machine) error {
	var allErrs field.ErrorList

	if newM3M.Spec.CustomDeploy == nil || newM3M.Spec.CustomDeploy.Method == "" {
		allErrs = append(allErrs, newM3M.Spec.Image.Validate(*field.NewPath("Spec", "Image"))...)
	}

	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(GroupVersion.WithKind("Metal3Machine").GroupKind(), newM3M.Name, allErrs)
}

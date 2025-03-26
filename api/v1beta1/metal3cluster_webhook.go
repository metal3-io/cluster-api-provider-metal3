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
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// Deprecated: This method is going to be removed in a next release.
func (c *Metal3Cluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(c).
		WithDefaulter(c, admission.DefaulterRemoveUnknownOrOmitableFields).
		WithValidator(c).
		Complete()
}

var _ webhook.CustomDefaulter = &Metal3Cluster{}
var _ webhook.CustomValidator = &Metal3Cluster{}

// Deprecated: This method is going to be removed in a next release.
func (c *Metal3Cluster) Default(_ context.Context, obj runtime.Object) error {
	m3c, ok := obj.(*Metal3Cluster)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a Metal3Cluster but got a %T", obj))
	}

	if m3c.Spec.ControlPlaneEndpoint.Port == 0 {
		m3c.Spec.ControlPlaneEndpoint.Port = 6443
	}

	if m3c.Spec.CloudProviderEnabled != nil && m3c.Spec.NoCloudProvider == nil {
		m3c.Spec.NoCloudProvider = ptr.To(!*m3c.Spec.CloudProviderEnabled)
	}
	if m3c.Spec.CloudProviderEnabled == nil && m3c.Spec.NoCloudProvider != nil {
		m3c.Spec.CloudProviderEnabled = ptr.To(!*m3c.Spec.NoCloudProvider)
	}
	if m3c.Spec.CloudProviderEnabled == nil && m3c.Spec.NoCloudProvider == nil {
		m3c.Spec.CloudProviderEnabled = ptr.To(true)
		m3c.Spec.NoCloudProvider = ptr.To(false)
	}
	return nil
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
// Deprecated: This method is going to be removed in a next release.
func (c *Metal3Cluster) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	m3c, ok := obj.(*Metal3Cluster)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a Metal3Cluster but got a %T", obj))
	}

	return nil, c.validate(nil, m3c)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
// Deprecated: This method is going to be removed in a next release.
func (c *Metal3Cluster) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	newM3c, ok := newObj.(*Metal3Cluster)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a Metal3Cluster but got a %T", newObj))
	}

	oldM3c, ok := oldObj.(*Metal3Cluster)
	if !ok || oldM3c == nil {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a Metal3Cluster but got a %T", oldObj))
	}

	return nil, c.validate(oldM3c, newM3c)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
// Deprecated: This method is going to be removed in a next release.
func (c *Metal3Cluster) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (c *Metal3Cluster) validate(oldM3c, newM3c *Metal3Cluster) error {
	var allErrs field.ErrorList
	if newM3c.Spec.ControlPlaneEndpoint.Host == "" {
		allErrs = append(
			allErrs,
			field.Invalid(
				field.NewPath("spec", "controlPlaneEndpoint"),
				newM3c.Spec.ControlPlaneEndpoint.Host,
				"is required",
			),
		)
	}

	if newM3c.Spec.CloudProviderEnabled != nil && newM3c.Spec.NoCloudProvider != nil {
		if *newM3c.Spec.CloudProviderEnabled == *newM3c.Spec.NoCloudProvider {
			allErrs = append(
				allErrs,
				field.Invalid(
					field.NewPath("spec", "cloudProviderEnabled"),
					newM3c.Spec.CloudProviderEnabled,
					"cloudProviderEnabled conflicts the value of noCloudProvider",
				),
			)
		}
	}

	if oldM3c != nil {
		// Validate cloudProviderEnabled
		if newM3c.Spec.CloudProviderEnabled != nil && oldM3c.Spec.NoCloudProvider != nil {
			if *newM3c.Spec.CloudProviderEnabled == *oldM3c.Spec.NoCloudProvider {
				allErrs = append(
					allErrs,
					field.Invalid(
						field.NewPath("spec", "cloudProviderEnabled"),
						newM3c.Spec.CloudProviderEnabled,
						"ValidateUpdate failed, cloudProviderEnabled conflicts the value of noCloudProvider",
					),
				)
			}
		}

		// Validate noCloudProvider
		if newM3c.Spec.NoCloudProvider != nil && oldM3c.Spec.CloudProviderEnabled != nil {
			if *newM3c.Spec.NoCloudProvider == *oldM3c.Spec.CloudProviderEnabled {
				allErrs = append(
					allErrs,
					field.Invalid(
						field.NewPath("spec", "noCloudProvider"),
						newM3c.Spec.NoCloudProvider,
						"ValidateUpdate failed, noCloudProvider conflicts the value of cloudProviderEnabled",
					),
				)
			}
		}
	}

	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(GroupVersion.WithKind("Metal3Cluster").GroupKind(), newM3c.Name, allErrs)
}

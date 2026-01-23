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
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (webhook *Metal3Cluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&infrav1.Metal3Cluster{}).
		WithDefaulter(webhook, admission.DefaulterRemoveUnknownOrOmitableFields).
		WithValidator(webhook).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta2-metal3cluster,mutating=false,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=metal3clusters,versions=v1beta2,name=validation.metal3cluster.infrastructure.cluster.x-k8s.io,matchPolicy=Equivalent,sideEffects=None,admissionReviewVersions=v1;v1beta1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-infrastructure-cluster-x-k8s-io-v1beta2-metal3cluster,mutating=true,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=metal3clusters,versions=v1beta2,name=default.metal3cluster.infrastructure.cluster.x-k8s.io,matchPolicy=Equivalent,sideEffects=None,admissionReviewVersions=v1;v1beta1

// Metal3Cluster implements a validation and defaulting webhook for Metal3Cluster.
type Metal3Cluster struct{}

var _ webhook.CustomDefaulter = &Metal3Cluster{}
var _ webhook.CustomValidator = &Metal3Cluster{}

func (webhook *Metal3Cluster) Default(_ context.Context, obj runtime.Object) error {
	m3c, ok := obj.(*infrav1.Metal3Cluster)
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
func (webhook *Metal3Cluster) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	c, ok := obj.(*infrav1.Metal3Cluster)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a Metal3Cluster but got a %T", obj))
	}

	return nil, webhook.validate(nil, c)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (webhook *Metal3Cluster) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	newM3C, ok := newObj.(*infrav1.Metal3Cluster)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a Metal3Cluster but got a %T", newObj))
	}

	oldM3C, ok := oldObj.(*infrav1.Metal3Cluster)
	if !ok || oldM3C == nil {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a Metal3Cluster but got a %T", oldObj))
	}

	return nil, webhook.validate(oldM3C, newM3C)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (webhook *Metal3Cluster) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (webhook *Metal3Cluster) validate(_ *infrav1.Metal3Cluster, newM3C *infrav1.Metal3Cluster) error {
	var allErrs field.ErrorList
	if newM3C.Spec.ControlPlaneEndpoint.Host == "" {
		allErrs = append(
			allErrs,
			field.Invalid(
				field.NewPath("spec", "controlPlaneEndpoint"),
				newM3C.Spec.ControlPlaneEndpoint.Host,
				"is required",
			),
		)
	}

	if newM3C.Spec.CloudProviderEnabled != nil && newM3C.Spec.NoCloudProvider != nil {
		if *newM3C.Spec.CloudProviderEnabled == *newM3C.Spec.NoCloudProvider {
			allErrs = append(
				allErrs,
				field.Invalid(
					field.NewPath("spec", "cloudProviderEnabled"),
					newM3C.Spec.CloudProviderEnabled,
					"cloudProviderEnabled conflicts the value of noCloudProvider",
				),
			)
		}
	}

	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(infrav1.GroupVersion.WithKind("Metal3Cluster").GroupKind(), newM3C.Name, allErrs)
}

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
	"errors"
	"reflect"
	"strconv"

	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (webhook *Metal3DataTemplate) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &infrav1.Metal3DataTemplate{}).
		WithValidator(webhook).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta2-metal3datatemplate,mutating=false,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=metal3datatemplates,versions=v1beta2,name=validation.metal3datatemplate.infrastructure.cluster.x-k8s.io,matchPolicy=Equivalent,sideEffects=None,admissionReviewVersions=v1;v1beta1

// Metal3DataTemplate implements a validation webhook for Metal3DataTemplate.
type Metal3DataTemplate struct{}

var _ admission.Validator[*infrav1.Metal3DataTemplate] = &Metal3DataTemplate{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (webhook *Metal3DataTemplate) ValidateCreate(_ context.Context, obj *infrav1.Metal3DataTemplate) (admission.Warnings, error) {
	return nil, webhook.validate(nil, obj)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (webhook *Metal3DataTemplate) ValidateUpdate(_ context.Context, oldM3dt, newM3dt *infrav1.Metal3DataTemplate) (admission.Warnings, error) {
	allErrs := field.ErrorList{}

	if newM3dt == nil {
		return nil, apierrors.NewBadRequest("expected a Metal3DataTemplate but got nil")
	}

	if oldM3dt == nil {
		return nil, apierrors.NewInternalError(errors.New("unable to convert existing object"))
	}

	if !reflect.DeepEqual(newM3dt.Spec.MetaData, oldM3dt.Spec.MetaData) {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "MetaData"),
				newM3dt.Spec.MetaData,
				"cannot be modified",
			),
		)
	}

	if !reflect.DeepEqual(newM3dt.Spec.NetworkData, oldM3dt.Spec.NetworkData) {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "NetworkData"),
				newM3dt.Spec.NetworkData,
				"cannot be modified",
			),
		)
	}

	if len(allErrs) == 0 {
		return nil, nil
	}
	return nil, apierrors.NewInvalid(infrav1.GroupVersion.WithKind("Metal3DataTemplate").GroupKind(), newM3dt.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (webhook *Metal3DataTemplate) ValidateDelete(_ context.Context, _ *infrav1.Metal3DataTemplate) (admission.Warnings, error) {
	return nil, nil
}

func (webhook *Metal3DataTemplate) validate(_, newM3dt *infrav1.Metal3DataTemplate) error {
	var allErrs field.ErrorList

	if newM3dt.Spec.MetaData != nil {
		for i, hostInterface := range newM3dt.Spec.MetaData.FromHostInterfaces {
			fromBootMAC := hostInterface.FromBootMAC != nil && *hostInterface.FromBootMAC
			if !fromBootMAC && hostInterface.Interface == "" {
				allErrs = append(allErrs, field.Required(
					field.NewPath("spec", "metaData", "fromHostInterfaces", strconv.Itoa(i), "interface"),
					"interface must be specified when fromBootMAC is false",
				))
			}
			if fromBootMAC && hostInterface.Interface != "" {
				allErrs = append(allErrs, field.Invalid(
					field.NewPath("spec", "metaData", "fromHostInterfaces", strconv.Itoa(i), "interface"),
					hostInterface.Interface,
					"interface must be empty when fromBootMAC is true",
				))
			}
		}
	}

	if newM3dt.Spec.NetworkData != nil {
		// Check if Networks is set before accessing its fields
		if newM3dt.Spec.NetworkData.Networks != nil {
			for i, network := range newM3dt.Spec.NetworkData.Networks.IPv4 {
				if network.FromPoolRef.Name == "" && network.IPAddressFromIPPool == "" && network.FromPoolAnnotation.Object == "" {
					allErrs = append(allErrs, field.Required(
						field.NewPath("spec", "networkData", "networks", "ipv4", strconv.Itoa(i), "fromPoolRef", "name"),
						"fromPoolRef needs to contain a reference to an IPPool",
					))
				}
			}
			for i, network := range newM3dt.Spec.NetworkData.Networks.IPv6 {
				if network.FromPoolRef.Name == "" && network.IPAddressFromIPPool == "" && network.FromPoolAnnotation.Object == "" {
					allErrs = append(allErrs, field.Required(
						field.NewPath("spec", "networkData", "networks", "ipv6", strconv.Itoa(i), "fromPoolRef", "name"),
						"fromPoolRef needs to contain a reference to an IPPool",
					))
				}
			}
		}
	}

	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(infrav1.GroupVersion.WithKind("Metal3DataTemplate").GroupKind(), newM3dt.Name, allErrs)
}

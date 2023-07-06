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
	"reflect"
	"strconv"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (c *Metal3DataTemplate) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(c).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-metal3datatemplate,mutating=false,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=metal3datatemplates,versions=v1beta1,name=validation.metal3datatemplate.infrastructure.cluster.x-k8s.io,matchPolicy=Equivalent,sideEffects=None,admissionReviewVersions=v1;v1beta1,sideEffects=None
// +kubebuilder:webhook:verbs=create;update,path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-metal3datatemplate,mutating=true,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=metal3datatemplates,versions=v1beta1,name=default.metal3datatemplate.infrastructure.cluster.x-k8s.io,matchPolicy=Equivalent,sideEffects=None,admissionReviewVersions=v1;v1beta1

var _ webhook.Defaulter = &Metal3DataTemplate{}
var _ webhook.Validator = &Metal3DataTemplate{}

func (c *Metal3DataTemplate) Default() {}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (c *Metal3DataTemplate) ValidateCreate() (admission.Warnings, error) {
	return nil, c.validate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (c *Metal3DataTemplate) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	allErrs := field.ErrorList{}
	oldM3dt, ok := old.(*Metal3DataTemplate)
	if !ok || oldM3dt == nil {
		return nil, apierrors.NewInternalError(errors.New("unable to convert existing object"))
	}

	if !reflect.DeepEqual(c.Spec.MetaData, oldM3dt.Spec.MetaData) {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "MetaData"),
				c.Spec.MetaData,
				"cannot be modified",
			),
		)
	}

	if !reflect.DeepEqual(c.Spec.NetworkData, oldM3dt.Spec.NetworkData) {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "NetworkData"),
				c.Spec.NetworkData,
				"cannot be modified",
			),
		)
	}

	if len(allErrs) == 0 {
		return nil, nil
	}
	return nil, apierrors.NewInvalid(GroupVersion.WithKind("Metal3Data").GroupKind(), c.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (c *Metal3DataTemplate) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}

func (c *Metal3DataTemplate) validate() error {
	var allErrs field.ErrorList

	if c.Spec.NetworkData != nil {
		for i, network := range c.Spec.NetworkData.Networks.IPv4 {
			if (network.FromPoolRef == nil || network.FromPoolRef.Name == "") && network.IPAddressFromIPPool == "" {
				allErrs = append(allErrs, field.Required(
					field.NewPath("spec", "networkData", "networks", "ipv4", strconv.Itoa(i), "fromPoolRef", "name"),
					"fromPoolRef needs to contain a reference to an IPPool",
				))
			}
		}
		for i, network := range c.Spec.NetworkData.Networks.IPv6 {
			if (network.FromPoolRef == nil || network.FromPoolRef.Name == "") && network.IPAddressFromIPPool == "" {
				allErrs = append(allErrs, field.Required(
					field.NewPath("spec", "networkData", "networks", "ipv6", strconv.Itoa(i), "fromPoolRef", "name"),
					"fromPoolRef needs to contain a reference to an IPPool",
				))
			}
		}
	}

	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(GroupVersion.WithKind("Metal3DataTemplate").GroupKind(), c.Name, allErrs)
}

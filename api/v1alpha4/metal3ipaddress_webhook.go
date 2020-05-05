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

package v1alpha4

import (
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (c *Metal3IPAddress) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(c).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1alpha4-metal3ipaddress,mutating=false,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=metal3ipaddresses,versions=v1alpha4,name=validation.metal3ipaddress.infrastructure.cluster.x-k8s.io,matchPolicy=Equivalent
// +kubebuilder:webhook:verbs=create;update,path=/mutate-infrastructure-cluster-x-k8s-io-v1alpha4-metal3ipaddress,mutating=true,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=metal3ipaddresses,versions=v1alpha4,name=default.metal3ipaddress.infrastructure.cluster.x-k8s.io,matchPolicy=Equivalent

var _ webhook.Defaulter = &Metal3IPAddress{}
var _ webhook.Validator = &Metal3IPAddress{}

func (c *Metal3IPAddress) Default() {
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (c *Metal3IPAddress) ValidateCreate() error {
	allErrs := field.ErrorList{}
	if c.Spec.IPPool == nil {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "ipPool"),
				c.Spec.IPPool,
				"cannot be empty",
			),
		)
	}

	if c.Spec.Address == "" {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "address"),
				c.Spec.Address,
				"cannot be empty",
			),
		)
	}

	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(GroupVersion.WithKind("Metal3IPAddress").GroupKind(), c.Name, allErrs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (c *Metal3IPAddress) ValidateUpdate(old runtime.Object) error {
	allErrs := field.ErrorList{}
	oldMetal3IPAddress, ok := old.(*Metal3IPAddress)
	if !ok || oldMetal3IPAddress == nil {
		return apierrors.NewInternalError(errors.New("unable to convert existing object"))
	}

	if c.Spec.Address != oldMetal3IPAddress.Spec.Address {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "address"),
				c.Spec.Address,
				"cannot be modified",
			),
		)
	}

	if oldMetal3IPAddress.Spec.IPPool != nil {
		if c.Spec.IPPool == nil {
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("spec", "ipPool"),
					c.Spec.IPPool,
					"cannot be modified",
				),
			)
		} else {
			if c.Spec.IPPool.Name != oldMetal3IPAddress.Spec.IPPool.Name {
				allErrs = append(allErrs,
					field.Invalid(
						field.NewPath("spec", "ipPool"),
						c.Spec.IPPool,
						"cannot be modified",
					),
				)
			} else if c.Spec.IPPool.Namespace != oldMetal3IPAddress.Spec.IPPool.Namespace {
				allErrs = append(allErrs,
					field.Invalid(
						field.NewPath("spec", "ipPool"),
						c.Spec.IPPool,
						"cannot be modified",
					),
				)
			} else if c.Spec.IPPool.Kind != oldMetal3IPAddress.Spec.IPPool.Kind {
				allErrs = append(allErrs,
					field.Invalid(
						field.NewPath("spec", "ipPool"),
						c.Spec.IPPool,
						"cannot be modified",
					),
				)
			}
		}
	}

	if oldMetal3IPAddress.Spec.Owner != nil {
		if c.Spec.Owner == nil {
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("spec", "owner"),
					c.Spec.Owner,
					"cannot be modified",
				),
			)
		} else {
			if c.Spec.Owner.Name != oldMetal3IPAddress.Spec.Owner.Name {
				allErrs = append(allErrs,
					field.Invalid(
						field.NewPath("spec", "owner"),
						c.Spec.Owner,
						"cannot be modified",
					),
				)
			} else if c.Spec.Owner.Namespace != oldMetal3IPAddress.Spec.Owner.Namespace {
				allErrs = append(allErrs,
					field.Invalid(
						field.NewPath("spec", "owner"),
						c.Spec.Owner,
						"cannot be modified",
					),
				)
			} else if c.Spec.Owner.Kind != oldMetal3IPAddress.Spec.Owner.Kind {
				allErrs = append(allErrs,
					field.Invalid(
						field.NewPath("spec", "owner"),
						c.Spec.Owner,
						"cannot be modified",
					),
				)
			}
		}
	}

	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(GroupVersion.WithKind("Metal3IPAddress").GroupKind(), c.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (c *Metal3IPAddress) ValidateDelete() error {
	return nil
}

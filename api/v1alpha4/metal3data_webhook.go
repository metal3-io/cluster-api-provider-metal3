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
	"strconv"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (c *Metal3Data) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(c).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1alpha4-metal3data,mutating=false,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=metal3datas,versions=v1alpha4,name=validation.metal3data.infrastructure.cluster.x-k8s.io,matchPolicy=Equivalent
// +kubebuilder:webhook:verbs=create;update,path=/mutate-infrastructure-cluster-x-k8s-io-v1alpha4-metal3data,mutating=true,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=metal3datas,versions=v1alpha4,name=default.metal3data.infrastructure.cluster.x-k8s.io,matchPolicy=Equivalent

var _ webhook.Defaulter = &Metal3Data{}
var _ webhook.Validator = &Metal3Data{}

func (c *Metal3Data) Default() {
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (c *Metal3Data) ValidateCreate() error {
	allErrs := field.ErrorList{}
	if c.Spec.DataTemplate == nil {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "dataTemplate"),
				c.Spec.DataTemplate,
				"cannot be empty",
			),
		)
	} else {
		if c.Name != c.Spec.DataTemplate.Name+"-"+strconv.Itoa(c.Spec.Index) {
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("name"),
					c.Name,
					"should follow the convention <Metal3DataTemplate Name>-<index>",
				),
			)
		}
	}

	if c.Spec.Index < 0 {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "Index"),
				c.Spec.Index,
				"must be positive value",
			),
		)
	}

	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(GroupVersion.WithKind("Metal3Data").GroupKind(), c.Name, allErrs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (c *Metal3Data) ValidateUpdate(old runtime.Object) error {
	allErrs := field.ErrorList{}
	oldMetal3Data, ok := old.(*Metal3Data)
	if !ok || oldMetal3Data == nil {
		return apierrors.NewInternalError(errors.New("unable to convert existing object"))
	}

	if c.Spec.Index != oldMetal3Data.Spec.Index {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "Index"),
				c.Spec.Index,
				"cannot be modified",
			),
		)
	}

	if oldMetal3Data.Spec.DataTemplate != nil {
		if c.Spec.DataTemplate == nil {
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("spec", "DataTemplate"),
					c.Spec.DataTemplate,
					"cannot be modified",
				),
			)
		} else {
			if c.Spec.DataTemplate.Name != oldMetal3Data.Spec.DataTemplate.Name {
				allErrs = append(allErrs,
					field.Invalid(
						field.NewPath("spec", "DataTemplate"),
						c.Spec.DataTemplate,
						"cannot be modified",
					),
				)
			} else if c.Spec.DataTemplate.Namespace != oldMetal3Data.Spec.DataTemplate.Namespace {
				allErrs = append(allErrs,
					field.Invalid(
						field.NewPath("spec", "DataTemplate"),
						c.Spec.DataTemplate,
						"cannot be modified",
					),
				)
			} else if c.Spec.DataTemplate.Kind != oldMetal3Data.Spec.DataTemplate.Kind {
				allErrs = append(allErrs,
					field.Invalid(
						field.NewPath("spec", "DataTemplate"),
						c.Spec.DataTemplate,
						"cannot be modified",
					),
				)
			}
		}
	}

	if oldMetal3Data.Spec.Metal3Machine != nil {
		if c.Spec.Metal3Machine == nil {
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("spec", "Metal3Machine"),
					c.Spec.Metal3Machine,
					"cannot be modified",
				),
			)
		} else {
			if c.Spec.Metal3Machine.Name != oldMetal3Data.Spec.Metal3Machine.Name {
				allErrs = append(allErrs,
					field.Invalid(
						field.NewPath("spec", "Metal3Machine"),
						c.Spec.Metal3Machine,
						"cannot be modified",
					),
				)
			} else if c.Spec.Metal3Machine.Namespace != oldMetal3Data.Spec.Metal3Machine.Namespace {
				allErrs = append(allErrs,
					field.Invalid(
						field.NewPath("spec", "Metal3Machine"),
						c.Spec.Metal3Machine,
						"cannot be modified",
					),
				)
			} else if c.Spec.Metal3Machine.Kind != oldMetal3Data.Spec.Metal3Machine.Kind {
				allErrs = append(allErrs,
					field.Invalid(
						field.NewPath("spec", "Metal3Machine"),
						c.Spec.Metal3Machine,
						"cannot be modified",
					),
				)
			}
		}
	}

	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(GroupVersion.WithKind("Metal3Data").GroupKind(), c.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (c *Metal3Data) ValidateDelete() error {
	return nil
}

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

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-metal3data,mutating=false,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=metal3datas,versions=v1beta1,name=validation.metal3data.infrastructure.cluster.x-k8s.io,matchPolicy=Equivalent,sideEffects=None,admissionReviewVersions=v1;v1beta1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-metal3data,mutating=true,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=metal3datas,versions=v1beta1,name=default.metal3data.infrastructure.cluster.x-k8s.io,matchPolicy=Equivalent,sideEffects=None,admissionReviewVersions=v1;v1beta1

var _ webhook.Defaulter = &Metal3Data{}
var _ webhook.Validator = &Metal3Data{}

func (c *Metal3Data) Default() {
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (c *Metal3Data) ValidateCreate() error {
	allErrs := field.ErrorList{}
	if (c.Spec.TemplateReference != "" && c.Name != c.Spec.TemplateReference+"-"+strconv.Itoa(c.Spec.Index)) ||
		(c.Spec.TemplateReference == "" && c.Name != c.Spec.Template.Name+"-"+strconv.Itoa(c.Spec.Index)) {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("name"),
				c.Name,
				"should follow the convention <Metal3Template Name>-<index>",
			),
		)
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

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
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

	if c.Spec.Template.Name != oldMetal3Data.Spec.Template.Name {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "Template"),
				c.Spec.Template,
				"cannot be modified",
			),
		)
	} else if c.Spec.Template.Namespace != oldMetal3Data.Spec.Template.Namespace {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "Template"),
				c.Spec.Template,
				"cannot be modified",
			),
		)
	} else if c.Spec.Template.Kind != oldMetal3Data.Spec.Template.Kind {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "Template"),
				c.Spec.Template,
				"cannot be modified",
			),
		)
	}

	if c.Spec.Claim.Name != oldMetal3Data.Spec.Claim.Name {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "claim"),
				c.Spec.Claim,
				"cannot be modified",
			),
		)
	} else if c.Spec.Claim.Namespace != oldMetal3Data.Spec.Claim.Namespace {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "claim"),
				c.Spec.Claim,
				"cannot be modified",
			),
		)
	} else if c.Spec.Claim.Kind != oldMetal3Data.Spec.Claim.Kind {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "claim"),
				c.Spec.Claim,
				"cannot be modified",
			),
		)
	}

	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(GroupVersion.WithKind("Metal3Data").GroupKind(), c.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (c *Metal3Data) ValidateDelete() error {
	return nil
}

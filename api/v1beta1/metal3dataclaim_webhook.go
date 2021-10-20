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
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (c *Metal3DataClaim) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(c).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-metal3dataclaim,mutating=false,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=metal3dataclaims,versions=v1beta1,name=validation.metal3dataclaim.infrastructure.cluster.x-k8s.io,matchPolicy=Equivalent,sideEffects=None,admissionReviewVersions=v1;v1beta1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-metal3dataclaim,mutating=true,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=metal3dataclaims,versions=v1beta1,name=default.metal3dataclaim.infrastructure.cluster.x-k8s.io,matchPolicy=Equivalent,sideEffects=None,admissionReviewVersions=v1;v1beta1

var _ webhook.Defaulter = &Metal3DataClaim{}
var _ webhook.Validator = &Metal3DataClaim{}

func (c *Metal3DataClaim) Default() {
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (c *Metal3DataClaim) ValidateCreate() error {
	allErrs := field.ErrorList{}
	if c.Spec.Template.Name == "" {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "template", "name"),
				c.Spec.Template.Name,
				"must be set",
			),
		)
	}

	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(GroupVersion.WithKind("Metal3DataClaim").GroupKind(), c.Name, allErrs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (c *Metal3DataClaim) ValidateUpdate(old runtime.Object) error {
	allErrs := field.ErrorList{}
	oldMetal3DataClaim, ok := old.(*Metal3DataClaim)
	if !ok || oldMetal3DataClaim == nil {
		return apierrors.NewInternalError(errors.New("unable to convert existing object"))
	}

	if c.Spec.Template.Name != oldMetal3DataClaim.Spec.Template.Name {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "template"),
				c.Spec.Template,
				"cannot be modified",
			),
		)
	} else if c.Spec.Template.Namespace != oldMetal3DataClaim.Spec.Template.Namespace {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "template"),
				c.Spec.Template,
				"cannot be modified",
			),
		)
	} else if c.Spec.Template.Kind != oldMetal3DataClaim.Spec.Template.Kind {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "template"),
				c.Spec.Template,
				"cannot be modified",
			),
		)
	}

	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(GroupVersion.WithKind("Metal3DataClaim").GroupKind(), c.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (c *Metal3DataClaim) ValidateDelete() error {
	return nil
}

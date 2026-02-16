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
	"fmt"
	"strconv"

	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (webhook *Metal3Data) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&infrav1.Metal3Data{}).
		WithValidator(webhook).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta2-metal3data,mutating=false,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=metal3datas,versions=v1beta2,name=validation.metal3data.infrastructure.cluster.x-k8s.io,matchPolicy=Equivalent,sideEffects=None,admissionReviewVersions=v1;v1beta1

// Metal3Data implements a validation webhook for Metal3Data.
type Metal3Data struct{}

var _ webhook.CustomValidator = &Metal3Data{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (webhook *Metal3Data) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	c, ok := obj.(*infrav1.Metal3Data)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a Metal3Data but got a %T", obj))
	}

	allErrs := field.ErrorList{}
	if c.Name != c.Spec.Template.Name+"-"+strconv.Itoa(c.Spec.Index) {
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
		return nil, nil
	}
	return nil, apierrors.NewInvalid(infrav1.GroupVersion.WithKind("Metal3Data").GroupKind(), c.Name, allErrs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (webhook *Metal3Data) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	allErrs := field.ErrorList{}

	newMetal3Data, ok := newObj.(*infrav1.Metal3Data)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a Metal3Data but got a %T", newObj))
	}

	oldMetal3Data, ok := oldObj.(*infrav1.Metal3Data)
	if !ok || oldMetal3Data == nil {
		return nil, apierrors.NewInternalError(errors.New("unable to convert existing object"))
	}

	if newMetal3Data.Spec.Index != oldMetal3Data.Spec.Index {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "Index"),
				newMetal3Data.Spec.Index,
				"cannot be modified",
			),
		)
	}

	if newMetal3Data.Spec.Template.Name != oldMetal3Data.Spec.Template.Name {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "Template"),
				newMetal3Data.Spec.Template,
				"cannot be modified",
			),
		)
	} else if newMetal3Data.Spec.Template.Namespace != oldMetal3Data.Spec.Template.Namespace {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "Template"),
				newMetal3Data.Spec.Template,
				"cannot be modified",
			),
		)
	} else if newMetal3Data.Spec.Template.Kind != oldMetal3Data.Spec.Template.Kind {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "Template"),
				newMetal3Data.Spec.Template,
				"cannot be modified",
			),
		)
	}

	if newMetal3Data.Spec.Claim.Name != oldMetal3Data.Spec.Claim.Name {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "claim"),
				newMetal3Data.Spec.Claim,
				"cannot be modified",
			),
		)
	} else if newMetal3Data.Spec.Claim.Namespace != oldMetal3Data.Spec.Claim.Namespace {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "claim"),
				newMetal3Data.Spec.Claim,
				"cannot be modified",
			),
		)
	} else if newMetal3Data.Spec.Claim.Kind != oldMetal3Data.Spec.Claim.Kind {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "claim"),
				newMetal3Data.Spec.Claim,
				"cannot be modified",
			),
		)
	}

	if len(allErrs) == 0 {
		return nil, nil
	}
	return nil, apierrors.NewInvalid(infrav1.GroupVersion.WithKind("Metal3Data").GroupKind(), newMetal3Data.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (webhook *Metal3Data) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

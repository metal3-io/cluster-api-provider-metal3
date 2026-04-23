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
	"strconv"

	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (webhook *Metal3Data) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &infrav1.Metal3Data{}).
		WithValidator(webhook).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta2-metal3data,mutating=false,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=metal3datas,versions=v1beta2,name=validation.metal3data.infrastructure.cluster.x-k8s.io,matchPolicy=Equivalent,sideEffects=None,admissionReviewVersions=v1;v1beta1

// Metal3Data implements a validation webhook for Metal3Data.
type Metal3Data struct{}

var _ admission.Validator[*infrav1.Metal3Data] = &Metal3Data{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (webhook *Metal3Data) ValidateCreate(_ context.Context, obj *infrav1.Metal3Data) (admission.Warnings, error) {
	allErrs := field.ErrorList{}
	if (obj.Spec.Index != nil && obj.Spec.Template != nil) && obj.Name != obj.Spec.Template.Name+"-"+strconv.Itoa(int(*obj.Spec.Index)) {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("name"),
				obj.Name,
				"should follow the convention <Metal3Template Name>-<index>",
			),
		)
	}

	if obj.Spec.Index != nil && *obj.Spec.Index < int32(0) {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "Index"),
				obj.Spec.Index,
				"must be positive value",
			),
		)
	}

	if len(allErrs) == 0 {
		return nil, nil
	}
	return nil, apierrors.NewInvalid(infrav1.GroupVersion.WithKind("Metal3Data").GroupKind(), obj.Name, allErrs)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (webhook *Metal3Data) ValidateUpdate(_ context.Context, oldMetal3Data, newMetal3Data *infrav1.Metal3Data) (admission.Warnings, error) {
	allErrs := field.ErrorList{}

	if oldMetal3Data == nil {
		return nil, apierrors.NewInternalError(errors.New("expected a Metal3Data but got nil"))
	}

	if newMetal3Data == nil {
		return nil, apierrors.NewInternalError(errors.New("expected a Metal3Data but got nil"))
	}

	if ptr.Deref(newMetal3Data.Spec.Index, 0) != ptr.Deref(oldMetal3Data.Spec.Index, 0) {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "Index"),
				newMetal3Data.Spec.Index,
				"cannot be modified",
			),
		)
	}

	// Helper function to check if Metal3ObjectRef fields match
	checkObjectRefChanged := func(fieldName string, newRef, oldRef *infrav1.Metal3ObjectRef) field.ErrorList {
		var errs field.ErrorList
		if newRef != nil || oldRef != nil {
			var newName, oldName string
			if newRef != nil {
				newName = newRef.Name
			}
			if oldRef != nil {
				oldName = oldRef.Name
			}
			if newName != oldName {
				errs = append(errs,
					field.Invalid(
						field.NewPath("spec", fieldName),
						newRef,
						"cannot be modified",
					),
				)
			} else {
				var newNamespace, oldNamespace string
				if newRef != nil {
					newNamespace = newRef.Namespace
				}
				if oldRef != nil {
					oldNamespace = oldRef.Namespace
				}
				if newNamespace != oldNamespace {
					errs = append(errs,
						field.Invalid(
							field.NewPath("spec", fieldName),
							newRef,
							"cannot be modified",
						),
					)
				}
			}
		}
		return errs
	}

	allErrs = append(allErrs, checkObjectRefChanged("Template", newMetal3Data.Spec.Template, oldMetal3Data.Spec.Template)...)
	allErrs = append(allErrs, checkObjectRefChanged("claim", newMetal3Data.Spec.Claim, oldMetal3Data.Spec.Claim)...)

	if len(allErrs) == 0 {
		return nil, nil
	}
	return nil, apierrors.NewInvalid(infrav1.GroupVersion.WithKind("Metal3Data").GroupKind(), newMetal3Data.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (webhook *Metal3Data) ValidateDelete(_ context.Context, _ *infrav1.Metal3Data) (admission.Warnings, error) {
	return nil, nil
}

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

	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (webhook *Metal3Machine) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &infrav1.Metal3Machine{}).
		WithValidator(webhook).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta2-metal3machine,mutating=false,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=metal3machines,versions=v1beta2,name=validation.metal3machine.infrastructure.cluster.x-k8s.io,matchPolicy=Equivalent,sideEffects=None,admissionReviewVersions=v1;v1beta1

// Metal3Machine implements a validation webhook for Metal3Machine.
type Metal3Machine struct{}

var _ admission.Validator[*infrav1.Metal3Machine] = &Metal3Machine{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (webhook *Metal3Machine) ValidateCreate(_ context.Context, obj *infrav1.Metal3Machine) (admission.Warnings, error) {
	return nil, webhook.validate(obj)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (webhook *Metal3Machine) ValidateUpdate(_ context.Context, _, newObj *infrav1.Metal3Machine) (admission.Warnings, error) {
	return nil, webhook.validate(newObj)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (webhook *Metal3Machine) ValidateDelete(_ context.Context, _ *infrav1.Metal3Machine) (admission.Warnings, error) {
	return nil, nil
}

func (webhook *Metal3Machine) validate(newM3M *infrav1.Metal3Machine) error {
	var allErrs field.ErrorList

	if newM3M.Spec.CustomDeploy.Method == "" {
		allErrs = append(allErrs, newM3M.Spec.Image.Validate(*field.NewPath("Spec", "Image"))...)
	}

	allErrs = append(allErrs, validateSecretNamespace(newM3M.Spec.UserData, newM3M.Namespace, field.NewPath("spec", "userData"))...)
	allErrs = append(allErrs, validateSecretNamespace(newM3M.Spec.MetaData, newM3M.Namespace, field.NewPath("spec", "metaData"))...)
	allErrs = append(allErrs, validateSecretNamespace(newM3M.Spec.NetworkData, newM3M.Namespace, field.NewPath("spec", "networkData"))...)

	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(infrav1.GroupVersion.WithKind("Metal3Machine").GroupKind(), newM3M.Name, allErrs)
}

// validateSecretNamespace rejects secret references that point to a namespace
// other than the Metal3Machine's own namespace, preventing cross-namespace
// secret disclosure.
func validateSecretNamespace(ref *corev1.SecretReference, m3mNamespace string, fldPath *field.Path) field.ErrorList {
	var errs field.ErrorList
	if ref != nil && ref.Namespace != "" && ref.Namespace != m3mNamespace {
		errs = append(errs, field.Forbidden(
			fldPath.Child("namespace"),
			"cross-namespace secret references are not allowed",
		))
	}
	return errs
}

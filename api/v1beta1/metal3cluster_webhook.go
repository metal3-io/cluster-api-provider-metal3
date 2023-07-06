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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (c *Metal3Cluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(c).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-metal3cluster,mutating=false,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=metal3clusters,versions=v1beta1,name=validation.metal3cluster.infrastructure.cluster.x-k8s.io,matchPolicy=Equivalent,sideEffects=None,admissionReviewVersions=v1;v1beta1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-metal3cluster,mutating=true,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=metal3clusters,versions=v1beta1,name=default.metal3cluster.infrastructure.cluster.x-k8s.io,matchPolicy=Equivalent,sideEffects=None,admissionReviewVersions=v1;v1beta1

var _ webhook.Defaulter = &Metal3Cluster{}
var _ webhook.Validator = &Metal3Cluster{}

func (c *Metal3Cluster) Default() {
	if c.Spec.ControlPlaneEndpoint.Port == 0 {
		c.Spec.ControlPlaneEndpoint.Port = 6443
	}
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (c *Metal3Cluster) ValidateCreate() (admission.Warnings, error) {
	return nil, c.validate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (c *Metal3Cluster) ValidateUpdate(_ runtime.Object) (admission.Warnings, error) {
	return nil, c.validate()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (c *Metal3Cluster) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}

func (c *Metal3Cluster) validate() error {
	var allErrs field.ErrorList
	if c.Spec.ControlPlaneEndpoint.Host == "" {
		allErrs = append(
			allErrs,
			field.Invalid(
				field.NewPath("spec", "controlPlaneEndpoint"),
				c.Spec.ControlPlaneEndpoint.Host,
				"is required",
			),
		)
	}

	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(GroupVersion.WithKind("Metal3Cluster").GroupKind(), c.Name, allErrs)
}

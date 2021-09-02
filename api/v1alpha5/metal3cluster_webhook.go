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

package v1alpha5

import (
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (c *Metal3Cluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(c).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1alpha5-metal3cluster,mutating=false,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=metal3clusters,versions=v1alpha5,name=validation.metal3cluster.infrastructure.cluster.x-k8s.io,matchPolicy=Equivalent,sideEffects=None,admissionReviewVersions=v1;v1beta1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-infrastructure-cluster-x-k8s-io-v1alpha5-metal3cluster,mutating=true,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=metal3clusters,versions=v1alpha5,name=default.metal3cluster.infrastructure.cluster.x-k8s.io,matchPolicy=Equivalent,sideEffects=None,admissionReviewVersions=v1;v1beta1

var _ webhook.Defaulter = &Metal3Cluster{}
var _ webhook.Validator = &Metal3Cluster{}

func (c *Metal3Cluster) Default() {
	if c.Spec.ControlPlaneEndpoint.Port == 0 {
		c.Spec.ControlPlaneEndpoint.Port = 6443
	}
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (c *Metal3Cluster) ValidateCreate() error {
	return c.validate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (c *Metal3Cluster) ValidateUpdate(old runtime.Object) error {
	return c.validate()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (c *Metal3Cluster) ValidateDelete() error {
	return nil
}

func (c *Metal3Cluster) validate() error {
	var allErrs field.ErrorList
	if len(c.Spec.ControlPlaneEndpoint.Host) == 0 {
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

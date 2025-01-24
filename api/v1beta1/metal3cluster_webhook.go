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
	"k8s.io/utils/ptr"
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

	if c.Spec.CloudProviderEnabled != nil && c.Spec.NoCloudProvider == nil {
		c.Spec.NoCloudProvider = ptr.To(!*c.Spec.CloudProviderEnabled)
	}
	if c.Spec.CloudProviderEnabled == nil && c.Spec.NoCloudProvider != nil {
		c.Spec.CloudProviderEnabled = ptr.To(!*c.Spec.NoCloudProvider)
	}
	if c.Spec.CloudProviderEnabled == nil && c.Spec.NoCloudProvider == nil {
		c.Spec.CloudProviderEnabled = ptr.To(true)
		c.Spec.NoCloudProvider = ptr.To(false)
	}
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (c *Metal3Cluster) ValidateCreate() (admission.Warnings, error) {
	return nil, c.validate(nil)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (c *Metal3Cluster) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	oldM3C, ok := old.(*Metal3Cluster)
	if !ok || oldM3C == nil {
		return nil, apierrors.NewInternalError(errors.New("unable to convert existing object"))
	}

	return nil, c.validate(oldM3C)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (c *Metal3Cluster) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}

func (c *Metal3Cluster) validate(oldM3C *Metal3Cluster) error {
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

	if c.Spec.CloudProviderEnabled != nil && c.Spec.NoCloudProvider != nil {
		if *c.Spec.CloudProviderEnabled == *c.Spec.NoCloudProvider {
			allErrs = append(
				allErrs,
				field.Invalid(
					field.NewPath("spec", "cloudProviderEnabled"),
					c.Spec.CloudProviderEnabled,
					"cloudProviderEnabled conflicts the value of noCloudProvider",
				),
			)
		}
	}

	if oldM3C != nil {
		// Validate cloudProviderEnabled
		if c.Spec.CloudProviderEnabled != nil && oldM3C.Spec.NoCloudProvider != nil {
			if *c.Spec.CloudProviderEnabled == *oldM3C.Spec.NoCloudProvider {
				allErrs = append(
					allErrs,
					field.Invalid(
						field.NewPath("spec", "cloudProviderEnabled"),
						c.Spec.CloudProviderEnabled,
						"ValidateUpdate failed, cloudProviderEnabled conflicts the value of noCloudProvider",
					),
				)
			}
		}

		// Validate noCloudProvider
		if c.Spec.NoCloudProvider != nil && oldM3C.Spec.CloudProviderEnabled != nil {
			if *c.Spec.NoCloudProvider == *oldM3C.Spec.CloudProviderEnabled {
				allErrs = append(
					allErrs,
					field.Invalid(
						field.NewPath("spec", "noCloudProvider"),
						c.Spec.NoCloudProvider,
						"ValidateUpdate failed, noCloudProvider conflicts the value of cloudProviderEnabled",
					),
				)
			}
		}
	}

	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(GroupVersion.WithKind("Metal3Cluster").GroupKind(), c.Name, allErrs)
}

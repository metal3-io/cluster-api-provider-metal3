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

package v1alpha3

import (
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (c *BareMetalCluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(c).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1alpha3-baremetalcluster,mutating=false,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=baremetalclusters,versions=v1alpha3,name=validation.baremetalcluster.infrastructure.cluster.x-k8s.io
// +kubebuilder:webhook:verbs=create;update,path=/mutate-infrastructure-cluster-x-k8s-io-v1alpha3-baremetalcluster,mutating=true,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=baremetalclusters,versions=v1alpha3,name=default.baremetalcluster.infrastructure.cluster.x-k8s.io

var _ webhook.Defaulter = &BareMetalCluster{}
var _ webhook.Validator = &BareMetalCluster{}

func (c *BareMetalCluster) Default() {
	if c.Spec.ControlPlaneEndpoint.Port == 0 {
		c.Spec.ControlPlaneEndpoint.Port = 6443
	}
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (c *BareMetalCluster) ValidateCreate() error {
	return c.validate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (c *BareMetalCluster) ValidateUpdate(old runtime.Object) error {
	return c.validate()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (c *BareMetalCluster) ValidateDelete() error {
	return nil
}

func (c *BareMetalCluster) validate() error {
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
	return apierrors.NewInvalid(GroupVersion.WithKind("BareMetalCluster").GroupKind(), c.Name, allErrs)
}

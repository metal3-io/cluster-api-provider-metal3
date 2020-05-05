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
	"reflect"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

func (c *Metal3IPPool) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(c).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1alpha4-metal3ippool,mutating=false,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=metal3ippools,versions=v1alpha4,name=validation.metal3ippool.infrastructure.cluster.x-k8s.io,matchPolicy=Equivalent
// +kubebuilder:webhook:verbs=create;update,path=/mutate-infrastructure-cluster-x-k8s-io-v1alpha4-metal3ippool,mutating=true,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=metal3ippools,versions=v1alpha4,name=default.metal3ippool.infrastructure.cluster.x-k8s.io,matchPolicy=Equivalent

var _ webhook.Defaulter = &Metal3IPPool{}
var _ webhook.Validator = &Metal3IPPool{}

func (c *Metal3IPPool) Default() {
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (c *Metal3IPPool) ValidateCreate() error {
	return c.validate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (c *Metal3IPPool) ValidateUpdate(old runtime.Object) error {
	allErrs := field.ErrorList{}
	oldM3ipp, ok := old.(*Metal3IPPool)
	if !ok || oldM3ipp == nil {
		return apierrors.NewInternalError(errors.New("unable to convert existing object"))
	}

	if !reflect.DeepEqual(c.Spec.NamePrefix, oldM3ipp.Spec.NamePrefix) {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("spec", "NamePrefix"),
				c.Spec.NamePrefix,
				"cannot be modified",
			),
		)
	}

	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(GroupVersion.WithKind("Metal3Data").GroupKind(), c.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (c *Metal3IPPool) ValidateDelete() error {
	return nil
}

//No further validation for now
func (c *Metal3IPPool) validate() error {
	var allErrs field.ErrorList

	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(GroupVersion.WithKind("Metal3IPPool").GroupKind(), c.Name, allErrs)
}

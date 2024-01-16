/*
Copyright 2024 The Kubernetes Authors.
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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// SetupWebhookWithManager sets up and registers the webhook with the manager.
func (c *Metal3ClusterTemplate) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(c).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-metal3clustertemplate,mutating=false,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=metal3clustertemplates,versions=v1beta1,name=validation.metal3clustertemplate.infrastructure.cluster.x-k8s.io,matchPolicy=Equivalent,sideEffects=None,admissionReviewVersions=v1;v1beta1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-metal3clustertemplate,mutating=true,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=metal3clustertemplates,versions=v1beta1,name=default.metal3clustertemplate.infrastructure.cluster.x-k8s.io,matchPolicy=Equivalent,sideEffects=None,admissionReviewVersions=v1;v1beta1

var _ webhook.Defaulter = &Metal3ClusterTemplate{}
var _ webhook.Validator = &Metal3ClusterTemplate{}

func (c *Metal3ClusterTemplate) Default() {
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (c *Metal3ClusterTemplate) ValidateCreate() (admission.Warnings, error) {
	return nil, c.validate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (c *Metal3ClusterTemplate) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	oldM3ct, ok := old.(*Metal3ClusterTemplate)
	if !ok || oldM3ct == nil {
		return nil, apierrors.NewInternalError(errors.New("unable to convert existing object"))
	}

	if err := oldM3ct.Spec.Template.Spec.IsValid(); err != nil {
		return nil, err
	}

	return nil, c.validate()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (c *Metal3ClusterTemplate) ValidateDelete() (admission.Warnings, error) {
	return nil, nil
}

func (c *Metal3ClusterTemplate) validate() error {
	return c.Spec.Template.Spec.IsValid()
}

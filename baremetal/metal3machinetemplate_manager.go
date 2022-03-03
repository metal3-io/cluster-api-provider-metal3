/*
Copyright 2019 The Kubernetes Authors.
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

package baremetal

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	"github.com/pkg/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	clonedFromName      = clusterv1.TemplateClonedFromNameAnnotation
	clonedFromGroupKind = clusterv1.TemplateClonedFromGroupKindAnnotation
)

// TemplateManagerInterface is an interface for a TemplateManager.
type TemplateManagerInterface interface {
	UpdateAutomatedCleaningMode(context.Context) error
}

// MachineTemplateManager is responsible for performing metal3MachineTemplate reconciliation.
type MachineTemplateManager struct {
	client client.Client

	Metal3MachineList     *capm3.Metal3MachineList
	Metal3MachineTemplate *capm3.Metal3MachineTemplate
	Log                   logr.Logger
}

// NewMachineTemplateManager returns a new helper for managing a metal3MachineTemplate.
func NewMachineTemplateManager(client client.Client,
	metal3MachineTemplate *capm3.Metal3MachineTemplate,

	metal3MachineList *capm3.Metal3MachineList,
	metal3MachineTemplateLog logr.Logger) (*MachineTemplateManager, error) {
	return &MachineTemplateManager{
		client: client,

		Metal3MachineTemplate: metal3MachineTemplate,
		Metal3MachineList:     metal3MachineList,
		Log:                   metal3MachineTemplateLog,
	}, nil
}

// UpdateAutomatedCleaningMode synchronizes automatedCleaningMode field value between metal3MachineTemplate
// and all the metal3Machines cloned from this metal3MachineTemplate.
func (m *MachineTemplateManager) UpdateAutomatedCleaningMode(ctx context.Context) error {
	m.Log.Info("Fetching metal3Machine objects")

	// get list of metal3Machine objects
	m3ms := &capm3.Metal3MachineList{}
	// without this ListOption, all namespaces would be included in the listing
	opts := &client.ListOptions{
		Namespace: m.Metal3MachineTemplate.Namespace,
	}

	if err := m.client.List(ctx, m3ms, opts); err != nil {
		return errors.Wrap(err, "failed to list metal3Machines")
	}

	matchedM3Machines := []*capm3.Metal3Machine{}

	// Fetch metal3Machines belonging to the same metal3MachineTemplate
	for i := range m3ms.Items {
		m3m := &m3ms.Items[i]

		if m3m.Annotations[clonedFromName] == m.Metal3MachineTemplate.Name && m3m.Annotations[clonedFromGroupKind] == m.Metal3MachineTemplate.GroupVersionKind().GroupKind().String() {
			matchedM3Machines = append(matchedM3Machines, m3m)
		}
	}

	if len(matchedM3Machines) > 0 {
		for _, m3m := range matchedM3Machines {
			// don't synchronize AutomatedCleaningMode between metal3MachineTemplate
			// and metal3Machine if unset in metal3MachineTemplate.
			if m.Metal3MachineTemplate.Spec.Template.Spec.AutomatedCleaningMode != nil {
				m3m.Spec.AutomatedCleaningMode = m.Metal3MachineTemplate.Spec.Template.Spec.AutomatedCleaningMode

				if err := m.client.Update(ctx, m3m); err != nil {
					return errors.Wrapf(err, "failed to update metal3Machine: %s", m3m.Name)
				}

				m.Log.Info("Synchronized automatedCleaningMode between ", "Metal3MachineTemplate", fmt.Sprintf("%v/%v", m.Metal3MachineTemplate.Namespace, m.Metal3MachineTemplate.Name), "Metal3Machine", fmt.Sprintf("%v/%v", m3m.Namespace, m3m.Name))
			}
		}
	}
	return nil
}

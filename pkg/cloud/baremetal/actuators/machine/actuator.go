/*
Copyright 2019 The Kubernetes authors.

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

package machine

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	bmh "github.com/metalkube/baremetal-operator/pkg/apis/metalkube/v1alpha1"
	machinev1 "github.com/openshift/cluster-api/pkg/apis/machine/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ProviderName = "solas"
	// HostAnnotation is the key for an annotation that should go on a Machine to
	// reference what BareMetalHost it corresponds to.
	HostAnnotation = "metalkube.org/BareMetalHost"
	// MachineLabel is the key for a label that should go on a BareMetalHost that
	// references the Machine it is associated with.
	MachineLabel = "machine.openshift.io/Machine"
)

// Add RBAC rules to access cluster-api resources
//+kubebuilder:rbac:groups=cluster.k8s.io,resources=machines;machines/status;machinedeployments;machinedeployments/status;machinesets;machinesets/status;machineclasses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cluster.k8s.io,resources=clusters;clusters/status,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=nodes;events,verbs=get;list;watch;create;update;patch;delete

// Actuator is responsible for performing machine reconciliation
type Actuator struct {
	client client.Client
}

// ActuatorParams holds parameter information for Actuator
type ActuatorParams struct {
	Client client.Client
}

// NewActuator creates a new Actuator
func NewActuator(params ActuatorParams) (*Actuator, error) {
	return &Actuator{
		client: params.Client,
	}, nil
}

// Create creates a machine and is invoked by the Machine Controller
func (a *Actuator) Create(ctx context.Context, cluster *machinev1.Cluster, machine *machinev1.Machine) error {
	log.Printf("Creating machine %v .", machine.Name)
	// look for associated BMH by searching for a label that matches this machine
	host, err := a.getHost(ctx, machine)
	if err != nil {
		return err
	}

	// none found, so try to choose one
	if host == nil {
		host, err = a.chooseHost(ctx, machine)
		if err != nil {
			return err
		}
		log.Printf("Associating machine %s with host %s", machine.Name, host.Name)
	} else {
		log.Printf("Machine %s already associated with host %s", machine.Name, host.Name)
	}

	_, err = ensureLabelOnHost(ctx, machine, host)
	if err != nil {
		return err
	}
	err = a.client.Update(ctx, host)
	if err != nil {
		return err
	}

	err = a.ensureAnnotation(ctx, machine, host)
	if err != nil {
		return err
	}

	log.Printf("Finished creating machine %v .", machine.Name)
	return nil
}

// Delete deletes a machine and is invoked by the Machine Controller
func (a *Actuator) Delete(ctx context.Context, cluster *machinev1.Cluster, machine *machinev1.Machine) error {
	log.Printf("Deleting machine %v .", machine.Name)
	return fmt.Errorf("TODO: Not yet implemented")
}

// Update updates a machine and is invoked by the Machine Controller
func (a *Actuator) Update(ctx context.Context, cluster *machinev1.Cluster, machine *machinev1.Machine) error {
	log.Printf("Updating machine %v .", machine.Name)
	host, err := a.getHost(ctx, machine)
	if err != nil {
		return err
	}
	if host == nil {
		return fmt.Errorf("host not found for machine %s", machine.Name)
	}

	changed, err := ensureLabelOnHost(ctx, machine, host)
	if err != nil {
		return err
	}
	if changed {
		err = a.client.Update(ctx, host)
		if err != nil {
			return err
		}
	}

	err = a.ensureAnnotation(ctx, machine, host)
	if err != nil {
		return err
	}
	log.Printf("Finished updating machine %v .", machine.Name)
	return nil
}

// Exists tests for the existence of a machine and is invoked by the Machine Controller
func (a *Actuator) Exists(ctx context.Context, cluster *machinev1.Cluster, machine *machinev1.Machine) (bool, error) {
	log.Printf("Checking if machine %v exists.", machine.Name)
	host, err := a.getHost(ctx, machine)
	if err != nil {
		return false, err
	}
	if host == nil {
		log.Printf("Machine %v does not exist.", machine.Name)
		return false, nil
	}
	log.Printf("Machine %v exists.", machine.Name)
	return true, nil
}

// The Machine Actuator interface must implement GetIP and GetKubeConfig functions as a workaround for issues
// cluster-api#158 (https://github.com/kubernetes-sigs/cluster-api/issues/158) and cluster-api#160
// (https://github.com/kubernetes-sigs/cluster-api/issues/160).

// GetIP returns IP address of the machine in the cluster.
func (a *Actuator) GetIP(cluster *machinev1.Cluster, machine *machinev1.Machine) (string, error) {
	log.Printf("Getting IP of machine %v .", machine.Name)
	return "", fmt.Errorf("TODO: Not yet implemented")
}

// GetKubeConfig gets a kubeconfig from the running control plane.
func (a *Actuator) GetKubeConfig(cluster *machinev1.Cluster, controlPlaneMachine *machinev1.Machine) (string, error) {
	log.Printf("Getting IP of machine %v .", controlPlaneMachine.Name)
	return "", fmt.Errorf("TODO: Not yet implemented")
}

// getHost gets the associated host by searching for a label on it that
// references the machine. Returns nil if not found.
func (a *Actuator) getHost(ctx context.Context, machine *machinev1.Machine) (*bmh.BareMetalHost, error) {
	hosts := bmh.BareMetalHostList{}
	opts := &client.ListOptions{
		LabelSelector: labels.Set{MachineLabel: machine.Name}.AsSelector(),
	}
	err := a.client.List(ctx, opts, &hosts)
	if err != nil {
		return nil, err
	}
	switch len(hosts.Items) {
	case 0:
		return nil, nil
	case 1:
		host := &hosts.Items[0]
		log.Printf("Found associated host %s", host.Name)
		return host, nil
	default:
		return nil, fmt.Errorf("Found %d hosts with label %s:%s", len(hosts.Items), "machine", machine.Name)
	}
}

// chooseHost iterates through known hosts and returns one that can be
// associated with the machine. It searches all hosts in case one already has an
// association with this machine, but lacks the label that would have made it
// discoverable. It will add a Machine reference before returning the host.
func (a *Actuator) chooseHost(ctx context.Context, machine *machinev1.Machine) (*bmh.BareMetalHost, error) {
	// get list of BMH
	hosts := bmh.BareMetalHostList{}
	opts := &client.ListOptions{}
	a.client.List(ctx, opts, &hosts)

	var chosenHost *bmh.BareMetalHost
	availableHosts := []*bmh.BareMetalHost{}

	for _, host := range hosts.Items {
		if host.Spec.MachineRef == nil {
			availableHosts = append(availableHosts, &host)
		} else if host.Spec.MachineRef.Name == machine.Name && host.Spec.MachineRef.Namespace == machine.Namespace {
			log.Printf("found host %s with missing or wrong label", host.Name)
			chosenHost = &host
			break
		}
	}
	if chosenHost == nil {
		if len(availableHosts) == 0 {
			return nil, fmt.Errorf("No available host found")
		}
		// choose a host at random from available hosts
		rand.Seed(time.Now().Unix())
		chosenHost = availableHosts[rand.Intn(len(availableHosts))]
		chosenHost.Spec.MachineRef = &corev1.ObjectReference{
			Name:      machine.Name,
			Namespace: machine.Namespace,
		}
	}

	return chosenHost, nil
}

// ensureAnnotation makes sure the machine has an annotation that references the
// host and uses the API to update the machine if necessary.
func (a *Actuator) ensureAnnotation(ctx context.Context, machine *machinev1.Machine, host *bmh.BareMetalHost) error {
	annotations := machine.ObjectMeta.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	existing, ok := annotations[HostAnnotation]
	if ok && existing != host.Name {
		log.Printf("Warning: found stray annotation for host %s on machine %s. Overwriting.", existing, machine.Name)
	}
	annotations[HostAnnotation] = host.Name
	machine.ObjectMeta.SetAnnotations(annotations)
	return a.client.Update(ctx, machine)
}

// ensureLabelOnHost makes sure there is a label on the host that references the
// machine. Returns true if the object was changed. Does not use the API to update the machine.
func ensureLabelOnHost(ctx context.Context, machine *machinev1.Machine, host *bmh.BareMetalHost) (bool, error) {
	labels := host.ObjectMeta.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	if labels[MachineLabel] == machine.Name {
		return false, nil
	}
	labels[MachineLabel] = machine.Name
	host.ObjectMeta.SetLabels(labels)

	return true, nil
}

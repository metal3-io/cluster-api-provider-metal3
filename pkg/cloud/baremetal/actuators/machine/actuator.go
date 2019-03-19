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
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	bmh "github.com/metalkube/baremetal-operator/pkg/apis/metalkube/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
	machinev1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ProviderName = "solas"
	// HostAnnotation is the key for an annotation that should go on a Machine to
	// reference what BareMetalHost it corresponds to.
	HostAnnotation = "metalkube.org/BareMetalHost"
	// FIXME(dhellmann): These image values should probably come from
	// configuration settings and something that can tell the IP
	// address of the web server hosting the image in the ironic pod.
	instanceImageSource      = "http://172.22.0.1/images/redhat-coreos-maipo-latest.qcow2"
	instanceImageChecksumURL = instanceImageSource + ".md5sum"
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
	// look for associated BMH
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
	host, err := a.getHost(ctx, machine)
	if err != nil {
		return err
	}
	if host != nil && host.Spec.MachineRef != nil {
		// don't remove the MachineRef if it references some other machine
		if host.Spec.MachineRef.Name == machine.Name {
			host.Spec.MachineRef = nil
			err = a.client.Update(ctx, host)
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
		}
	}
	log.Printf("finished deleting machine %v.", machine.Name)
	return nil
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

// getHost gets the associated host by looking for an annotation on the machine
// that contains a reference to the host. Returns nil if not found. Assumes the
// host is in the same namespace as the machine.
func (a *Actuator) getHost(ctx context.Context, machine *machinev1.Machine) (*bmh.BareMetalHost, error) {
	annotations := machine.ObjectMeta.GetAnnotations()
	if annotations == nil {
		return nil, nil
	}
	hostKey, ok := annotations[HostAnnotation]
	if !ok {
		return nil, nil
	}
	hostNamespace, hostName, err := cache.SplitMetaNamespaceKey(hostKey)
	if err != nil {
		log.Printf("Error parsing annotation value \"%s\": %v", hostKey, err)
		return nil, err
	}

	host := bmh.BareMetalHost{}
	key := client.ObjectKey{
		Name:      hostName,
		Namespace: hostNamespace,
	}
	err = a.client.Get(ctx, key, &host)
	if errors.IsNotFound(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return &host, nil
}

// chooseHost iterates through known hosts and returns one that can be
// associated with the machine. It searches all hosts in case one already has an
// association with this machine. It will add a Machine reference and update the
// host via the kube API before returning the host.
func (a *Actuator) chooseHost(ctx context.Context, machine *machinev1.Machine) (*bmh.BareMetalHost, error) {
	// get list of BMH
	hosts := bmh.BareMetalHostList{}
	// TODO - We should add filtering here for known conditions that make
	// a host not a valid choice, such as an operational state of error.
	opts := &client.ListOptions{
		Namespace: machine.Namespace,
	}
	a.client.List(ctx, opts, &hosts)

	availableHosts := []*bmh.BareMetalHost{}

	for _, host := range hosts.Items {
		if host.Available() {
			availableHosts = append(availableHosts, &host)
		} else if host.Spec.MachineRef.Name == machine.Name && host.Spec.MachineRef.Namespace == machine.Namespace {
			log.Printf("found host %s with existing MachineRef", host.Name)
			return &host, nil
		}
	}
	if len(availableHosts) == 0 {
		return nil, fmt.Errorf("No available host found")
	}
	// choose a host at random from available hosts
	rand.Seed(time.Now().Unix())
	chosenHost := availableHosts[rand.Intn(len(availableHosts))]
	chosenHost.Spec.MachineRef = &corev1.ObjectReference{
		Name:      machine.Name,
		Namespace: machine.Namespace,
	}

	// FIXME(dhellmann): The Stein version of Ironic supports passing
	// a URL. When we upgrade, we can stop doing this work ourself.
	log.Printf("looking for checksum for %s at %s",
		instanceImageSource, instanceImageChecksumURL)
	resp, err := http.Get(instanceImageChecksumURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	instanceImageChecksum, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// FIXME(dhellmann): When we stop using the consts for these
	// settings, we need to pass the right values.
	chosenHost.Spec.Image = &bmh.Image{
		URL:      instanceImageSource,
		Checksum: strings.TrimSpace(string(instanceImageChecksum)),
	}
	err = a.client.Update(ctx, chosenHost)
	if err != nil {
		return nil, err
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
	hostKey, err := cache.MetaNamespaceKeyFunc(host)
	if err != nil {
		log.Printf("Error parsing annotation value \"%s\": %v", hostKey, err)
		return err
	}
	existing, ok := annotations[HostAnnotation]
	if ok {
		if existing == hostKey {
			return nil
		}
		log.Printf("Warning: found stray annotation for host %s on machine %s. Overwriting.", existing, machine.Name)
	}
	annotations[HostAnnotation] = hostKey
	machine.ObjectMeta.SetAnnotations(annotations)
	return a.client.Update(ctx, machine)
}

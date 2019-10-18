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
	"encoding/base64"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/go-logr/logr"
	pkgerrors "github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	// TODO Why blank import ?
	_ "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/pointer"

	bmh "github.com/metal3-io/baremetal-operator/pkg/apis/metal3/v1alpha1"
	capbm "sigs.k8s.io/cluster-api-provider-baremetal/api/v1alpha2"
	capi "sigs.k8s.io/cluster-api/api/v1alpha2"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// HostAnnotation is the key for an annotation that should go on a Machine to
	// reference what BareMetalHost it corresponds to.
	HostAnnotation = "metal3.io/BareMetalHost"
	requeueAfter   = time.Second * 30
)

// MachineManager is responsible for performing machine reconciliation
type MachineManager struct {
	client      client.Client
	patchHelper *patch.Helper

	Cluster          *capi.Cluster
	BareMetalCluster *capbm.BareMetalCluster
	Machine          *capi.Machine
	BareMetalMachine *capbm.BareMetalMachine
	Log              logr.Logger
}

// NewMachineManager returns a new helper for managing a cluster with a given name.
func newMachineManager(client client.Client,
	cluster *capi.Cluster, baremetalCluster *capbm.BareMetalCluster,
	machine *capi.Machine, baremetalMachine *capbm.BareMetalMachine,
	machineLog logr.Logger) (*MachineManager, error) {

	helper, err := patch.NewHelper(machine, client)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "failed to init patch helper")
	}

	return &MachineManager{
		client:      client,
		patchHelper: helper,

		Cluster:          cluster,
		BareMetalCluster: baremetalCluster,
		Machine:          machine,
		BareMetalMachine: baremetalMachine,
		Log:              machineLog,
	}, nil
}

// Name returns the BareMetalMachine name.
func (mgr *MachineManager) Name() string {
	return mgr.BareMetalMachine.Name
}

// Namespace returns the namespace name.
func (mgr *MachineManager) Namespace() string {
	return mgr.BareMetalMachine.Namespace
}

// IsControlPlane returns true if the machine is a control plane.
func (mgr *MachineManager) IsControlPlane() bool {
	return util.IsControlPlaneMachine(mgr.Machine)
}

// Role returns the machine role from the labels.
func (mgr *MachineManager) Role() string {
	if util.IsControlPlaneMachine(mgr.Machine) {
		return "control-plane"
	}
	return "node"
}

// GetProviderID return the provider identifier for this machine
func (mgr *MachineManager) GetProviderID() string {
	if mgr.BareMetalMachine.Spec.ProviderID != nil {
		return *mgr.BareMetalMachine.Spec.ProviderID
	}
	return ""
}

// SetProviderID sets the docker provider ID for the kubernetes node
func (mgr *MachineManager) SetProviderID(v string) {
	mgr.BareMetalMachine.Spec.ProviderID = pointer.StringPtr(v)
}

// SetReady sets the AzureMachine Ready Status
func (mgr *MachineManager) SetReady() {
	mgr.BareMetalMachine.Status.Ready = true
}

// SetAnnotation sets a key value annotation on the BareMetalMachine.
func (mgr *MachineManager) SetAnnotation(key, value string) {
	if mgr.BareMetalMachine.Annotations == nil {
		mgr.BareMetalMachine.Annotations = map[string]string{}
	}
	mgr.BareMetalMachine.Annotations[key] = value
}

// Close the MachineManager by updating the machine spec, machine status.
func (mgr *MachineManager) Close() error {
	return mgr.patchHelper.Patch(context.TODO(), mgr.BareMetalMachine)
}

// ExecBootstrap runs bootstrap on a node, this is generally `kubeadm <init|join>`
func (mgr *MachineManager) ExecBootstrap(data string) error {
	return nil
}

// KubeadmReset will run `kubeadm reset` on the machine.
func (mgr *MachineManager) KubeadmReset() error {
	return nil
}

// Create creates a machine and is invoked by the Machine Controller
func (mgr *MachineManager) Create(ctx context.Context) (string, error) {
	mgr.Log.Info("Creating machine")
	providerID := ""

	// load and validate the config
	if mgr.BareMetalMachine == nil {
		// Should have been picked earlier. Do not requeue
		return providerID, nil
	}

	config := mgr.BareMetalMachine.Spec
	err := config.IsValid()
	if err != nil {
		// Should have been picked earlier. Do not requeue
		mgr.setError(ctx, err.Error())
		return providerID, nil
	}

	// clear an error if one was previously set
	mgr.clearError(ctx)

	// look for associated BMH
	host, err := mgr.getHost(ctx)
	if err != nil {
		return providerID, err
	}

	// none found, so try to choose one
	if host == nil {
		host, err = mgr.chooseHost(ctx)
		if err != nil {
			return providerID, err
		}
		if host == nil {
			mgr.Log.Info("No available host found. Requeuing.")
			return providerID, &RequeueAfterError{RequeueAfter: requeueAfter}
		}
		mgr.Log.Info("Associating machine with host", "host", host.Name)
	} else {
		mgr.Log.Info("Machine already associated with host", "host", host.Name)
	}

	err = mgr.mergeUserData(ctx)
	if err != nil {
		return providerID, err
	}

	providerID = host.Name
	err = mgr.setHostSpec(ctx, host)
	if err != nil {
		return providerID, err
	}

	err = mgr.ensureAnnotation(ctx, host)
	if err != nil {
		return providerID, err
	}

	if err := mgr.updateMachineStatus(ctx, host); err != nil {
		return providerID, err
	}

	mgr.Log.Info("Finished creating machine")
	return providerID, nil
}

// Merge the UserData from the machine and the user
func (mgr *MachineManager) mergeUserData(ctx context.Context) error {
	if mgr.Machine.Spec.Bootstrap.Data != nil {
		decodedUserDataBytes, err := base64.StdEncoding.DecodeString(*mgr.Machine.Spec.Bootstrap.Data)
		decodedUserData := string(decodedUserDataBytes)
		if err != nil {
			return err
		}

		bootstrapSecret := &corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Secret",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      mgr.Machine.Name + "-user-data",
				Namespace: mgr.Machine.Namespace,
			},
			Data: map[string][]byte{
				"userData": []byte(decodedUserData),
			},
			Type: "Opaque",
		}

		tmpBootstrapSecret := corev1.Secret{}
		key := client.ObjectKey{
			Name:      mgr.Machine.Name + "-user-data",
			Namespace: mgr.Machine.Namespace,
		}
		err = mgr.client.Get(ctx, key, &tmpBootstrapSecret)
		if errors.IsNotFound(err) {
			// Create the secret with use data
			err = mgr.client.Create(ctx, bootstrapSecret)
		} else if err != nil {
			return err
		} else {
			// Update the secret with use data
			err = mgr.client.Update(ctx, bootstrapSecret)
		}

		if err != nil {
			mgr.Log.Info("Unable to create secret for bootstrap")
			return err
		}
		mgr.BareMetalMachine.Spec.UserData = &corev1.SecretReference{
			Name:      mgr.Machine.Name + "-user-data",
			Namespace: mgr.Machine.Namespace,
		}

	}
	return nil
}

// Delete deletes a machine and is invoked by the Machine Controller
func (mgr *MachineManager) Delete(ctx context.Context) (string, error) {
	mgr.Log.Info("Deleting machine")
	providerID := ""

	host, err := mgr.getHost(ctx)
	if err != nil {
		return providerID, err
	}

	if host != nil && host.Spec.ConsumerRef != nil {
		mgr.Log.Info("Updating host", "host", host.Name)
		// don't remove the ConsumerRef if it references some other machine
		if !consumerRefMatches(host.Spec.ConsumerRef, mgr.Machine) {
			mgr.Log.Info("host associated with another machine",
				"host", host.Name)
			return providerID, nil
		}

		providerID = host.Name
		if host.Spec.Image != nil || host.Spec.Online || host.Spec.UserData != nil {
			host.Spec.Image = nil
			host.Spec.Online = false
			host.Spec.UserData = nil
			err = mgr.client.Update(ctx, host)
			if err != nil && !errors.IsNotFound(err) {
				return host.Name, err
			}
			return providerID, &RequeueAfterError{}
		}

		waiting := true
		switch host.Status.Provisioning.State {
		// TODO? remove empty string that is the status without BMO running
		case bmh.StateRegistrationError, bmh.StateRegistering,
			bmh.StateMatchProfile, bmh.StateInspecting,
			bmh.StateReady, bmh.StateValidationError, "":
			// Host is not provisioned
			waiting = false
		case bmh.StateExternallyProvisioned:
			// We have no control over provisioning, so just wait until the
			// host is powered off
			waiting = host.Status.PoweredOn
		}
		if waiting {
			return providerID, &RequeueAfterError{RequeueAfter: requeueAfter}
		}
		host.Spec.ConsumerRef = nil
		err = mgr.client.Update(ctx, host)
		if err != nil && !errors.IsNotFound(err) {
			return providerID, err
		}
	}
	mgr.Log.Info("Deleting User data secret for machine")
	tmpBootstrapSecret := corev1.Secret{}
	key := client.ObjectKey{
		Name:      mgr.Machine.Name + "-user-data",
		Namespace: mgr.Machine.Namespace,
	}
	err = mgr.client.Get(ctx, key, &tmpBootstrapSecret)
	if err != nil && !errors.IsNotFound(err) {
		return providerID, err
	} else if err == nil {
		// Delete the secret with use data
		err = mgr.client.Delete(ctx, &tmpBootstrapSecret)
		if err != nil {
			return providerID, err
		}
	}
	mgr.Log.Info("finished deleting machine")
	return providerID, nil
}

// Update updates a machine and is invoked by the Machine Controller
func (mgr *MachineManager) Update(ctx context.Context) (string, error) {
	mgr.Log.Info("Updating machine")
	providerID := ""

	// clear any error message that was previously set. This method doesn't set
	// error messages yet, so we know that it's incorrect to have one here.
	mgr.clearError(ctx)

	host, err := mgr.getHost(ctx)
	if err != nil {
		return providerID, err
	}
	if host == nil {
		return providerID, fmt.Errorf("host not found for machine %s", mgr.Machine.Name)
	}

	providerID = host.Name
	err = mgr.ensureAnnotation(ctx, host)
	if err != nil {
		return providerID, err
	}

	if err := mgr.updateMachineStatus(ctx, host); err != nil {
		return providerID, err
	}

	mgr.Log.Info("Finished updating machine")
	return providerID, nil
}

// Exists tests for the existence of a machine and is invoked by the Machine Controller
func (mgr *MachineManager) Exists(ctx context.Context) (bool, error) {
	mgr.Log.Info("Checking if machine exists.")
	host, err := mgr.getHost(ctx)
	if err != nil {
		return false, err
	}
	if host == nil {
		mgr.Log.Info("Machine does not exist.")
		return false, nil
	}
	mgr.Log.Info("Machine exists.")
	return true, nil
}

// The Machine Actuator interface must implement GetIP and GetKubeConfig functions as a workaround for issues
// cluster-api#158 (https://github.com/kubernetes-sigs/cluster-api/issues/158) and cluster-api#160
// (https://github.com/kubernetes-sigs/cluster-api/issues/160).

// GetIP returns IP address of the machine in the cluster.
func (mgr *MachineManager) GetIP() (string, error) {
	mgr.Log.Info("Getting IP of machine")
	return "", fmt.Errorf("TODO: Not yet implemented")
}

// GetKubeConfig gets a kubeconfig from the running control plane.
func (mgr *MachineManager) GetKubeConfig() (string, error) {
	mgr.Log.Info("Getting Kubeconfig.")
	return "", fmt.Errorf("TODO: Not yet implemented")
}

// getHost gets the associated host by looking for an annotation on the machine
// that contains a reference to the host. Returns nil if not found. Assumes the
// host is in the same namespace as the machine.
func (mgr *MachineManager) getHost(ctx context.Context) (*bmh.BareMetalHost, error) {
	annotations := mgr.BareMetalMachine.ObjectMeta.GetAnnotations()
	if annotations == nil {
		return nil, nil
	}
	hostKey, ok := annotations[HostAnnotation]
	if !ok {
		return nil, nil
	}
	hostNamespace, hostName, err := cache.SplitMetaNamespaceKey(hostKey)
	if err != nil {
		mgr.Log.Error(err, "Error parsing annotation value", "annotation key", hostKey)
		return nil, err
	}

	host := bmh.BareMetalHost{}
	key := client.ObjectKey{
		Name:      hostName,
		Namespace: hostNamespace,
	}
	err = mgr.client.Get(ctx, key, &host)
	if errors.IsNotFound(err) {
		mgr.Log.Info("Annotated host not found", "host", hostKey)
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return &host, nil
}

// chooseHost iterates through known hosts and returns one that can be
// associated with the machine. It searches all hosts in case one already has an
// association with this machine.
func (mgr *MachineManager) chooseHost(ctx context.Context) (*bmh.BareMetalHost, error) {

	// get list of BMH
	hosts := bmh.BareMetalHostList{}
	opts := &client.ListOptions{
		Namespace: mgr.Machine.Namespace,
	}

	err := mgr.client.List(ctx, &hosts, opts)
	if err != nil {
		return nil, err
	}

	// Using the label selector on ListOptions above doesn't seem to work.
	// I think it's because we have a local cache of all BareMetalHosts.
	labelSelector := labels.NewSelector()
	var reqs labels.Requirements
	for labelKey, labelVal := range mgr.BareMetalMachine.Spec.HostSelector.MatchLabels {
		mgr.Log.Info("Adding requirement to match label", "label key", labelKey,
			"label value", labelVal)
		r, err := labels.NewRequirement(labelKey, selection.Equals, []string{labelVal})
		if err != nil {
			mgr.Log.Error(err, "Failed to create MatchLabel requirement, not choosing host")
			return nil, err
		}
		reqs = append(reqs, *r)
	}
	for _, req := range mgr.BareMetalMachine.Spec.HostSelector.MatchExpressions {
		mgr.Log.Info("Adding requirement to match label", "label key", req.Key,
			"label operator", req.Operator, "label value", req.Values)
		lowercaseOperator := selection.Operator(strings.ToLower(string(req.Operator)))
		r, err := labels.NewRequirement(req.Key, lowercaseOperator, req.Values)
		if err != nil {
			mgr.Log.Error(err, "Failed to create MatchExpression requirement, not choosing host")
			return nil, err
		}
		reqs = append(reqs, *r)
	}
	labelSelector = labelSelector.Add(reqs...)

	availableHosts := []*bmh.BareMetalHost{}

	for i, host := range hosts.Items {
		if host.Available() {
			if labelSelector.Matches(labels.Set(host.ObjectMeta.Labels)) {
				mgr.Log.Info("Host matched hostSelector for Machine", "host", host.Name)
				availableHosts = append(availableHosts, &hosts.Items[i])
			} else {
				mgr.Log.Info("Host did not match hostSelector for Machine", "host", host.Name)
			}
		} else if host.Spec.ConsumerRef != nil && consumerRefMatches(host.Spec.ConsumerRef, mgr.Machine) {
			mgr.Log.Info("found host with existing ConsumerRef", "host", host.Name)
			return &hosts.Items[i], nil
		}
	}
	mgr.Log.Info(fmt.Sprintf("%d hosts available while choosing host for machine", len(availableHosts)))
	if len(availableHosts) == 0 {
		return nil, nil
	}

	// choose a host at random from available hosts
	rand.Seed(time.Now().Unix())
	chosenHost := availableHosts[rand.Intn(len(availableHosts))]

	return chosenHost, nil
}

// consumerRefMatches returns a boolean based on whether the consumer
// reference and machine metadata match
func consumerRefMatches(consumer *corev1.ObjectReference, machine *capi.Machine) bool {
	if consumer.Name != machine.Name {
		return false
	}
	if consumer.Namespace != machine.Namespace {
		return false
	}
	if consumer.Kind != machine.Kind {
		return false
	}
	if consumer.APIVersion != machine.APIVersion {
		return false
	}
	return true
}

// setHostSpec will ensure the host's Spec is set according to the machine's
// details. It will then update the host via the kube API. If UserData does not
// include a Namespace, it will default to the Machine's namespace.
func (mgr *MachineManager) setHostSpec(ctx context.Context, host *bmh.BareMetalHost) error {

	// We only want to update the image setting if the host does not
	// already have an image.
	//
	// A host with an existing image is already provisioned and
	// upgrades are not supported at this time. To re-provision a
	// host, we must fully deprovision it and then provision it again.
	// Not provisioning while we do not have the UserData
	if host.Spec.Image == nil && mgr.BareMetalMachine.Spec.UserData != nil {
		host.Spec.Image = &bmh.Image{
			URL:      mgr.BareMetalMachine.Spec.Image.URL,
			Checksum: mgr.BareMetalMachine.Spec.Image.Checksum,
		}
		host.Spec.UserData = mgr.BareMetalMachine.Spec.UserData
		if host.Spec.UserData != nil && host.Spec.UserData.Namespace == "" {
			host.Spec.UserData.Namespace = mgr.Machine.Namespace
		}
	}

	host.Spec.ConsumerRef = &corev1.ObjectReference{
		Kind:       "Machine",
		Name:       mgr.Machine.Name,
		Namespace:  mgr.Machine.Namespace,
		APIVersion: mgr.Machine.APIVersion,
	}

	host.Spec.Online = true
	return mgr.client.Update(ctx, host)
}

// ensureAnnotation makes sure the machine has an annotation that references the
// host and uses the API to update the machine if necessary.
func (mgr *MachineManager) ensureAnnotation(ctx context.Context, host *bmh.BareMetalHost) error {
	annotations := mgr.BareMetalMachine.ObjectMeta.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	hostKey, err := cache.MetaNamespaceKeyFunc(host)
	if err != nil {
		mgr.Log.Error(err, "Error parsing annotation value", "annotation key", hostKey)
		return err
	}
	existing, ok := annotations[HostAnnotation]
	if ok {
		if existing == hostKey {
			return nil
		}
		mgr.Log.Info("Warning: found stray annotation for host on machine. Overwriting.", "host", existing)
	}
	annotations[HostAnnotation] = hostKey
	mgr.BareMetalMachine.ObjectMeta.SetAnnotations(annotations)
	// Will be done by mgr.Close()
	// return mgr.client.Update(ctx, mgr.BareMetalMachine)
	return nil
}

// setError sets the ErrorMessage and ErrorReason fields on the machine and logs
// the message. It assumes the reason is invalid configuration, since that is
// currently the only relevant MachineStatusError choice.
func (mgr *MachineManager) setError(ctx context.Context, message string) {
	mgr.BareMetalMachine.Status.ErrorMessage = &message
	reason := capierrors.InvalidConfigurationMachineError
	mgr.BareMetalMachine.Status.ErrorReason = &reason
}

// clearError removes the ErrorMessage from the machine's Status if set. Returns
// nil if ErrorMessage was already nil. Returns a RequeueAfterError if the
// machine was updated.
func (mgr *MachineManager) clearError(ctx context.Context) {
	if mgr.BareMetalMachine.Status.ErrorMessage != nil || mgr.BareMetalMachine.Status.ErrorReason != nil {
		mgr.BareMetalMachine.Status.ErrorMessage = nil
		mgr.BareMetalMachine.Status.ErrorReason = nil
	}
}

// updateMachineStatus updates a machine object's status.
func (mgr *MachineManager) updateMachineStatus(ctx context.Context, host *bmh.BareMetalHost) error {
	addrs := mgr.nodeAddresses(host)

	machineCopy := mgr.BareMetalMachine.DeepCopy()
	machineCopy.Status.Addresses = addrs

	if equality.Semantic.DeepEqual(mgr.Machine.Status, machineCopy.Status) {
		// Status did not change
		return nil
	}

	now := metav1.Now()
	mgr.BareMetalMachine.Status.LastUpdated = &now
	mgr.BareMetalMachine.Status.Addresses = addrs

	return nil
}

// NodeAddresses returns a slice of corev1.NodeAddress objects for a
// given Baremetal machine.
func (mgr *MachineManager) nodeAddresses(host *bmh.BareMetalHost) []capi.MachineAddress {
	addrs := []capi.MachineAddress{}

	// If the host is nil or we have no hw details, return an empty address array.
	if host == nil || host.Status.HardwareDetails == nil {
		return addrs
	}

	for _, nic := range host.Status.HardwareDetails.NIC {
		address := capi.MachineAddress{
			Type:    capi.MachineInternalIP,
			Address: nic.IP,
		}
		addrs = append(addrs, address)
	}

	if host.Status.HardwareDetails.Hostname != "" {
		addrs = append(addrs, capi.MachineAddress{
			Type:    capi.MachineHostName,
			Address: host.Status.HardwareDetails.Hostname,
		})
		addrs = append(addrs, capi.MachineAddress{
			Type:    capi.MachineInternalDNS,
			Address: host.Status.HardwareDetails.Hostname,
		})
	}

	return addrs
}

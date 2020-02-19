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

	// comment for go-lint
	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/pointer"

	bmh "github.com/metal3-io/baremetal-operator/pkg/apis/metal3/v1alpha1"
	capbm "github.com/metal3-io/cluster-api-provider-baremetal/api/v1alpha3"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// ProviderName is exported
	ProviderName = "baremetal"
	// HostAnnotation is the key for an annotation that should go on a Machine to
	// reference what BareMetalHost it corresponds to.
	HostAnnotation     = "metal3.io/BareMetalHost"
	requeueAfter       = time.Second * 30
	bmRoleControlPlane = "control-plane"
	bmRoleNode         = "node"
	userDataFinalizer  = "baremetalmachine.infrastructure.cluster.x-k8s.io/userData"
)

// MachineManagerInterface is an interface for a ClusterManager
type MachineManagerInterface interface {
	SetFinalizer()
	UnsetFinalizer()
	IsProvisioned() bool
	IsBootstrapReady() bool
	GetBaremetalHostID(context.Context) (*string, error)
	Associate(context.Context) error
	Delete(context.Context) error
	Update(context.Context) error
	HasAnnotation() bool
	SetNodeProviderID(context.Context, string, string, ClientGetter) error
	SetProviderID(string)
}

// MachineManager is responsible for performing machine reconciliation
type MachineManager struct {
	client client.Client

	Cluster          *capi.Cluster
	BareMetalCluster *capbm.BareMetalCluster
	Machine          *capi.Machine
	BareMetalMachine *capbm.BareMetalMachine
	Log              logr.Logger
}

// NewMachineManager returns a new helper for managing a machine
func NewMachineManager(client client.Client,
	cluster *capi.Cluster, baremetalCluster *capbm.BareMetalCluster,
	machine *capi.Machine, baremetalMachine *capbm.BareMetalMachine,
	machineLog logr.Logger) (*MachineManager, error) {

	return &MachineManager{
		client: client,

		Cluster:          cluster,
		BareMetalCluster: baremetalCluster,
		Machine:          machine,
		BareMetalMachine: baremetalMachine,
		Log:              machineLog,
	}, nil
}

// SetFinalizer sets finalizer
func (m *MachineManager) SetFinalizer() {
	// If the BareMetalMachine doesn't have finalizer, add it.
	if !util.Contains(m.BareMetalMachine.Finalizers, capbm.MachineFinalizer) {
		m.BareMetalMachine.Finalizers = append(m.BareMetalMachine.Finalizers,
			capbm.MachineFinalizer,
		)
	}
}

// UnsetFinalizer unsets finalizer
func (m *MachineManager) UnsetFinalizer() {
	// Cluster is deleted so remove the finalizer.
	m.BareMetalMachine.Finalizers = util.Filter(m.BareMetalMachine.Finalizers,
		capbm.MachineFinalizer,
	)
}

// IsProvisioned checks if the machine is provisioned
func (m *MachineManager) IsProvisioned() bool {
	if m.BareMetalMachine.Spec.ProviderID != nil && m.BareMetalMachine.Status.Ready {
		return true
	}
	return false
}

// IsBootstrapReady checks if the machine is given Bootstrap data
func (m *MachineManager) IsBootstrapReady() bool {
	if !m.Machine.Status.BootstrapReady {
		m.Log.Info("Waiting for the Bootstrap provider controller to set bootstrap data")
	}
	return m.Machine.Status.BootstrapReady
}

// isControlPlane returns true if the machine is a control plane.
func (m *MachineManager) isControlPlane() bool {
	return util.IsControlPlaneMachine(m.Machine)
}

// role returns the machine role from the labels.
func (m *MachineManager) role() string {
	if util.IsControlPlaneMachine(m.Machine) {
		return bmRoleControlPlane
	}
	return bmRoleNode
}

// GetBaremetalHostID return the provider identifier for this machine
func (m *MachineManager) GetBaremetalHostID(ctx context.Context) (*string, error) {
	// look for associated BMH
	host, err := m.getHost(ctx)
	if err != nil {
		m.setError("Failed to get a BaremetalHost for the BareMetalMachine",
			capierrors.CreateMachineError,
		)
		return nil, err
	}
	if host == nil {
		m.Log.Info("BaremetalHost not associated, requeuing")
		return nil, &RequeueAfterError{RequeueAfter: requeueAfter}
	}
	if host.Status.Provisioning.State == bmh.StateProvisioned {
		return pointer.StringPtr(string(host.ObjectMeta.UID)), nil
	}
	m.Log.Info("Provisioning BaremetalHost, requeuing")
	return nil, &RequeueAfterError{RequeueAfter: requeueAfter}
}

// Associate associates a machine and is invoked by the Machine Controller
func (m *MachineManager) Associate(ctx context.Context) error {
	m.Log.Info("Associating machine", "machine", m.Machine.Name)

	// load and validate the config
	if m.BareMetalMachine == nil {
		// Should have been picked earlier. Do not requeue
		return nil
	}

	config := m.BareMetalMachine.Spec
	err := config.IsValid()
	if err != nil {
		// Should have been picked earlier. Do not requeue
		m.setError(err.Error(), capierrors.InvalidConfigurationMachineError)
		return nil
	}

	// clear an error if one was previously set
	m.clearError()

	// look for associated BMH
	host, err := m.getHost(ctx)
	if err != nil {
		m.setError("Failed to get the BaremetalHost for the BareMetalMachine",
			capierrors.CreateMachineError,
		)
		return err
	}

	// no BMH found, trying to choose from available ones
	if host == nil {
		host, err = m.chooseHost(ctx)
		if err != nil {
			m.setError("Failed to pick a BaremetalHost for the BareMetalMachine",
				capierrors.CreateMachineError,
			)
			return err
		}
		if host == nil {
			m.Log.Info("No available host found. Requeuing.")
			return &RequeueAfterError{RequeueAfter: requeueAfter}
		}
		m.Log.Info("Associating machine with host", "host", host.Name)
	} else {
		m.Log.Info("Machine already associated with host", "host", host.Name)
	}

	// A machine bootstrap not ready case is caught in the controller
	// ReconcileNormal function
	err = m.GetUserData(ctx)
	if err != nil {
		m.setError("Failed to set the UserData for the BareMetalMachine",
			capierrors.CreateMachineError,
		)
		return err
	}

	err = m.setHostLabel(ctx, host)
	if err != nil {
		m.setError("Failed to set the Cluster label in the BareMetalHost",
			capierrors.CreateMachineError,
		)
		return err
	}

	err = m.setBMCSecretLabel(ctx, host)
	if err != nil {
		m.Log.Info("Failed to set the Cluster label in the BMC Credentials for BareMetalHost", host.Name)
	}

	err = m.setHostSpec(ctx, host)
	if err != nil {
		m.setError("Failed to associate the BaremetalHost to the BareMetalMachine",
			capierrors.CreateMachineError,
		)
		return err
	}

	err = m.ensureAnnotation(ctx, host)
	if err != nil {
		m.setError("Failed to annotate the BareMetalMachine",
			capierrors.CreateMachineError,
		)
		return err
	}

	m.Log.Info("Finished creating machine")
	return nil
}

// Merge the UserData from the machine and the user
func (m *MachineManager) GetUserData(ctx context.Context) error {
	var err error
	var decodedUserDataBytes []byte
	// if datasecretname is set get userdata from secret
	if m.Machine.Spec.Bootstrap.DataSecretName != nil {
		capiBootstrapSecret := corev1.Secret{}
		capikey := client.ObjectKey{
			Name:      *m.Machine.Spec.Bootstrap.DataSecretName,
			Namespace: m.Machine.Namespace,
		}
		err := m.client.Get(ctx, capikey, &capiBootstrapSecret)
		if err != nil {
			return err
		}
		decodedUserDataBytes = capiBootstrapSecret.Data["value"]
	} else if m.Machine.Spec.Bootstrap.Data != nil {
		// if datasecretname is not set
		decodedUserData := *m.Machine.Spec.Bootstrap.Data
		// decode the base64 cloud-config
		decodedUserDataBytes, err = base64.StdEncoding.DecodeString(decodedUserData)
		if err != nil {
			return err
		}
	} else {
		return nil
	}

	bootstrapSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.BareMetalMachine.Name + "-user-data",
			Namespace: m.BareMetalMachine.Namespace,
			Labels: map[string]string{
				capi.ClusterLabelName: m.Machine.Spec.ClusterName,
			},
			OwnerReferences: []metav1.OwnerReference{
				metav1.OwnerReference{
					Controller: pointer.BoolPtr(true),
					APIVersion: m.BareMetalMachine.APIVersion,
					Kind:       m.BareMetalMachine.Kind,
					Name:       m.BareMetalMachine.Name,
					UID:        m.BareMetalMachine.UID,
				},
			},
			Finalizers: []string{userDataFinalizer},
		},
		Data: map[string][]byte{
			"userData": decodedUserDataBytes,
		},
		Type: "Opaque",
	}

	tmpBootstrapSecret := corev1.Secret{}
	key := client.ObjectKey{
		Name:      m.BareMetalMachine.Name + "-user-data",
		Namespace: m.BareMetalMachine.Namespace,
	}
	err = m.client.Get(ctx, key, &tmpBootstrapSecret)
	if apierrors.IsNotFound(err) {
		// Create the secret with user data
		err = m.client.Create(ctx, bootstrapSecret)
	} else if err != nil {
		return err
	} else {
		// Update the secret with user data
		err = m.client.Update(ctx, bootstrapSecret)
	}

	if err != nil {
		m.Log.Info("Unable to create secret for bootstrap")
		return err
	}
	m.BareMetalMachine.Spec.UserData = &corev1.SecretReference{
		Name:      m.BareMetalMachine.Name + "-user-data",
		Namespace: m.BareMetalMachine.Namespace,
	}

	return nil
}

// Delete deletes a bare metal machine and is invoked by the Machine Controller
func (m *MachineManager) Delete(ctx context.Context) error {
	m.Log.Info("Deleting bare metal machine", "baremetalmachine", m.BareMetalMachine.Name)

	// clear an error if one was previously set
	m.clearError()

	host, err := m.getHost(ctx)
	if err != nil {
		return err
	}

	if host != nil && host.Spec.ConsumerRef != nil {
		// don't remove the ConsumerRef if it references some other bare metal machine
		if !consumerRefMatches(host.Spec.ConsumerRef, m.BareMetalMachine) {
			m.Log.Info("host already associated with another bare metal machine",
				"host", host.Name)
			return nil
		}

		//Remove clusterLabel from BMC secret
		tmpBMCSecret, errBMC := m.getBMCSecret(ctx, host)
		if errBMC != nil && apierrors.IsNotFound(errBMC) {
			m.Log.Info("BMC credential not found for BareMetalhost", host.Name)
		} else if errBMC == nil && tmpBMCSecret != nil {
			m.Log.Info("Deleting cluster label from BMC credential", host.Spec.BMC.CredentialsName)
			if tmpBMCSecret.Labels != nil && tmpBMCSecret.Labels[capi.ClusterLabelName] == m.Machine.Spec.ClusterName {
				delete(tmpBMCSecret.Labels, capi.ClusterLabelName)
			}
			errBMC = m.client.Update(ctx, tmpBMCSecret)
			if errBMC != nil {
				m.Log.Info("Failed to delete the clusterLabel from BMC Secret")
				return errBMC
			}
		}

		if host.Spec.Image != nil || host.Spec.Online || host.Spec.UserData != nil {
			host.Spec.Image = nil
			host.Spec.Online = false
			host.Spec.UserData = nil
			err = m.client.Update(ctx, host)
			if err != nil && !apierrors.IsNotFound(err) {
				m.setError("Failed to delete BareMetalMachine",
					capierrors.DeleteMachineError,
				)
				return err
			}
			m.Log.Info("Deprovisioning BaremetalHost, requeuing")
			return &RequeueAfterError{}
		}

		waiting := true
		switch host.Status.Provisioning.State {
		case bmh.StateRegistrationError, bmh.StateRegistering,
			bmh.StateMatchProfile, bmh.StateInspecting,
			bmh.StateReady, bmh.StateNone:
			// Host is not provisioned
			waiting = false
		case bmh.StateExternallyProvisioned:
			// We have no control over provisioning, so just wait until the
			// host is powered off
			waiting = host.Status.PoweredOn
		}
		if waiting {
			m.Log.Info("Deprovisioning BaremetalHost, requeuing")
			return &RequeueAfterError{RequeueAfter: requeueAfter}
		}

		host.Spec.ConsumerRef = nil
		if host.Labels != nil && host.Labels[capi.ClusterLabelName] == m.Machine.Spec.ClusterName {
			delete(host.Labels, capi.ClusterLabelName)
		}

		err = m.client.Update(ctx, host)
		if err != nil && !apierrors.IsNotFound(err) {
			m.setError("Failed to delete BareMetalMachine",
				capierrors.DeleteMachineError,
			)
			return err
		}
	}

	m.Log.Info("Deleting User data secret for machine")
	tmpBootstrapSecret := corev1.Secret{}
	key := client.ObjectKey{
		Name:      m.BareMetalMachine.Name + "-user-data",
		Namespace: m.BareMetalMachine.Namespace,
	}
	err = m.client.Get(ctx, key, &tmpBootstrapSecret)
	if err != nil && !apierrors.IsNotFound(err) {
		m.setError("Failed to delete BareMetalMachine",
			capierrors.DeleteMachineError,
		)
		return err
	} else if err == nil {
		//unset the finalizers (remove all since we do not expect anything else
		// to control that object)
		tmpBootstrapSecret.Finalizers = []string{}
		err = m.client.Update(ctx, &tmpBootstrapSecret)
		if err != nil {
			m.setError("Failed to delete BareMetalMachine",
				capierrors.DeleteMachineError,
			)
			return err
		}
		// Delete the secret with use data
		err = m.client.Delete(ctx, &tmpBootstrapSecret)
		if err != nil {
			m.setError("Failed to delete BareMetalMachine",
				capierrors.DeleteMachineError,
			)
			return err
		}
	}
	m.Log.Info("finished deleting bare metal machine")
	return nil
}

// Update updates a machine and is invoked by the Machine Controller
func (m *MachineManager) Update(ctx context.Context) error {
	m.Log.Info("Updating machine")

	// clear any error message that was previously set. This method doesn't set
	// error messages yet, so we know that it's incorrect to have one here.
	m.clearError()

	host, err := m.getHost(ctx)
	if err != nil {
		return err
	}
	if host == nil {
		return fmt.Errorf("host not found for machine %s", m.Machine.Name)
	}

	err = m.ensureAnnotation(ctx, host)
	if err != nil {
		return err
	}

	if err := m.updateMachineStatus(ctx, host); err != nil {
		return err
	}

	m.Log.Info("Finished updating machine")
	return nil
}

// exists tests for the existence of a bare metal machine and is invoked by the Machine Controller
func (m *MachineManager) exists(ctx context.Context) (bool, error) {
	m.Log.Info("Checking if machine exists.")
	host, err := m.getHost(ctx)
	if err != nil {
		return false, err
	}
	if host == nil {
		m.Log.Info("Machine does not exist.")
		return false, nil
	}
	m.Log.Info("Machine exists.")
	return true, nil
}

// getHost gets the associated host by looking for an annotation on the machine
// that contains a reference to the host. Returns nil if not found. Assumes the
// host is in the same namespace as the machine.
func (m *MachineManager) getHost(ctx context.Context) (*bmh.BareMetalHost, error) {
	annotations := m.BareMetalMachine.ObjectMeta.GetAnnotations()
	if annotations == nil {
		return nil, nil
	}
	hostKey, ok := annotations[HostAnnotation]
	if !ok {
		return nil, nil
	}
	hostNamespace, hostName, err := cache.SplitMetaNamespaceKey(hostKey)
	if err != nil {
		m.Log.Error(err, "Error parsing annotation value", "annotation key", hostKey)
		return nil, err
	}

	host := bmh.BareMetalHost{}
	key := client.ObjectKey{
		Name:      hostName,
		Namespace: hostNamespace,
	}
	err = m.client.Get(ctx, key, &host)
	if apierrors.IsNotFound(err) {
		m.Log.Info("Annotated host not found", "host", hostKey)
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return &host, nil
}

// chooseHost iterates through known hosts and returns one that can be
// associated with the bare metal machine. It searches all hosts in case one already has an
// association with this bare metal machine.
func (m *MachineManager) chooseHost(ctx context.Context) (*bmh.BareMetalHost, error) {

	// get list of BMH
	hosts := bmh.BareMetalHostList{}
	opts := &client.ListOptions{
		Namespace: m.Machine.Namespace,
	}

	err := m.client.List(ctx, &hosts, opts)
	if err != nil {
		return nil, err
	}

	// Using the label selector on ListOptions above doesn't seem to work.
	// I think it's because we have a local cache of all BareMetalHosts.
	labelSelector := labels.NewSelector()
	var reqs labels.Requirements

	for labelKey, labelVal := range m.BareMetalMachine.Spec.HostSelector.MatchLabels {
		m.Log.Info("Adding requirement to match label",
			"label key", labelKey,
			"label value", labelVal)
		r, err := labels.NewRequirement(labelKey, selection.Equals, []string{labelVal})
		if err != nil {
			m.Log.Error(err, "Failed to create MatchLabel requirement, not choosing host")
			return nil, err
		}
		reqs = append(reqs, *r)
	}
	for _, req := range m.BareMetalMachine.Spec.HostSelector.MatchExpressions {
		m.Log.Info("Adding requirement to match label",
			"label key", req.Key,
			"label operator", req.Operator,
			"label value", req.Values)
		lowercaseOperator := selection.Operator(strings.ToLower(string(req.Operator)))
		r, err := labels.NewRequirement(req.Key, lowercaseOperator, req.Values)
		if err != nil {
			m.Log.Error(err, "Failed to create MatchExpression requirement, not choosing host")
			return nil, err
		}
		reqs = append(reqs, *r)
	}
	labelSelector = labelSelector.Add(reqs...)

	availableHosts := []*bmh.BareMetalHost{}

	for i, host := range hosts.Items {
		if host.Available() {
			if labelSelector.Matches(labels.Set(host.ObjectMeta.Labels)) {
				m.Log.Info("Host matched hostSelector for BareMetalMachine", "host", host.Name)
				availableHosts = append(availableHosts, &hosts.Items[i])
			} else {
				m.Log.Info("Host did not match hostSelector for BareMetalMachine", "host", host.Name)
			}
		} else if host.Spec.ConsumerRef != nil && consumerRefMatches(host.Spec.ConsumerRef, m.BareMetalMachine) {
			m.Log.Info("Found host with existing ConsumerRef", "host", host.Name)
			return &hosts.Items[i], nil
		}
	}
	m.Log.Info(fmt.Sprintf("%d hosts available while choosing host for bare metal machine", len(availableHosts)))
	if len(availableHosts) == 0 {
		return nil, nil
	}

	// choose a host at random from available hosts
	rand.Seed(time.Now().Unix())
	chosenHost := availableHosts[rand.Intn(len(availableHosts))]

	return chosenHost, nil
}

// consumerRefMatches returns a boolean based on whether the consumer
// reference and bare metal machine metadata match
func consumerRefMatches(consumer *corev1.ObjectReference, bmmachine *capbm.BareMetalMachine) bool {
	if consumer.Name != bmmachine.Name {
		return false
	}
	if consumer.Namespace != bmmachine.Namespace {
		return false
	}
	if consumer.Kind != bmmachine.Kind {
		return false
	}
	if consumer.APIVersion != bmmachine.APIVersion {
		return false
	}
	return true
}

// getBMCSecret will return the BMCSecret associated with BMH
func (m *MachineManager) getBMCSecret(ctx context.Context, host *bmh.BareMetalHost) (*corev1.Secret, error) {

	tmpBMCSecret := corev1.Secret{}
	key := host.CredentialsKey()
	err := m.client.Get(ctx, key, &tmpBMCSecret)
	if apierrors.IsNotFound(err) {
		return nil, err
	} else if err != nil {
		m.Log.Info("Cannot retrieve BMC credential for BareMetalhost ", host.Name, err)
		return nil, nil
	}
	return &tmpBMCSecret, nil
}

// setBMCSecretLabel will set the set cluster.x-k8s.io/cluster-name to BMCSecret
func (m *MachineManager) setBMCSecretLabel(ctx context.Context, host *bmh.BareMetalHost) error {

	tmpBMCSecret, err := m.getBMCSecret(ctx, host)
	if err != nil {
		return err
	}

	if tmpBMCSecret != nil {
		if tmpBMCSecret.Labels == nil {
			tmpBMCSecret.Labels = make(map[string]string)
		}
		tmpBMCSecret.Labels[capi.ClusterLabelName] = m.Machine.Spec.ClusterName
	}

	return m.client.Update(ctx, tmpBMCSecret)
}

// setHostLabel will set the set cluster.x-k8s.io/cluster-name to bmh
func (m *MachineManager) setHostLabel(ctx context.Context, host *bmh.BareMetalHost) error {

	if host.Labels == nil {
		host.Labels = make(map[string]string)
	}
	host.Labels[capi.ClusterLabelName] = m.Machine.Spec.ClusterName

	return m.client.Update(ctx, host)
}

// setHostSpec will ensure the host's Spec is set according to the machine's
// details. It will then update the host via the kube API. If UserData does not
// include a Namespace, it will default to the Machine's namespace.
func (m *MachineManager) setHostSpec(ctx context.Context, host *bmh.BareMetalHost) error {

	// We only want to update the image setting if the host does not
	// already have an image.
	//
	// A host with an existing image is already provisioned and
	// upgrades are not supported at this time. To re-provision a
	// host, we must fully deprovision it and then provision it again.
	// Not provisioning while we do not have the UserData
	if host.Spec.Image == nil && m.BareMetalMachine.Spec.UserData != nil {
		host.Spec.Image = &bmh.Image{
			URL:      m.BareMetalMachine.Spec.Image.URL,
			Checksum: m.BareMetalMachine.Spec.Image.Checksum,
		}
		host.Spec.UserData = m.BareMetalMachine.Spec.UserData
		if host.Spec.UserData != nil && host.Spec.UserData.Namespace == "" {
			host.Spec.UserData.Namespace = m.Machine.Namespace
		}
	}

	host.Spec.ConsumerRef = &corev1.ObjectReference{
		Kind:       "BareMetalMachine",
		Name:       m.BareMetalMachine.Name,
		Namespace:  m.BareMetalMachine.Namespace,
		APIVersion: m.BareMetalMachine.APIVersion,
	}

	host.Spec.Online = true
	return m.client.Update(ctx, host)
}

// ensureAnnotation makes sure the machine has an annotation that references the
// host and uses the API to update the machine if necessary.
func (m *MachineManager) ensureAnnotation(ctx context.Context, host *bmh.BareMetalHost) error {
	annotations := m.BareMetalMachine.ObjectMeta.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	hostKey, err := cache.MetaNamespaceKeyFunc(host)
	if err != nil {
		m.Log.Error(err, "Error parsing annotation value", "annotation key", hostKey)
		return err
	}
	existing, ok := annotations[HostAnnotation]
	if ok {
		if existing == hostKey {
			return nil
		}
		m.Log.Info("Warning: found stray annotation for host on machine. Overwriting.", "host", existing)
	}
	annotations[HostAnnotation] = hostKey
	m.BareMetalMachine.ObjectMeta.SetAnnotations(annotations)

	return m.client.Update(ctx, m.BareMetalMachine)
}

// HasAnnotation makes sure the machine has an annotation that references a host
func (m *MachineManager) HasAnnotation() bool {
	annotations := m.BareMetalMachine.ObjectMeta.GetAnnotations()
	if annotations == nil {
		return false
	}
	_, ok := annotations[HostAnnotation]
	return ok
}

// setError sets the ErrorMessage and ErrorReason fields on the machine and logs
// the message. It assumes the reason is invalid configuration, since that is
// currently the only relevant MachineStatusError choice.
func (m *MachineManager) setError(message string, reason capierrors.MachineStatusError) {
	m.BareMetalMachine.Status.FailureMessage = &message
	m.BareMetalMachine.Status.FailureReason = &reason
}

// clearError removes the ErrorMessage from the machine's Status if set. Returns
// nil if ErrorMessage was already nil. Returns a RequeueAfterError if the
// machine was updated.
func (m *MachineManager) clearError() {
	if m.BareMetalMachine.Status.FailureMessage != nil || m.BareMetalMachine.Status.FailureReason != nil {
		m.BareMetalMachine.Status.FailureMessage = nil
		m.BareMetalMachine.Status.FailureReason = nil
	}
}

// updateMachineStatus updates a machine object's status.
func (m *MachineManager) updateMachineStatus(ctx context.Context, host *bmh.BareMetalHost) error {
	addrs := m.nodeAddresses(host)

	machineCopy := m.BareMetalMachine.DeepCopy()
	machineCopy.Status.Addresses = addrs

	if equality.Semantic.DeepEqual(m.Machine.Status, machineCopy.Status) {
		// Status did not change
		return nil
	}

	now := metav1.Now()
	m.BareMetalMachine.Status.LastUpdated = &now
	m.BareMetalMachine.Status.Addresses = addrs

	return nil
}

// NodeAddresses returns a slice of corev1.NodeAddress objects for a
// given Baremetal machine.
func (m *MachineManager) nodeAddresses(host *bmh.BareMetalHost) []capi.MachineAddress {
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

type ClientGetter func(ctx context.Context, c client.Client, cluster *capi.Cluster) (clientcorev1.CoreV1Interface, error)

// SetNodeProviderID sets the bare metal provider ID on the kubernetes node
func (m *MachineManager) SetNodeProviderID(ctx context.Context, bmhID, providerID string, clientFactory ClientGetter) error {
	if !m.BareMetalCluster.Spec.NoCloudProvider {
		return nil
	}
	corev1Remote, err := clientFactory(ctx, m.client, m.Cluster)
	if err != nil {
		return errors.Wrap(err, "Error creating a remote client")
	}

	nodes, err := corev1Remote.Nodes().List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("metal3.io/uuid=%v", bmhID),
	})
	if err != nil {
		m.Log.Info(fmt.Sprintf("error while accessing cluster: %v", err))
		return &RequeueAfterError{RequeueAfter: requeueAfter}
	}
	if len(nodes.Items) == 0 {
		// The node could either be still running cloud-init or have been
		// deleted manually. TODO: handle a manual deletion case
		m.Log.Info("Target node is not found, requeuing")
		return &RequeueAfterError{RequeueAfter: requeueAfter}
	}
	for _, node := range nodes.Items {
		if node.Spec.ProviderID == providerID {
			continue
		}
		node.Spec.ProviderID = providerID
		_, err = corev1Remote.Nodes().Update(&node)
		if err != nil {
			return errors.Wrap(err, "unable to update the target node")
		}
	}
	m.Log.Info("ProviderID set on target node")

	return nil
}

// SetProviderID sets the bare metal provider ID on the BaremetalMachine
func (m *MachineManager) SetProviderID(providerID string) {
	m.BareMetalMachine.Spec.ProviderID = &providerID
	m.BareMetalMachine.Status.Ready = true
}

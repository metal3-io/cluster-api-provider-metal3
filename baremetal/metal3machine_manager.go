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

	bmh "github.com/metal3-io/baremetal-operator/pkg/apis/metal3/v1alpha1"
	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1alpha4"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/pointer"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// ProviderName is exported
	ProviderName = "metal3"
	// HostAnnotation is the key for an annotation that should go on a Metal3Machine to
	// reference what BareMetalHost it corresponds to.
	HostAnnotation      = "metal3.io/BareMetalHost"
	requeueAfter        = time.Second * 30
	bmRoleControlPlane  = "control-plane"
	bmRoleNode          = "node"
	pausedAnnotationKey = "metal3.io/capm3"
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
	SetPauseAnnotation(context.Context) error
	RemovePauseAnnotation(context.Context) error
	DissociateM3Metadata(context.Context) error
	AssociateM3Metadata(context.Context) error
	SetError(string, capierrors.MachineStatusError)
}

// MachineManager is responsible for performing machine reconciliation
type MachineManager struct {
	client client.Client

	Cluster       *capi.Cluster
	Metal3Cluster *capm3.Metal3Cluster
	Machine       *capi.Machine
	Metal3Machine *capm3.Metal3Machine
	Log           logr.Logger
}

// NewMachineManager returns a new helper for managing a machine
func NewMachineManager(client client.Client,
	cluster *capi.Cluster, metal3Cluster *capm3.Metal3Cluster,
	machine *capi.Machine, metal3machine *capm3.Metal3Machine,
	machineLog logr.Logger) (*MachineManager, error) {

	return &MachineManager{
		client: client,

		Cluster:       cluster,
		Metal3Cluster: metal3Cluster,
		Machine:       machine,
		Metal3Machine: metal3machine,
		Log:           machineLog,
	}, nil
}

// SetFinalizer sets finalizer
func (m *MachineManager) SetFinalizer() {
	// If the Metal3Machine doesn't have finalizer, add it.
	if !Contains(m.Metal3Machine.Finalizers, capm3.MachineFinalizer) {
		m.Metal3Machine.Finalizers = append(m.Metal3Machine.Finalizers,
			capm3.MachineFinalizer,
		)
	}
}

// UnsetFinalizer unsets finalizer
func (m *MachineManager) UnsetFinalizer() {
	// Cluster is deleted so remove the finalizer.
	m.Metal3Machine.Finalizers = Filter(m.Metal3Machine.Finalizers,
		capm3.MachineFinalizer,
	)
}

// IsProvisioned checks if the metal3machine is provisioned
func (m *MachineManager) IsProvisioned() bool {
	if m.Metal3Machine.Spec.ProviderID != nil && m.Metal3Machine.Status.Ready {
		return true
	}
	return false
}

// IsBootstrapReady checks if the machine is given Bootstrap data
func (m *MachineManager) IsBootstrapReady() bool {
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

// RemovePauseAnnotation checks and/or Removes the pause annotations on associated bmh
func (m *MachineManager) RemovePauseAnnotation(ctx context.Context) error {
	// look for associated BMH
	host, err := m.getHost(ctx)
	if err != nil {
		m.SetError("Failed to get a BaremetalHost for the Metal3Machine",
			capierrors.CreateMachineError,
		)
		return err
	}

	if host == nil {
		return nil
	}

	annotations := host.GetAnnotations()

	if annotations != nil {
		if _, ok := annotations[bmh.PausedAnnotation]; ok {
			if m.Cluster.Name == host.Labels[capi.ClusterLabelName] && annotations[bmh.PausedAnnotation] == pausedAnnotationKey {
				// Removing BMH Paused Annotation Since Owner Cluster is not paused
				delete(host.Annotations, bmh.PausedAnnotation)
			} else if m.Cluster.Name == host.Labels[capi.ClusterLabelName] && annotations[bmh.PausedAnnotation] != pausedAnnotationKey {
				m.Log.Info("BMH is paused by user. Not removing Pause Annotation")
				return nil
			}
		}
	}
	return updateObject(m.client, ctx, host)
}

// SetPauseAnnotation sets the pause annotations on associated bmh
func (m *MachineManager) SetPauseAnnotation(ctx context.Context) error {
	// look for associated BMH
	host, err := m.getHost(ctx)
	if err != nil {
		m.SetError("Failed to get a BaremetalHost for the Metal3Machine",
			capierrors.CreateMachineError,
		)
		return err
	}
	if host == nil {
		return nil
	}

	annotations := host.GetAnnotations()

	if annotations != nil {
		if _, ok := annotations[bmh.PausedAnnotation]; ok {
			m.Log.Info("BaremetalHost is already paused")
			return nil
		}
		m.Log.Info("Adding PausedAnnotation in BareMetalHost")
		host.Annotations[bmh.PausedAnnotation] = pausedAnnotationKey
	} else {
		host.Annotations = make(map[string]string)
		host.Annotations[bmh.PausedAnnotation] = pausedAnnotationKey
	}
	return updateObject(m.client, ctx, host)
}

// GetBaremetalHostID return the provider identifier for this machine
func (m *MachineManager) GetBaremetalHostID(ctx context.Context) (*string, error) {
	// look for associated BMH
	host, err := m.getHost(ctx)
	if err != nil {
		m.SetError("Failed to get a BaremetalHost for the Metal3Machine",
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
	if m.Metal3Machine == nil {
		// Should have been picked earlier. Do not requeue
		return nil
	}

	config := m.Metal3Machine.Spec
	err := config.IsValid()
	if err != nil {
		// Should have been picked earlier. Do not requeue
		m.SetError(err.Error(), capierrors.InvalidConfigurationMachineError)
		return nil
	}

	// clear an error if one was previously set
	m.clearError()

	// look for associated BMH
	host, err := m.getHost(ctx)
	if err != nil {
		m.SetError("Failed to get the BaremetalHost for the Metal3Machine",
			capierrors.CreateMachineError,
		)
		return err
	}

	// no BMH found, trying to choose from available ones
	if host == nil {
		host, err = m.chooseHost(ctx)
		if err != nil {
			m.SetError("Failed to pick a BaremetalHost for the Metal3Machine",
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
	err = m.getUserData(ctx, host)
	if err != nil {
		if _, ok := err.(HasRequeueAfterError); !ok {
			m.SetError("Failed to set the UserData for the Metal3Machine",
				capierrors.CreateMachineError,
			)
		}
		return err
	}

	err = m.setHostLabel(ctx, host)
	if err != nil {
		if _, ok := err.(HasRequeueAfterError); !ok {
			m.SetError("Failed to set the Cluster label in the BareMetalHost",
				capierrors.CreateMachineError,
			)
		}
		return err
	}

	err = m.setHostConsumerRef(ctx, host)
	if err != nil {
		if _, ok := err.(HasRequeueAfterError); !ok {
			m.SetError("Failed to associate the BaremetalHost to the Metal3Machine",
				capierrors.CreateMachineError,
			)
		}
		return err
	}

	// If the user did not provide a DataTemplate, we can directly set the host
	// specs, nothing to wait for.
	if m.Metal3Machine.Spec.DataTemplate == nil {
		if err = m.setHostSpec(ctx, host); err != nil {
			if _, ok := err.(HasRequeueAfterError); !ok {
				m.SetError("Failed to associate the BaremetalHost to the Metal3Machine",
					capierrors.CreateMachineError,
				)
			}
			return err
		}
	}

	err = m.setBMCSecretLabel(ctx, host)
	if err != nil {
		if _, ok := err.(HasRequeueAfterError); !ok {
			m.SetError("Failed to associate the BaremetalHost to the Metal3Machine",
				capierrors.CreateMachineError,
			)
		}
		return err
	}

	err = updateObject(m.client, ctx, host)
	if err != nil {
		return err
	}

	err = m.ensureAnnotation(ctx, host)
	if err != nil {
		if _, ok := err.(HasRequeueAfterError); !ok {
			m.SetError("Failed to annotate the Metal3Machine",
				capierrors.CreateMachineError,
			)
		}
		return err
	}

	if m.Metal3Machine.Spec.DataTemplate != nil {
		// Requeue to get the DataTemplate output. We need to requeue to trigger the
		// wait on the Metal3DataTemplate
		if err := m.WaitForM3Metadata(ctx); err != nil {
			return err
		}

		// If the requeue is not needed, then set the host specs
		if err = m.setHostSpec(ctx, host); err != nil {
			if _, ok := err.(HasRequeueAfterError); !ok {
				m.SetError("Failed to set the BaremetalHost Specs",
					capierrors.CreateMachineError,
				)
			}
			return err
		}

		// Update the BMH object.
		err = updateObject(m.client, ctx, host)
		if err != nil {
			return err
		}
	}

	m.Log.Info("Finished creating machine")
	return nil
}

// getUserData gets the UserData from the machine and exposes it as a secret
// for the BareMetalHost. The UserData might already be in a secret with
// CABPK v0.3.0+, but if it is in a different namespace than the BareMetalHost,
// then we need to create the secret. Same as if the UserData is in the data
// field.
func (m *MachineManager) getUserData(ctx context.Context, host *bmh.BareMetalHost) error {
	var err error
	var decodedUserDataBytes []byte

	if m.Metal3Machine.Status.UserData != nil {
		return nil
	}

	if m.Metal3Machine.Spec.UserData != nil {
		m.Metal3Machine.Status.UserData = m.Metal3Machine.Spec.UserData
	}

	// if datasecretname is set just pass the reference
	if m.Machine.Spec.Bootstrap.DataSecretName != nil {
		m.Metal3Machine.Status.UserData = &corev1.SecretReference{
			Name:      *m.Machine.Spec.Bootstrap.DataSecretName,
			Namespace: m.Machine.Namespace,
		}
		return nil

	} else if m.Machine.Spec.Bootstrap.Data == nil {
		// If we do not have DataSecretName or Data then exit
		return nil

	}

	// If we have Data, use it
	decodedUserData := *m.Machine.Spec.Bootstrap.Data
	// decode the base64 cloud-config
	decodedUserDataBytes, err = base64.StdEncoding.DecodeString(decodedUserData)
	if err != nil {
		return err
	}

	err = m.createSecret(ctx, m.Metal3Machine.Name+"-user-data",
		m.Metal3Machine.Namespace, map[string][]byte{
			"userData": decodedUserDataBytes,
		},
	)
	if err != nil {
		return err
	}

	m.Metal3Machine.Status.UserData = &corev1.SecretReference{
		Name:      m.Metal3Machine.Name + "-user-data",
		Namespace: host.Namespace,
	}

	return nil
}

func (m *MachineManager) createSecret(ctx context.Context, name string,
	namespace string, content map[string][]byte,
) error {

	err := createSecret(m.client, ctx, name, namespace,
		m.Machine.Spec.ClusterName,
		[]metav1.OwnerReference{metav1.OwnerReference{
			Controller: pointer.BoolPtr(true),
			APIVersion: m.Metal3Machine.APIVersion,
			Kind:       m.Metal3Machine.Kind,
			Name:       m.Metal3Machine.Name,
			UID:        m.Metal3Machine.UID,
		}}, content,
	)

	if err != nil {
		if _, ok := err.(HasRequeueAfterError); !ok {
			m.Log.Info("Unable to create secret for bootstrap")
		}
		return err
	}
	return nil
}

// Delete deletes a metal3 machine and is invoked by the Machine Controller
func (m *MachineManager) Delete(ctx context.Context) error {
	m.Log.Info("Deleting metal3 machine", "metal3machine", m.Metal3Machine.Name)

	// clear an error if one was previously set
	m.clearError()

	host, err := m.getHost(ctx)
	if err != nil {
		return err
	}

	if host != nil && host.Spec.ConsumerRef != nil {
		// don't remove the ConsumerRef if it references some other  metal3 machine
		if !consumerRefMatches(host.Spec.ConsumerRef, m.Metal3Machine) {
			m.Log.Info("host already associated with another metal3 machine",
				"host", host.Name)
			// Remove the ownerreference to this machine, even if the consumer ref
			// references another machine.
			host.OwnerReferences, err = m.DeleteOwnerRef(host.OwnerReferences)
			if err != nil {
				return err
			}
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
				errBMC = updateObject(m.client, ctx, tmpBMCSecret)
				if errBMC != nil {
					if _, ok := errBMC.(HasRequeueAfterError); !ok {
						m.Log.Info("Failed to delete the clusterLabel from BMC Secret")
					}
					return errBMC
				}
			}
		}

		if host.Spec.Image != nil || host.Spec.Online || host.Spec.UserData != nil {
			host.Spec.Image = nil
			host.Spec.Online = false
			if m.Metal3Machine.Status.UserData != nil {
				host.Spec.UserData = nil
			}
			if m.Metal3Machine.Status.MetaData != nil {
				host.Spec.MetaData = nil
			}
			if m.Metal3Machine.Status.NetworkData != nil {
				host.Spec.NetworkData = nil
			}

			err = updateObject(m.client, ctx, host)
			if err != nil && !apierrors.IsNotFound(err) {
				if _, ok := err.(HasRequeueAfterError); !ok {
					m.SetError("Failed to delete Metal3Machine",
						capierrors.DeleteMachineError,
					)
				}
				return err
			}
			m.Log.Info("Deprovisioning BaremetalHost, requeuing")
			return &RequeueAfterError{}
		}

		waiting := true
		switch host.Status.Provisioning.State {
		case bmh.StateRegistrationError, bmh.StateRegistering,
			bmh.StateMatchProfile, bmh.StateInspecting,
			bmh.StateReady, bmh.StateAvailable, bmh.StateNone:
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

		// Remove the ownerreference to this machine
		host.OwnerReferences, err = m.DeleteOwnerRef(host.OwnerReferences)
		if err != nil {
			return err
		}

		if host.Labels != nil && host.Labels[capi.ClusterLabelName] == m.Machine.Spec.ClusterName {
			delete(host.Labels, capi.ClusterLabelName)
		}

		m.Log.Info("Removing Paused Annotation (if any)")
		if host.Annotations != nil && host.Annotations[bmh.PausedAnnotation] == pausedAnnotationKey {
			delete(host.Annotations, bmh.PausedAnnotation)
		}

		// Delete created secret, if data was set without DataSecretName but with
		// Data
		if m.Machine.Spec.Bootstrap.DataSecretName == nil &&
			m.Machine.Spec.Bootstrap.Data != nil {
			m.Log.Info("Deleting User data secret for machine")
			if m.Metal3Machine.Status.UserData != nil {
				err = deleteSecret(m.client, ctx, m.Metal3Machine.Status.UserData.Name,
					m.Metal3Machine.Namespace,
				)
				if err != nil {
					if _, ok := err.(HasRequeueAfterError); !ok {
						m.SetError("Failed to delete userdata secret",
							capierrors.DeleteMachineError,
						)
					}
					return err
				}
			}
		}

		host.Spec.ConsumerRef = nil

		// Remove the ownerreference to this machine
		host.OwnerReferences, err = m.DeleteOwnerRef(host.OwnerReferences)
		if err != nil {
			return err
		}

		if host.Labels != nil && host.Labels[capi.ClusterLabelName] == m.Machine.Spec.ClusterName {
			delete(host.Labels, capi.ClusterLabelName)
		}

		err = updateObject(m.client, ctx, host)
		if err != nil && !apierrors.IsNotFound(err) {
			if _, ok := err.(HasRequeueAfterError); !ok {
				m.SetError("Failed to delete Metal3Machine",
					capierrors.DeleteMachineError,
				)
			}
			return err
		}
	}

	m.Log.Info("finished deleting metal3 machine")
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

	if err := m.WaitForM3Metadata(ctx); err != nil {
		if _, ok := err.(HasRequeueAfterError); !ok {
			m.SetError("Failed to get the DataTemplate",
				capierrors.CreateMachineError,
			)
		}
		return err
	}

	// ensure that the BMH specs are correctly set
	err = m.setHostConsumerRef(ctx, host)
	if err != nil {
		if _, ok := err.(HasRequeueAfterError); !ok {
			m.SetError("Failed to associate the BaremetalHost to the Metal3Machine",
				capierrors.CreateMachineError,
			)
		}
		return err
	}

	// ensure that the BMH specs are correctly set
	err = m.setHostSpec(ctx, host)
	if err != nil {
		if _, ok := err.(HasRequeueAfterError); !ok {
			m.SetError("Failed to associate the BaremetalHost to the Metal3Machine",
				capierrors.CreateMachineError,
			)
		}
		return err
	}

	err = updateObject(m.client, ctx, host)
	if err != nil {
		return err
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

// exists tests for the existence of a baremetalHost
func (m *MachineManager) exists(ctx context.Context) (bool, error) {
	m.Log.Info("Checking if host exists.")
	host, err := m.getHost(ctx)
	if err != nil {
		return false, err
	}
	if host == nil {
		m.Log.Info("Host does not exist.")
		return false, nil
	}
	m.Log.Info("Host exists.")
	return true, nil
}

// getHost gets the associated host by looking for an annotation on the machine
// that contains a reference to the host. Returns nil if not found. Assumes the
// host is in the same namespace as the machine.
func (m *MachineManager) getHost(ctx context.Context) (*bmh.BareMetalHost, error) {
	return getHost(ctx, m.Metal3Machine, m.client, m.Log)
}

func getHost(ctx context.Context, m3Machine *capm3.Metal3Machine, cl client.Client,
	mLog logr.Logger,
) (*bmh.BareMetalHost, error) {
	annotations := m3Machine.ObjectMeta.GetAnnotations()
	if annotations == nil {
		return nil, nil
	}
	hostKey, ok := annotations[HostAnnotation]
	if !ok {
		return nil, nil
	}
	hostNamespace, hostName, err := cache.SplitMetaNamespaceKey(hostKey)
	if err != nil {
		mLog.Error(err, "Error parsing annotation value", "annotation key", hostKey)
		return nil, err
	}

	host := bmh.BareMetalHost{}
	key := client.ObjectKey{
		Name:      hostName,
		Namespace: hostNamespace,
	}
	err = cl.Get(ctx, key, &host)
	if apierrors.IsNotFound(err) {
		mLog.Info("Annotated host not found", "host", hostKey)
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return &host, nil
}

// chooseHost iterates through known hosts and returns one that can be
// associated with the metal3 machine. It searches all hosts in case one already has an
// association with this metal3 machine.
func (m *MachineManager) chooseHost(ctx context.Context) (*bmh.BareMetalHost, error) {

	// get list of BMH
	hosts := bmh.BareMetalHostList{}
	// without this ListOption, all namespaces would be including in the listing
	opts := &client.ListOptions{
		Namespace: m.Metal3Machine.Namespace,
	}

	err := m.client.List(ctx, &hosts, opts)
	if err != nil {
		return nil, err
	}

	// Using the label selector on ListOptions above doesn't seem to work.
	// I think it's because we have a local cache of all BareMetalHosts.
	labelSelector := labels.NewSelector()
	var reqs labels.Requirements

	for labelKey, labelVal := range m.Metal3Machine.Spec.HostSelector.MatchLabels {
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
	for _, req := range m.Metal3Machine.Spec.HostSelector.MatchExpressions {
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
		if host.Spec.ConsumerRef != nil && consumerRefMatches(host.Spec.ConsumerRef, m.Metal3Machine) {
			m.Log.Info("Found host with existing ConsumerRef", "host", host.Name)
			return &hosts.Items[i], nil
		}
		if !host.Available() {
			continue
		}
		switch host.Status.Provisioning.State {
		case bmh.StateReady, bmh.StateAvailable:
		default:
			continue
		}
		// continue if BaremetalHost is paused
		annotations := host.GetAnnotations()
		if annotations != nil {
			if _, ok := annotations[bmh.PausedAnnotation]; ok {
				continue
			}
		}
		if labelSelector.Matches(labels.Set(host.ObjectMeta.Labels)) {
			m.Log.Info("Host matched hostSelector for Metal3Machine", "host", host.Name)
			availableHosts = append(availableHosts, &hosts.Items[i])
		} else {
			m.Log.Info("Host did not match hostSelector for Metal3Machine", "host", host.Name)
		}
	}
	m.Log.Info(fmt.Sprintf("%d hosts available while choosing host for Metal3 machine", len(availableHosts)))
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
func consumerRefMatches(consumer *corev1.ObjectReference, bmmachine *capm3.Metal3Machine) bool {
	if consumer.Name != bmmachine.Name {
		return false
	}
	if consumer.Namespace != bmmachine.Namespace {
		return false
	}
	if consumer.Kind != bmmachine.Kind {
		return false
	}
	if consumer.GroupVersionKind().Group != bmmachine.GroupVersionKind().Group {
		return false
	}
	return true
}

// getBMCSecret will return the BMCSecret associated with BMH
func (m *MachineManager) getBMCSecret(ctx context.Context, host *bmh.BareMetalHost) (*corev1.Secret, error) {

	if host.Spec.BMC.CredentialsName == "" {
		return nil, nil
	}
	tmpBMCSecret := corev1.Secret{}
	key := host.CredentialsKey()
	err := m.client.Get(ctx, key, &tmpBMCSecret)
	if err != nil {
		m.Log.Info("Cannot retrieve BMC credential for BareMetalhost ", host.Name, err)
		return nil, err
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
		return updateObject(m.client, ctx, tmpBMCSecret)
	}

	return nil
}

// setHostLabel will set the set cluster.x-k8s.io/cluster-name to bmh
func (m *MachineManager) setHostLabel(ctx context.Context, host *bmh.BareMetalHost) error {

	if host.Labels == nil {
		host.Labels = make(map[string]string)
	}
	host.Labels[capi.ClusterLabelName] = m.Machine.Spec.ClusterName

	return nil
}

// setHostSpec will ensure the host's Spec is set according to the machine's
// details. It will then update the host via the kube API. If UserData does not
// include a Namespace, it will default to the Metal3Machine's namespace.
func (m *MachineManager) setHostSpec(ctx context.Context, host *bmh.BareMetalHost) error {

	// We only want to update the image setting if the host does not
	// already have an image.
	//
	// A host with an existing image is already provisioned and
	// upgrades are not supported at this time. To re-provision a
	// host, we must fully deprovision it and then provision it again.
	// Not provisioning while we do not have the UserData
	if host.Spec.Image == nil && m.Metal3Machine.Status.UserData != nil {
		host.Spec.Image = &bmh.Image{
			URL:      m.Metal3Machine.Spec.Image.URL,
			Checksum: m.Metal3Machine.Spec.Image.Checksum,
		}
		host.Spec.UserData = m.Metal3Machine.Status.UserData
		if host.Spec.UserData != nil && host.Spec.UserData.Namespace == "" {
			host.Spec.UserData.Namespace = host.Namespace
		}

		// Set metadata from gathering from Spec.metadata and from the template.
		if m.Metal3Machine.Status.MetaData != nil {
			host.Spec.MetaData = m.Metal3Machine.Status.MetaData
		}
		if host.Spec.MetaData != nil && host.Spec.MetaData.Namespace == "" {
			host.Spec.MetaData.Namespace = m.Machine.Namespace
		}
		if m.Metal3Machine.Status.NetworkData != nil {
			host.Spec.NetworkData = m.Metal3Machine.Status.NetworkData
		}
		if host.Spec.NetworkData != nil && host.Spec.NetworkData.Namespace == "" {
			host.Spec.NetworkData.Namespace = m.Machine.Namespace
		}
	}

	host.Spec.Online = true

	return nil
}

// setHostConsumerRef will ensure the host's Spec is set to link to this
// Metal3Machine
func (m *MachineManager) setHostConsumerRef(ctx context.Context, host *bmh.BareMetalHost) error {

	host.Spec.ConsumerRef = &corev1.ObjectReference{
		Kind:       "Metal3Machine",
		Name:       m.Metal3Machine.Name,
		Namespace:  m.Metal3Machine.Namespace,
		APIVersion: m.Metal3Machine.APIVersion,
	}

	// Set OwnerReferences
	hostOwnerReferences, err := m.SetOwnerRef(host.OwnerReferences, true)
	if err != nil {
		return err
	}
	host.OwnerReferences = hostOwnerReferences

	return nil
}

// ensureAnnotation makes sure the machine has an annotation that references the
// host and uses the API to update the machine if necessary.
func (m *MachineManager) ensureAnnotation(ctx context.Context, host *bmh.BareMetalHost) error {
	annotations := m.Metal3Machine.ObjectMeta.GetAnnotations()
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
	m.Metal3Machine.ObjectMeta.SetAnnotations(annotations)

	return nil
}

// HasAnnotation makes sure the machine has an annotation that references a host
func (m *MachineManager) HasAnnotation() bool {
	annotations := m.Metal3Machine.ObjectMeta.GetAnnotations()
	if annotations == nil {
		return false
	}
	_, ok := annotations[HostAnnotation]
	return ok
}

// SetError sets the ErrorMessage and ErrorReason fields on the machine and logs
// the message. It assumes the reason is invalid configuration, since that is
// currently the only relevant MachineStatusError choice.
func (m *MachineManager) SetError(message string, reason capierrors.MachineStatusError) {
	m.Metal3Machine.Status.FailureMessage = &message
	m.Metal3Machine.Status.FailureReason = &reason
}

// clearError removes the ErrorMessage from the machine's Status if set. Returns
// nil if ErrorMessage was already nil. Returns a RequeueAfterError if the
// machine was updated.
func (m *MachineManager) clearError() {
	if m.Metal3Machine.Status.FailureMessage != nil || m.Metal3Machine.Status.FailureReason != nil {
		m.Metal3Machine.Status.FailureMessage = nil
		m.Metal3Machine.Status.FailureReason = nil
	}
}

// updateMachineStatus updates a machine object's status.
func (m *MachineManager) updateMachineStatus(ctx context.Context, host *bmh.BareMetalHost) error {
	addrs := m.nodeAddresses(host)

	machineCopy := m.Metal3Machine.DeepCopy()
	machineCopy.Status.Addresses = addrs

	if equality.Semantic.DeepEqual(m.Machine.Status, machineCopy.Status) {
		// Status did not change
		return nil
	}

	now := metav1.Now()
	m.Metal3Machine.Status.LastUpdated = &now
	m.Metal3Machine.Status.Addresses = addrs

	return nil
}

// NodeAddresses returns a slice of corev1.NodeAddress objects for a
// given Metal3 machine.
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

// ClientGetter prototype
type ClientGetter func(ctx context.Context, c client.Client, cluster *capi.Cluster) (clientcorev1.CoreV1Interface, error)

// SetNodeProviderID sets the metal3 provider ID on the kubernetes node
func (m *MachineManager) SetNodeProviderID(ctx context.Context, bmhID, providerID string, clientFactory ClientGetter) error {
	if !m.Metal3Cluster.Spec.NoCloudProvider {
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

// SetProviderID sets the metal3 provider ID on the metal3machine
func (m *MachineManager) SetProviderID(providerID string) {
	m.Metal3Machine.Spec.ProviderID = &providerID
	m.Metal3Machine.Status.Ready = true
}

// SetOwnerRef adds an ownerreference to this Metal3 machine
func (m *MachineManager) SetOwnerRef(refList []metav1.OwnerReference, controller bool) ([]metav1.OwnerReference, error) {
	return setOwnerRefInList(refList, controller, m.Metal3Machine.TypeMeta,
		m.Metal3Machine.ObjectMeta,
	)
}

// DeleteOwnerRef removes the ownerreference to this Metal3 machine
func (m *MachineManager) DeleteOwnerRef(refList []metav1.OwnerReference) ([]metav1.OwnerReference, error) {
	if len(refList) == 0 {
		return refList, nil
	}
	index, err := m.FindOwnerRef(refList)
	if err != nil {
		if _, ok := err.(*NotFoundError); !ok {
			return nil, err
		}
		return refList, nil
	}
	if len(refList) == 1 {
		return []metav1.OwnerReference{}, nil
	}
	refListLen := len(refList) - 1
	refList[index] = refList[refListLen]
	refList, err = m.DeleteOwnerRef(refList[:refListLen-1])
	if err != nil {
		return nil, err
	}
	return refList, nil
}

// FindOwnerRef checks if an ownerreference to this Metal3 machine exists
// and returns the index
func (m *MachineManager) FindOwnerRef(refList []metav1.OwnerReference) (int, error) {
	return findOwnerRefFromList(refList, m.Metal3Machine.TypeMeta,
		m.Metal3Machine.ObjectMeta,
	)
}

// SetOwnerRef adds an ownerreference to this Metal3 machine
func setOwnerRefInList(refList []metav1.OwnerReference, controller bool,
	objType metav1.TypeMeta, objMeta metav1.ObjectMeta,
) ([]metav1.OwnerReference, error) {
	index, err := findOwnerRefFromList(refList, objType, objMeta)
	if err != nil {
		if _, ok := err.(*NotFoundError); !ok {
			return nil, err
		}
		refList = append(refList, metav1.OwnerReference{
			APIVersion: objType.APIVersion,
			Kind:       objType.Kind,
			Name:       objMeta.Name,
			UID:        objMeta.UID,
			Controller: pointer.BoolPtr(controller),
		})
	} else {
		//The UID and the APIVersion might change due to move or version upgrade
		refList[index].APIVersion = objType.APIVersion
		refList[index].UID = objMeta.UID
		refList[index].Controller = pointer.BoolPtr(controller)
	}
	return refList, nil
}

func findOwnerRefFromList(refList []metav1.OwnerReference, objType metav1.TypeMeta,
	objMeta metav1.ObjectMeta,
) (int, error) {

	for i, curOwnerRef := range refList {
		aGV, err := schema.ParseGroupVersion(curOwnerRef.APIVersion)
		if err != nil {
			return 0, err
		}

		bGV, err := schema.ParseGroupVersion(objType.APIVersion)
		if err != nil {
			return 0, err
		}
		// not matching on UID since when pivoting it might change
		// Not matching on API version as this might change
		if curOwnerRef.Name == objMeta.Name &&
			curOwnerRef.Kind == objType.Kind &&
			aGV.Group == bGV.Group {
			return i, nil
		}
	}
	return 0, &NotFoundError{}
}

// RetrieveMetadata fetches the Metal3DataTemplate object and sets the
// owner references
func (m *MachineManager) AssociateM3Metadata(ctx context.Context) error {
	// If the secrets were provided by the user, use them
	if m.Metal3Machine.Spec.MetaData != nil {
		m.Metal3Machine.Status.MetaData = m.Metal3Machine.Spec.MetaData
	}
	if m.Metal3Machine.Spec.NetworkData != nil {
		m.Metal3Machine.Status.NetworkData = m.Metal3Machine.Spec.NetworkData
	}

	// If we have RenderedData set already, it means that the owner reference was
	// already set
	if m.Metal3Machine.Status.RenderedData != nil {
		return nil
	}

	if m.Metal3Machine.Spec.DataTemplate.Namespace == "" {
		m.Metal3Machine.Spec.DataTemplate.Namespace = m.Metal3Machine.Namespace
	}
	metal3DataTemplate, err := fetchM3DataTemplate(ctx,
		m.Metal3Machine.Spec.DataTemplate, m.client, m.Log,
		m.Machine.Spec.ClusterName,
	)
	if err != nil {
		return err
	}
	if metal3DataTemplate == nil {
		return nil
	}

	if _, err := m.FindOwnerRef(metal3DataTemplate.OwnerReferences); err != nil {
		// If the error is not NotFound, return the error
		if _, ok := err.(*NotFoundError); !ok {
			return err
		}

		// Set the owner ref and the cluster label.
		metal3DataTemplate.OwnerReferences, err = m.SetOwnerRef(metal3DataTemplate.OwnerReferences, false)
		if err != nil {
			return err
		}
		if metal3DataTemplate.Labels == nil {
			metal3DataTemplate.Labels = make(map[string]string)
		}
		metal3DataTemplate.Labels[capi.ClusterLabelName] = m.Machine.Spec.ClusterName
		err = m.updateM3Metadata(ctx, metal3DataTemplate)
		if err != nil {
			return err
		}
	}
	return nil
}

// WaitForM3Metadata fetches the Metal3DataTemplate object and sets the
// owner references
func (m *MachineManager) WaitForM3Metadata(ctx context.Context) error {

	// If we do not have RenderedData set yet, try to find it in
	// Metal3DataTemplate. If it is not there yet, it means that the reconciliation
	// of Metal3DataTemplate did not yet complete, requeue.
	if m.Metal3Machine.Status.RenderedData == nil {
		metal3DataTemplate, err := fetchM3DataTemplate(ctx,
			m.Metal3Machine.Spec.DataTemplate, m.client, m.Log,
			m.Machine.Spec.ClusterName,
		)
		if err != nil {
			return err
		}
		if metal3DataTemplate == nil {
			return nil
		}

		if dataName, ok := metal3DataTemplate.Status.DataNames[m.Metal3Machine.Name]; ok {
			m.Metal3Machine.Status.RenderedData = &corev1.ObjectReference{
				Name:      dataName,
				Namespace: metal3DataTemplate.Namespace,
			}
		} else {
			return &RequeueAfterError{RequeueAfter: requeueAfter}
		}
	}

	// Fetch the Metal3Data
	metal3Data, err := fetchM3Data(ctx, m.client, m.Log,
		m.Metal3Machine.Status.RenderedData.Name, m.Metal3Machine.Namespace,
	)
	if err != nil {
		return err
	}
	if metal3Data == nil {
		return errors.New("Unexpected nil rendered data")
	}

	// If it is not ready yet, wait.
	if !metal3Data.Status.Ready {
		// Secret generation not ready
		return &RequeueAfterError{RequeueAfter: requeueAfter}
	}

	// Get the secrets if given in Metal3Data and not already set
	if m.Metal3Machine.Status.MetaData == nil &&
		metal3Data.Spec.MetaData != nil {
		if metal3Data.Spec.MetaData.Name != "" {
			m.Metal3Machine.Status.MetaData = &corev1.SecretReference{
				Name:      metal3Data.Spec.MetaData.Name,
				Namespace: metal3Data.Namespace,
			}
		}
	}

	if m.Metal3Machine.Status.NetworkData == nil &&
		metal3Data.Spec.NetworkData != nil {
		if metal3Data.Spec.NetworkData.Name != "" {
			m.Metal3Machine.Status.NetworkData = &corev1.SecretReference{
				Name:      metal3Data.Spec.NetworkData.Name,
				Namespace: metal3Data.Namespace,
			}
		}
	}

	return nil
}

// remove machine from OwnerReferences of meta3DataTemplate, on failure requeue
func (m *MachineManager) DissociateM3Metadata(ctx context.Context) error {
	// Get the Metal3DataTemplate object
	metal3DataTemplate, err := fetchM3DataTemplate(ctx,
		m.Metal3Machine.Spec.DataTemplate, m.client, m.Log,
		m.Machine.Spec.ClusterName,
	)
	if err != nil {
		if _, ok := err.(HasRequeueAfterError); !ok {
			return err
		}
		return nil
	}
	if metal3DataTemplate == nil {
		return nil
	}

	// Remove the ownerreference if it is set.
	if _, err := m.FindOwnerRef(metal3DataTemplate.OwnerReferences); err == nil {
		metal3DataTemplate.OwnerReferences, err = m.DeleteOwnerRef(
			metal3DataTemplate.OwnerReferences,
		)
		if err != nil {
			return err
		}
		err = m.updateM3Metadata(ctx, metal3DataTemplate)
		if err != nil {
			return err
		}
	} else {
		if _, ok := err.(*NotFoundError); !ok {
			return err
		}
	}

	return nil
}

// updateMetadata updates the Metal3DataTemplate object
func (m *MachineManager) updateM3Metadata(ctx context.Context, metal3DataTemplate *capm3.Metal3DataTemplate) error {
	if err := m.client.Update(ctx, metal3DataTemplate); err != nil {
		if apierrors.IsConflict(err) {
			m.Log.Info("Conflict on Metadata update, requeuing")
			return &RequeueAfterError{RequeueAfter: requeueAfter}
		} else {
			return errors.Wrap(err, "Failed to update metadata")
		}
	}
	return nil
}

/*
Copyright 2021 The Kubernetes Authors.

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
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strings"
	"sync"
	"time"

	// comment for go-lint.
	"github.com/go-logr/logr"
	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// ProviderName is exported.
	ProviderName = "metal3"
	// HostAnnotation is the key for an annotation that should go on a Metal3Machine to
	// reference what BareMetalHost it corresponds to.
	HostAnnotation = "metal3.io/BareMetalHost"
	// nodeReuseLabelName is the label set on BMH when node reuse feature is enabled.
	nodeReuseLabelName = "infrastructure.cluster.x-k8s.io/node-reuse"
	requeueAfter       = time.Second * 30
	bmRoleControlPlane = "control-plane"
	bmRoleNode         = "node"
	// PausedAnnotationKey is an annotation to be used for pausing a BMH.
	PausedAnnotationKey = "metal3.io/capm3"
	// ProviderIDPrefix is a prefix for ProviderID.
	ProviderIDPrefix = "metal3://"
	// ProviderLabelPrefix is a label prefix for ProviderID.
	ProviderLabelPrefix = "metal3.io/uuid"
)

var (
	// Capm3FastTrack is the variable fetched from the CAPM3_FAST_TRACK environment variable.
	Capm3FastTrack    = os.Getenv("CAPM3_FAST_TRACK")
	errNotFound       *NotFoundError
	associateBMHMutex sync.Mutex
)

// MachineManagerInterface is an interface for a MachineManager.
type MachineManagerInterface interface {
	SetFinalizer()
	UnsetFinalizer()
	IsProvisioned() bool
	IsBaremetalHostProvisioned(context.Context) bool
	IsBootstrapReady() bool
	MachineHasNodeRef() bool
	Associate(context.Context) error
	Delete(context.Context) error
	Update(context.Context) error
	HasAnnotation() bool
	GetProviderIDAndBMHID() (string, *string)
	SetProviderID(string)
	SetDefaultProviderID() error
	SetProviderIDFromCloudProviderNode(context.Context, ClientGetter) error
	SetProviderIDFromNodeLabel(context.Context, ClientGetter) (bool, error)
	SetNodeProviderIDByHostname(context.Context, ClientGetter) error
	NodeWithMatchingProviderIDExists(context.Context, ClientGetter) bool
	Metal3MachineHasProviderID() bool
	SetPauseAnnotation(context.Context) error
	RemovePauseAnnotation(context.Context) error
	DissociateM3Metadata(context.Context) error
	AssociateM3Metadata(context.Context) error
	SetError(string, capierrors.MachineStatusError)
	SetConditionMetal3MachineToFalse(clusterv1.ConditionType, string, clusterv1.ConditionSeverity, string, ...interface{})
	SetConditionMetal3MachineToTrue(clusterv1.ConditionType)
	CloudProviderEnabled() bool
	SetReadyTrue()
}

// MachineManager is responsible for performing machine reconciliation.
type MachineManager struct {
	client client.Client

	Cluster               *clusterv1.Cluster
	Metal3Cluster         *infrav1.Metal3Cluster
	MachineList           *clusterv1.MachineList
	Machine               *clusterv1.Machine
	Metal3Machine         *infrav1.Metal3Machine
	Metal3MachineTemplate *infrav1.Metal3MachineTemplate
	MachineSet            *clusterv1.MachineSet
	MachineSetList        *clusterv1.MachineSetList
	Log                   logr.Logger
}

// NewMachineManager returns a new helper for managing a machine.
func NewMachineManager(client client.Client,
	cluster *clusterv1.Cluster, metal3Cluster *infrav1.Metal3Cluster,
	machine *clusterv1.Machine, metal3machine *infrav1.Metal3Machine,
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

// NewMachineSetManager returns a new helper for managing a machineset.
func NewMachineSetManager(client client.Client,
	machine *clusterv1.Machine, machineSetList *clusterv1.MachineSetList,
	machineLog logr.Logger) (*MachineManager, error) {
	return &MachineManager{
		client:         client,
		Machine:        machine,
		MachineSetList: machineSetList,
		Log:            machineLog,
	}, nil
}

// SetFinalizer sets finalizer.
func (m *MachineManager) SetFinalizer() {
	// If the Metal3Machine doesn't have finalizer, add it.
	if !controllerutil.ContainsFinalizer(m.Metal3Machine, infrav1.MachineFinalizer) {
		controllerutil.AddFinalizer(m.Metal3Machine, infrav1.MachineFinalizer)
	}
}

// UnsetFinalizer unsets finalizer.
func (m *MachineManager) UnsetFinalizer() {
	// Cluster is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(m.Metal3Machine, infrav1.MachineFinalizer)
}

// IsProvisioned checks if the metal3machine is provisioned.
func (m *MachineManager) IsProvisioned() bool {
	if m.Metal3Machine.Spec.ProviderID != nil && m.Metal3Machine.Status.Ready {
		return true
	}
	return false
}

// IsBaremetalHostProvisioned returns true if the provisioning state of the underlying baremetalhost is `Provisioned`.
func (m *MachineManager) IsBaremetalHostProvisioned(ctx context.Context) bool {
	m.Log.Info("checking if baremetalhost is provisioned")
	host, _, err := m.getHost(ctx)
	if err != nil {
		m.Log.Info("failed to get host", "err", err)
		return false
	}
	if host == nil {
		m.Log.Info("getHost returned nil")
		return false
	}
	return host.Status.Provisioning.State == bmov1alpha1.StateProvisioned
}

// IsBootstrapReady checks if the machine is given Bootstrap data.
func (m *MachineManager) IsBootstrapReady() bool {
	if m.Machine.Spec.Bootstrap.ConfigRef != nil {
		return m.Machine.Status.BootstrapReady
	}

	return m.Machine.Spec.Bootstrap.DataSecretName != nil
}

func (m *MachineManager) MachineHasNodeRef() bool {
	return m.Machine.Status.NodeRef != nil
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

// RemovePauseAnnotation checks and/or Removes the pause annotations on associated bmh.
func (m *MachineManager) RemovePauseAnnotation(ctx context.Context) error {
	// look for associated BMH
	host, helper, err := m.getHost(ctx)
	if err != nil {
		errMessage := "Failed to get a BaremetalHost for the Metal3Machine, requeuing"
		m.Log.Info(errMessage)
		return WithTransientError(errors.New(errMessage), requeueAfter)
	}

	if host == nil {
		return nil
	}

	annotations := host.GetAnnotations()

	if annotations != nil {
		if _, ok := annotations[bmov1alpha1.PausedAnnotation]; ok {
			if m.Cluster.Name == host.Labels[clusterv1.ClusterNameLabel] && annotations[bmov1alpha1.PausedAnnotation] == PausedAnnotationKey {
				// Removing BMH Paused Annotation Since Owner Cluster is not paused.
				delete(host.Annotations, bmov1alpha1.PausedAnnotation)
			} else if m.Cluster.Name == host.Labels[clusterv1.ClusterNameLabel] && annotations[bmov1alpha1.PausedAnnotation] != PausedAnnotationKey {
				m.Log.Info("BMH is paused by user. Not removing Pause Annotation")
				return nil
			}
		}
	}
	return helper.Patch(ctx, host)
}

// SetPauseAnnotation sets the pause annotations on associated bmh.
func (m *MachineManager) SetPauseAnnotation(ctx context.Context) error {
	// look for associated BMH
	host, helper, err := m.getHost(ctx)
	if err != nil {
		errMessage := "Failed to get a BaremetalHost for the Metal3Machine, requeuing"
		m.Log.Info(errMessage)
		return WithTransientError(errors.New(errMessage), requeueAfter)
	}
	if host == nil {
		return nil
	}

	annotations := host.GetAnnotations()

	if annotations != nil {
		if _, ok := annotations[bmov1alpha1.PausedAnnotation]; ok {
			m.Log.Info("BaremetalHost is already paused")
			return nil
		}
	} else {
		host.Annotations = make(map[string]string)
	}
	m.Log.Info("Adding PausedAnnotation in BareMetalHost")
	host.Annotations[bmov1alpha1.PausedAnnotation] = PausedAnnotationKey

	// Setting annotation with BMH status
	newAnnotation, err := json.Marshal(&host.Status)
	if err != nil {
		m.SetError("Failed to marshal the BareMetalHost status",
			capierrors.UpdateMachineError,
		)
		return errors.Wrap(err, "failed to marshall status annotation")
	}
	obj := map[string]interface{}{}
	if err := json.Unmarshal(newAnnotation, &obj); err != nil {
		return errors.Wrap(err, "failed to unmarshall status annotation")
	}
	delete(obj, "hardware")
	newAnnotation, err = json.Marshal(obj)
	if err != nil {
		m.SetError("Failed to marshal the BareMetalHost status",
			capierrors.UpdateMachineError,
		)
		return errors.Wrap(err, "failed to marshall status annotation")
	}
	host.Annotations[bmov1alpha1.StatusAnnotation] = string(newAnnotation)
	return helper.Patch(ctx, host)
}

// Associate associates a machine and is invoked by the Machine Controller.
func (m *MachineManager) Associate(ctx context.Context) error {
	// Parallel attempts to associate is problematic since the same BMH
	// could be selected for multiple M3Ms. Therefore we use a mutex lock here.
	associateBMHMutex.Lock()
	defer associateBMHMutex.Unlock()
	m.Log.Info("Associating machine", "machine", m.Machine.Name)

	// load and validate the config
	if m.Metal3Machine == nil {
		// Should have been picked earlier. Do not requeue
		return nil
	}

	// look for associated BMH
	host, helper, err := m.getHost(ctx)
	if err != nil {
		return err
	}

	// no BMH found, trying to choose from available ones
	if host == nil {
		host, helper, err = m.chooseHost(ctx)
		if err != nil {
			return err
		}
		if host == nil {
			errMessage := "No available host found. Requeuing."
			m.Log.Info(errMessage)
			return WithTransientError(errors.New(errMessage), requeueAfter)
		}
		m.Log.Info("Associating machine with host", "host", host.Name)
	} else {
		m.Log.Info("Machine already associated with host", "host", host.Name)
	}

	// A machine bootstrap not ready case is caught in the controller
	// ReconcileNormal function
	m.getUserDataSecretName(ctx)

	m.setHostLabel(ctx, host)

	err = m.setHostConsumerRef(ctx, host)
	if err != nil {
		return err
	}

	// If the user did not provide a DataTemplate, we can directly set the host
	// specs, nothing to wait for.
	if m.Metal3Machine.Spec.DataTemplate == nil {
		if err = m.setHostSpec(ctx, host); err != nil {
			return err
		}
	}

	err = m.setBMCSecretLabel(ctx, host)
	if err != nil {
		return err
	}

	err = helper.Patch(ctx, host)
	if err != nil {
		var aggr kerrors.Aggregate
		if ok := errors.As(err, &aggr); ok {
			for _, kerr := range aggr.Errors() {
				if apierrors.IsConflict(kerr) {
					return WithTransientError(nil, requeueAfter)
				}
			}
		}
		return err
	}

	err = m.ensureAnnotation(ctx, host)
	if err != nil {
		return err
	}

	if m.Metal3Machine.Spec.DataTemplate != nil {
		// Requeue to get the DataTemplate output. We need to requeue to trigger the
		// wait on the Metal3DataTemplate
		if err := m.WaitForM3Metadata(ctx); err != nil {
			return err
		}

		// If the requeue is not needed, then get the updated host and set the host
		// specs
		host, helper, err = m.getHost(ctx)
		if err != nil {
			errMessage := "Failed to get BaremetalHost while setting Host Spec, requeuing"
			m.Log.Info(errMessage)
			return WithTransientError(errors.New(errMessage), requeueAfter)
		}

		if err = m.setHostSpec(ctx, host); err != nil {
			return err
		}

		// Update the BMH object.
		err = helper.Patch(ctx, host)
		if err != nil {
			var aggr kerrors.Aggregate
			if ok := errors.As(err, &aggr); ok {
				for _, kerr := range aggr.Errors() {
					if apierrors.IsConflict(kerr) {
						return WithTransientError(kerr, requeueAfter)
					}
				}
			}
			return err
		}
	}

	m.Log.Info("Finished associating machine")
	return nil
}

// getUserDataSecretName gets the UserDataSecretName from the machine and exposes it as a secret
// for the BareMetalHost through Metal3Machine. The UserDataSecretName might already be in a secret with
// CABPK v0.3.0+, but if it is in a different namespace than the BareMetalHost,
// then we need to create the secret.
func (m *MachineManager) getUserDataSecretName(_ context.Context) {
	if m.Metal3Machine.Status.UserData != nil {
		return
	}

	if m.Metal3Machine.Spec.UserData != nil {
		m.Metal3Machine.Status.UserData = m.Metal3Machine.Spec.UserData
	}

	// if datasecretname is set just pass the reference.
	if m.Machine.Spec.Bootstrap.DataSecretName != nil {
		m.Metal3Machine.Status.UserData = &corev1.SecretReference{
			Name:      *m.Machine.Spec.Bootstrap.DataSecretName,
			Namespace: m.Machine.Namespace,
		}
		return
	} else if m.Machine.Spec.Bootstrap.ConfigRef != nil {
		m.Metal3Machine.Status.UserData = &corev1.SecretReference{
			Name:      m.Machine.Spec.Bootstrap.ConfigRef.Name,
			Namespace: m.Machine.Spec.Bootstrap.ConfigRef.Namespace,
		}
	}
}

// Delete deletes a metal3 machine and is invoked by the Machine Controller.
func (m *MachineManager) Delete(ctx context.Context) error {
	m.Log.Info("Deleting metal3 machine", "metal3machine", m.Metal3Machine.Name)

	if Capm3FastTrack == "" {
		Capm3FastTrack = "false"
		m.Log.Info("Capm3FastTrack is not set, setting it to default value false")
	}
	host, helper, err := m.getHost(ctx)
	if err != nil {
		return err
	}
	if host == nil {
		m.Log.Info("host not found for metal3machine", "metal3machine", m.Metal3Machine.Name)
		return nil
	}

	if host.Spec.ConsumerRef != nil {
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

		// Remove clusterLabel from BMC secret.
		tmpBMCSecret, errBMC := m.getBMCSecret(ctx, host)
		if errBMC != nil && apierrors.IsNotFound(errBMC) {
			m.Log.Info("BMC credential not found for BareMetalhost", "host", host.Name)
		} else if errBMC == nil && tmpBMCSecret != nil {
			m.Log.Info("Deleting cluster label from BMC credential", "bmccredential", host.Spec.BMC.CredentialsName)
			if tmpBMCSecret.Labels != nil && tmpBMCSecret.Labels[clusterv1.ClusterNameLabel] == m.Machine.Spec.ClusterName {
				delete(tmpBMCSecret.Labels, clusterv1.ClusterNameLabel)
				errBMC = updateObject(ctx, m.client, tmpBMCSecret)
				if errBMC != nil {
					var reconcileError ReconcileError
					if !(errors.As(errBMC, &reconcileError) && reconcileError.IsTransient()) {
						m.Log.Info("Failed to delete the clusterLabel from BMC Secret")
					}
					return errBMC
				}
			}
		}

		bmhUpdated := false

		if host.Spec.Image != nil {
			host.Spec.Image = nil
			bmhUpdated = true
		}
		if host.Spec.CustomDeploy != nil {
			host.Spec.CustomDeploy = nil
			bmhUpdated = true
		}
		if m.Metal3Machine.Status.UserData != nil && host.Spec.UserData != nil {
			host.Spec.UserData = nil
			bmhUpdated = true
		}
		if m.Metal3Machine.Status.MetaData != nil && host.Spec.MetaData != nil {
			host.Spec.MetaData = nil
			bmhUpdated = true
		}
		if m.Metal3Machine.Status.NetworkData != nil && host.Spec.NetworkData != nil {
			host.Spec.NetworkData = nil
			bmhUpdated = true
		}

		//	Change bmh's online status to on/off  based on AutomatedCleaningMode and Capm3FastTrack values
		//	AutomatedCleaningMode |	Capm3FastTrack|   BMH
		//		disabled				false 			turn off
		//		disabled				true 			turn off
		//		metadata				false 			turn off
		//		metadata				true 			turn on

		onlineStatus := host.Spec.Online

		if host.Spec.AutomatedCleaningMode == "disabled" {
			host.Spec.Online = false
		} else if Capm3FastTrack == "true" {
			host.Spec.Online = true
		} else if Capm3FastTrack == "false" {
			host.Spec.Online = false
		}
		m.Log.Info("Set host Online field by AutomatedCleaningMode",
			"host", host.Name,
			"automatedCleaningMode", host.Spec.AutomatedCleaningMode,
			"hostSpecOnline", host.Spec.Online)

		if onlineStatus != host.Spec.Online {
			bmhUpdated = true
		}

		if bmhUpdated {
			// Update the BMH object, if the errors are NotFound, do not return the
			// errors.
			if err := patchIfFound(ctx, helper, host); err != nil {
				return err
			}

			errMessage := "Deprovisioning BareMetalHost, requeuing"
			m.Log.Info(errMessage)
			return WithTransientError(errors.New(errMessage), 0*time.Second)
		}

		waiting := true
		switch host.Status.Provisioning.State {
		case bmov1alpha1.StateRegistering,
			bmov1alpha1.StateMatchProfile, bmov1alpha1.StateInspecting,
			bmov1alpha1.StateReady, bmov1alpha1.StateAvailable, bmov1alpha1.StateNone,
			bmov1alpha1.StateUnmanaged:
			// Host is not provisioned.
			waiting = false
		case bmov1alpha1.StateExternallyProvisioned:
			// We have no control over provisioning, so just wait until the
			// host is powered off.
			waiting = host.Status.PoweredOn
		}
		if waiting {
			errMessage := "Deprovisioning BareMetalHost, requeuing"
			m.Log.Info(errMessage)
			return WithTransientError(errors.New(errMessage), requeueAfter)
		}

		if m.Cluster != nil {
			// If cluster has DeletionTimestamp set, skip checking if nodeReuse
			// feature is enabled.
			if m.Cluster.DeletionTimestamp.IsZero() {
				// Fetch corresponding Metal3MachineTemplate, to see if nodeReuse
				// feature is enabled. If set to true, check the machine role. In case
				// machine role is ControlPlane, set nodeReuseLabelName to ControlPlane
				// name, otherwise to MachineDeployment name.
				m.Log.Info("Getting Metal3MachineTemplate")
				m3mt := &infrav1.Metal3MachineTemplate{}
				if m.Metal3Machine == nil {
					return errors.New("Metal3Machine associated with Metal3MachineTemplate is not found")
				}
				if m.hasTemplateAnnotation() {
					m3mtKey := client.ObjectKey{
						Name:      m.Metal3Machine.ObjectMeta.GetAnnotations()[clusterv1.TemplateClonedFromNameAnnotation],
						Namespace: m.Metal3Machine.Namespace,
					}
					if err := m.client.Get(ctx, m3mtKey, m3mt); err != nil {
						// we are here, because while normal deprovisioning, Metal3MachineTemplate will be deleted first
						// and we can't get it even though Metal3Machine has reference to it. We consider it nil and move
						// forward with normal deprovisioning.
						m3mt = nil
						m.Log.Info("Metal3MachineTemplate associated with Metal3Machine is deleted")
					} else {
						// in case of upgrading, Metal3MachineTemplate will not be deleted and we can fetch it,
						// in order to check for node reuse feature in the next step.
						m.Log.Info("Found Metal3machineTemplate", "metal3machineTemplate", m3mtKey.Name)
					}
				}
				if m3mt != nil {
					if m3mt.Spec.NodeReuse {
						if host.Labels == nil {
							host.Labels = make(map[string]string)
						}
						// Check if machine is ControlPlane
						if m.isControlPlane() {
							// Fetch ControlPlane name for controlplane machine
							m.Log.Info("Fetch ControlPlane name while deprovisioning host", "host", host.Name)
							cpName, err := m.getControlPlaneName(ctx)
							if err != nil {
								return err
							}
							// Set nodeReuseLabelName on the host to ControlPlane name
							m.Log.Info("Setting nodeReuseLabelName in host to fetched ControlPlane", "host", host.Name, "controlPlane", cpName)
							host.Labels[nodeReuseLabelName] = cpName
						} else {
							// Fetch MachineDeployment name for worker machine
							m.Log.Info("Fetch MachineDeployment name while deprovisioning host", "host", host.Name)
							mdName, err := m.getMachineDeploymentName(ctx)
							if err != nil {
								return err
							}
							// Set nodeReuseLabelName on the host to MachineDeployment name
							m.Log.Info("Setting nodeReuseLabelName in host to fetched MachineDeployment", "host", host.Name, "machinedeployment", mdName)
							host.Labels[nodeReuseLabelName] = mdName
						}
					}
				}
			}
		}

		host.Spec.ConsumerRef = nil

		// Delete created secret, if data was set without DataSecretName
		if m.Machine.Spec.Bootstrap.DataSecretName == nil {
			m.Log.Info("Deleting User data secret for machine")
			if m.Metal3Machine.Status.UserData != nil {
				err = deleteSecret(ctx, m.client, m.Metal3Machine.Status.UserData.Name,
					m.Metal3Machine.Namespace,
				)
				if err != nil {
					return err
				}
			}
		}

		host.Spec.ConsumerRef = nil

		// Remove the ownerreference to this machine.
		host.OwnerReferences, err = m.DeleteOwnerRef(host.OwnerReferences)
		if err != nil {
			return err
		}

		if host.Labels != nil && host.Labels[clusterv1.ClusterNameLabel] == m.Machine.Spec.ClusterName {
			delete(host.Labels, clusterv1.ClusterNameLabel)
		}

		m.Log.Info("Removing Paused Annotation (if any)")
		if host.Annotations != nil && host.Annotations[bmov1alpha1.PausedAnnotation] == PausedAnnotationKey {
			delete(host.Annotations, bmov1alpha1.PausedAnnotation)
		}

		// Update the BMH object, if the errors are NotFound, do not return the
		// errors.
		if err := patchIfFound(ctx, helper, host); err != nil {
			return err
		}
	}
	m.Log.Info("finished deleting metal3 machine")
	return nil
}

// Update updates a machine and is invoked by the Machine Controller.
func (m *MachineManager) Update(ctx context.Context) error {
	m.Log.Info("Updating machine")

	host, helper, err := m.getHost(ctx)
	if err != nil {
		return err
	}
	if host == nil {
		errMessage := "BareMetalHost not found for machine " + m.Machine.Name
		return WithTransientError(errors.New(errMessage), requeueAfter)
	}

	if err := m.WaitForM3Metadata(ctx); err != nil {
		return err
	}

	// ensure that the BMH specs are correctly set.
	err = m.setHostConsumerRef(ctx, host)
	if err != nil {
		return err
	}

	// ensure that the BMH specs are correctly set.
	err = m.setHostSpec(ctx, host)
	if err != nil {
		return err
	}

	err = helper.Patch(ctx, host)
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

// exists tests for the existence of a baremetalHost.
func (m *MachineManager) exists(ctx context.Context) (bool, error) {
	m.Log.Info("Checking if host exists.")
	host, _, err := m.getHost(ctx)
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
func (m *MachineManager) getHost(ctx context.Context) (*bmov1alpha1.BareMetalHost, *patch.Helper, error) {
	host, err := getHost(ctx, m.Metal3Machine, m.client, m.Log)
	if err != nil || host == nil {
		return host, nil, err
	}
	helper, err := patch.NewHelper(host, m.client)
	return host, helper, err
}

func getHost(ctx context.Context, m3Machine *infrav1.Metal3Machine, cl client.Client,
	mLog logr.Logger,
) (*bmov1alpha1.BareMetalHost, error) {
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

	host := bmov1alpha1.BareMetalHost{}
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
func (m *MachineManager) chooseHost(ctx context.Context) (*bmov1alpha1.BareMetalHost, *patch.Helper, error) {
	// get list of BMH.
	hosts := bmov1alpha1.BareMetalHostList{}
	// without this ListOption, all namespaces would be including in the listing.
	opts := &client.ListOptions{
		Namespace: m.Metal3Machine.Namespace,
	}

	err := m.client.List(ctx, &hosts, opts)
	if err != nil {
		return nil, nil, err
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
			return nil, nil, err
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
			return nil, nil, err
		}
		reqs = append(reqs, *r)
	}
	labelSelector = labelSelector.Add(reqs...)

	availableHosts := []*bmov1alpha1.BareMetalHost{}
	availableHostsWithNodeReuse := []*bmov1alpha1.BareMetalHost{}

	for i, host := range hosts.Items {
		if host.Spec.ConsumerRef != nil && consumerRefMatches(host.Spec.ConsumerRef, m.Metal3Machine) {
			m.Log.Info("Found host with existing ConsumerRef", "host", host.Name)
			helper, err := patch.NewHelper(&hosts.Items[i], m.client)
			return &hosts.Items[i], helper, err
		}
		if host.Spec.ConsumerRef != nil ||
			(m.nodeReuseLabelExists(ctx, &host) &&
				!m.nodeReuseLabelMatches(ctx, &host)) {
			continue
		}
		if host.GetDeletionTimestamp() != nil {
			continue
		}
		if host.Status.ErrorMessage != "" {
			continue
		}

		// continue if BaremetalHost is paused or marked with UnhealthyAnnotation.
		annotations := host.GetAnnotations()
		if annotations != nil {
			if _, ok := annotations[bmov1alpha1.PausedAnnotation]; ok {
				continue
			}
			if _, ok := annotations[infrav1.UnhealthyAnnotation]; ok {
				continue
			}
		}

		if labelSelector.Matches(labels.Set(host.ObjectMeta.Labels)) {
			if m.nodeReuseLabelExists(ctx, &host) && m.nodeReuseLabelMatches(ctx, &host) {
				m.Log.Info("Found host with nodeReuseLabelName and it matches, adding it to availableHostsWithNodeReuse list", "host", host.Name)
				availableHostsWithNodeReuse = append(availableHostsWithNodeReuse, &hosts.Items[i])
			} else if !m.nodeReuseLabelExists(ctx, &host) {
				switch host.Status.Provisioning.State {
				case bmov1alpha1.StateReady, bmov1alpha1.StateAvailable:
				default:
					continue
				}
				m.Log.Info("Host matched hostSelector for Metal3Machine, adding it to availableHosts list", "host", host.Name)
				availableHosts = append(availableHosts, &hosts.Items[i])
			}
		} else {
			m.Log.Info("Host did not match hostSelector for Metal3Machine", "host", host.Name)
		}
	}

	m.Log.Info("Host count available with nodeReuseLabelName while choosing host for Metal3 machine", "hostcount", len(availableHostsWithNodeReuse))
	m.Log.Info("Host count available while choosing host for Metal3 machine", "hostcount", len(availableHosts))
	if len(availableHostsWithNodeReuse) == 0 && len(availableHosts) == 0 {
		return nil, nil, nil
	}

	// choose a host.
	var chosenHost *bmov1alpha1.BareMetalHost

	// If there are hosts with nodeReuseLabelName:
	if len(availableHostsWithNodeReuse) != 0 {
		for _, host := range availableHostsWithNodeReuse {
			// Build list of hosts in Ready state with nodeReuseLabelName
			hostsInAvailableStateWithNodeReuse := []*bmov1alpha1.BareMetalHost{}
			// Build list of hosts in any other state than Ready state with nodeReuseLabelName
			hostsInNotAvailableStateWithNodeReuse := []*bmov1alpha1.BareMetalHost{}
			if host.Status.Provisioning.State == bmov1alpha1.StateReady || host.Status.Provisioning.State == bmov1alpha1.StateAvailable {
				hostsInAvailableStateWithNodeReuse = append(hostsInAvailableStateWithNodeReuse, host)
			} else {
				hostsInNotAvailableStateWithNodeReuse = append(hostsInNotAvailableStateWithNodeReuse, host)
			}

			// If host is found in `Ready` state, pick it
			if len(hostsInAvailableStateWithNodeReuse) != 0 {
				m.Log.Info("Found host(s) with nodeReuseLabelName in Ready/Available state, choosing the host", "availabeHostCount", len(hostsInAvailableStateWithNodeReuse), "host", host.Name)
				rHost, _ := rand.Int(rand.Reader, big.NewInt(int64(len(hostsInAvailableStateWithNodeReuse))))
				randomHost := rHost.Int64()
				chosenHost = hostsInAvailableStateWithNodeReuse[randomHost]
			} else if len(hostsInNotAvailableStateWithNodeReuse) != 0 {
				errMessage := fmt.Sprint("Found BareMetalHost(s) with nodeReuseLabelName in not-available state, requeuing the BareMetalHost", "notAvailabeHostCount", len(hostsInNotAvailableStateWithNodeReuse), "hoststate", host.Status.Provisioning.State, "host", host.Name)
				m.Log.Info(errMessage)
				return nil, nil, WithTransientError(errors.New(errMessage), requeueAfter)
			}
		}
	} else {
		// If there are no hosts with nodeReuseLabelName, fall back
		// to the current flow and select hosts randomly.
		m.Log.Info("host(s) count available, choosing a random host", "availabeHostCount", len(availableHosts))
		rHost, _ := rand.Int(rand.Reader, big.NewInt(int64(len(availableHosts))))
		randomHost := rHost.Int64()
		chosenHost = availableHosts[randomHost]
	}

	helper, err := patch.NewHelper(chosenHost, m.client)
	return chosenHost, helper, err
}

// consumerRefMatches returns a boolean based on whether the consumer
// reference and bare metal machine metadata match.
func consumerRefMatches(consumer *corev1.ObjectReference, m3machine *infrav1.Metal3Machine) bool {
	if consumer.Name != m3machine.Name {
		return false
	}
	if consumer.Namespace != m3machine.Namespace {
		return false
	}
	if consumer.Kind != m3machine.Kind {
		return false
	}
	if consumer.GroupVersionKind().Group != m3machine.GroupVersionKind().Group {
		return false
	}
	return true
}

// nodeReuseLabelMatches returns true if nodeReuseLabelName matches ControlPlane or MachineDeployment name on the host.
func (m *MachineManager) nodeReuseLabelMatches(ctx context.Context, host *bmov1alpha1.BareMetalHost) bool {
	if host == nil {
		return false
	}
	if host.Labels == nil {
		return false
	}
	if m.isControlPlane() {
		cp, err := m.getControlPlaneName(ctx)
		if err != nil {
			return false
		}
		if host.Labels[nodeReuseLabelName] == "" {
			return false
		}
		if host.Labels[nodeReuseLabelName] != cp {
			return false
		}
		m.Log.Info("nodeReuseLabelName on the host matches ControlPlane name", "host", host.Name, "controlPlane", cp)
		return true
	}
	md, err := m.getMachineDeploymentName(ctx)
	if err != nil {
		return false
	}
	if host.Labels[nodeReuseLabelName] == "" {
		return false
	}
	if host.Labels[nodeReuseLabelName] != md {
		return false
	}
	m.Log.Info("nodeReuseLabelName on the host matches MachineDeployment", "host", host.Name, "machinedeployment", md)
	return true
}

// nodeReuseLabelExists returns true if host contains nodeReuseLabelName label.
func (m *MachineManager) nodeReuseLabelExists(_ context.Context, host *bmov1alpha1.BareMetalHost) bool {
	if host == nil {
		return false
	}
	if host.Labels == nil {
		return false
	}
	_, ok := host.Labels[nodeReuseLabelName]
	if ok {
		m.Log.Info("nodeReuseLabelName exists on the host", "host", host.Name)
	}
	return ok
}

// getBMCSecret will return the BMCSecret associated with BMH.
func (m *MachineManager) getBMCSecret(ctx context.Context, host *bmov1alpha1.BareMetalHost) (*corev1.Secret, error) {
	if host == nil || host.Spec.BMC.CredentialsName == "" {
		return nil, nil
	}
	tmpBMCSecret := corev1.Secret{}
	key := host.CredentialsKey()
	err := m.client.Get(ctx, key, &tmpBMCSecret)
	if err != nil {
		m.Log.Error(err, "Cannot retrieve BMC credential for BareMetalhost", "host", host.Name)
		return nil, err
	}
	return &tmpBMCSecret, nil
}

// setBMCSecretLabel will set the set cluster.x-k8s.io/cluster-name to BMCSecret.
func (m *MachineManager) setBMCSecretLabel(ctx context.Context, host *bmov1alpha1.BareMetalHost) error {
	tmpBMCSecret, err := m.getBMCSecret(ctx, host)
	if err != nil {
		return err
	}

	if tmpBMCSecret != nil {
		if tmpBMCSecret.Labels == nil {
			tmpBMCSecret.Labels = make(map[string]string)
		}
		tmpBMCSecret.Labels[clusterv1.ClusterNameLabel] = m.Machine.Spec.ClusterName
		return updateObject(ctx, m.client, tmpBMCSecret)
	}

	return nil
}

// setHostLabel will set the set cluster.x-k8s.io/cluster-name to bmh.
func (m *MachineManager) setHostLabel(_ context.Context, host *bmov1alpha1.BareMetalHost) {
	if host.Labels == nil {
		host.Labels = make(map[string]string)
	}
	host.Labels[clusterv1.ClusterNameLabel] = m.Machine.Spec.ClusterName
}

// setHostSpec will ensure the host's Spec is set according to the machine's
// details. It will then update the host via the kube API. If UserData does not
// include a Namespace, it will default to the Metal3Machine's namespace.
func (m *MachineManager) setHostSpec(_ context.Context, host *bmov1alpha1.BareMetalHost) error {
	// We only want to update the image setting if the host does not
	// already have an image.
	//
	// A host with an existing image or customDeploy is already provisioned
	// and upgrades are not supported at this time. To re-provision a
	// host, we must fully deprovision it and then provision it again.
	// Not provisioning while we do not have the UserData.
	if host.Spec.Image == nil && host.Spec.CustomDeploy == nil && m.Metal3Machine.Status.UserData != nil {
		checksumType := ""
		if m.Metal3Machine.Spec.Image.ChecksumType != nil {
			checksumType = *m.Metal3Machine.Spec.Image.ChecksumType
		}
		if m.Metal3Machine.Spec.Image.URL != "" {
			host.Spec.Image = &bmov1alpha1.Image{
				URL:          m.Metal3Machine.Spec.Image.URL,
				Checksum:     m.Metal3Machine.Spec.Image.Checksum,
				ChecksumType: bmov1alpha1.ChecksumType(checksumType),
				DiskFormat:   m.Metal3Machine.Spec.Image.DiskFormat,
			}
		}
		if m.Metal3Machine.Spec.CustomDeploy != nil {
			host.Spec.CustomDeploy = &bmov1alpha1.CustomDeploy{
				Method: m.Metal3Machine.Spec.CustomDeploy.Method,
			}
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
	// Set automatedCleaningMode from metal3Machine.spec.automatedCleaningMode.
	if m.Metal3Machine.Spec.AutomatedCleaningMode != nil {
		if host.Spec.AutomatedCleaningMode != bmov1alpha1.AutomatedCleaningMode(*m.Metal3Machine.Spec.AutomatedCleaningMode) {
			host.Spec.AutomatedCleaningMode = bmov1alpha1.AutomatedCleaningMode(*m.Metal3Machine.Spec.AutomatedCleaningMode)
		}
	}

	host.Spec.Online = true

	return nil
}

// setHostConsumerRef will ensure the host's Spec is set to link to this
// Metal3Machine.
func (m *MachineManager) setHostConsumerRef(_ context.Context, host *bmov1alpha1.BareMetalHost) error {
	host.Spec.ConsumerRef = &corev1.ObjectReference{
		Kind:       "Metal3Machine",
		Name:       m.Metal3Machine.Name,
		Namespace:  m.Metal3Machine.Namespace,
		APIVersion: m.Metal3Machine.APIVersion,
	}

	// Set OwnerReferences.
	hostOwnerReferences, err := m.SetOwnerRef(host.OwnerReferences, true)
	if err != nil {
		return err
	}
	host.OwnerReferences = hostOwnerReferences

	// Delete nodeReuseLabelName from host.
	m.Log.Info("Deleting nodeReuseLabelName from host, if any")

	labels := host.GetLabels()
	if labels != nil {
		if _, ok := labels[nodeReuseLabelName]; ok {
			delete(host.Labels, nodeReuseLabelName)
			m.Log.Info("Finished deleting nodeReuseLabelName")
		}
	}

	return nil
}

// ensureAnnotation makes sure the machine has an annotation that references the
// host and uses the API to update the machine if necessary.
func (m *MachineManager) ensureAnnotation(_ context.Context, host *bmov1alpha1.BareMetalHost) error {
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

// HasAnnotation makes sure the machine has an annotation that references a host.
func (m *MachineManager) HasAnnotation() bool {
	annotations := m.Metal3Machine.ObjectMeta.GetAnnotations()
	if annotations == nil {
		return false
	}
	_, ok := annotations[HostAnnotation]
	return ok
}

// hasTemplateAnnotation makes sure the metal3 machine has infrastructure machine
// annotation that stores the name of the infrastructure template resource.
func (m *MachineManager) hasTemplateAnnotation() bool {
	annotations := m.Metal3Machine.ObjectMeta.GetAnnotations()
	if annotations == nil {
		return false
	}
	_, ok := annotations[clusterv1.TemplateClonedFromNameAnnotation]
	return ok
}

// SetError sets the ErrorMessage and ErrorReason fields on the machine and logs
// the message. It assumes the reason is invalid configuration, since that is
// currently the only relevant MachineStatusError choice.
func (m *MachineManager) SetError(message string, reason capierrors.MachineStatusError) {
	m.Metal3Machine.Status.FailureMessage = &message
	m.Metal3Machine.Status.FailureReason = &reason
}

// SetConditionMetal3MachineToFalse sets Metal3Machine condition status to False.
func (m *MachineManager) SetConditionMetal3MachineToFalse(t clusterv1.ConditionType, reason string, severity clusterv1.ConditionSeverity, messageFormat string, messageArgs ...interface{}) {
	conditions.MarkFalse(m.Metal3Machine, t, reason, severity, messageFormat, messageArgs...)
}

// SetConditionMetal3MachineToTrue sets Metal3Machine condition status to True.
func (m *MachineManager) SetConditionMetal3MachineToTrue(t clusterv1.ConditionType) {
	conditions.MarkTrue(m.Metal3Machine, t)
}

func (m *MachineManager) CloudProviderEnabled() bool {
	if m.Metal3Cluster.Spec.NoCloudProvider != nil && !*m.Metal3Cluster.Spec.NoCloudProvider {
		return true
	}

	if m.Metal3Cluster.Spec.CloudProviderEnabled != nil && *m.Metal3Cluster.Spec.CloudProviderEnabled {
		return true
	}

	return false
}

// updateMachineStatus updates a Metal3Machine object's status.
func (m *MachineManager) updateMachineStatus(_ context.Context, host *bmov1alpha1.BareMetalHost) error {
	addrs := m.nodeAddresses(host)

	metal3MachineOld := m.Metal3Machine.DeepCopy()

	m.Metal3Machine.Status.Addresses = addrs
	conditions.MarkTrue(m.Metal3Machine, infrav1.AssociateBMHCondition)

	if equality.Semantic.DeepEqual(m.Metal3Machine.Status, metal3MachineOld.Status) {
		// Status did not change
		return nil
	}

	now := metav1.Now()
	m.Metal3Machine.Status.LastUpdated = &now
	return nil
}

// NodeAddresses returns a slice of corev1.NodeAddress objects for a
// given Metal3 machine.
func (m *MachineManager) nodeAddresses(host *bmov1alpha1.BareMetalHost) []clusterv1.MachineAddress {
	addrs := []clusterv1.MachineAddress{}

	// If the host is nil or we have no hw details, return an empty address array.
	if host == nil || host.Status.HardwareDetails == nil {
		return addrs
	}

	for _, nic := range host.Status.HardwareDetails.NIC {
		address := clusterv1.MachineAddress{
			Type:    clusterv1.MachineInternalIP,
			Address: nic.IP,
		}
		if address.Address == "" {
			continue
		}
		addrs = append(addrs, address)
	}

	if host.Status.HardwareDetails.Hostname != "" {
		addrs = append(addrs, clusterv1.MachineAddress{
			Type:    clusterv1.MachineHostName,
			Address: host.Status.HardwareDetails.Hostname,
		})
		addrs = append(addrs, clusterv1.MachineAddress{
			Type:    clusterv1.MachineInternalDNS,
			Address: host.Status.HardwareDetails.Hostname,
		})
	}

	return addrs
}

// GetProviderIDAndBMHID returns providerID and bmhID.
func (m *MachineManager) GetProviderIDAndBMHID() (string, *string) {
	providerID := m.Metal3Machine.Spec.ProviderID
	if providerID == nil {
		return "", nil
	}
	bmhID := *providerID
	if strings.Contains(bmhID, ProviderIDPrefix) {
		bmhID = strings.TrimPrefix(bmhID, ProviderIDPrefix)
	}
	// If the providerID is in new format, it does not contain the BMH ID, but
	// instead contains / to separate the names. In that case we return nil for
	// the bmh ID to force the controller to fetch it differently.
	if strings.Contains(bmhID, "/") {
		m.Log.Info("ProviderID is in new format, it does not contain the BMH ID", "providerID", *providerID)
		return *providerID, nil
	}
	m.Log.V(VerbosityLevelDebug).Info("ProviderID contains the BMH ID", "providerID", *providerID)
	return *providerID, ptr.To(bmhID)
}

// ClientGetter prototype.
type ClientGetter func(ctx context.Context, c client.Client, cluster *clusterv1.Cluster) (clientcorev1.CoreV1Interface, error)

func (m *MachineManager) setNodeProviderID(ctx context.Context, client clientcorev1.CoreV1Interface, node corev1.Node, providerID string) error {
	oldData, err := json.Marshal(node)
	if err != nil {
		return fmt.Errorf("failed to json.Marshal node: %w", err)
	}

	node.Spec.ProviderID = providerID

	nodeVar := node.DeepCopy()
	newData, err := json.Marshal(*nodeVar)
	if err != nil {
		return fmt.Errorf("failed to json.Marshal node: %w", err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, corev1.Node{})
	if err != nil {
		return fmt.Errorf("failed to create patch for node %q: %w", node.GetName(), err)
	}
	_, err = client.Nodes().Patch(ctx, nodeVar.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("unable to update the target node with providerID: %w", err)
	}

	return nil
}

func (m *MachineManager) getMetal3MachineHostnames() []string {
	hostnames := []string{}

	for _, address := range m.Metal3Machine.Status.Addresses {
		if address.Type == clusterv1.MachineHostName {
			hostnames = append(hostnames, address.Address)
		}
	}

	return hostnames
}

// SetProviderID sets the metal3 provider ID on the Metal3Machine.
func (m *MachineManager) SetProviderID(providerID string) {
	m.Log.Info("ProviderID set on the Metal3Machine", "providerID", providerID)
	m.Metal3Machine.Spec.ProviderID = &providerID
}

// SetDefaultProviderID sets the ProviderID on this Metal3Machine using the new format.
func (m *MachineManager) SetDefaultProviderID() error {
	namespace := m.Metal3Machine.GetNamespace()
	m3mName := m.Metal3Machine.GetName()
	bmhName, err := m.getBmhNameFromM3Machine()

	if err != nil {
		return err
	}

	providerID := fmt.Sprintf("metal3://%s/%s/%s", namespace, bmhName, m3mName)
	m.Log.Info("Setting default providerID on the Metal3Machine", "providerID", providerID, "m3mName", m3mName, "bmhName", bmhName)
	m.SetProviderID(providerID)
	return nil
}

// getPossibleProviderIDs returns the ProviderID for this Metal3Machine in the legacy and the new format.
func (m *MachineManager) getPossibleProviderIDs(ctx context.Context) (providerIDLegacy string, providerIDNew string, err error) {
	namespace := m.Metal3Machine.GetNamespace()
	m3mName := m.Metal3Machine.GetName()
	bmhName, err := m.getBmhNameFromM3Machine()
	if err != nil {
		errMessage := "unable to retrieve BMH name from Metal3Machine"
		m.Log.Info(errMessage)
		err = errors.Wrap(err, errMessage)
		return
	}
	bmhUID, err := m.getBmhUIDFromM3Machine(ctx)
	if err != nil {
		errMessage := "unable to retrieve BMH UID from Metal3Machine"
		m.Log.Info(errMessage)
		err = errors.Wrap(err, errMessage)
		return
	}
	providerIDLegacy = "metal3://" + bmhUID
	providerIDNew = fmt.Sprintf("metal3://%s/%s/%s", namespace, bmhName, m3mName)

	return
}

// SetProviderIDFromCloudProviderNode finds a Node by ProviderID and copies that ProviderID to the Metal3Machine.
func (m *MachineManager) SetProviderIDFromCloudProviderNode(ctx context.Context, clientFactory ClientGetter) error {
	providerIDLegacy, providerIDNew, err := m.getPossibleProviderIDs(ctx)
	if err != nil {
		return WithTransientError(err, requeueAfter)
	}

	node, err := m.getNodeByProviderID(ctx, providerIDLegacy, providerIDNew, clientFactory)
	if err != nil {
		errMessage := "error retrieving node, requeuing"
		m.Log.Info(errMessage)
		return WithTransientError(errors.New(errMessage), requeueAfter)
	}

	m.SetProviderID(node.Spec.ProviderID)
	m.SetReadyTrue()
	return nil
}

func (m *MachineManager) NodeWithMatchingProviderIDExists(ctx context.Context, clientFactory ClientGetter) bool {
	if !m.Metal3MachineHasProviderID() {
		return false
	}

	providerIDLegacy, providerIDNew, err := m.getPossibleProviderIDs(ctx)
	if err != nil {
		return false
	}

	node, err := m.getNodeByProviderID(ctx, providerIDLegacy, providerIDNew, clientFactory)
	if err != nil {
		errMessage := "error retrieving node, requeuing"
		m.Log.Info(errMessage)
		return false
	}

	m.Log.Info("matching node found", "node", node.GetName(), "providerID", node.Spec.ProviderID)
	return true
}

// SetProviderIDFromNodeLabel finds a Node by label and sets ProviderID on it.
func (m *MachineManager) SetProviderIDFromNodeLabel(ctx context.Context, clientFactory ClientGetter) (success bool, err error) {
	corev1Remote, err := clientFactory(ctx, m.client, m.Cluster)
	if err != nil {
		return false, errors.Wrap(err, "Error creating a remote client")
	}
	bmhUID, err := m.getBmhUIDFromM3Machine(ctx)
	if err != nil {
		errMessage := "unable to retrieve BMH UID from Metal3Machine"
		m.Log.Info(errMessage)
		return false, WithTransientError(errors.New(errMessage), requeueAfter)
	}

	nodeLabel := fmt.Sprintf("%s=%s", ProviderLabelPrefix, bmhUID)
	nodes, countNodesWithLabel, err := m.getNodesWithLabel(ctx, nodeLabel, clientFactory)
	if err != nil {
		errMessage := fmt.Sprintf("error retrieving node with label %s, requeuing", nodeLabel)
		m.Log.Info(errMessage)
		return false, WithTransientError(errors.Wrap(err, errMessage), requeueAfter)
	}
	if countNodesWithLabel == 0 && m.Machine.Spec.Bootstrap.ConfigRef != nil {
		// The node could either be still running cloud-init or have been
		// deleted manually. TODO: handle a manual deletion case.
		errMessage := "requeuing, could not find node with label: " + nodeLabel
		m.Log.Info(errMessage)
		return false, WithTransientError(errors.New(errMessage), requeueAfter)
	}
	if countNodesWithLabel > 1 {
		return false, errors.Wrap(err, fmt.Sprintf("Found multiple target nodes with the same label: (%s)", nodeLabel))
	}

	providerIDLegacy, providerIDNew, err := m.getPossibleProviderIDs(ctx)
	if err != nil {
		errMessage := fmt.Sprintf("unable to retrieve BMH name from Metal3Machine: %v", err)
		m.Log.Info(errMessage)
		return false, WithTransientError(errors.New(errMessage), requeueAfter)
	}

	if countNodesWithLabel == 1 {
		node := nodes.Items[0]
		providerIDOnNode := node.Spec.ProviderID
		if providerIDOnNode == "" {
			// By default we use the new format, if not set on the node.
			m.SetProviderID(providerIDNew)
			m.SetReadyTrue()
			err := m.setNodeProviderID(ctx, corev1Remote, node, providerIDNew)

			if err != nil {
				return false, err
			}

			return true, nil
		}

		if providerIDOnNode == providerIDNew {
			m.SetProviderID(providerIDNew)
			m.SetReadyTrue()
			return true, nil
		}

		if providerIDOnNode == providerIDLegacy {
			m.SetProviderID(providerIDLegacy)
			m.SetReadyTrue()
			return true, nil
		}

		m.Log.Info("node using unsupported providerID format", "providerID", providerIDOnNode, "providerIDLegacy", providerIDLegacy, "providerIDNew", providerIDNew)
		return false, errors.Wrap(err, "node using unsupported providerID format")
	}

	return false, nil
}

func (m *MachineManager) Metal3MachineHasProviderID() bool {
	return m.Metal3Machine.Spec.ProviderID != nil
}

func (m *MachineManager) SetReadyTrue() {
	m.Metal3Machine.Status.Ready = true
}

// SetOwnerRef adds an ownerreference to this Metal3Machine.
func (m *MachineManager) SetOwnerRef(refList []metav1.OwnerReference, controller bool) ([]metav1.OwnerReference, error) {
	return setOwnerRefInList(refList, controller, m.Metal3Machine.TypeMeta,
		m.Metal3Machine.ObjectMeta,
	)
}

// DeleteOwnerRef removes the ownerreference to this Metal3Machine.
func (m *MachineManager) DeleteOwnerRef(refList []metav1.OwnerReference) ([]metav1.OwnerReference, error) {
	return deleteOwnerRefFromList(refList, m.Metal3Machine.TypeMeta,
		m.Metal3Machine.ObjectMeta,
	)
}

// DeleteOwnerRefFromList removes the ownerreference to this Metal3Machine.
func deleteOwnerRefFromList(refList []metav1.OwnerReference,
	objType metav1.TypeMeta, objMeta metav1.ObjectMeta,
) ([]metav1.OwnerReference, error) {
	if len(refList) == 0 {
		return refList, nil
	}
	index, err := findOwnerRefFromList(refList, objType, objMeta)
	if err != nil {
		if ok := errors.As(err, &errNotFound); !ok {
			return nil, err
		}
		return refList, nil
	}
	if len(refList) == 1 {
		return []metav1.OwnerReference{}, nil
	}
	refListLen := len(refList) - 1
	refList[index] = refList[refListLen]
	refList, err = deleteOwnerRefFromList(refList[:refListLen], objType, objMeta)
	if err != nil {
		return nil, err
	}
	return refList, nil
}

// FindOwnerRef checks if an ownerreference to this Metal3Machine exists
// and returns the index.
func (m *MachineManager) FindOwnerRef(refList []metav1.OwnerReference) (int, error) {
	return findOwnerRefFromList(refList, m.Metal3Machine.TypeMeta,
		m.Metal3Machine.ObjectMeta,
	)
}

// SetOwnerRef adds an ownerreference to this Metal3Machine.
func setOwnerRefInList(refList []metav1.OwnerReference, controller bool,
	objType metav1.TypeMeta, objMeta metav1.ObjectMeta,
) ([]metav1.OwnerReference, error) {
	index, err := findOwnerRefFromList(refList, objType, objMeta)
	if err != nil {
		if ok := errors.As(err, &errNotFound); !ok {
			return nil, err
		}
		refList = append(refList, metav1.OwnerReference{
			APIVersion: objType.APIVersion,
			Kind:       objType.Kind,
			Name:       objMeta.Name,
			UID:        objMeta.UID,
			Controller: ptr.To(controller),
		})
	} else {
		// The UID and the APIVersion might change due to move or version upgrade.
		refList[index].APIVersion = objType.APIVersion
		refList[index].UID = objMeta.UID
		refList[index].Controller = ptr.To(controller)
	}
	return refList, nil
}

// findOwnerRefFromList finds OwnerRef to this Metal3Machine.
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
		// not matching on UID since when pivoting it might change.
		// Not matching on API version as this might change.
		if curOwnerRef.Name == objMeta.Name &&
			curOwnerRef.Kind == objType.Kind &&
			aGV.Group == bGV.Group {
			return i, nil
		}
	}
	return 0, &NotFoundError{}
}

// AssociateM3Metadata fetches the Metal3DataTemplate object and sets the
// owner references.
func (m *MachineManager) AssociateM3Metadata(ctx context.Context) error {
	// If the secrets were provided by the user, use them.
	if m.Metal3Machine.Spec.MetaData != nil {
		m.Metal3Machine.Status.MetaData = m.Metal3Machine.Spec.MetaData
	}
	if m.Metal3Machine.Spec.NetworkData != nil {
		m.Metal3Machine.Status.NetworkData = m.Metal3Machine.Spec.NetworkData
	}

	// If we have RenderedData set already, it means that the owner reference was
	// already set.
	if m.Metal3Machine.Status.RenderedData != nil {
		return nil
	}

	if m.Metal3Machine.Spec.DataTemplate == nil {
		return nil
	}
	if m.Metal3Machine.Spec.DataTemplate.Namespace == "" {
		m.Metal3Machine.Spec.DataTemplate.Namespace = m.Metal3Machine.Namespace
	}
	_, err := fetchM3DataClaim(ctx, m.client, m.Log,
		m.Metal3Machine.Name, m.Metal3Machine.Namespace,
	)
	if err != nil {
		var reconcileError ReconcileError
		if !(errors.As(err, &reconcileError) && reconcileError.IsTransient()) {
			return err
		}
	} else {
		return nil
	}

	dataClaim := &infrav1.Metal3DataClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.Metal3Machine.Name,
			Namespace: m.Metal3Machine.Namespace,
			Finalizers: []string{
				infrav1.MachineFinalizer,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: m.Metal3Machine.APIVersion,
					Kind:       m.Metal3Machine.Kind,
					Name:       m.Metal3Machine.Name,
					UID:        m.Metal3Machine.UID,
					Controller: ptr.To(true),
				},
			},
			Labels: m.Metal3Machine.Labels,
		},
		Spec: infrav1.Metal3DataClaimSpec{
			Template: *m.Metal3Machine.Spec.DataTemplate,
		},
	}

	err = createObject(ctx, m.client, dataClaim)
	if err != nil {
		return err
	}
	return nil
}

// WaitForM3Metadata fetches the Metal3DataTemplate object and sets the
// owner references.
func (m *MachineManager) WaitForM3Metadata(ctx context.Context) error {
	// If we do not have RenderedData set yet, try to find it in
	// Metal3DataTemplate. If it is not there yet, it means that the reconciliation
	// of Metal3DataTemplate did not yet complete, requeue.
	if m.Metal3Machine.Status.RenderedData == nil {
		if m.Metal3Machine.Spec.DataTemplate == nil {
			return nil
		}
		if m.Metal3Machine.Spec.DataTemplate.Namespace == "" {
			m.Metal3Machine.Spec.DataTemplate.Namespace = m.Metal3Machine.Namespace
		}
		metal3DataClaim, err := fetchM3DataClaim(ctx, m.client, m.Log,
			m.Metal3Machine.Name, m.Metal3Machine.Namespace,
		)
		if err != nil {
			return err
		}
		if metal3DataClaim == nil {
			return WithTransientError(errors.New("Metal3DataClaim is empty, requeuing"), requeueAfter)
		}

		if metal3DataClaim.Status.RenderedData != nil &&
			metal3DataClaim.Status.RenderedData.Name != "" {
			m.Metal3Machine.Status.RenderedData = metal3DataClaim.Status.RenderedData
		} else {
			return WithTransientError(errors.New("Waiting for Metal3DataTemplate to be available"), requeueAfter)
		}
	}

	// Fetch the Metal3Data.
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
		errMessage := "Waiting for Metal3Data to become ready"
		m.Log.Info(errMessage)
		m.SetConditionMetal3MachineToFalse(infrav1.Metal3DataReadyCondition, infrav1.WaitingForMetal3DataReason, clusterv1.ConditionSeverityInfo, "")
		// Secret generation not ready
		return WithTransientError(errors.New(errMessage), requeueAfter)
	}

	// At this point, Metal3Data is ready
	m.Log.Info("Metal3data is ready")
	m.SetConditionMetal3MachineToTrue(infrav1.Metal3DataReadyCondition)

	// Get the secrets if given in Metal3Data and not already set.
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

// DissociateM3Metadata removes machine from OwnerReferences of meta3DataTemplate, on failure requeue.
func (m *MachineManager) DissociateM3Metadata(ctx context.Context) error {
	if m.Metal3Machine.Status.MetaData != nil && m.Metal3Machine.Spec.MetaData == nil {
		m.Metal3Machine.Status.MetaData = nil
	}

	if m.Metal3Machine.Status.NetworkData != nil && m.Metal3Machine.Spec.NetworkData == nil {
		m.Metal3Machine.Status.NetworkData = nil
	}

	m.Metal3Machine.Status.RenderedData = nil

	// Get the Metal3DataClaim object.
	metal3DataClaim, err := fetchM3DataClaim(ctx, m.client, m.Log,
		m.Metal3Machine.Name, m.Metal3Machine.Namespace,
	)
	if err != nil {
		var reconcileError ReconcileError
		if !(errors.As(err, &reconcileError) && reconcileError.IsTransient()) {
			return err
		}
		return nil
	}
	if metal3DataClaim == nil {
		return nil
	}

	controllerutil.RemoveFinalizer(metal3DataClaim, infrav1.MachineFinalizer)
	err = updateObject(ctx, m.client, metal3DataClaim)
	if err != nil && !apierrors.IsNotFound(err) {
		m.Log.Info("Unable to remove finalizers from Metal3DataClaim", "Metal3DataClaim", metal3DataClaim.Name)
		return err
	}

	return deleteObject(ctx, m.client, metal3DataClaim)
}

// getControlPlaneName retrieves the ControlPlane object corresponding to the CAPI machine.
func (m *MachineManager) getControlPlaneName(_ context.Context) (string, error) {
	m.Log.Info("Fetching ControlPlane name")
	if m.Machine == nil {
		return "", errors.New("Could not find corresponding machine object")
	}
	if m.Machine.ObjectMeta.OwnerReferences == nil {
		return "", errors.New("Machine owner reference is not populated")
	}
	for _, mOwnerRef := range m.Machine.ObjectMeta.OwnerReferences {
		aGV, err := schema.ParseGroupVersion(mOwnerRef.APIVersion)
		if err != nil {
			return "", errors.New("Failed to parse the group and version")
		}
		if aGV.Group != controlplanev1.GroupVersion.Group {
			continue
		}
		// adding prefix to ControlPlane name in order to be able to differentiate
		// ControlPlane and MachineDeployment in case they have the same names set in the cluster.
		m.Log.Info("Fetched ControlPlane name", "controlPlane", "cp-"+mOwnerRef.Name)
		return "cp-" + mOwnerRef.Name, nil
	}
	return "", errors.New("ControlPlane name is not found")
}

// getMachineDeploymentName retrieves the MachineDeployment object name corresponding to the MachineSet.
func (m *MachineManager) getMachineDeploymentName(ctx context.Context) (string, error) {
	m.Log.Info("Fetching MachineDeployment name")

	// Fetch MachineSet.
	m.Log.Info("Fetching MachineSet first to find corresponding MachineDeployment later")

	machineSet, err := m.getMachineSet(ctx)
	if err != nil {
		return "", err
	}
	if machineSet.ObjectMeta.OwnerReferences == nil {
		return "", errors.New("Machineset owner reference is not populated")
	}
	for _, msOwnerRef := range machineSet.ObjectMeta.OwnerReferences {
		if msOwnerRef.Kind != "MachineDeployment" {
			continue
		}
		aGV, err := schema.ParseGroupVersion(msOwnerRef.APIVersion)
		if err != nil {
			return "", errors.New("Failed to parse the group and version")
		}
		if aGV.Group != clusterv1.GroupVersion.Group {
			continue
		}
		// adding prefix to MachineDeployment name in order to be able to differentiate
		// MachineDeployment and ControlPlane in case they have the same names set in the cluster.
		m.Log.Info("Fetched MachineDeployment name", "machinedeployment", "md-"+msOwnerRef.Name)
		return "md-" + msOwnerRef.Name, nil
	}
	return "", errors.New("MachineDeployment name is not found")
}

// getMachineSet retrieves the MachineSet object corresponding to the CAPI machine.
func (m *MachineManager) getMachineSet(ctx context.Context) (*clusterv1.MachineSet, error) {
	m.Log.Info("Fetching MachineSet name")
	// Get list of MachineSets.
	machineSets := &clusterv1.MachineSetList{}
	if m.Machine == nil {
		return nil, errors.New("Could not find corresponding machine object")
	}
	if m.isControlPlane() {
		return nil, errors.New("Machine is controlplane, MachineSet can not be associated with it")
	}
	if m.Machine.ObjectMeta.OwnerReferences == nil {
		return nil, errors.New("Machine owner reference is not populated")
	}
	if err := m.client.List(ctx, machineSets, client.InNamespace(m.Machine.Namespace)); err != nil {
		return nil, err
	}

	// Iterate over MachineSets list and find MachineSet which references specific machine.
	for index := range machineSets.Items {
		machineset := &machineSets.Items[index]
		for _, mOwnerRef := range m.Machine.ObjectMeta.OwnerReferences {
			if mOwnerRef.Kind != machineset.Kind {
				continue
			}
			if mOwnerRef.APIVersion != machineset.APIVersion {
				continue
			}
			if mOwnerRef.UID != machineset.UID {
				continue
			}
			if mOwnerRef.Name == machineset.Name {
				m.Log.Info("Found MachineSet corresponding to machine", "machineset", machineset.Name)
				return machineset, nil
			}
		}
	}
	return nil, errors.New("MachineSet is not found")
}

// getBmhNameFromM3Machine retrieves bmhName from m3m annotations.
func (m *MachineManager) getBmhNameFromM3Machine() (string, error) {
	annotationValue := m.Metal3Machine.ObjectMeta.GetAnnotations()[HostAnnotation]
	valueParts := strings.Split(annotationValue, "/")
	//nolint:mnd
	if (len(valueParts) < 2) || (valueParts[0] != m.Metal3Machine.GetNamespace()) {
		errMessage := fmt.Sprintf("unable to retrieve bmh name from metal3machine: %s , using annotation: %s", m.Metal3Machine.GetName(), annotationValue)
		return "", errors.New(errMessage)
	}
	bmhName := valueParts[1]
	return bmhName, nil
}

// getBmhUIDFromM3Machine retrieves bmhUID from m3m.
func (m *MachineManager) getBmhUIDFromM3Machine(ctx context.Context) (string, error) {
	host, err := getHost(ctx, m.Metal3Machine, m.client, m.Log)
	if err != nil || host == nil {
		errMessage := "Failed to get a BaremetalHost for the metal3machine: " + m.Metal3Machine.GetName()
		return "", errors.New(errMessage)
	}
	if host.UID == "" {
		return "", errors.New("Missing BaremetalHost UID")
	}
	return string(host.UID), nil
}

// getNodesWithLabel gets kubernetes nodes with a given label.
func (m *MachineManager) getNodesWithLabel(ctx context.Context, nodeLabel string, clientFactory ClientGetter) (*corev1.NodeList, int, error) {
	corev1Remote, err := clientFactory(ctx, m.client, m.Cluster)
	if err != nil {
		return nil, 0, errors.Wrap(err, "Error creating a remote client")
	}
	nodesCount := 0
	filter := metav1.ListOptions{
		LabelSelector: nodeLabel,
	}

	nodes, err := corev1Remote.Nodes().List(ctx, filter)
	if err != nil {
		m.Log.Error(err, "error while retrieving nodes with label", "nodelabel", nodeLabel)
		return nil, 0, err
	}
	if nodes != nil {
		nodesCount = len(nodes.Items)
	}
	return nodes, nodesCount, err
}

// SetNodeProviderIDByHostname finds a Node whose hostname matches at least one of the Metal3Machine's hostnames and sets a ProviderID on it.
func (m *MachineManager) SetNodeProviderIDByHostname(ctx context.Context, clientFactory ClientGetter) error {
	corev1Remote, err := clientFactory(ctx, m.client, m.Cluster)
	if err != nil {
		return errors.Wrap(err, "Error creating a remote client")
	}

	metal3MachineHostnames := m.getMetal3MachineHostnames()

	if len(metal3MachineHostnames) == 0 {
		errMessage := "No machine hostname"
		m.Log.Info(errMessage)
		return WithTransientError(errors.New(errMessage), requeueAfter)
	}

	nodes, err := corev1Remote.Nodes().List(ctx, metav1.ListOptions{})

	if err != nil {
		m.Log.Error(err, "error while retrieving nodes")
		return WithTransientError(err, requeueAfter)
	}

	if len(nodes.Items) == 0 {
		errMessage := "no Nodes found"
		m.Log.Info(errMessage)
		return WithTransientError(errors.New(errMessage), requeueAfter)
	}

	matchingNodes := []corev1.Node{}

	for _, node := range nodes.Items {
		hostnameLabel, ok := node.GetLabels()["kubernetes.io/hostname"]
		if !ok {
			continue
		}

		for _, metal3MachineHostname := range metal3MachineHostnames {
			if metal3MachineHostname == hostnameLabel {
				matchingNodes = append(matchingNodes, node)
				break
			}
		}
	}

	if len(matchingNodes) == 0 {
		errMessage := "no nodes matching hostnames: " + strings.Join(metal3MachineHostnames, ", ")
		m.Log.Info(errMessage)
		return WithTransientError(errors.New(errMessage), requeueAfter)
	}

	if len(matchingNodes) > 1 {
		errMessage := "multiple nodes matching hostnames: " + strings.Join(metal3MachineHostnames, ", ")
		m.Log.Info(errMessage)
		return WithTransientError(errors.New(errMessage), requeueAfter)
	}

	node := matchingNodes[0]

	m.Log.Info("found a node, setting provider id on it", "node", node.Name)

	err = m.setNodeProviderID(ctx, corev1Remote, node, *m.Metal3Machine.Spec.ProviderID)

	if err != nil {
		return errors.Wrap(err, "unable to update the target node with providerID")
	}

	m.Log.Info("node updated with provider id")
	return nil
}

func (m *MachineManager) getNodeByProviderID(ctx context.Context, providerIDLegacy, providerIDNew string, clientFactory ClientGetter) (corev1.Node, error) {
	corev1Remote, err := clientFactory(ctx, m.client, m.Cluster)
	if err != nil {
		return corev1.Node{}, errors.Wrap(err, "Error creating a remote client")
	}

	filter := metav1.ListOptions{}
	nodes, err := corev1Remote.Nodes().List(ctx, filter)
	if err != nil {
		m.Log.Error(err, "error while retrieving nodes")
		return corev1.Node{}, err
	}

	// kubernetes nodes with their providerID and name fields set
	validNodes := make(map[string][]corev1.Node)
	matchingNodeProviderID := ""
	for _, node := range nodes.Items {
		providerIDOnNode := node.Spec.ProviderID
		if providerIDOnNode == "" {
			m.Log.Info("no providerID value found on node", "node", node.GetName())
		} else if providerIDOnNode == providerIDNew {
			matchingNodeProviderID = providerIDNew
		} else if providerIDOnNode == providerIDLegacy {
			matchingNodeProviderID = providerIDLegacy
		} else {
			m.Log.Info("The node does not match expected providerID. Considering other nodes ", "node", node.GetName(), "providerID", providerIDOnNode)
		}
		if providerIDOnNode != "" && node.GetName() != "" {
			validNodes[providerIDOnNode] = append(validNodes[providerIDOnNode], node)
		}
	}
	err = m.duplicateProviderIDsExist(validNodes, providerIDLegacy, providerIDNew)
	if err != nil {
		// There are, at least, two nodes. Details are in err
		return corev1.Node{}, err
	}

	if matchingNodeProviderID != "" {
		nodes := validNodes[matchingNodeProviderID]
		return nodes[0], nil
	}

	return corev1.Node{}, errors.New("unable to find node with matching ProviderID")
}

func getNodeNames(nodes []corev1.Node) string {
	names := []string{}

	for _, node := range nodes {
		names = append(names, node.GetName())
	}

	return strings.Join(names, ", ")
}

// duplicateProviderIDsExist determnes if a providerID is already in use by other nodes.
func (m *MachineManager) duplicateProviderIDsExist(validNodes map[string][]corev1.Node, providerIDLegacy, providerIDNew string) error {
	duplicateUsageCounter := 0
	var duplicateNodes []corev1.Node
	// Check if any node has metal3://<namespace>/<bmh>/ as a beginning of providerID
	newProviderIDMatch := providerIDNew[:strings.LastIndex(providerIDNew, "/")+1]
	for providerID, nodes := range validNodes {
		// verify if the same providerId is not consumed twice. Example:
		// legacy provider-id: metal3://d668eb95-5df6-4c10-a01a-fc69f4299fc6
		// new format provider-id:   metal3://metal3/node-0/test-controlplane-xyz
		// The above two providerIDs are the same and would consume the same bmh node,
		// the former using uuid and the latter using a name.
		// The check below prevents such double providerID consumptions.
		if providerID == providerIDLegacy || strings.Contains(providerID, newProviderIDMatch) {
			duplicateUsageCounter++
			duplicateNodes = append(duplicateNodes, nodes...)
		}
		// duplicates due to the same, at least, two instances of legacy OR new OR unknown formats being used in multiple nodes.
		if len(nodes) > 1 {
			matchingNodes := getNodeNames(nodes)
			errMessage := fmt.Sprintf("providerID %s is in use by multiple nodes: (%s)", providerID, matchingNodes)
			m.Log.Info(errMessage)
			return errors.New(errMessage)
		}
	}
	// duplicates due to both the legacy AND new providerIDs being used by multiple nodes.
	if duplicateUsageCounter > 1 {
		duplicateNodesNames := getNodeNames(duplicateNodes)
		errMessage := fmt.Sprintf("both providerIDs (%s and %s) cannot be used at the same time by node: (%s)", providerIDLegacy, providerIDNew, duplicateNodesNames)
		m.Log.Info(errMessage)
		return errors.New(errMessage)
	}
	return nil
}

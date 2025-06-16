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

package baremetal

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	powerOffAnnotation              = "reboot.metal3.io/metal3-remediation-%s"
	nodeAnnotationsBackupAnnotation = "remediation.metal3.io/node-annotations-backup"
	nodeLabelsBackupAnnotation      = "remediation.metal3.io/node-labels-backup"
)

// RemediationManagerInterface is an interface for a RemediationManager.
type RemediationManagerInterface interface {
	SetFinalizer()
	UnsetFinalizer()
	HasFinalizer() bool
	TimeToRemediate(timeout time.Duration) (bool, time.Duration)
	SetPowerOffAnnotation(ctx context.Context) error
	RemovePowerOffAnnotation(ctx context.Context) error
	IsPowerOffRequested(ctx context.Context) (bool, error)
	IsPoweredOn(ctx context.Context) (bool, error)
	SetUnhealthyAnnotation(ctx context.Context) error
	GetUnhealthyHost(ctx context.Context) (*bmov1alpha1.BareMetalHost, *patch.Helper, error)
	OnlineStatus(host *bmov1alpha1.BareMetalHost) bool
	GetRemediationType() infrav1.RemediationType
	RetryLimitIsSet() bool
	HasReachRetryLimit() bool
	SetRemediationPhase(phase string)
	GetRemediationPhase() string
	GetLastRemediatedTime() *metav1.Time
	SetLastRemediationTime(remediationTime *metav1.Time)
	GetTimeout() *metav1.Duration
	IncreaseRetryCount()
	SetOwnerRemediatedConditionNew(ctx context.Context) error
	GetCapiMachine(ctx context.Context) (*clusterv1.Machine, error)
	GetNode(ctx context.Context, clusterClient v1.CoreV1Interface) (*corev1.Node, error)
	UpdateNode(ctx context.Context, clusterClient v1.CoreV1Interface, node *corev1.Node) error
	DeleteNode(ctx context.Context, clusterClient v1.CoreV1Interface, node *corev1.Node) error
	GetClusterClient(ctx context.Context) (v1.CoreV1Interface, error)
	SetNodeBackupAnnotations(annotations string, labels string) bool
	GetNodeBackupAnnotations() (annotations, labels string)
	RemoveNodeBackupAnnotations()
	AddOutOfServiceTaint(ctx context.Context, clusterClient v1.CoreV1Interface, node *corev1.Node) error
	RemoveOutOfServiceTaint(ctx context.Context, clusterClient v1.CoreV1Interface, node *corev1.Node) error
	HasOutOfServiceTaint(node *corev1.Node) bool
	IsNodeDrained(ctx context.Context, clusterClient v1.CoreV1Interface, node *corev1.Node) bool
}

var outOfServiceTaint = &corev1.Taint{
	Key:    "node.kubernetes.io/out-of-service",
	Value:  "nodeshutdown",
	Effect: corev1.TaintEffectNoExecute,
}
var ErrNodeIsNotFound = errors.New("Node is not found")

// RemediationManager is responsible for performing remediation reconciliation.
type RemediationManager struct {
	Client            client.Client
	CapiClientGetter  ClientGetter
	Metal3Remediation *infrav1.Metal3Remediation
	Metal3Machine     *infrav1.Metal3Machine
	Machine           *clusterv1.Machine
	Log               logr.Logger
}

// enforce implementation of interface.
var _ RemediationManagerInterface = &RemediationManager{}

// NewRemediationManager returns a new helper for managing a Metal3Remediation object.
func NewRemediationManager(client client.Client, capiClientGetter ClientGetter,
	metal3remediation *infrav1.Metal3Remediation, metal3Machine *infrav1.Metal3Machine, machine *clusterv1.Machine,
	remediationLog logr.Logger) (*RemediationManager, error) {
	return &RemediationManager{
		Client:            client,
		CapiClientGetter:  capiClientGetter,
		Metal3Remediation: metal3remediation,
		Metal3Machine:     metal3Machine,
		Machine:           machine,
		Log:               remediationLog,
	}, nil
}

// SetFinalizer sets finalizer. Return if it was set.
func (r *RemediationManager) SetFinalizer() {
	controllerutil.AddFinalizer(r.Metal3Remediation, infrav1.RemediationFinalizer)
}

// UnsetFinalizer unsets finalizer.
func (r *RemediationManager) UnsetFinalizer() {
	controllerutil.RemoveFinalizer(r.Metal3Remediation, infrav1.RemediationFinalizer)
}

// HasFinalizer returns if finalizer is set.
func (r *RemediationManager) HasFinalizer() bool {
	return controllerutil.ContainsFinalizer(r.Metal3Remediation, infrav1.RemediationFinalizer)
}

// TimeToRemediate checks if it is time to execute a next remediation step
// and returns seconds to next remediation time.
func (r *RemediationManager) TimeToRemediate(timeout time.Duration) (bool, time.Duration) {
	now := time.Now()

	// status is not updated yet
	if r.Metal3Remediation.Status.LastRemediated == nil {
		return false, timeout
	}

	if r.Metal3Remediation.Status.LastRemediated.Add(timeout).Before(now) {
		return true, time.Duration(0)
	}

	lastRemediated := now.Sub(r.Metal3Remediation.Status.LastRemediated.Time)
	nextRemediation := timeout - lastRemediated + time.Second
	return false, nextRemediation
}

// SetPowerOffAnnotation sets poweroff annotation on unhealthy host.
func (r *RemediationManager) SetPowerOffAnnotation(ctx context.Context) error {
	host, helper, err := r.GetUnhealthyHost(ctx)
	if err != nil {
		return err
	}
	if host == nil {
		return errors.New("Unable to set a PowerOff Annotation, Host not found")
	}

	r.Log.Info("Adding PowerOff annotation to host", "host", host.Name)
	rebootMode := bmov1alpha1.RebootAnnotationArguments{}
	rebootMode.Mode = bmov1alpha1.RebootModeHard
	marshalledMode, err := json.Marshal(rebootMode)

	if err != nil {
		return err
	}

	if host.Annotations == nil {
		host.Annotations = make(map[string]string)
	}
	host.Annotations[r.getPowerOffAnnotationKey()] = string(marshalledMode)
	return helper.Patch(ctx, host)
}

// RemovePowerOffAnnotation removes poweroff annotation from unhealthy host.
func (r *RemediationManager) RemovePowerOffAnnotation(ctx context.Context) error {
	host, helper, err := r.GetUnhealthyHost(ctx)
	if err != nil {
		return err
	}
	if host == nil {
		return errors.New("Unable to remove PowerOff Annotation, Host not found")
	}

	r.Log.Info("Removing PowerOff annotation from host", "host name", host.Name)
	delete(host.Annotations, r.getPowerOffAnnotationKey())
	return helper.Patch(ctx, host)
}

// IsPowerOffRequested returns true if poweroff annotation is set.
func (r *RemediationManager) IsPowerOffRequested(ctx context.Context) (bool, error) {
	host, _, err := r.GetUnhealthyHost(ctx)
	if err != nil {
		return false, err
	}
	if host == nil {
		return false, errors.New("Unable to check PowerOff Annotation, Host not found")
	}

	if _, ok := host.Annotations[r.getPowerOffAnnotationKey()]; ok {
		return true, nil
	}
	return false, nil
}

// IsPoweredOn returns true if the host is powered on.
func (r *RemediationManager) IsPoweredOn(ctx context.Context) (bool, error) {
	host, _, err := r.GetUnhealthyHost(ctx)
	if err != nil {
		return false, err
	}
	if host == nil {
		return false, errors.New("Unable to check power status, Host not found")
	}

	return host.Status.PoweredOn, nil
}

// SetUnhealthyAnnotation sets capm3.UnhealthyAnnotation on unhealthy host.
func (r *RemediationManager) SetUnhealthyAnnotation(ctx context.Context) error {
	host, helper, err := r.GetUnhealthyHost(ctx)
	if err != nil {
		return err
	}
	if host == nil {
		return errors.New("Unable to set an Unhealthy Annotation, Host not found")
	}

	r.Log.Info("Adding Unhealthy annotation to host", "host", host.Name)
	if host.Annotations == nil {
		host.Annotations = make(map[string]string, 1)
	}
	host.Annotations[infrav1.UnhealthyAnnotation] = "capm3/UnhealthyNode"
	return helper.Patch(ctx, host)
}

// GetUnhealthyHost gets the associated host for unhealthy machine. Returns nil if not found. Assumes the
// host is in the same namespace as the unhealthy machine.
func (r *RemediationManager) GetUnhealthyHost(ctx context.Context) (*bmov1alpha1.BareMetalHost, *patch.Helper, error) {
	host, err := getUnhealthyHost(ctx, r.Metal3Machine, r.Client, r.Log)
	if err != nil || host == nil {
		return host, nil, err
	}
	helper, err := patch.NewHelper(host, r.Client)
	return host, helper, err
}

func getUnhealthyHost(ctx context.Context, m3Machine *infrav1.Metal3Machine, cl client.Client,
	rLog logr.Logger,
) (*bmov1alpha1.BareMetalHost, error) {
	annotations := m3Machine.ObjectMeta.GetAnnotations()
	if annotations == nil {
		err := fmt.Errorf("unable to get %s annotations", m3Machine.Name)
		return nil, err
	}
	hostKey, ok := annotations[HostAnnotation]
	if !ok {
		err := fmt.Errorf("unable to get %s HostAnnotation", m3Machine.Name)
		return nil, err
	}
	hostNamespace, hostName, err := cache.SplitMetaNamespaceKey(hostKey)
	if err != nil {
		rLog.Error(err, "Error parsing annotation value", "annotation key", hostKey)
		return nil, err
	}

	host := bmov1alpha1.BareMetalHost{}
	key := client.ObjectKey{
		Name:      hostName,
		Namespace: hostNamespace,
	}
	err = cl.Get(ctx, key, &host)
	if apierrors.IsNotFound(err) {
		rLog.Info("Annotated host not found", "host", hostKey)
		return nil, err
	} else if err != nil {
		return nil, err
	}
	return &host, nil
}

// OnlineStatus returns hosts Online field value.
func (r *RemediationManager) OnlineStatus(host *bmov1alpha1.BareMetalHost) bool {
	return host.Spec.Online
}

// GetRemediationType return type of remediation strategy.
func (r *RemediationManager) GetRemediationType() infrav1.RemediationType {
	if r.Metal3Remediation.Spec.Strategy == nil {
		return ""
	}
	return r.Metal3Remediation.Spec.Strategy.Type
}

// RetryLimitIsSet returns true if retryLimit is set, false if not.
func (r *RemediationManager) RetryLimitIsSet() bool {
	if r.Metal3Remediation.Spec.Strategy == nil {
		return false
	}
	return r.Metal3Remediation.Spec.Strategy.RetryLimit > 0
}

// HasReachRetryLimit returns true if retryLimit is reached.
func (r *RemediationManager) HasReachRetryLimit() bool {
	if r.Metal3Remediation.Spec.Strategy == nil {
		return false
	}
	return r.Metal3Remediation.Spec.Strategy.RetryLimit == r.Metal3Remediation.Status.RetryCount
}

// SetRemediationPhase setting the state of the remediation.
func (r *RemediationManager) SetRemediationPhase(phase string) {
	r.Log.Info("Switching remediation phase", "remediationPhase", phase)
	r.Metal3Remediation.Status.Phase = phase
}

// GetRemediationPhase returns current status of the remediation.
func (r *RemediationManager) GetRemediationPhase() string {
	return r.Metal3Remediation.Status.Phase
}

// GetLastRemediatedTime returns last remediation time.
func (r *RemediationManager) GetLastRemediatedTime() *metav1.Time {
	return r.Metal3Remediation.Status.LastRemediated
}

// SetLastRemediationTime setting last remediation timestamp on Status.
func (r *RemediationManager) SetLastRemediationTime(remediationTime *metav1.Time) {
	r.Log.Info("Last remediation time", "remediationTime", remediationTime)
	r.Metal3Remediation.Status.LastRemediated = remediationTime
}

// GetTimeout returns timeout duration from remediation request Spec.
func (r *RemediationManager) GetTimeout() *metav1.Duration {
	return r.Metal3Remediation.Spec.Strategy.Timeout
}

// IncreaseRetryCount increases the retry count on Status.
func (r *RemediationManager) IncreaseRetryCount() {
	r.Metal3Remediation.Status.RetryCount++
}

// SetOwnerRemediatedConditionNew sets MachineOwnerRemediatedCondition on CAPI machine object
// that have failed a healthcheck.
func (r *RemediationManager) SetOwnerRemediatedConditionNew(ctx context.Context) error {
	capiMachine, err := r.GetCapiMachine(ctx)
	if err != nil {
		r.Log.Info("Unable to fetch CAPI Machine")
		return err
	}

	machineHelper, err := patch.NewHelper(capiMachine, r.Client)
	if err != nil {
		r.Log.Info("Unable to create patch helper for Machine")
		return err
	}
	conditions.MarkFalse(capiMachine, clusterv1.MachineOwnerRemediatedCondition, clusterv1.WaitingForRemediationReason, clusterv1.ConditionSeverityWarning, "")
	err = machineHelper.Patch(ctx, capiMachine)
	if err != nil {
		r.Log.Info("Unable to patch Machine", "machine", capiMachine)
		return err
	}
	return nil
}

// GetCapiMachine returns CAPI machine object owning the current resource.
func (r *RemediationManager) GetCapiMachine(ctx context.Context) (*clusterv1.Machine, error) {
	capiMachine, err := util.GetOwnerMachine(ctx, r.Client, r.Metal3Remediation.ObjectMeta)
	if err != nil {
		r.Log.Error(err, "metal3Remediation's owner Machine could not be retrieved")
		return nil, errors.Wrapf(err, "metal3Remediation's owner Machine could not be retrieved")
	}
	return capiMachine, nil
}

// GetNode returns the Node associated with the machine in the current context.
func (r *RemediationManager) GetNode(ctx context.Context, clusterClient v1.CoreV1Interface) (*corev1.Node, error) {
	capiMachine, err := r.GetCapiMachine(ctx)
	if err != nil {
		r.Log.Error(err, "metal3Remediation's node could not be retrieved")
		return nil, errors.Wrapf(err, "metal3Remediation's node could not be retrieved")
	}
	if capiMachine.Status.NodeRef == nil {
		r.Log.Error(nil, "metal3Remediation's node could not be retrieved, machine's nodeRef is nil")
		return nil, errors.Errorf("metal3Remediation's node could not be retrieved, machine's nodeRef is nil")
	}

	node, err := clusterClient.Nodes().Get(ctx, capiMachine.Status.NodeRef.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil, ErrNodeIsNotFound
	} else if err != nil {
		r.Log.Error(err, "Could not get cluster node")
		return nil, errors.Wrapf(err, "Could not get cluster node")
	}
	return node, nil
}

// UpdateNode updates the given node.
func (r *RemediationManager) UpdateNode(ctx context.Context, clusterClient v1.CoreV1Interface, node *corev1.Node) error {
	_, err := clusterClient.Nodes().Update(ctx, node, metav1.UpdateOptions{})
	if err != nil {
		r.Log.Error(err, "Could not update cluster node")
		return errors.Wrapf(err, "Could not update cluster node")
	}
	return nil
}

// DeleteNode deletes the given node.
func (r *RemediationManager) DeleteNode(ctx context.Context, clusterClient v1.CoreV1Interface, node *corev1.Node) error {
	if !node.DeletionTimestamp.IsZero() {
		return nil
	}

	err := clusterClient.Nodes().Delete(ctx, node.Name, metav1.DeleteOptions{})
	if err != nil {
		r.Log.Error(err, "Could not delete cluster node")
		return errors.Wrapf(err, "Could not delete cluster node")
	}
	return nil
}

// GetClusterClient returns the client for interacting with the target cluster.
func (r *RemediationManager) GetClusterClient(ctx context.Context) (v1.CoreV1Interface, error) {
	capiMachine, err := r.GetCapiMachine(ctx)
	if err != nil {
		r.Log.Error(err, "metal3Remediation's node could not be retrieved")
		return nil, errors.Wrapf(err, "metal3Remediation's node could not be retrieved")
	}

	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, capiMachine.ObjectMeta)
	if err != nil {
		r.Log.Error(err, "Machine is missing cluster label or cluster does not exist")
		return nil, errors.Wrapf(err, "Machine is missing cluster label or cluster does not exist")
	}

	clusterClient, err := r.CapiClientGetter(ctx, r.Client, cluster)
	if err != nil {
		r.Log.Error(err, "Could not get cluster client")
		return nil, errors.Wrapf(err, "Could not get cluster client")
	}

	return clusterClient, nil
}

// SetNodeBackupAnnotations sets the given node annotations and labels as remediation annotations.
// Returns whether annotations were set or modified, or not.
func (r *RemediationManager) SetNodeBackupAnnotations(annotations string, labels string) bool {
	rem := r.Metal3Remediation
	if rem.Annotations == nil {
		rem.Annotations = make(map[string]string)
	}
	if rem.Annotations[nodeAnnotationsBackupAnnotation] != annotations ||
		rem.Annotations[nodeLabelsBackupAnnotation] != labels {
		rem.Annotations[nodeAnnotationsBackupAnnotation] = annotations
		rem.Annotations[nodeLabelsBackupAnnotation] = labels
		return true
	}
	return false
}

// GetNodeBackupAnnotations gets the stringified annotations and labels from the remediation annotations.
func (r *RemediationManager) GetNodeBackupAnnotations() (annotations, labels string) {
	rem := r.Metal3Remediation
	if rem.Annotations == nil {
		return "", ""
	}
	annotations = rem.Annotations[nodeAnnotationsBackupAnnotation]
	labels = rem.Annotations[nodeLabelsBackupAnnotation]
	return
}

// RemoveNodeBackupAnnotations removes the node backup annotation from the remediation resource.
func (r *RemediationManager) RemoveNodeBackupAnnotations() {
	rem := r.Metal3Remediation
	if rem.Annotations == nil {
		return
	}
	delete(rem.Annotations, nodeAnnotationsBackupAnnotation)
	delete(rem.Annotations, nodeLabelsBackupAnnotation)
}

// getPowerOffAnnotationKey returns the key of the power off annotation.
func (r *RemediationManager) getPowerOffAnnotationKey() string {
	return fmt.Sprintf(powerOffAnnotation, r.Metal3Remediation.UID)
}

func (r *RemediationManager) HasOutOfServiceTaint(node *corev1.Node) bool {
	for _, taint := range node.Spec.Taints {
		if taint.MatchTaint(outOfServiceTaint) {
			return true
		}
	}
	return false
}

func (r *RemediationManager) AddOutOfServiceTaint(ctx context.Context, clusterClient v1.CoreV1Interface, node *corev1.Node) error {
	taint := outOfServiceTaint
	now := metav1.Now()
	taint.TimeAdded = &now
	node.Spec.Taints = append(node.Spec.Taints, *taint)
	if err := r.UpdateNode(ctx, clusterClient, node); err != nil {
		msg := "failed to add out-of-service taint on node " + node.Name
		r.Log.Error(err, msg)
		return errors.Wrap(err, msg)
	}
	r.Log.Info("Out-of-service taint added", "node", node.Name)
	return nil
}

func (r *RemediationManager) RemoveOutOfServiceTaint(ctx context.Context, clusterClient v1.CoreV1Interface, node *corev1.Node) error {
	newTaints := []corev1.Taint{}

	var isPopOutOfServiceTaint bool
	for _, taint := range node.Spec.Taints {
		if taint.MatchTaint(outOfServiceTaint) {
			isPopOutOfServiceTaint = true
			continue
		}
		newTaints = append(newTaints, taint)
	}

	if isPopOutOfServiceTaint {
		r.Log.Info("Removing out-of-service taint from node taints", "node", node.Name)
		node.Spec.Taints = newTaints
	} else {
		r.Log.Info("Out-of-service taint not found. Nothing to do", "node name", node.Name)
		return nil
	}

	if err := r.UpdateNode(ctx, clusterClient, node); err != nil {
		msg := "failed to remove out-of-service taint on node " + node.Name
		r.Log.Error(err, msg)
		return errors.Wrap(err, msg)
	}

	r.Log.Info("Out-of-service taint removed", "node", node.Name)
	return nil
}

func (r *RemediationManager) IsNodeDrained(ctx context.Context, clusterClient v1.CoreV1Interface, node *corev1.Node) bool {
	pods, err := clusterClient.Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		r.Log.Error(err, "failed to get pod list in the cluster")
		return false
	}

	for _, pod := range pods.Items {
		if pod.Spec.NodeName == node.Name {
			if pod.ObjectMeta.DeletionTimestamp != nil {
				r.Log.Info("Waiting for terminating pod", "node", node.Name, "pod name", pod.Name, "phase", pod.Status.Phase)
				return false
			}
		}
	}

	volumeAttachments := &storagev1.VolumeAttachmentList{}
	if err := r.Client.List(ctx, volumeAttachments); err != nil {
		r.Log.Error(err, "failed to get volumeAttachments list")
		return false
	}
	for _, va := range volumeAttachments.Items {
		if va.Spec.NodeName == node.Name {
			r.Log.Info("Waiting for deleting volumeAttachement", "node", node.Name, "name", va.Name)
			return false
		}
	}

	r.Log.Info("Node is drained", "node", node.Name)
	return true
}

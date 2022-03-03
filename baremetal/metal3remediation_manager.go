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
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"

	bmh "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	rebootAnnotation = "reboot.metal3.io"
)

// RemediationManagerInterface is an interface for a RemediationManager.
type RemediationManagerInterface interface {
	SetFinalizer()
	UnsetFinalizer()
	TimeToRemediate(timeout time.Duration) (bool, time.Duration)
	SetRebootAnnotation(ctx context.Context) error
	SetUnhealthyAnnotation(ctx context.Context) error
	GetUnhealthyHost(ctx context.Context) (*bmh.BareMetalHost, *patch.Helper, error)
	OnlineStatus(host *bmh.BareMetalHost) bool
	GetRemediationType() capm3.RemediationType
	RetryLimitIsSet() bool
	SetRemediationPhase(phase string)
	GetRemediationPhase() string
	GetLastRemediatedTime() *metav1.Time
	SetLastRemediationTime(remediationTime *metav1.Time)
	HasReachRetryLimit() bool
	GetTimeout() *metav1.Duration
	IncreaseRetryCount()
	SetOwnerRemediatedConditionNew(ctx context.Context) error
	GetCapiMachine(ctx context.Context) (*clusterv1.Machine, error)
}

// RemediationManager is responsible for performing remediation reconciliation.
type RemediationManager struct {
	Client            client.Client
	Metal3Remediation *capm3.Metal3Remediation
	Metal3Machine     *capm3.Metal3Machine
	Machine           *clusterv1.Machine
	Log               logr.Logger
}

// NewRemediationManager returns a new helper for managing a Metal3Remediation object.
func NewRemediationManager(client client.Client,
	metal3remediation *capm3.Metal3Remediation, metal3Machine *capm3.Metal3Machine, machine *clusterv1.Machine,
	remediationLog logr.Logger) (*RemediationManager, error) {
	return &RemediationManager{
		Client:            client,
		Metal3Remediation: metal3remediation,
		Metal3Machine:     metal3Machine,
		Machine:           machine,
		Log:               remediationLog,
	}, nil
}

// SetFinalizer sets finalizer.
func (r *RemediationManager) SetFinalizer() {
	// If the Metal3Remediation doesn't have finalizer, add it.
	if !Contains(r.Metal3Remediation.Finalizers, capm3.RemediationFinalizer) {
		r.Metal3Remediation.Finalizers = append(r.Metal3Remediation.Finalizers,
			capm3.RemediationFinalizer,
		)
	}
}

// UnsetFinalizer unsets finalizer.
func (r *RemediationManager) UnsetFinalizer() {
	// Cluster is deleted so remove the finalizer.
	r.Metal3Remediation.Finalizers = Filter(r.Metal3Remediation.Finalizers,
		capm3.RemediationFinalizer,
	)
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

// SetRebootAnnotation sets reboot annotation on unhealthy host.
func (r *RemediationManager) SetRebootAnnotation(ctx context.Context) error {
	host, helper, err := r.GetUnhealthyHost(ctx)
	if err != nil {
		return err
	}
	if host == nil {
		return errors.New("Unable to set an Reboot Annotation, Host not found")
	}

	r.Log.Info("Adding Reboot annotation to host", host.Name)
	rebootMode := bmh.RebootAnnotationArguments{}
	rebootMode.Mode = bmh.RebootModeHard
	marshalledMode, err := json.Marshal(rebootMode)

	if err != nil {
		return err
	}

	host.Annotations[rebootAnnotation] = string(marshalledMode)
	return helper.Patch(ctx, host)
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

	r.Log.Info("Adding Unhealthy annotation to host", host.Name)
	host.Annotations[capm3.UnhealthyAnnotation] = "capm3/UnhealthyNode"
	return helper.Patch(ctx, host)
}

// GetUnhealthyHost gets the associated host for unhealthy machine. Returns nil if not found. Assumes the
// host is in the same namespace as the unhealthy machine.
func (r *RemediationManager) GetUnhealthyHost(ctx context.Context) (*bmh.BareMetalHost, *patch.Helper, error) {
	host, err := getUnhealthyHost(ctx, r.Metal3Machine, r.Client, r.Log)
	if err != nil || host == nil {
		return host, nil, err
	}
	helper, err := patch.NewHelper(host, r.Client)
	return host, helper, err
}

func getUnhealthyHost(ctx context.Context, m3Machine *capm3.Metal3Machine, cl client.Client,
	rLog logr.Logger,
) (*bmh.BareMetalHost, error) {
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

	host := bmh.BareMetalHost{}
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
func (r *RemediationManager) OnlineStatus(host *bmh.BareMetalHost) bool {
	return host.Spec.Online
}

// GetRemediationType return type of remediation strategy.
func (r *RemediationManager) GetRemediationType() capm3.RemediationType {
	return r.Metal3Remediation.Spec.Strategy.Type
}

// RetryLimitIsSet returns true if retryLimit is set, false if not.
func (r *RemediationManager) RetryLimitIsSet() bool {
	return r.Metal3Remediation.Spec.Strategy.RetryLimit > 0
}

// HasReachRetryLimit returns true if retryLimit is reached.
func (r *RemediationManager) HasReachRetryLimit() bool {
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
		r.Log.Info("Unable to patch Machine %d", capiMachine)
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

/*
Copyright The Kubernetes Authors.

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

package controllers

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	"github.com/metal3-io/cluster-api-provider-metal3/baremetal"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Metal3RemediationReconciler reconciles a Metal3Remediation object.
type Metal3RemediationReconciler struct {
	client.Client
	ManagerFactory baremetal.ManagerFactoryInterface
	Log            logr.Logger
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metal3remediations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metal3remediations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch;create;update;patch;delete

// Reconcile handles Metal3Remediation events.
func (r *Metal3RemediationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, rerr error) {
	remediationLog := r.Log.WithValues("metal3remediation", req.NamespacedName)

	// Fetch the Metal3Remediation instance.
	metal3Remediation := &capm3.Metal3Remediation{}

	helper, err := patch.NewHelper(metal3Remediation, r.Client)
	if err != nil {
		remediationLog.Error(err, "failed to init patch helper")
		return ctrl.Result{}, err
	}

	defer func() {
		// Always attempt to Patch the Remediation object and status after each reconciliation.
		// Patch ObservedGeneration only if the reconciliation completed successfully
		patchOpts := []patch.Option{}
		patchOpts = append(patchOpts, patch.WithStatusObservedGeneration{})

		err := helper.Patch(ctx, metal3Remediation, patchOpts...)
		if err != nil {
			remediationLog.Error(err, "failed to Patch metal3Remediation")
		}
	}()

	if err := r.Client.Get(ctx, req.NamespacedName, metal3Remediation); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		remediationLog.Error(err, "unable to get metal3Remediation")
		return ctrl.Result{}, err
	}

	// Fetch the Machine.
	capiMachine, err := util.GetOwnerMachine(ctx, r.Client, metal3Remediation.ObjectMeta)
	if err != nil {
		remediationLog.Error(err, "metal3Remediation's owner Machine could not be retrieved")
		return ctrl.Result{}, errors.Wrapf(err, "metal3Remediation's owner Machine could not be retrieved")
	}
	remediationLog = remediationLog.WithValues("unhealthy machine detected", capiMachine.Name)

	// Fetch Metal3Machine
	metal3Machine := capm3.Metal3Machine{}
	key := client.ObjectKey{
		Name:      capiMachine.Spec.InfrastructureRef.Name,
		Namespace: capiMachine.Spec.InfrastructureRef.Namespace,
	}
	err = r.Get(ctx, key, &metal3Machine)
	if err != nil {
		remediationLog.Error(err, "metal3machine not found")
		return ctrl.Result{}, errors.Wrapf(err, "metal3machine not found")
	}

	remediationLog = remediationLog.WithValues("metal3machine", metal3Machine.Name)

	// Create a helper for managing the remediation object.
	remediationMgr, err := r.ManagerFactory.NewRemediationManager(metal3Remediation, &metal3Machine, capiMachine, remediationLog)
	if err != nil {
		remediationLog.Error(err, "failed to create helper for managing the metal3remediation")
		return ctrl.Result{}, errors.Wrapf(err, "failed to create helper for managing the metal3remediation")
	}

	// Handle deleted remediation
	if !metal3Remediation.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, remediationMgr)
	}

	// Handle non-deleted remediation
	return r.reconcileNormal(ctx, remediationMgr)
}

func (r *Metal3RemediationReconciler) reconcileNormal(ctx context.Context,
	remediationMgr baremetal.RemediationManagerInterface,
) (ctrl.Result, error) {
	// If host is gone, exit early
	host, _, err := remediationMgr.GetUnhealthyHost(ctx)
	if err != nil {
		r.Log.Error(err, "unable to find a host for unhealthy machine")
		return ctrl.Result{}, errors.Wrapf(err, "unable to find a host for unhealthy machine")
	}

	// If user has set bmh.Spec.Online to false
	// do not try to remediate the host
	if !remediationMgr.OnlineStatus(host) {
		r.Log.Info("Unable to remediate, Host is powered off (spec.Online is false)")
		remediationMgr.SetRemediationPhase(capm3.PhaseFailed)
		return ctrl.Result{}, nil
	}

	remediationType := remediationMgr.GetRemediationType()

	if remediationType != capm3.RebootRemediationStrategy {
		r.Log.Info("unsupported remediation strategy")
		return ctrl.Result{}, nil
	}

	if remediationType == capm3.RebootRemediationStrategy {
		// If no phase set, default to running
		if remediationMgr.GetRemediationPhase() == "" {
			remediationMgr.SetRemediationPhase(capm3.PhaseRunning)
		}

		switch remediationMgr.GetRemediationPhase() {
		case capm3.PhaseRunning:
			// host is not rebooted yet
			if remediationMgr.GetLastRemediatedTime() == nil {
				r.Log.Info("Rebooting the host")
				err := remediationMgr.SetRebootAnnotation(ctx)
				if err != nil {
					r.Log.Error(err, "error setting reboot annotation")
					return ctrl.Result{}, errors.Wrap(err, "error setting reboot annotation")
				}
				now := metav1.Now()
				remediationMgr.SetLastRemediationTime(&now)
				remediationMgr.IncreaseRetryCount()
			}

			if remediationMgr.RetryLimitIsSet() && !remediationMgr.HasReachRetryLimit() {
				okToRemediate, nextRemediation := remediationMgr.TimeToRemediate(remediationMgr.GetTimeout().Duration)

				if okToRemediate {
					err := remediationMgr.SetRebootAnnotation(ctx)
					if err != nil {
						r.Log.Error(err, "error setting reboot annotation")
						return ctrl.Result{}, errors.Wrapf(err, "error setting reboot annotation")
					}
					now := metav1.Now()
					remediationMgr.SetLastRemediationTime(&now)
					remediationMgr.IncreaseRetryCount()
				}

				if nextRemediation > 0 {
					// Not yet time to remediate, requeue
					return ctrl.Result{RequeueAfter: nextRemediation}, nil
				}
			} else {
				remediationMgr.SetRemediationPhase(capm3.PhaseWaiting)
			}
		case capm3.PhaseWaiting:
			okToStop, nextCheck := remediationMgr.TimeToRemediate(remediationMgr.GetTimeout().Duration)

			if okToStop {
				remediationMgr.SetRemediationPhase(capm3.PhaseDeleting)
				// When machine is still unhealthy after remediation, setting of OwnerRemediatedCondition
				// moves control to CAPI machine controller. The owning controller will do
				// preflight checks and handles the Machine deletion

				err = remediationMgr.SetOwnerRemediatedConditionNew(ctx)
				if err != nil {
					r.Log.Error(err, "error setting cluster api conditions")
					return ctrl.Result{}, errors.Wrapf(err, "error setting cluster api conditions")
				}

				// Remediation failed to set unhealthy annotation on BMH
				// This prevents BMH to be selected as a host.
				err = remediationMgr.SetUnhealthyAnnotation(ctx)
				if err != nil {
					r.Log.Error(err, "error setting unhealthy annotation")
					return ctrl.Result{}, errors.Wrapf(err, "error setting unhealthy annotation")
				}
			}

			if nextCheck > 0 {
				// Not yet time to stop remediation, requeue
				return ctrl.Result{RequeueAfter: nextCheck}, nil
			}
		default:
		}
	}
	return ctrl.Result{}, nil
}

func (r *Metal3RemediationReconciler) reconcileDelete(ctx context.Context,
	remediationMgr baremetal.RemediationManagerInterface,
) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

// SetupWithManager will add watches for Metal3Remediation controller.
func (r *Metal3RemediationReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&capm3.Metal3Remediation{}).
		Complete(r)
}

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
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta2"
	"github.com/metal3-io/cluster-api-provider-metal3/baremetal"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"sigs.k8s.io/cluster-api/controllers/clustercache"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

const (
	defaultTimeout = 5 * time.Second
)

// Metal3RemediationReconciler reconciles a Metal3Remediation object.
type Metal3RemediationReconciler struct {
	client.Client
	ClusterCache               clustercache.ClusterCache
	ManagerFactory             baremetal.ManagerFactoryInterface
	Log                        logr.Logger
	IsOutOfServiceTaintEnabled bool
}

// +kubebuilder:rbac:groups=core,resources=pods,verbs=list
// +kubebuilder:rbac:groups=storage.k8s.io,resources=volumeattachments,verbs=list;watch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metal3remediations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metal3remediations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch;update;patch;delete

// Reconcile handles Metal3Remediation events.
func (r *Metal3RemediationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, rerr error) {
	remediationLog := r.Log.WithValues(
		baremetal.LogFieldMetal3Remediation, req.NamespacedName,
	)
	remediationLog.V(baremetal.VerbosityLevelTrace).Info("Reconcile: starting Metal3Remediation reconciliation")

	// Fetch the Metal3Remediation instance.
	remediationLog.V(baremetal.VerbosityLevelTrace).Info("Fetching Metal3Remediation")
	metal3Remediation := &infrav1.Metal3Remediation{}
	if err := r.Client.Get(ctx, req.NamespacedName, metal3Remediation); err != nil {
		if apierrors.IsNotFound(err) {
			remediationLog.V(baremetal.VerbosityLevelDebug).Info("Metal3Remediation not found, may have been deleted")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("unable to get metal3Remediation: %w", err)
	}
	remediationLog.V(baremetal.VerbosityLevelDebug).Info("Metal3Remediation fetched",
		"phase", metal3Remediation.Status.Phase,
		"retryCount", metal3Remediation.Status.RetryCount)

	remediationLog.V(baremetal.VerbosityLevelTrace).Info("Creating patch helper")
	helper, err := patch.NewHelper(metal3Remediation, r.Client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to init patch helper: %w", err)
	}

	defer func() {
		// Always attempt to Patch the Remediation object and status after each reconciliation.
		// Patch ObservedGeneration only if the reconciliation completed successfully
		remediationLog.V(baremetal.VerbosityLevelTrace).Info("Patching Metal3Remediation on exit")
		patchOpts := make([]patch.Option, 0, 1)
		patchOpts = append(patchOpts, patch.WithStatusObservedGeneration{})

		patchErr := helper.Patch(ctx, metal3Remediation, patchOpts...)
		if patchErr != nil {
			remediationLog.Error(patchErr, "failed to Patch metal3Remediation")
			// trigger requeue!
			rerr = patchErr
		}
	}()

	// Fetch the Machine.
	remediationLog.V(baremetal.VerbosityLevelTrace).Info("Fetching owner Machine")
	capiMachine, err := util.GetOwnerMachine(ctx, r.Client, metal3Remediation.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("metal3Remediation's owner Machine could not be retrieved: %w", err)
	}
	if capiMachine == nil {
		remediationLog.Info("metal3Remediation's owner Machine not set")
		return ctrl.Result{}, errors.New("metal3Remediation's owner Machine not set")
	}
	remediationLog = remediationLog.WithValues(
		baremetal.LogFieldMachine, capiMachine.Name,
	)
	remediationLog.V(baremetal.VerbosityLevelDebug).Info("Owner Machine found",
		"machinePhase", capiMachine.Status.Phase)

	// Fetch Metal3Machine
	remediationLog.V(baremetal.VerbosityLevelTrace).Info("Fetching Metal3Machine")
	metal3Machine := infrav1.Metal3Machine{}
	key := client.ObjectKey{
		Name:      capiMachine.Spec.InfrastructureRef.Name,
		Namespace: capiMachine.Namespace,
	}
	err = r.Get(ctx, key, &metal3Machine)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("metal3machine not found: %w", err)
	}

	remediationLog = remediationLog.WithValues(baremetal.LogFieldMetal3Machine, metal3Machine.Name)
	remediationLog.V(baremetal.VerbosityLevelDebug).Info("Metal3Machine fetched",
		baremetal.LogFieldBMH, metal3Machine.Annotations[baremetal.HostAnnotation])

	// Create a helper for managing the remediation object.
	remediationLog.V(baremetal.VerbosityLevelTrace).Info("Creating RemediationManager")
	remediationMgr, err := r.ManagerFactory.NewRemediationManager(metal3Remediation, &metal3Machine, capiMachine, remediationLog)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create helper for managing the metal3remediation: %w", err)
	}
	remediationLog.V(baremetal.VerbosityLevelDebug).Info("RemediationManager created")

	// Handle both deleted and non-deleted remediations
	remediationLog.V(baremetal.VerbosityLevelTrace).Info("Proceeding with remediation")
	return r.reconcileNormal(ctx, remediationMgr, remediationLog)
}

func (r *Metal3RemediationReconciler) reconcileNormal(ctx context.Context,
	remediationMgr baremetal.RemediationManagerInterface, log logr.Logger,
) (ctrl.Result, error) {
	log.V(baremetal.VerbosityLevelTrace).Info("reconcileNormal: starting remediation workflow")

	// If host is gone, exit early
	log.V(baremetal.VerbosityLevelTrace).Info("Getting unhealthy host")
	host, _, err := remediationMgr.GetUnhealthyHost(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to find a host for unhealthy machine: %w", err)
	}
	if host != nil {
		log.V(baremetal.VerbosityLevelDebug).Info("Unhealthy host found",
			baremetal.LogFieldBMH, host.Name,
			baremetal.LogFieldPoweredOn, host.Status.PoweredOn)
	}

	// If user has set bmh.Spec.Online to false
	// do not try to remediate the host
	log.V(baremetal.VerbosityLevelTrace).Info("Checking host online status")
	if !remediationMgr.OnlineStatus(host) {
		log.Info("Unable to remediate, Host is powered off (spec.Online is false)")
		log.V(baremetal.VerbosityLevelDebug).Info("Setting remediation phase to Failed",
			baremetal.LogFieldOnline, false)
		remediationMgr.SetRemediationPhase(infrav1.PhaseFailed)
		return ctrl.Result{}, nil
	}

	remediationType := remediationMgr.GetRemediationType()
	log.V(baremetal.VerbosityLevelDebug).Info("Remediation type determined",
		"remediationType", remediationType)

	if remediationType != infrav1.RebootRemediationStrategy {
		log.Info("unsupported remediation strategy")
		return ctrl.Result{}, nil
	}

	if remediationType == infrav1.RebootRemediationStrategy {
		currentPhase := remediationMgr.GetRemediationPhase()
		log.V(baremetal.VerbosityLevelDebug).Info("Current remediation phase",
			"phase", currentPhase)

		// If no phase set, default to running and set time and retry count
		if currentPhase == "" {
			log.V(baremetal.VerbosityLevelTrace).Info("No phase set, initializing to PhaseRunning")
			remediationMgr.SetRemediationPhase(infrav1.PhaseRunning)
			now := metav1.Now()
			remediationMgr.SetLastRemediationTime(&now)
			log.V(baremetal.VerbosityLevelDebug).Info("Remediation initialized",
				"phase", infrav1.PhaseRunning,
				"lastRemediationTime", now.String())
			return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
		}

		// try to get node
		log.V(baremetal.VerbosityLevelTrace).Info("Getting cluster client for node access")
		clusterClient, err := remediationMgr.GetClusterClient(ctx)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error getting cluster client: %w", err)
		}

		// handle old clusters which were not setup with RBAC for accessing nodes
		isNodeForbidden := false
		log.V(baremetal.VerbosityLevelTrace).Info("Getting node for remediation")
		node, err := remediationMgr.GetNode(ctx, clusterClient)
		if err != nil {
			if apierrors.IsForbidden(err) {
				log.Info("Node access is forbidden, will skip node deletion")
				isNodeForbidden = true
			} else if !apierrors.IsNotFound(err) {
				return ctrl.Result{}, fmt.Errorf("error getting node for remediation: %w", err)
			} else {
				log.V(baremetal.VerbosityLevelDebug).Info("Node not found, may have been deleted")
			}
		} else if node != nil {
			log.V(baremetal.VerbosityLevelDebug).Info("Node found for remediation",
				baremetal.LogFieldNode, node.Name)
		}

		log.V(baremetal.VerbosityLevelTrace).Info("Processing remediation phase",
			"phase", remediationMgr.GetRemediationPhase())

		switch remediationMgr.GetRemediationPhase() {
		case infrav1.PhaseRunning:
			log.V(baremetal.VerbosityLevelTrace).Info("Phase: Running - executing reboot strategy")
			return r.remediateRebootStrategy(ctx, remediationMgr, clusterClient, node, log)

		case infrav1.PhaseWaiting:
			log.V(baremetal.VerbosityLevelTrace).Info("Phase: Waiting - waiting for power on and node restore")

			// Node is deleted: remove power off annotation
			log.V(baremetal.VerbosityLevelTrace).Info("Checking power off annotation status")
			ok, err := remediationMgr.IsPowerOffRequested(ctx)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("error getting poweroff annotation status: %w", err)
			} else if ok {
				log.Info("Powering on the host")
				err = remediationMgr.RemovePowerOffAnnotation(ctx)
				if err != nil {
					return ctrl.Result{}, fmt.Errorf("error removing poweroff annotation: %w", err)
				}
				log.V(baremetal.VerbosityLevelDebug).Info("Power off annotation removed")
			}

			// Wait until powered on
			log.V(baremetal.VerbosityLevelTrace).Info("Checking power status")
			var on bool
			if on, err = remediationMgr.IsPoweredOn(ctx); err != nil {
				return ctrl.Result{}, fmt.Errorf("error getting power status: %w", err)
			} else if !on {
				log.V(baremetal.VerbosityLevelDebug).Info("Host not yet powered on, requeuing")
				// wait a bit before checking again if we are powered on
				return ctrl.Result{RequeueAfter: defaultTimeout}, nil
			}
			log.V(baremetal.VerbosityLevelDebug).Info("Host is powered on")

			// Restore node if available and not done yet
			log.V(baremetal.VerbosityLevelTrace).Info("Checking finalizer and node status")
			if remediationMgr.HasFinalizer() {
				log.V(baremetal.VerbosityLevelDebug).Info("Remediation has finalizer",
					"hasNode", node != nil,
					"outOfServiceTaintEnabled", r.IsOutOfServiceTaintEnabled)

				if node != nil {
					if r.IsOutOfServiceTaintEnabled {
						if remediationMgr.HasOutOfServiceTaint(node) {
							log.V(baremetal.VerbosityLevelTrace).Info("Removing out-of-service taint from node",
								baremetal.LogFieldNode, node.Name)
							if err = remediationMgr.RemoveOutOfServiceTaint(ctx, clusterClient, node); err != nil {
								return ctrl.Result{}, fmt.Errorf("error removing out-of-service taint from node %s: %w", node.Name, err)
							}
							log.V(baremetal.VerbosityLevelDebug).Info("Out-of-service taint removed")
						}
					} else {
						// Node was recreated, restore annotations and labels
						log.Info("Restoring node annotations and labels", baremetal.LogFieldNode, node.Name)
						if err = r.restoreNode(ctx, remediationMgr, clusterClient, node, log); err != nil {
							return ctrl.Result{}, err
						}
						log.V(baremetal.VerbosityLevelDebug).Info("Node restored successfully")
					}

					// clean up

					log.Info("Remediation done, cleaning up remediation CR")
					if !r.IsOutOfServiceTaintEnabled {
						log.V(baremetal.VerbosityLevelDebug).Info("Removing node backup annotations")
						remediationMgr.RemoveNodeBackupAnnotations()
					}
					log.V(baremetal.VerbosityLevelDebug).Info("Removing finalizer")
					remediationMgr.UnsetFinalizer()
					log.V(baremetal.VerbosityLevelTrace).Info("reconcileNormal: completed successfully (node restored)")
					return ctrl.Result{RequeueAfter: defaultTimeout}, nil
				} else if isNodeForbidden {
					// we don't have a node, just remove finalizer
					log.V(baremetal.VerbosityLevelDebug).Info("Node access forbidden, removing finalizer without restore")
					remediationMgr.UnsetFinalizer()

					log.Info("Skipping node restore, remediation done, CR should be deleted soon")
					return ctrl.Result{RequeueAfter: defaultTimeout}, nil
				}
			}

			// Check timeout, either node wasn't recreated yet, or CR is not deleted because of still unhealthy node
			log.V(baremetal.VerbosityLevelTrace).Info("Checking remediation timeout")
			timeout := remediationMgr.GetTimeout().Duration
			timedOut, _ := remediationMgr.TimeToRemediate(timeout)
			log.V(baremetal.VerbosityLevelDebug).Info("Timeout check result",
				"timedOut", timedOut,
				baremetal.LogFieldTimeout, timeout.String())

			if !timedOut {
				// Not yet time to retry or stop remediation, requeue
				log.Info("Waiting for node to get healthy and CR being deleted")
				return ctrl.Result{RequeueAfter: defaultTimeout}, nil
			}

			// Try again if limit not reached
			log.V(baremetal.VerbosityLevelTrace).Info("Checking retry limit")
			retryLimitSet := remediationMgr.RetryLimitIsSet()
			hasReachedLimit := remediationMgr.HasReachRetryLimit()
			log.V(baremetal.VerbosityLevelDebug).Info("Retry limit status",
				"retryLimitSet", retryLimitSet,
				"hasReachedLimit", hasReachedLimit)

			if retryLimitSet && !hasReachedLimit {
				log.Info("Remediation timed out, will retry")
				remediationMgr.SetRemediationPhase(infrav1.PhaseRunning)
				now := metav1.Now()
				remediationMgr.SetLastRemediationTime(&now)
				remediationMgr.IncreaseRetryCount()
				log.V(baremetal.VerbosityLevelDebug).Info("Retrying remediation")
				return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
			}

			log.Info("Remediation timed out and retry limit reached")
			log.V(baremetal.VerbosityLevelTrace).Info("Setting owner remediated condition")

			// When machine is still unhealthy after remediation, setting of OwnerRemediatedCondition
			// moves control to CAPI machine controller. The owning controller will do
			// preflight checks and handles the Machine deletion
			err = remediationMgr.SetOwnerRemediatedConditionNew(ctx)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("error setting cluster api conditions: %w", err)
			}
			log.V(baremetal.VerbosityLevelDebug).Info("Owner remediated condition set")

			// Remediation failed, so set unhealthy annotation on BMH
			// This prevents BMH to be selected as a host.
			log.V(baremetal.VerbosityLevelTrace).Info("Setting unhealthy annotation on BMH")
			err = remediationMgr.SetUnhealthyAnnotation(ctx)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("error setting unhealthy annotation: %w", err)
			}
			log.V(baremetal.VerbosityLevelDebug).Info("Unhealthy annotation set")

			log.V(baremetal.VerbosityLevelTrace).Info("Setting phase to Deleting")
			remediationMgr.SetRemediationPhase(infrav1.PhaseDeleting)
			// no requeue, we are done
			log.V(baremetal.VerbosityLevelTrace).Info("reconcileNormal: completed (remediation failed)")
			return ctrl.Result{}, nil

		case infrav1.PhaseDeleting:
			log.V(baremetal.VerbosityLevelDebug).Info("Phase: Deleting - nothing to do")
			// nothing to do anymore

		case infrav1.PhaseFailed:
			log.V(baremetal.VerbosityLevelDebug).Info("Phase: Failed - nothing to do")
			// nothing to do anymore

		default:
			log.Error(nil, "unknown phase!", "phase", remediationMgr.GetRemediationPhase())
		}
	}
	log.V(baremetal.VerbosityLevelTrace).Info("reconcileNormal: completed")
	return ctrl.Result{}, nil
}

// remediateRebootStrategy executes the remediation using the reboot strategy.
// Returns nil, nil when reconcile can continue.
// Return a Result and optionally an error when reconcile should return.
func (r *Metal3RemediationReconciler) remediateRebootStrategy(ctx context.Context,
	remediationMgr baremetal.RemediationManagerInterface, clusterClient v1.CoreV1Interface,
	node *corev1.Node, log logr.Logger) (ctrl.Result, error) {
	log.V(baremetal.VerbosityLevelTrace).Info("remediateRebootStrategy: starting")

	// add finalizer
	log.V(baremetal.VerbosityLevelTrace).Info("Checking finalizer")
	if !remediationMgr.HasFinalizer() {
		log.V(baremetal.VerbosityLevelDebug).Info("Setting finalizer on remediation")
		remediationMgr.SetFinalizer()
		return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
	}
	log.V(baremetal.VerbosityLevelDebug).Info("Finalizer already set")

	// power off if needed
	log.V(baremetal.VerbosityLevelTrace).Info("Checking power off status")
	if ok, err := remediationMgr.IsPowerOffRequested(ctx); err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting poweroff annotation status: %w", err)
	} else if !ok {
		log.Info("Powering off the host")
		err = remediationMgr.SetPowerOffAnnotation(ctx)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error setting poweroff annotation: %w", err)
		}
		log.V(baremetal.VerbosityLevelDebug).Info("Power off annotation set")

		// done for now, wait a bit before checking if we are powered off already
		return ctrl.Result{RequeueAfter: defaultTimeout}, nil
	}
	log.V(baremetal.VerbosityLevelDebug).Info("Power off already requested")

	// wait until powered off
	log.V(baremetal.VerbosityLevelTrace).Info("Checking if host is powered off")
	if on, err := remediationMgr.IsPoweredOn(ctx); err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting power status: %w", err)
	} else if on {
		log.V(baremetal.VerbosityLevelDebug).Info("Host still powered on, waiting")
		// wait a bit before checking again if we are powered off already
		return ctrl.Result{RequeueAfter: defaultTimeout}, nil
	}
	log.V(baremetal.VerbosityLevelDebug).Info("Host is powered off")

	log.V(baremetal.VerbosityLevelTrace).Info("Processing node for remediation",
		"hasNode", node != nil,
		"outOfServiceTaintEnabled", r.IsOutOfServiceTaintEnabled)

	if node != nil {
		if r.IsOutOfServiceTaintEnabled {
			log.V(baremetal.VerbosityLevelTrace).Info("Out-of-service taint mode enabled")
			if !remediationMgr.HasOutOfServiceTaint(node) {
				log.V(baremetal.VerbosityLevelDebug).Info("Adding out-of-service taint to node",
					baremetal.LogFieldNode, node.Name)
				if err := remediationMgr.AddOutOfServiceTaint(ctx, clusterClient, node); err != nil {
					return ctrl.Result{}, err
				}
				log.V(baremetal.VerbosityLevelDebug).Info("Out-of-service taint added")
				// If we immediately check if the node is drained, we might find no pods with
				// Deletion timestamp set yet and assume the node is drained.
				return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
			}

			log.V(baremetal.VerbosityLevelTrace).Info("Checking if node is drained")
			if !remediationMgr.IsNodeDrained(ctx, clusterClient, node) {
				log.V(baremetal.VerbosityLevelDebug).Info("Node not yet drained, waiting")
				return ctrl.Result{RequeueAfter: defaultTimeout}, nil
			}
			log.V(baremetal.VerbosityLevelDebug).Info("Node is drained")
		} else {
			log.V(baremetal.VerbosityLevelTrace).Info("Using node deletion mode")
			/*
				Delete the node only after the host is powered off. Otherwise, if we would delete the node
				when the host is powered on, the scheduler would assign the workload to other nodes, with the
				possibility that two instances of the same application are running in parallel. This might result
				in corruption or other issues for applications with singleton requirement. After the host is powered
				off we know for sure that it is safe to re-assign that workload to other nodes.
			*/
			modified := r.backupNode(remediationMgr, node, log)
			if modified {
				log.Info("Backing up node before deletion", baremetal.LogFieldNode, node.Name)
				// save annotations before deleting node
				return ctrl.Result{RequeueAfter: 1 * time.Second}, nil
			}
			log.Info("Deleting node", baremetal.LogFieldNode, node.Name)
			err := remediationMgr.DeleteNode(ctx, clusterClient, node)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("error deleting node: %w", err)
			}
			log.V(baremetal.VerbosityLevelDebug).Info("Node deletion initiated")
			// wait until node is gone
			return ctrl.Result{RequeueAfter: defaultTimeout}, nil
		}
	}

	// we are done for this phase, switch to waiting for power on and the node restore
	log.V(baremetal.VerbosityLevelTrace).Info("Switching to waiting phase")
	remediationMgr.SetRemediationPhase(infrav1.PhaseWaiting)
	log.Info("Switch to waiting phase for power on and node restore")
	log.V(baremetal.VerbosityLevelTrace).Info("remediateRebootStrategy: completed")
	return ctrl.Result{RequeueAfter: defaultTimeout}, nil
}

// Returns whether annotations or labels were set / updated.
func (r *Metal3RemediationReconciler) backupNode(remediationMgr baremetal.RemediationManagerInterface,
	node *corev1.Node, log logr.Logger) bool {
	log.V(baremetal.VerbosityLevelTrace).Info("backupNode: backing up node annotations and labels",
		baremetal.LogFieldNode, node.Name)

	marshaledAnnotations, err := marshal(node.Annotations)
	if err != nil {
		log.Error(err, "failed to marshal node annotations", "node", node.Name)
		// if marshal fails we want to continue without blocking on this, as this error
		// not likely to be resolved in the next run
	}

	marshaledLabels, err := marshal(node.Labels)
	if err != nil {
		log.Error(err, "failed to marshal node labels", "node", node.Name)
	}

	modified := remediationMgr.SetNodeBackupAnnotations(marshaledAnnotations, marshaledLabels)
	log.V(baremetal.VerbosityLevelDebug).Info("Node backup result",
		"modified", modified)
	return modified
}

func (r *Metal3RemediationReconciler) restoreNode(ctx context.Context, remediationMgr baremetal.RemediationManagerInterface,
	clusterClient v1.CoreV1Interface, node *corev1.Node, log logr.Logger) error { //nolint:unparam
	log.V(baremetal.VerbosityLevelTrace).Info("restoreNode: restoring node annotations and labels",
		baremetal.LogFieldNode, node.Name)

	annotations, labels := remediationMgr.GetNodeBackupAnnotations()
	log.V(baremetal.VerbosityLevelDebug).Info("Retrieved backup annotations",
		"hasAnnotations", annotations != "",
		"hasLabels", labels != "")

	if annotations == "" && labels == "" {
		log.V(baremetal.VerbosityLevelDebug).Info("No backup data to restore")
		return nil
	}

	// set annotations
	if annotations != "" {
		log.V(baremetal.VerbosityLevelTrace).Info("Restoring node annotations")
		nodeAnnotations, err := unmarshal(annotations)
		if err != nil {
			log.Error(err, "failed to unmarshal node annotations", "node", node.Name, "annotations", annotations)
			// if unmarshal fails we want to continue without blocking on this, as this error
			// is not likely to be resolved in the next run
		}
		if len(nodeAnnotations) > 0 {
			node.Annotations = mergeMaps(node.Annotations, nodeAnnotations)
			log.V(baremetal.VerbosityLevelDebug).Info("Node annotations restored",
				baremetal.LogFieldCount, len(nodeAnnotations))
		}
	}

	// set labels
	if labels != "" {
		log.V(baremetal.VerbosityLevelTrace).Info("Restoring node labels")
		nodeLabels, err := unmarshal(labels)
		if err != nil {
			log.Error(err, "failed to unmarshal node labels", "node", node.Name, "labels", labels)
			// if unmarshal fails we want to continue without blocking on this, as this error
			// is not likely to be resolved in the next run
		}
		if len(nodeLabels) > 0 {
			node.Labels = mergeMaps(node.Labels, nodeLabels)
			log.V(baremetal.VerbosityLevelDebug).Info("Node labels restored",
				baremetal.LogFieldCount, len(nodeLabels))
		}
	}

	log.V(baremetal.VerbosityLevelTrace).Info("Updating node with restored data")
	if err := remediationMgr.UpdateNode(ctx, clusterClient, node); err != nil {
		log.Error(err, "failed to update node", "node", node.Name)
	}
	log.V(baremetal.VerbosityLevelDebug).Info("Node updated successfully")

	return nil
}

// marshal is a wrapper for json.marshal() and converts its output to string.
// if m is nil - an empty string will be returned.
func marshal(m map[string]string) (string, error) {
	if m == nil {
		return "", nil
	}

	marshaled, err := json.Marshal(m)
	if err != nil {
		return "", err
	}

	return string(marshaled), nil
}

// unmarshal is a wrapper for json.Unmarshal() for marshaled strings that represent map[string]string.
func unmarshal(marshaled string) (map[string]string, error) {
	if marshaled == "" {
		return make(map[string]string), nil
	}

	decodedValue := make(map[string]string)

	if err := json.Unmarshal([]byte(marshaled), &decodedValue); err != nil {
		return nil, err
	}

	return decodedValue, nil
}

// mergeMaps takes entries from mapToMerge and adds them to prioritizedMap, if entry key
// does not already exist in prioritizedMap. It returns the merged map.
func mergeMaps(prioritizedMap map[string]string, mapToMerge map[string]string) map[string]string {
	if prioritizedMap == nil {
		prioritizedMap = make(map[string]string)
	}

	for key, value := range mapToMerge {
		if _, exists := prioritizedMap[key]; !exists {
			prioritizedMap[key] = value
		}
	}

	return prioritizedMap
}

// SetupWithManager will add watches for Metal3Remediation controller.
func (r *Metal3RemediationReconciler) SetupWithManager(_ context.Context, mgr ctrl.Manager, options controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.Metal3Remediation{}).
		WithOptions(options).
		Complete(r)
}

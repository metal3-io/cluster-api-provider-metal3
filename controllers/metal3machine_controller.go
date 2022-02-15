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

package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	bmh "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	"github.com/metal3-io/cluster-api-provider-metal3/baremetal"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	machineControllerName = "Metal3Machine-controller"
)

var hasRequeueAfterError baremetal.HasRequeueAfterError

// Metal3MachineReconciler reconciles a Metal3Machine object.
type Metal3MachineReconciler struct {
	Client           client.Client
	ManagerFactory   baremetal.ManagerFactoryInterface
	Log              logr.Logger
	CapiClientGetter baremetal.ClientGetter
	WatchFilterValue string
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metal3machines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metal3machines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metal3dataclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metal3dataclaims/status,verbs=get
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metal3datas,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metal3datas/status,verbs=get
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metal3machinetemplates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machinedeployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machinesets,verbs=get;list;watch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=kubeadmcontrolplanes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Add RBAC rules to access cluster-api resources
// +kubebuilder:rbac:groups=metal3.io,resources=baremetalhosts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metal3.io,resources=baremetalhosts/status,verbs=get;update;patch

// Reconcile handles Metal3Machine events.
func (r *Metal3MachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, rerr error) {
	machineLog := r.Log.WithName(machineControllerName).WithValues("metal3-machine", req.NamespacedName)

	// Fetch the Metal3Machine instance.
	capm3Machine := &capm3.Metal3Machine{}

	if err := r.Client.Get(ctx, req.NamespacedName, capm3Machine); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	// Always patch capm3Machine exiting this function so we can persist any Metal3Machine changes.
	patchHelper, err := patch.NewHelper(capm3Machine, r.Client)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to init patch helper")
	}
	defer func() {
		if err := patchMetal3Machine(ctx, patchHelper, capm3Machine); err != nil {
			machineLog.Error(err, "failed to Patch metal3Machine")
		}
	}()
	// clear an error if one was previously set
	clearErrorM3Machine(capm3Machine)

	// Fetch the Machine.
	capiMachine, err := util.GetOwnerMachine(ctx, r.Client, capm3Machine.ObjectMeta)

	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "Metal3Machine's owner Machine could not be retrieved")
	}
	if capiMachine == nil {
		machineLog.Info("Waiting for Machine Controller to set OwnerRef on Metal3Machine")
		return ctrl.Result{}, nil
	}

	machineLog = machineLog.WithValues("machine", capiMachine.Name)

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, capiMachine.ObjectMeta)
	if err != nil {
		setErrorM3Machine(capm3Machine, "", "")
		machineLog.Info("Machine is missing cluster label or cluster does not exist")
		return ctrl.Result{}, nil
	}

	machineLog = machineLog.WithValues("cluster", cluster.Name)

	// Make sure infrastructure is ready
	if !cluster.Status.InfrastructureReady {
		machineLog.Info("Waiting for Metal3Cluster Controller to create cluster infrastructure")
		conditions.MarkFalse(capm3Machine, capm3.AssociateBMHCondition, capm3.WaitingForClusterInfrastructureReason, clusterv1.ConditionSeverityInfo, "")
		return ctrl.Result{}, nil
	}

	// Fetch the Metal3 cluster.
	metal3Cluster := &capm3.Metal3Cluster{}
	metal3ClusterName := types.NamespacedName{
		Namespace: capm3Machine.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if err := r.Client.Get(ctx, metal3ClusterName, metal3Cluster); err != nil {
		machineLog.Info("Waiting for Metal3Cluster Controller to create the Metal3Cluster")
		return ctrl.Result{}, nil
	}

	machineLog = machineLog.WithValues("metal3-cluster", metal3Cluster.Name)

	// Create a helper for managing the baremetal container hosting the machine.
	machineMgr, err := r.ManagerFactory.NewMachineManager(cluster, metal3Cluster, capiMachine, capm3Machine, machineLog)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to create helper for managing the machineMgr")
	}

	// Check pause annotation on associated bmh (if any)
	if !cluster.Spec.Paused {
		err := machineMgr.RemovePauseAnnotation(ctx)
		if err != nil {
			machineLog.Info("failed to check pause annotation on associated bmh")
			return ctrl.Result{}, nil
		}
	} else {
		// set pause annotation on associated bmh (if any)
		err := machineMgr.SetPauseAnnotation(ctx)
		if err != nil {
			machineLog.Info("failed to set pause annotation on associated bmh")
			return ctrl.Result{}, nil
		}
	}

	// Return early if the M3Machine or Cluster is paused.
	if annotations.IsPaused(cluster, capm3Machine) {
		machineLog.Info("reconciliation is paused for this object")
		return ctrl.Result{Requeue: true, RequeueAfter: requeueAfter}, nil
	}

	// Handle deleted machines
	if !capm3Machine.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, machineMgr)
	}

	// Handle non-deleted machines
	return r.reconcileNormal(ctx, machineMgr)
}

func patchMetal3Machine(ctx context.Context, patchHelper *patch.Helper, metal3Machine *capm3.Metal3Machine, options ...patch.Option) error {
	// Always update the readyCondition by summarizing the state of other conditions.
	conditions.SetSummary(metal3Machine,
		conditions.WithConditions(
			capm3.AssociateBMHCondition,
			capm3.KubernetesNodeReadyCondition,
		),
	)

	// Patch the object, ignoring conflicts on the conditions owned by this controller.
	options = append(options,
		patch.WithOwnedConditions{Conditions: []clusterv1.ConditionType{
			clusterv1.ReadyCondition,
			capm3.AssociateBMHCondition,
			capm3.KubernetesNodeReadyCondition,
		}},
		patch.WithStatusObservedGeneration{},
	)
	return patchHelper.Patch(ctx, metal3Machine, options...)
}

func (r *Metal3MachineReconciler) reconcileNormal(ctx context.Context,
	machineMgr baremetal.MachineManagerInterface,
) (ctrl.Result, error) {
	// If the Metal3Machine doesn't have finalizer, add it.
	machineMgr.SetFinalizer()

	// if the machine is already provisioned, update and return
	if machineMgr.IsProvisioned() {
		errType := capierrors.UpdateMachineError
		return checkMachineError(machineMgr, machineMgr.Update(ctx),
			"Failed to update the Metal3Machine", errType,
		)
	}

	// Make sure bootstrap data is available and populated. If not, return, we
	// will get an event from the machine update when the flag is set to true.
	if !machineMgr.IsBootstrapReady() {
		machineMgr.SetConditionMetal3MachineToFalse(capm3.AssociateBMHCondition, capm3.WaitingForBootstrapReadyReason, clusterv1.ConditionSeverityInfo, "")
		return ctrl.Result{}, nil
	}

	errType := capierrors.CreateMachineError

	// Check if the metal3machine was associated with a baremetalhost
	if !machineMgr.HasAnnotation() {
		// Associate the baremetalhost hosting the machine
		err := machineMgr.Associate(ctx)
		if err != nil {
			machineMgr.SetConditionMetal3MachineToFalse(capm3.AssociateBMHCondition, capm3.AssociateBMHFailedReason, clusterv1.ConditionSeverityError, err.Error())
			return checkMachineError(machineMgr, err,
				"failed to associate the Metal3Machine to a BaremetalHost", errType,
			)
		}
	}
	// Update Condition to reflect that we have an associated BMH
	machineMgr.SetConditionMetal3MachineToTrue(capm3.AssociateBMHCondition)

	// Make sure that the metadata is ready if any
	err := machineMgr.AssociateM3Metadata(ctx)
	if err != nil {
		return checkMachineError(machineMgr, err,
			"Failed to get the Metal3Metadata", errType,
		)
	}

	err = machineMgr.Update(ctx)
	if err != nil {
		return checkMachineError(machineMgr, err,
			"failed to update BaremetalHost", errType,
		)
	}

	providerID, bmhID := machineMgr.GetProviderIDAndBMHID()
	if bmhID == nil {
		bmhID, err = machineMgr.GetBaremetalHostID(ctx)
		if err != nil {
			machineMgr.SetConditionMetal3MachineToFalse(capm3.KubernetesNodeReadyCondition, capm3.MissingBMHReason, clusterv1.ConditionSeverityError, err.Error())
			return checkMachineError(machineMgr, err,
				"failed to get the providerID for the metal3machine", errType,
			)
		}
		if bmhID != nil {
			providerID = fmt.Sprintf("%s%s", baremetal.ProviderIDPrefix, *bmhID)
		}
	}
	if bmhID != nil {
		// Set the providerID on the node if no Cloud provider
		err = machineMgr.SetNodeProviderID(ctx, *bmhID, providerID, r.CapiClientGetter)
		if err != nil {
			machineMgr.SetConditionMetal3MachineToFalse(capm3.KubernetesNodeReadyCondition, capm3.SettingProviderIDOnNodeFailedReason, clusterv1.ConditionSeverityError, err.Error())
			return checkMachineError(machineMgr, err,
				"failed to set the target node providerID", errType,
			)
		}
		// Make sure Spec.ProviderID is set and mark the capm3Machine ready
		machineMgr.SetProviderID(providerID)
	}

	return ctrl.Result{}, err
}

func (r *Metal3MachineReconciler) reconcileDelete(ctx context.Context,
	machineMgr baremetal.MachineManagerInterface,
) (ctrl.Result, error) {
	errType := capierrors.DeleteMachineError

	// delete the machine
	if err := machineMgr.Delete(ctx); err != nil {
		return checkMachineError(machineMgr, err,
			"failed to delete Metal3Machine", errType,
		)
	}

	if err := machineMgr.DissociateM3Metadata(ctx); err != nil {
		return checkMachineError(machineMgr, err,
			"failed to dissociate Metadata", errType,
		)
	}

	// metal3machine is marked for deletion and ready to be deleted,
	// so remove the finalizer.
	machineMgr.UnsetFinalizer()

	return ctrl.Result{}, nil
}

// SetupWithManager will add watches for this controller.
func (r *Metal3MachineReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&capm3.Metal3Machine{}).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(ctrl.LoggerFrom(ctx), r.WatchFilterValue)).
		Watches(
			&source.Kind{Type: &clusterv1.Machine{}},
			handler.EnqueueRequestsFromMapFunc(util.MachineToInfrastructureMapFunc(capm3.GroupVersion.WithKind("Metal3Machine"))),
		).
		Watches(
			&source.Kind{Type: &clusterv1.Cluster{}},
			handler.EnqueueRequestsFromMapFunc(r.ClusterToMetal3Machines),
		).
		Watches(
			&source.Kind{Type: &capm3.Metal3Cluster{}},
			handler.EnqueueRequestsFromMapFunc(r.Metal3ClusterToMetal3Machines),
		).
		Watches(
			&source.Kind{Type: &capm3.Metal3DataClaim{}},
			handler.EnqueueRequestsFromMapFunc(r.Metal3DataClaimToMetal3Machines),
		).
		Watches(
			&source.Kind{Type: &capm3.Metal3Data{}},
			handler.EnqueueRequestsFromMapFunc(r.Metal3DataToMetal3Machines),
		).
		Watches(
			&source.Kind{Type: &bmh.BareMetalHost{}},
			handler.EnqueueRequestsFromMapFunc(r.BareMetalHostToMetal3Machines),
		).
		Complete(r)
}

// ClusterToMetal3Machines is a handler.ToRequestsFunc to be used to enqeue
// requests for reconciliation of Metal3Machines.
func (r *Metal3MachineReconciler) ClusterToMetal3Machines(o client.Object) []ctrl.Request {
	result := []ctrl.Request{}
	c, ok := o.(*clusterv1.Cluster)

	if !ok {
		r.Log.Error(errors.Errorf("expected a Cluster but got a %T", o),
			"failed to get Metal3Machine for Cluster",
		)
		return nil
	}

	labels := map[string]string{clusterv1.ClusterLabelName: c.Name}
	capiMachineList := &clusterv1.MachineList{}
	if err := r.Client.List(context.TODO(), capiMachineList, client.InNamespace(c.Namespace),
		client.MatchingLabels(labels),
	); err != nil {
		r.Log.Error(err, "failed to list Metal3Machines")
		return nil
	}
	for _, m := range capiMachineList.Items {
		if m.Spec.InfrastructureRef.Name == "" {
			continue
		}
		name := client.ObjectKey{Namespace: m.Namespace, Name: m.Spec.InfrastructureRef.Name}
		if m.Spec.InfrastructureRef.Namespace != "" {
			name = client.ObjectKey{Namespace: m.Spec.InfrastructureRef.Namespace, Name: m.Spec.InfrastructureRef.Name}
		}
		result = append(result, ctrl.Request{NamespacedName: name})
	}

	return result
}

// Metal3ClusterToMetal3Machines is a handler.ToRequestsFunc to be used to enqeue
// requests for reconciliation of Metal3Machines.
func (r *Metal3MachineReconciler) Metal3ClusterToMetal3Machines(o client.Object) []ctrl.Request {
	result := []ctrl.Request{}
	c, ok := o.(*capm3.Metal3Cluster)
	if !ok {
		r.Log.Error(errors.Errorf("expected a Metal3Cluster but got a %T", o),
			"failed to get Metal3Machine for Metal3Cluster",
		)
		return nil
	}
	log := r.Log.WithValues("Metal3Cluster", c.Name, "Namespace", c.Namespace)

	cluster, err := util.GetOwnerCluster(context.TODO(), r.Client, c.ObjectMeta)
	switch {
	case apierrors.IsNotFound(err) || cluster == nil:
		return result
	case err != nil:
		log.Error(err, "failed to get owning cluster")
		return result
	}

	labels := map[string]string{clusterv1.ClusterLabelName: cluster.Name}
	capiMachineList := &clusterv1.MachineList{}
	if err := r.Client.List(context.TODO(), capiMachineList, client.InNamespace(c.Namespace), client.MatchingLabels(labels)); err != nil {
		log.Error(err, "failed to list Metal3Machines")
		return nil
	}
	for _, m := range capiMachineList.Items {
		if m.Spec.InfrastructureRef.Name == "" {
			continue
		}
		name := client.ObjectKey{Namespace: m.Namespace, Name: m.Spec.InfrastructureRef.Name}
		if m.Spec.InfrastructureRef.Namespace != "" {
			name = client.ObjectKey{Namespace: m.Spec.InfrastructureRef.Namespace, Name: m.Spec.InfrastructureRef.Name}
		}
		result = append(result, ctrl.Request{NamespacedName: name})
	}
	return result
}

// BareMetalHostToMetal3Machines will return a reconcile request for a Metal3Machine if the event is for a
// BareMetalHost and that BareMetalHost references a Metal3Machine.
func (r *Metal3MachineReconciler) BareMetalHostToMetal3Machines(obj client.Object) []ctrl.Request {
	if host, ok := obj.(*bmh.BareMetalHost); ok {
		if host.Spec.ConsumerRef != nil &&
			host.Spec.ConsumerRef.Kind == Metal3Machine &&
			host.Spec.ConsumerRef.GroupVersionKind().Group == capm3.GroupVersion.Group {
			return []ctrl.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      host.Spec.ConsumerRef.Name,
						Namespace: host.Spec.ConsumerRef.Namespace,
					},
				},
			}
		}
	} else {
		r.Log.Error(errors.Errorf("expected a BareMetalHost but got a %T", obj),
			"failed to get Metal3Machine for BareMetalHost",
		)
	}
	return []ctrl.Request{}
}

// Metal3DataClaimToMetal3Machines will return a reconcile request for a Metal3Machine if the event is for a
// Metal3Data and that Metal3Data references a Metal3Machine.
func (r *Metal3MachineReconciler) Metal3DataClaimToMetal3Machines(obj client.Object) []ctrl.Request {
	requests := []ctrl.Request{}
	if m3dc, ok := obj.(*capm3.Metal3DataClaim); ok {
		for _, ownerRef := range m3dc.OwnerReferences {
			oGV, err := schema.ParseGroupVersion(ownerRef.APIVersion)
			if err != nil {
				r.Log.Error(errors.Errorf("Failed to parse the group and version %v", ownerRef.APIVersion),
					"failed to get Metal3Machine for BareMetalHost",
				)
				continue
			}
			// not matching on UID since when pivoting it might change
			// Not matching on API version as this might change
			if ownerRef.Kind == "Metal3Machine" &&
				oGV.Group == capm3.GroupVersion.Group {
				requests = append(requests, ctrl.Request{
					NamespacedName: types.NamespacedName{
						Name:      ownerRef.Name,
						Namespace: m3dc.Namespace,
					},
				})
			}
		}
	} else {
		r.Log.Error(errors.Errorf("expected a Metal3DataClaim but got a %T", obj),
			"failed to get Metal3Machine for Metal3DataClaim",
		)
	}
	return requests
}

// Metal3DataToMetal3Machines will return a reconcile request for a Metal3Machine if the event is for a
// Metal3Data and that Metal3Data references a Metal3Machine.
func (r *Metal3MachineReconciler) Metal3DataToMetal3Machines(obj client.Object) []ctrl.Request {
	requests := []ctrl.Request{}
	if m3d, ok := obj.(*capm3.Metal3Data); ok {
		for _, ownerRef := range m3d.OwnerReferences {
			if ownerRef.Kind != "Metal3Machine" {
				continue
			}
			aGV, err := schema.ParseGroupVersion(ownerRef.APIVersion)
			if err != nil {
				r.Log.Error(errors.Errorf("Failed to parse the group and version %v", ownerRef.APIVersion),
					"failed to get Metal3Machine for BareMetalHost",
				)
				continue
			}
			if aGV.Group != capm3.GroupVersion.Group {
				continue
			}
			requests = append(requests, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      ownerRef.Name,
					Namespace: m3d.Namespace,
				},
			})
		}
	} else {
		r.Log.Error(errors.Errorf("expected a Metal3Data but got a %T", obj),
			"failed to get Metal3Machine for Metal3Data",
		)
	}
	return requests
}

// setErrorM3Machine sets the ErrorMessage and ErrorReason fields on the metal3machine.
func setErrorM3Machine(m3m *capm3.Metal3Machine, message string, reason capierrors.MachineStatusError) {
	m3m.Status.FailureMessage = pointer.StringPtr(message)
	m3m.Status.FailureReason = &reason
}

// clearError removes the ErrorMessage from the metal3machine's Status if set.
func clearErrorM3Machine(m3m *capm3.Metal3Machine) {
	if m3m.Status.FailureMessage != nil || m3m.Status.FailureReason != nil {
		m3m.Status.FailureMessage = nil
		m3m.Status.FailureReason = nil
	}
}

func checkMachineError(machineMgr baremetal.MachineManagerInterface, err error,
	errMessage string, errType capierrors.MachineStatusError,
) (ctrl.Result, error) {
	if err == nil {
		return ctrl.Result{}, nil
	}
	if ok := errors.As(err, &hasRequeueAfterError); ok {
		return ctrl.Result{Requeue: true, RequeueAfter: hasRequeueAfterError.GetRequeueAfter()}, nil
	}
	machineMgr.SetError(errMessage, errType)
	return ctrl.Result{}, errors.Wrap(err, errMessage)
}

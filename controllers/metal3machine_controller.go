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

	"github.com/go-logr/logr"
	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	"github.com/metal3-io/cluster-api-provider-metal3/baremetal"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/controllers/clustercache"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	deprecatedv1beta1conditions "sigs.k8s.io/cluster-api/util/conditions/deprecated/v1beta1"
	v1beta1conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"
	v1beta2conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions/v1beta2"
	v1beta1patch "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	machineControllerName = "Metal3Machine-controller"
)

// Metal3MachineReconciler reconciles a Metal3Machine object.
type Metal3MachineReconciler struct {
	Client           client.Client
	ManagerFactory   baremetal.ManagerFactoryInterface
	ClusterCache     clustercache.ClusterCache
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
	capm3Machine := &infrav1.Metal3Machine{}

	if err := r.Client.Get(ctx, req.NamespacedName, capm3Machine); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	// Always patch capm3Machine exiting this function so we can persist any Metal3Machine changes.
	patchHelper, err := v1beta1patch.NewHelper(capm3Machine, r.Client)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to init patch helper")
	}
	defer func() {
		if err = patchMetal3Machine(ctx, patchHelper, capm3Machine); err != nil {
			machineLog.Error(err, "failed to Patch metal3Machine")
			rerr = err
		}
	}()

	// Fetch the Machine.
	capiMachine, err := util.GetOwnerMachine(ctx, r.Client, capm3Machine.ObjectMeta)

	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "Metal3Machine's owner Machine could not be retrieved")
	}
	if capiMachine == nil {
		machineLog.Info("Waiting for Machine Controller to set OwnerRef on Metal3Machine")
		v1beta1conditions.MarkFalse(capm3Machine, infrav1.AssociateBMHCondition, infrav1.WaitingForMetal3MachineOwnerRefReason, clusterv1beta1.ConditionSeverityInfo, "")
		v1beta2conditions.Set(capm3Machine, metav1.Condition{
			Type:    infrav1.AssociateBareMetalHostV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.WaitingForMetal3MachineOwnerRefV1Beta2Reason,
			Message: "Waiting for Machine Controller to set OwnerRef on Metal3Machine",
		})
		return ctrl.Result{}, nil
	}

	machineLog = machineLog.WithValues("machine", capiMachine.Name)

	// Set Failuredomain from machine to metal3Machine
	if capm3Machine.Spec.FailureDomain != capiMachine.Spec.FailureDomain {
		capm3Machine.Spec.FailureDomain = capiMachine.Spec.FailureDomain
	}

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, capiMachine.ObjectMeta)
	if err != nil {
		setErrorM3Machine(capm3Machine, "", "")
		machineLog.Info("Machine is missing cluster label or cluster does not exist")
		return ctrl.Result{}, nil
	}

	machineLog = machineLog.WithValues("cluster", cluster.Name)

	// Make sure infrastructure is ready
	infrastructureReadyCondition := deprecatedv1beta1conditions.Get(cluster, clusterv1.InfrastructureReadyV1Beta1Condition)
	if infrastructureReadyCondition.Status != corev1.ConditionTrue {
		machineLog.Info("Waiting for Metal3Cluster Controller to create cluster infrastructure")
		v1beta1conditions.MarkFalse(capm3Machine, infrav1.AssociateBMHCondition, infrav1.WaitingForClusterInfrastructureReason, clusterv1beta1.ConditionSeverityInfo, "")
		v1beta2conditions.Set(capm3Machine, metav1.Condition{
			Type:    infrav1.AssociateBareMetalHostV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.WaitingForClusterInfrastructureReadyV1Beta2Reason,
			Message: "Waiting for Metal3Cluster Controller to create cluster infrastructure",
		})
		return ctrl.Result{}, nil
	}

	// Fetch the Metal3 cluster.
	metal3Cluster := &infrav1.Metal3Cluster{}
	metal3ClusterName := types.NamespacedName{
		Namespace: capm3Machine.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if err = r.Client.Get(ctx, metal3ClusterName, metal3Cluster); err != nil {
		machineLog.Info("Waiting for Metal3Cluster Controller to create the Metal3Cluster")
		v1beta1conditions.MarkFalse(capm3Machine, infrav1.AssociateBMHCondition, infrav1.WaitingforMetal3ClusterReason, clusterv1beta1.ConditionSeverityInfo, "")
		v1beta2conditions.Set(capm3Machine, metav1.Condition{
			Type:    infrav1.AssociateBareMetalHostV1Beta2Condition,
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.WaitingforMetal3ClusterV1Beta2Reason,
			Message: "Waiting for Metal3Cluster Controller to create the Metal3Cluster",
		})
		return ctrl.Result{}, nil
	}

	machineLog = machineLog.WithValues("metal3-cluster", metal3Cluster.Name)

	// Create a helper for managing the baremetal container hosting the machine.
	machineMgr, err := r.ManagerFactory.NewMachineManager(cluster, metal3Cluster, capiMachine, capm3Machine, machineLog)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to create helper for managing the machineMgr")
	}

	// Check pause annotation on associated bmh (if any)
	if annotations.IsPaused(cluster, capm3Machine) {
		annotations.AddAnnotations(capm3Machine, map[string]string{"clusterctl.cluster.x-k8s.io/block-move": ""})
		err = machineMgr.SetPauseAnnotation(ctx)
		if err != nil {
			machineLog.Info("failed to set pause annotation on associated bmh")
			v1beta1conditions.MarkFalse(capm3Machine, infrav1.AssociateBMHCondition, infrav1.PauseAnnotationSetFailedReason, clusterv1beta1.ConditionSeverityInfo, "")
			message := "Failed to set pause annotation on associated BareMetalHost. Error: " + err.Error()
			v1beta2conditions.Set(capm3Machine, metav1.Condition{
				Type:    clusterv1beta1.PausedV1Beta2Condition,
				Status:  metav1.ConditionFalse,
				Reason:  infrav1.BareMetalHostPauseAnnotationSetFailedV1Beta2Reason,
				Message: message,
			})
			return ctrl.Result{}, nil
		}
		machineLog.Info("reconciliation is paused for this object")
		v1beta1conditions.MarkFalse(capm3Machine, infrav1.AssociateBMHCondition, infrav1.Metal3MachinePausedReason, clusterv1beta1.ConditionSeverityInfo, "")
		v1beta2conditions.Set(capm3Machine, metav1.Condition{
			Type:   clusterv1beta1.PausedV1Beta2Condition,
			Status: metav1.ConditionTrue,
			Reason: clusterv1beta1.PausedV1Beta2Reason,
		})
		return ctrl.Result{Requeue: true, RequeueAfter: requeueAfter}, nil
	}
	err = machineMgr.RemovePauseAnnotation(ctx)
	if err != nil {
		machineLog.Info("failed to remove pause annotation on associated bmh")
		v1beta1conditions.MarkFalse(capm3Machine, infrav1.AssociateBMHCondition, infrav1.PauseAnnotationRemoveFailedReason, clusterv1beta1.ConditionSeverityInfo, "")
		return ctrl.Result{}, nil
	}

	// Handle deleted machines
	if !capm3Machine.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, machineMgr)
	}

	// Handle non-deleted machines
	return r.reconcileNormal(ctx, machineMgr)
}

func patchMetal3Machine(ctx context.Context, patchHelper *v1beta1patch.Helper, metal3Machine *infrav1.Metal3Machine, options ...v1beta1patch.Option) error {
	// Always update the readyCondition by summarizing the state of other conditions.
	v1beta1conditions.SetSummary(metal3Machine,
		v1beta1conditions.WithConditions(
			infrav1.AssociateBMHCondition,
			infrav1.Metal3DataReadyCondition,
			infrav1.KubernetesNodeReadyCondition,
		),
	)

	if err := v1beta2conditions.SetSummaryCondition(metal3Machine, metal3Machine, infrav1.Metal3MachineReadyV1Beta2Condition,
		v1beta2conditions.ForConditionTypes{
			infrav1.AssociateBareMetalHostV1Beta2Condition,
			infrav1.AssociateMetal3MachineMetaDataV1Beta2Condition,
			infrav1.Metal3DataReadyV1Beta2Condition,
		},
		// Using a custom merge strategy to override reasons applied during merge.
		v1beta2conditions.CustomMergeStrategy{
			MergeStrategy: v1beta2conditions.DefaultMergeStrategy(
				// Use custom reasons.
				v1beta2conditions.ComputeReasonFunc(v1beta2conditions.GetDefaultComputeMergeReasonFunc(
					infrav1.Metal3MachineNotReadyV1Beta2Reason,
					infrav1.Metal3MachineReadyUnknownV1Beta2Reason,
					infrav1.Metal3MachineReadyV1Beta2Reason,
				)),
			),
		},
	); err != nil {
		return err
	}

	// Patch the object, ignoring conflicts on the conditions owned by this controller.
	options = append(options,
		v1beta1patch.WithOwnedConditions{Conditions: []clusterv1beta1.ConditionType{
			clusterv1beta1.ReadyCondition,
			infrav1.AssociateBMHCondition,
			infrav1.Metal3DataReadyCondition,
			infrav1.KubernetesNodeReadyCondition,
		}},
		v1beta1patch.WithOwnedV1Beta2Conditions{Conditions: []string{
			infrav1.Metal3MachineReadyV1Beta2Condition,
			infrav1.AssociateBareMetalHostV1Beta2Condition,
			infrav1.AssociateMetal3MachineMetaDataV1Beta2Condition,
			infrav1.Metal3DataReadyV1Beta2Condition,
			clusterv1beta1.PausedV1Beta2Condition,
		}},
		v1beta1patch.WithStatusObservedGeneration{},
	)
	return patchHelper.Patch(ctx, metal3Machine, options...)
}

func (r *Metal3MachineReconciler) reconcileNormal(ctx context.Context,
	machineMgr baremetal.MachineManagerInterface,
) (ctrl.Result, error) {
	// If the Metal3Machine doesn't have finalizer, add it.
	machineMgr.SetFinalizer()

	// if the machine is already provisioned, update and return
	if machineMgr.IsProvisioned() && machineMgr.MachineHasNodeRef() {
		errType := capierrors.UpdateMachineError
		err := machineMgr.Update(ctx)
		return checkMachineError(machineMgr, err,
			"Failed to update the Metal3Machine", errType)
	}

	// Make sure bootstrap data is available and populated. If not, return, we
	// will get an event from the machine update when the flag is set to true.
	if !machineMgr.IsBootstrapReady() {
		machineMgr.SetConditionMetal3MachineToFalse(infrav1.AssociateBMHCondition, infrav1.WaitingForBootstrapReadyReason, clusterv1beta1.ConditionSeverityInfo, "")
		machineMgr.SetV1beta2Condition(infrav1.AssociateBareMetalHostV1Beta2Condition, metav1.ConditionFalse, infrav1.WaitingForBootstrapDataV1Beta2Reason, "Waiting for bootstrap data to be ready before proceeding")
		return ctrl.Result{}, nil
	}

	errType := capierrors.CreateMachineError

	// Check if the metal3machine was associated with a baremetalhost
	if !machineMgr.HasAnnotation() {
		// Associate the baremetalhost hosting the machine
		err := machineMgr.Associate(ctx)
		if err != nil {
			machineMgr.SetConditionMetal3MachineToFalse(infrav1.AssociateBMHCondition, infrav1.AssociateBMHFailedReason, clusterv1beta1.ConditionSeverityError, err.Error())
			machineMgr.SetV1beta2Condition(infrav1.AssociateBareMetalHostV1Beta2Condition, metav1.ConditionFalse, infrav1.AssociateBareMetalHostFailedV1Beta2Reason, err.Error())
			return checkMachineError(machineMgr, err,
				"failed to associate the Metal3Machine to a BareMetalHost", errType)
		}

		return ctrl.Result{}, nil
	}
	// Update Condition to reflect that we have an associated BMH
	machineMgr.SetConditionMetal3MachineToTrue(infrav1.AssociateBMHCondition)
	machineMgr.SetV1beta2Condition(infrav1.AssociateBareMetalHostV1Beta2Condition, metav1.ConditionTrue, infrav1.AssociateBareMetalHostSuccessV1Beta2Reason, "")

	// Make sure that the metadata is ready if any
	err := machineMgr.AssociateM3Metadata(ctx)
	if err != nil {
		machineMgr.SetConditionMetal3MachineToFalse(infrav1.KubernetesNodeReadyCondition, infrav1.AssociateM3MetaDataFailedReason, clusterv1beta1.ConditionSeverityWarning, err.Error())
		machineMgr.SetV1beta2Condition(infrav1.AssociateMetal3MachineMetaDataV1Beta2Condition, metav1.ConditionFalse, infrav1.AssociateMetal3MachineMetaDataFailedV1Beta2Reason, err.Error())
		return checkMachineError(machineMgr, err,
			"Failed to get the Metal3Metadata", errType)
	}
	machineMgr.SetV1beta2Condition(infrav1.AssociateMetal3MachineMetaDataV1Beta2Condition, metav1.ConditionTrue, infrav1.AssociateMetal3MachineMetaDataSuccessV1Beta2Reason, "")

	err = machineMgr.Update(ctx)
	if err != nil {
		return checkMachineError(machineMgr, err,
			"failed to update BareMetalHost", errType)
	}

	if !machineMgr.IsBaremetalHostProvisioned(ctx) {
		return ctrl.Result{}, nil
	}

	if machineMgr.NodeWithMatchingProviderIDExists(ctx, r.CapiClientGetter) {
		// Nothing to be done but wait for Machine.Spec.NodeRef
		machineMgr.SetReadyTrue()
		return ctrl.Result{}, nil
	}

	if machineMgr.CloudProviderEnabled() {
		err = machineMgr.SetProviderIDFromCloudProviderNode(ctx, r.CapiClientGetter)
		if err != nil {
			return checkMachineError(machineMgr, err, "failed to set ProviderID on Metal3Machine based on Cloud Provider Node ProviderID", errType)
		}
		return ctrl.Result{}, nil
	}

	// If we have "moved", the Node label will not match the baremetalhost.  However,
	// The Node should have a matching ProviderID already set and we will have
	// caught it above.
	success, err := machineMgr.SetProviderIDFromNodeLabel(ctx, r.CapiClientGetter)
	if err != nil {
		return checkMachineError(machineMgr, err, "failed to set ProviderID on Metal3Machine based on Node label", errType)
	}

	if success {
		machineMgr.SetReadyTrue()
		return ctrl.Result{}, nil
	}

	if !machineMgr.Metal3MachineHasProviderID() {
		err = machineMgr.SetDefaultProviderID()
		if err != nil {
			return checkMachineError(machineMgr, err,
				"Failed to set default ProviderID the Metal3Machine", errType)
		}
		machineMgr.SetReadyTrue()

		errType = capierrors.UpdateMachineError
		err = machineMgr.Update(ctx)
		if err != nil {
			return checkMachineError(machineMgr, err,
				"Failed to update the Metal3Machine", errType)
		}
	}

	err = machineMgr.SetNodeProviderIDByHostname(ctx, r.CapiClientGetter)
	if err != nil {
		errType = capierrors.UpdateMachineError
		return checkMachineError(machineMgr, err, "unable to find Node by hostname, it may not be ready yet", errType)
	}

	errType = capierrors.UpdateMachineError
	err = machineMgr.Update(ctx)
	if err != nil {
		return checkMachineError(machineMgr, err,
			"Failed to update the Metal3Machine", errType)
	}

	return ctrl.Result{}, nil
}

func (r *Metal3MachineReconciler) reconcileDelete(ctx context.Context,
	machineMgr baremetal.MachineManagerInterface,
) (ctrl.Result, error) {
	// set machine condition to Deleting
	machineMgr.SetConditionMetal3MachineToFalse(infrav1.KubernetesNodeReadyCondition, infrav1.DeletingReason, clusterv1beta1.ConditionSeverityInfo, "")
	machineMgr.SetV1beta2Condition(infrav1.AssociateMetal3MachineMetaDataV1Beta2Condition, metav1.ConditionFalse, infrav1.Metal3MachineDeletingV1Beta2Reason, "")

	errType := capierrors.DeleteMachineError

	// dissociate metadata if any
	if err := machineMgr.DissociateM3Metadata(ctx); err != nil {
		machineMgr.SetConditionMetal3MachineToFalse(infrav1.KubernetesNodeReadyCondition, infrav1.DisassociateM3MetaDataFailedReason, clusterv1beta1.ConditionSeverityWarning, err.Error())
		machineMgr.SetV1beta2Condition(infrav1.AssociateMetal3MachineMetaDataV1Beta2Condition, metav1.ConditionFalse, infrav1.DisassociateM3MetaDataFailedV1Beta2Reason, err.Error())

		return checkMachineError(machineMgr, err,
			"failed to dissociate Metadata", errType)
	}

	// delete the machine
	if err := machineMgr.Delete(ctx); err != nil {
		machineMgr.SetConditionMetal3MachineToFalse(infrav1.KubernetesNodeReadyCondition, infrav1.DeletionFailedReason, clusterv1beta1.ConditionSeverityWarning, err.Error())
		machineMgr.SetV1beta2Condition(infrav1.AssociateMetal3MachineMetaDataV1Beta2Condition, metav1.ConditionFalse, infrav1.Metal3MachineDeletingFailedV1Beta2Reason, err.Error())

		return checkMachineError(machineMgr, err,
			"failed to delete Metal3Machine", errType)
	}

	// metal3machine is marked for deletion and ready to be deleted,
	// so remove the finalizer.
	machineMgr.UnsetFinalizer()

	return ctrl.Result{}, nil
}

// SetupWithManager will add watches for this controller.
func (r *Metal3MachineReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.Metal3Machine{}).
		WithOptions(options).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(mgr.GetScheme(), ctrl.LoggerFrom(ctx), r.WatchFilterValue)).
		Watches(
			&clusterv1.Machine{},
			handler.EnqueueRequestsFromMapFunc(util.MachineToInfrastructureMapFunc(infrav1.GroupVersion.WithKind("Metal3Machine"))),
		).
		Watches(
			&clusterv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(r.ClusterToMetal3Machines),
		).
		Watches(
			&infrav1.Metal3Cluster{},
			handler.EnqueueRequestsFromMapFunc(r.Metal3ClusterToMetal3Machines),
		).
		Watches(
			&infrav1.Metal3DataClaim{},
			handler.EnqueueRequestsFromMapFunc(r.Metal3DataClaimToMetal3Machines),
		).
		Watches(
			&infrav1.Metal3Data{},
			handler.EnqueueRequestsFromMapFunc(r.Metal3DataToMetal3Machines),
		).
		Watches(
			&bmov1alpha1.BareMetalHost{},
			handler.EnqueueRequestsFromMapFunc(r.BareMetalHostToMetal3Machines),
		).
		Complete(r)
}

// ClusterToMetal3Machines is a handler.ToRequestsFunc to be used to enqeue
// requests for reconciliation of Metal3Machines.
func (r *Metal3MachineReconciler) ClusterToMetal3Machines(ctx context.Context, o client.Object) []ctrl.Request {
	result := []ctrl.Request{}
	c, ok := o.(*clusterv1.Cluster)

	if !ok {
		r.Log.Error(errors.Errorf("expected a Cluster but got a %T", o),
			"failed to get Metal3Machine for Cluster",
		)
		return nil
	}

	labels := map[string]string{clusterv1.ClusterNameLabel: c.Name}
	capiMachineList := &clusterv1.MachineList{}
	if err := r.Client.List(ctx, capiMachineList, client.InNamespace(c.Namespace),
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
		if m.Namespace != "" {
			name = client.ObjectKey{Namespace: m.Namespace, Name: m.Spec.InfrastructureRef.Name}
		}
		result = append(result, ctrl.Request{NamespacedName: name})
	}

	return result
}

// Metal3ClusterToMetal3Machines is a handler.ToRequestsFunc to be used to enqeue
// requests for reconciliation of Metal3Machines.
func (r *Metal3MachineReconciler) Metal3ClusterToMetal3Machines(ctx context.Context, o client.Object) []ctrl.Request {
	result := []ctrl.Request{}
	c, ok := o.(*infrav1.Metal3Cluster)
	if !ok {
		r.Log.Error(errors.Errorf("expected a Metal3Cluster but got a %T", o),
			"failed to get Metal3Machine for Metal3Cluster",
		)
		return nil
	}
	log := r.Log.WithValues("Metal3Cluster", c.Name, "Namespace", c.Namespace)

	cluster, err := util.GetOwnerCluster(ctx, r.Client, c.ObjectMeta)
	switch {
	case apierrors.IsNotFound(err) || cluster == nil:
		return result
	case err != nil:
		log.Error(err, "failed to get owning cluster")
		return result
	}

	labels := map[string]string{clusterv1.ClusterNameLabel: cluster.Name}
	capiMachineList := &clusterv1.MachineList{}
	if err := r.Client.List(ctx, capiMachineList, client.InNamespace(c.Namespace), client.MatchingLabels(labels)); err != nil {
		log.Error(err, "failed to list Metal3Machines")
		return nil
	}
	for _, m := range capiMachineList.Items {
		if m.Spec.InfrastructureRef.Name == "" {
			continue
		}
		name := client.ObjectKey{Namespace: m.Namespace, Name: m.Spec.InfrastructureRef.Name}
		if m.Namespace != "" {
			name = client.ObjectKey{Namespace: m.Namespace, Name: m.Spec.InfrastructureRef.Name}
		}
		result = append(result, ctrl.Request{NamespacedName: name})
	}
	return result
}

// BareMetalHostToMetal3Machines will return a reconcile request for a Metal3Machine if the event is for a
// BareMetalHost and that BareMetalHost references a Metal3Machine.
func (r *Metal3MachineReconciler) BareMetalHostToMetal3Machines(_ context.Context, obj client.Object) []ctrl.Request {
	if host, ok := obj.(*bmov1alpha1.BareMetalHost); ok {
		if host.Spec.ConsumerRef != nil &&
			host.Spec.ConsumerRef.Kind == Metal3Machine &&
			host.Spec.ConsumerRef.GroupVersionKind().Group == infrav1.GroupVersion.Group {
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
func (r *Metal3MachineReconciler) Metal3DataClaimToMetal3Machines(_ context.Context, obj client.Object) []ctrl.Request {
	requests := []ctrl.Request{}
	if m3dc, ok := obj.(*infrav1.Metal3DataClaim); ok {
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
				oGV.Group == infrav1.GroupVersion.Group {
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
func (r *Metal3MachineReconciler) Metal3DataToMetal3Machines(_ context.Context, obj client.Object) []ctrl.Request {
	requests := []ctrl.Request{}
	if m3d, ok := obj.(*infrav1.Metal3Data); ok {
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
			if aGV.Group != infrav1.GroupVersion.Group {
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
func setErrorM3Machine(m3m *infrav1.Metal3Machine, message string, reason capierrors.MachineStatusError) {
	m3m.Status.FailureMessage = ptr.To(message)
	m3m.Status.FailureReason = &reason
}

func checkMachineError(machineMgr baremetal.MachineManagerInterface, err error,
	errMessage string, errType capierrors.MachineStatusError) (ctrl.Result, error) {
	if err == nil {
		return ctrl.Result{}, nil
	}

	var reconcileError baremetal.ReconcileError
	if errors.As(err, &reconcileError) {
		if reconcileError.IsTransient() {
			return reconcile.Result{Requeue: true, RequeueAfter: reconcileError.GetRequeueAfter()}, nil
		}
		if reconcileError.IsTerminal() {
			machineMgr.SetError(errMessage, errType)
			return reconcile.Result{}, nil
		}
	}
	return ctrl.Result{}, errors.Wrap(err, errMessage)
}

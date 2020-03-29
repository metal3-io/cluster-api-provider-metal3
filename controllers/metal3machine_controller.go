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

package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	bmh "github.com/metal3-io/baremetal-operator/pkg/apis/metal3/v1alpha1"
	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1alpha3"
	"github.com/metal3-io/cluster-api-provider-metal3/baremetal"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	machineControllerName = "Metal3Machine-controller"
)

// Metal3MachineReconciler reconciles a Metal3Machine object
type Metal3MachineReconciler struct {
	Client           client.Client
	ManagerFactory   baremetal.ManagerFactoryInterface
	Log              logr.Logger
	CapiClientGetter baremetal.ClientGetter
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metal3machines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metal3machines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Add RBAC rules to access cluster-api resources
// +kubebuilder:rbac:groups=metal3.io,resources=baremetalhosts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metal3.io,resources=baremetalhosts/status,verbs=get;update;patch

// Reconcile handles Metal3Machine events
func (r *Metal3MachineReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, rerr error) {
	ctx := context.Background()
	machineLog := r.Log.WithName(machineControllerName).WithValues("metal3-machine", req.NamespacedName)

	// Fetch the Metal3Machine instance.
	capm3Machine := &capm3.Metal3Machine{}

	if err := r.Client.Get(ctx, req.NamespacedName, capm3Machine); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	helper, err := patch.NewHelper(capm3Machine, r.Client)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to init patch helper")
	}
	// Always patch capm3Machine exiting this function so we can persist any Metal3Machine changes.
	defer func() {
		err := helper.Patch(ctx, capm3Machine)
		if err != nil {
			machineLog.Info("failed to Patch capm3Machine")
		}
	}()
	//clear an error if one was previously set
	clearErrorBMMachine(capm3Machine)

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
		machineLog.Info("Metal3Machine's owner Machine is missing cluster label or cluster does not exist")
		setErrorBMMachine(capm3Machine, "Metal3Machine's owner Machine is missing cluster label or cluster does not exist", capierrors.InvalidConfigurationMachineError)

		return ctrl.Result{}, errors.Wrapf(err, "Metal3Machine's owner Machine is missing label or the cluster does not exist")
	}
	if cluster == nil {
		setErrorBMMachine(capm3Machine, fmt.Sprintf(
			"The machine is NOT associated with a cluster using the label %s: <name of cluster>",
			capi.ClusterLabelName,
		), capierrors.InvalidConfigurationMachineError)
		machineLog.Info(fmt.Sprintf("Please associate this machine with a cluster using the label %s: <name of cluster>", capi.ClusterLabelName))
		return ctrl.Result{}, nil
	}

	machineLog = machineLog.WithValues("cluster", cluster.Name)

	// Return early if the BMMachine or Cluster is paused.
	if util.IsPaused(cluster, capm3Machine) {
		machineLog.Info("reconciliation is paused for this object")
		return ctrl.Result{Requeue: true, RequeueAfter: requeueAfter}, nil
	}

	// Make sure infrastructure is ready
	if !cluster.Status.InfrastructureReady {
		machineLog.Info("Waiting for Metal3Cluster Controller to create cluster infrastructure")
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

	// Handle deleted machines
	if !capm3Machine.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, machineMgr)
	}

	// Handle non-deleted machines
	return r.reconcileNormal(ctx, machineMgr)
}

func (r *Metal3MachineReconciler) reconcileNormal(ctx context.Context,
	machineMgr baremetal.MachineManagerInterface,
) (ctrl.Result, error) {
	// If the Metal3Machine doesn't have finalizer, add it.
	machineMgr.SetFinalizer()

	// if the machine is already provisioned, return
	if machineMgr.IsProvisioned() {
		err := machineMgr.Update(ctx)
		return ctrl.Result{}, err
	}

	// Make sure bootstrap data is available and populated. If not, return, we
	// will get an event from the machine update when the flag is set to true.
	if !machineMgr.IsBootstrapReady() {
		return ctrl.Result{}, nil
	}

	// Check if the metal3machine was associated with a baremetalhost
	if !machineMgr.HasAnnotation() {
		//Associate the baremetalhost hosting the machine
		err := machineMgr.Associate(ctx)
		if err != nil {
			return checkError(err, "failed to associate the Metal3Machine to a BaremetalHost")
		}
	} else {
		err := machineMgr.Update(ctx)
		if err != nil {
			return checkError(err, "failed to update BaremetalHost")
		}
	}

	bmhID, err := machineMgr.GetBaremetalHostID(ctx)
	if err != nil {
		return checkError(err, "failed to get the providerID for the metal3machine")
	}
	if bmhID != nil {
		providerID := fmt.Sprintf("metal3://%s", *bmhID)
		// Set the providerID on the node if no Cloud provider
		err = machineMgr.SetNodeProviderID(ctx, *bmhID, providerID, r.CapiClientGetter)
		if err != nil {
			return checkError(err, "failed to get the providerID for the metal3machine")
		}
		// Make sure Spec.ProviderID is set and mark the capm3Machine ready
		machineMgr.SetProviderID(providerID)
	}

	return ctrl.Result{}, err
}

func (r *Metal3MachineReconciler) reconcileDelete(ctx context.Context,
	machineMgr baremetal.MachineManagerInterface,
) (ctrl.Result, error) {

	// delete the machine
	if err := machineMgr.Delete(ctx); err != nil {
		return checkError(err, "failed to delete Metal3Machine")
	}

	// metal3machine is marked for deletion and ready to be deleted,
	// so remove the finalizer.
	machineMgr.UnsetFinalizer()

	return ctrl.Result{}, nil
}

// SetupWithManager will add watches for this controller
func (r *Metal3MachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&capm3.Metal3Machine{}).
		Watches(
			&source.Kind{Type: &capi.Machine{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: util.MachineToInfrastructureMapFunc(capm3.GroupVersion.WithKind("Metal3Machine")),
			},
		).
		Watches(
			&source.Kind{Type: &capm3.Metal3Cluster{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: handler.ToRequestsFunc(r.Metal3ClusterToMetal3Machines),
			},
		).
		Watches(
			&source.Kind{Type: &bmh.BareMetalHost{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: handler.ToRequestsFunc(r.BareMetalHostToMetal3Machines),
			},
		).
		Complete(r)
}

// Metal3ClusterToMetal3Machines is a handler.ToRequestsFunc to be used to enqeue
// requests for reconciliation of Metal3Machines.
func (r *Metal3MachineReconciler) Metal3ClusterToMetal3Machines(o handler.MapObject) []ctrl.Request {
	result := []ctrl.Request{}
	c, ok := o.Object.(*capm3.Metal3Cluster)
	if !ok {
		r.Log.Error(errors.Errorf("expected a Metal3Cluster but got a %T", o.Object), "failed to get Metal3Machine for Metal3Cluster")
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

	labels := map[string]string{capi.ClusterLabelName: cluster.Name}
	capiMachineList := &capi.MachineList{}
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
func (r *Metal3MachineReconciler) BareMetalHostToMetal3Machines(obj handler.MapObject) []ctrl.Request {
	if host, ok := obj.Object.(*bmh.BareMetalHost); ok {
		if host.Spec.ConsumerRef != nil &&
			host.Spec.ConsumerRef.Kind == "Metal3Machine" &&
			host.Spec.ConsumerRef.APIVersion == capm3.GroupVersion.String() {
			return []ctrl.Request{
				ctrl.Request{
					NamespacedName: types.NamespacedName{
						Name:      host.Spec.ConsumerRef.Name,
						Namespace: host.Spec.ConsumerRef.Namespace,
					},
				},
			}
		}
	}
	return []ctrl.Request{}
}

// setError sets the ErrorMessage and ErrorReason fields on the metal3machine
func setErrorBMMachine(bmm *capm3.Metal3Machine, message string, reason capierrors.MachineStatusError) {

	bmm.Status.FailureMessage = pointer.StringPtr(message)
	bmm.Status.FailureReason = &reason

}

// clearError removes the ErrorMessage from the metal3machine's Status if set.
func clearErrorBMMachine(bmm *capm3.Metal3Machine) {

	if bmm.Status.FailureMessage != nil || bmm.Status.FailureReason != nil {
		bmm.Status.FailureMessage = nil
		bmm.Status.FailureReason = nil
	}

}

func checkError(err error, errMessage string) (ctrl.Result, error) {
	if requeueErr, ok := errors.Cause(err).(baremetal.HasRequeueAfterError); ok {
		return ctrl.Result{Requeue: true, RequeueAfter: requeueErr.GetRequeueAfter()}, nil
	}
	return ctrl.Result{}, errors.Wrap(err, errMessage)
}

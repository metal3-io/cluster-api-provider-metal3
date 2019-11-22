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
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"

	capbm "sigs.k8s.io/cluster-api-provider-baremetal/api/v1alpha2"
	"sigs.k8s.io/cluster-api-provider-baremetal/baremetal"
	capi "sigs.k8s.io/cluster-api/api/v1alpha2"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	machineControllerName = "BareMetalMachine-controller"
)

// BareMetalMachineReconciler reconciles a BareMetalMachine object
type BareMetalMachineReconciler struct {
	Client           client.Client
	ManagerFactory   baremetal.ManagerFactoryInterface
	Log              logr.Logger
	CapiClientGetter baremetal.ClientGetter
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=baremetalmachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=baremetalmachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Add RBAC rules to access cluster-api resources
// +kubebuilder:rbac:groups=metal3.io,resources=baremetalhosts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metal3.io,resources=baremetalhosts/status,verbs=get;update;patch

// Reconcile handles BareMetalMachine events
func (r *BareMetalMachineReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, rerr error) {
	ctx := context.Background()
	machineLog := r.Log.WithName(machineControllerName).WithValues("baremetal-machine", req.NamespacedName)

	// Fetch the BareMetalMachine instance.
	capbmMachine := &capbm.BareMetalMachine{}

	if err := r.Client.Get(ctx, req.NamespacedName, capbmMachine); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	helper, err := patch.NewHelper(capbmMachine, r.Client)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to init patch helper")
	}
	// Always patch capbMachine exiting this function so we can persist any BareMetalMachine changes.
	defer func() {
		err := helper.Patch(ctx, capbmMachine)
		if err != nil {
			machineLog.Info("failed to Patch capbmMachine")
		}
	}()
	//clear an error if one was previously set
	clearErrorBMMachine(capbmMachine)

	// Fetch the Machine.
	capiMachine, err := util.GetOwnerMachine(ctx, r.Client, capbmMachine.ObjectMeta)

	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "BareMetalMachine's owner Machine could not be retrieved")
	}
	if capiMachine == nil {
		machineLog.Info("Waiting for Machine Controller to set OwnerRef on BareMetalMachine")
		return ctrl.Result{}, nil
	}

	machineLog = machineLog.WithValues("machine", capiMachine.Name)

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, capiMachine.ObjectMeta)
	if err != nil {
		er := errors.New("BareMetalMachine's owner Machine is missing cluster label or cluster does not exist")
		machineLog.Info("BareMetalMachine's owner Machine is missing cluster label or cluster does not exist")
		setErrorBMMachine(capbmMachine, er, capierrors.InvalidConfigurationMachineError)

		return ctrl.Result{}, errors.Wrapf(err, "BareMetalMachine's owner Machine is missing label or the cluster does not exist")
	}
	if cluster == nil {
		machineLog.Info(fmt.Sprintf("The machine is NOT associated with a cluster using the label %s: <name of cluster>", capi.MachineClusterLabelName))
		return ctrl.Result{}, nil
	}

	machineLog = machineLog.WithValues("cluster", cluster.Name)

	// Make sure infrastructure is ready
	if !cluster.Status.InfrastructureReady {
		machineLog.Info("Waiting for BareMetalCluster Controller to create cluster infrastructure")
		return ctrl.Result{}, nil
	}

	// Fetch the BareMetal Cluster.
	baremetalCluster := &capbm.BareMetalCluster{}
	baremetalClusterName := types.NamespacedName{
		Namespace: capbmMachine.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if err := r.Client.Get(ctx, baremetalClusterName, baremetalCluster); err != nil {
		machineLog.Info("Waiting for BareMetalCluster Controller to create the BareMetalCluster")
		return ctrl.Result{}, nil
	}

	machineLog = machineLog.WithValues("baremetal-cluster", baremetalCluster.Name)

	// Create a helper for managing the baremetal container hosting the machine.
	machineMgr, err := r.ManagerFactory.NewMachineManager(cluster, baremetalCluster, capiMachine, capbmMachine, machineLog)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to create helper for managing the machineMgr")
	}

	// Create a helper for managing a baremetal container hosting the loadbalancer.
	// NB. the machine controller has to manage the cluster load balancer because the current implementation of the
	// baremetal load balancer does not support auto-discovery of control plane nodes, so CAPD should take care of
	// updating the cluster load balancer configuration when control plane machines are added/removed
	clusterMgr, err := r.ManagerFactory.NewClusterManager(cluster, baremetalCluster, machineLog)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to create helper for managing the clusterMgr")
	}

	// Handle deleted machines
	if !capbmMachine.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, machineMgr, clusterMgr)
	}

	// Handle non-deleted machines
	return r.reconcileNormal(ctx, machineMgr, clusterMgr)
}

func (r *BareMetalMachineReconciler) reconcileNormal(ctx context.Context,
	machineMgr baremetal.MachineManagerInterface,
	clusterMgr baremetal.ClusterManagerInterface,
) (ctrl.Result, error) {
	// If the BareMetalMachine doesn't have finalizer, add it.
	if !util.Contains(machineMgr.GetBareMetalMachine().Finalizers, capbm.MachineFinalizer) {
		machineMgr.GetBareMetalMachine().Finalizers = append(machineMgr.GetBareMetalMachine().Finalizers, capbm.MachineFinalizer)
	}

	// if the machine is already provisioned, return
	if machineMgr.GetBareMetalMachine().Spec.ProviderID != nil && machineMgr.GetBareMetalMachine().Status.Ready {
		err := machineMgr.Update(ctx)
		return ctrl.Result{}, err
	}

	// Make sure bootstrap data is available and populated.
	if !machineMgr.GetMachine().Status.BootstrapReady {
		machineMgr.GetLog().Info("Waiting for the Bootstrap provider controller to set bootstrap data")
		return ctrl.Result{}, nil
	}

	// Check if the baremetalmachine was associated with a baremetalhost
	if !machineMgr.HasAnnotation() {
		//Associate the baremetalhost hosting the machine
		err := machineMgr.Associate(ctx)
		if err != nil {
			er := errors.Wrap(err, "failed to associate the BareMetalMachine to a BaremetalHost")
			setErrorBMMachine(machineMgr.GetBareMetalMachine(), er, capierrors.CreateMachineError)
			return ctrl.Result{}, er
		}
	}

	bmhID, err := machineMgr.GetBaremetalHostID(ctx)
	if err != nil {
		if requeueErr, ok := errors.Cause(err).(baremetal.HasRequeueAfterError); ok {
			machineMgr.GetLog().Info("Provisioning BaremetalHost, requeuing")
			return ctrl.Result{Requeue: true, RequeueAfter: requeueErr.GetRequeueAfter()}, nil
		}
		er := errors.Wrap(err, "failed to get the providerID for the BaremetalMachine")
		setErrorBMMachine(machineMgr.GetBareMetalMachine(), er, capierrors.CreateMachineError)
		return ctrl.Result{}, er
	}
	if bmhID != nil {
		providerID := fmt.Sprintf("metal3://%s", *bmhID)
		// Set the providerID on the node if no Cloud provider
		if machineMgr.GetBareMetalCluster().Spec.NoCloudProvider {
			err = machineMgr.SetNodeProviderID(ctx, *bmhID, providerID, r.CapiClientGetter)
			if err != nil {
				if requeueErr, ok := errors.Cause(err).(baremetal.HasRequeueAfterError); ok {
					return ctrl.Result{Requeue: true, RequeueAfter: requeueErr.GetRequeueAfter()}, nil
				}
				return ctrl.Result{}, errors.Wrap(err, "failed to get the providerID for the BaremetalMachine")
			}
			machineMgr.GetLog().Info("ProviderID set on target node")
		}

		// Make sure Spec.ProviderID is set and mark the capbmMachine ready
		machineMgr.SetProviderID(ctx, &providerID)
	}

	err = machineMgr.Update(ctx)
	return ctrl.Result{}, err
}

func (r *BareMetalMachineReconciler) reconcileDelete(ctx context.Context,
	machineMgr baremetal.MachineManagerInterface,
	clusterMgr baremetal.ClusterManagerInterface,
) (ctrl.Result, error) {

	// delete the machine
	if err := machineMgr.Delete(ctx); err != nil {
		if requeueErr, ok := errors.Cause(err).(baremetal.HasRequeueAfterError); ok {
			machineMgr.GetLog().Info("Deprovisioning BaremetalHost, requeuing")
			return ctrl.Result{Requeue: true, RequeueAfter: requeueErr.GetRequeueAfter()}, nil
		}

		er := errors.Wrap(err, "failed to delete BareMetalMachine")
		setErrorBMMachine(machineMgr.GetBareMetalMachine(), er, capierrors.DeleteMachineError)
		return ctrl.Result{}, er
	}

	// Machine is deleted so remove the finalizer.
	machineMgr.GetBareMetalMachine().Finalizers = util.Filter(machineMgr.GetBareMetalMachine().Finalizers, capbm.MachineFinalizer)

	return ctrl.Result{}, nil
}

// SetupWithManager will add watches for this controller
func (r *BareMetalMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&capbm.BareMetalMachine{}).
		Watches(
			&source.Kind{Type: &capi.Machine{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: util.MachineToInfrastructureMapFunc(capbm.GroupVersion.WithKind("BareMetalMachine")),
			},
		).
		Watches(
			&source.Kind{Type: &capbm.BareMetalCluster{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: handler.ToRequestsFunc(r.BareMetalClusterToBareMetalMachines),
			},
		).
		Watches(
			&source.Kind{Type: &bmh.BareMetalHost{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: handler.ToRequestsFunc(r.BareMetalHostToBareMetalMachines),
			},
		).
		Complete(r)
}

// BareMetalClusterToBareMetalMachines is a handler.ToRequestsFunc to be used to enqeue
// requests for reconciliation of BareMetalMachines.
func (r *BareMetalMachineReconciler) BareMetalClusterToBareMetalMachines(o handler.MapObject) []ctrl.Request {
	result := []ctrl.Request{}
	c, ok := o.Object.(*capbm.BareMetalCluster)
	if !ok {
		r.Log.Error(errors.Errorf("expected a BareMetalCluster but got a %T", o.Object), "failed to get BareMetalMachine for BareMetalCluster")
		return nil
	}
	log := r.Log.WithValues("BareMetalCluster", c.Name, "Namespace", c.Namespace)

	cluster, err := util.GetOwnerCluster(context.TODO(), r.Client, c.ObjectMeta)
	switch {
	case apierrors.IsNotFound(err) || cluster == nil:
		return result
	case err != nil:

		er := errors.New("failed to get owning cluster")
		log.Error(err, "failed to get owning cluster")
		setErrorBMCluster(c, er, capierrors.InvalidConfigurationClusterError)
		return result
	}

	labels := map[string]string{capi.MachineClusterLabelName: cluster.Name}
	capiMachineList := &capi.MachineList{}
	if err := r.Client.List(context.TODO(), capiMachineList, client.InNamespace(c.Namespace), client.MatchingLabels(labels)); err != nil {
		log.Error(err, "failed to list BareMetalMachines")
		return nil
	}
	for _, m := range capiMachineList.Items {
		if m.Spec.InfrastructureRef.Name == "" {
			continue
		}
		name := client.ObjectKey{Namespace: m.Namespace, Name: m.Name}
		result = append(result, ctrl.Request{NamespacedName: name})
	}

	return result
}

// BareMetalHostToBareMetalMachines will return a reconcile request for a BareMetalMachine if the event is for a
// BareMetalHost and that BareMetalHost references a BareMetalMachine.
func (r *BareMetalMachineReconciler) BareMetalHostToBareMetalMachines(obj handler.MapObject) []ctrl.Request {
	if host, ok := obj.Object.(*bmh.BareMetalHost); ok {
		if host.Spec.ConsumerRef != nil &&
			host.Spec.ConsumerRef.Kind == "BareMetalMachine" &&
			host.Spec.ConsumerRef.APIVersion == capbm.GroupVersion.String() {
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

// setError sets the ErrorMessage and ErrorReason fields on the baremetalmachine
func setErrorBMMachine(bmm *capbm.BareMetalMachine, message error, reason capierrors.MachineStatusError) {

	bmm.Status.ErrorMessage = pointer.StringPtr(message.Error())
	bmm.Status.ErrorReason = &reason

}

// clearError removes the ErrorMessage from the baremetalmachine's Status if set.
func clearErrorBMMachine(bmm *capbm.BareMetalMachine) {

	if bmm.Status.ErrorMessage != nil || bmm.Status.ErrorReason != nil {
		bmm.Status.ErrorMessage = nil
		bmm.Status.ErrorReason = nil
	}

}

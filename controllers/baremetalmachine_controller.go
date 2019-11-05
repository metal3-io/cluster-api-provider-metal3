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
	capbm "sigs.k8s.io/cluster-api-provider-baremetal/api/v1alpha2"
	"sigs.k8s.io/cluster-api-provider-baremetal/baremetal"
	capi "sigs.k8s.io/cluster-api/api/v1alpha2"
	"sigs.k8s.io/cluster-api/util"
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
	Client         client.Client
	ManagerFactory baremetal.ManagerFactory
	Log            logr.Logger
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

		return ctrl.Result{}, errors.Wrapf(err, fmt.Sprintf("%#v", apierrors.ReasonForError(err)))
	}

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
		machineLog.Info("BareMetalMachine's owner Machine is missing cluster label or cluster does not exist")
		return ctrl.Result{}, errors.Wrapf(err, "BareMetalMachine's owner Machine is missing cluster label or cluster does not exist")
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

	// Always close the scope when exiting this function so we can persist any BareMetalMachine changes.
	defer func() {
		if err := machineMgr.Close(); err != nil && rerr == nil {
			rerr = err
		}
	}()

	// Handle deleted machines
	if !capbmMachine.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, machineMgr, clusterMgr, machineLog)
	}

	// Handle non-deleted machines
	return r.reconcileNormal(ctx, machineMgr, clusterMgr, machineLog)
}

func (r *BareMetalMachineReconciler) reconcileNormal(ctx context.Context,
	machineMgr *baremetal.MachineManager,
	clusterMgr *baremetal.ClusterManager, log logr.Logger) (ctrl.Result, error) {
	// If the BareMetalMachine doesn't have finalizer, add it.
	if !util.Contains(machineMgr.BareMetalMachine.Finalizers, capbm.MachineFinalizer) {
		machineMgr.BareMetalMachine.Finalizers = append(machineMgr.BareMetalMachine.Finalizers, capbm.MachineFinalizer)
	}

	// if the machine is already provisioned, return
	if machineMgr.BareMetalMachine.Spec.ProviderID != nil && machineMgr.BareMetalMachine.Status.Ready {
		err := machineMgr.Update(ctx)
		return ctrl.Result{}, err
	}

	// Make sure bootstrap data is available and populated.
	if !machineMgr.Machine.Status.BootstrapReady {
		log.Info("Waiting for the Bootstrap provider controller to set bootstrap data")
		return ctrl.Result{}, nil
	}

	// Check if the baremetalmachine was associated with a baremetalhost
	if !machineMgr.HasAnnotation() {
		//Associate the baremetalhost hosting the machine
		err := machineMgr.Associate(ctx)
		if err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to associate the BareMetalMachine to a BaremetalHost")
		}
	}

	providerID, err := machineMgr.GetProviderID(ctx)
	if err != nil {
		if requeueErr, ok := errors.Cause(err).(baremetal.HasRequeueAfterError); ok {
			log.Info("Provisioning BaremetalHost, requeuing")
			return ctrl.Result{Requeue: true, RequeueAfter: requeueErr.GetRequeueAfter()}, nil
		}
		return ctrl.Result{}, errors.Wrap(err, "failed to get the providerID for the BaremetalMachine")
	}
	if providerID != nil {
		// Make sure Spec.ProviderID is always set.
		machineMgr.SetProviderID(ctx, providerID)

		// Mark the capbmMachine ready
		machineMgr.SetReady()
	}

	err = machineMgr.Update(ctx)
	return ctrl.Result{}, err
}

func (r *BareMetalMachineReconciler) reconcileDelete(ctx context.Context,
	machineMgr *baremetal.MachineManager, clusterMgr *baremetal.ClusterManager,
	log logr.Logger) (ctrl.Result, error) {

	// delete the machine
	if err := machineMgr.Delete(ctx); err != nil {
		if requeueErr, ok := errors.Cause(err).(baremetal.HasRequeueAfterError); ok {
			log.Info("Deprovisioning BaremetalHost, requeuing")
			return ctrl.Result{Requeue: true, RequeueAfter: requeueErr.GetRequeueAfter()}, nil
		}
		return ctrl.Result{}, errors.Wrap(err, "failed to delete BareMetalMachine")
	}

	// Machine is deleted so remove the finalizer.
	machineMgr.BareMetalMachine.Finalizers = util.Filter(machineMgr.BareMetalMachine.Finalizers, capbm.MachineFinalizer)

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
		log.Error(err, "failed to get owning cluster")
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

// BareMetalHostToBareMetalMachines will return a reconcile request for a Machine if the event is for a
// BareMetalHost and that BareMetalHost references a Machine.
func (r *BareMetalMachineReconciler) BareMetalHostToBareMetalMachines(obj handler.MapObject) []ctrl.Request {
	if host, ok := obj.Object.(*bmh.BareMetalHost); ok {
		if host.Spec.ConsumerRef != nil && host.Spec.ConsumerRef.Kind == "Machine" && host.Spec.ConsumerRef.APIVersion == capi.GroupVersion.String() {
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

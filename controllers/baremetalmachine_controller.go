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

// Add RBAC rules to access cluster-api resources
// +kubebuilder:rbac:groups=metal3.io,resources=baremetalhosts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metal3.io,resources=baremetalhosts/status,verbs=get;update;patch

// Reconcile handles BareMetalMachine events
func (r *BareMetalMachineReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, rerr error) {
	ctx := context.Background()
	log := r.Log.WithName(machineControllerName).WithValues("baremetal-machine", req.NamespacedName)

	// Fetch the BareMetalMachine instance.
	capbmMachine := &capbm.BareMetalMachine{}
	if err := r.Client.Get(ctx, req.NamespacedName, capbmMachine); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Fetch the Machine.
	capiMachine, err := util.GetOwnerMachine(ctx, r.Client, capbmMachine.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if capiMachine == nil {
		log.Info("Waiting for Machine Controller to set OwnerRef on BareMetalMachine")
		return ctrl.Result{}, nil
	}

	log = log.WithValues("machine", capiMachine.Name)

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, capiMachine.ObjectMeta)
	if err != nil {
		log.Info("BareMetalMachine owner Machine is missing cluster label or cluster does not exist")
		return ctrl.Result{}, err
	}
	if cluster == nil {
		log.Info(fmt.Sprintf("Please associate this machine with a cluster using the label %s: <name of cluster>", capi.MachineClusterLabelName))
		return ctrl.Result{}, nil
	}

	log = log.WithValues("cluster", cluster.Name)

	// Make sure infrastructure is ready
	if !cluster.Status.InfrastructureReady {
		log.Info("Waiting for BareMetalCluster Controller to create cluster infrastructure")
		return ctrl.Result{}, nil
	}

	// Fetch the BareMetal Cluster.
	baremetalCluster := &capbm.BareMetalCluster{}
	baremetalClusterName := types.NamespacedName{
		Namespace: capbmMachine.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if err := r.Client.Get(ctx, baremetalClusterName, baremetalCluster); err != nil {
		log.Info("BareMetalCluster is not available yet")
		return ctrl.Result{}, nil
	}

	log = log.WithValues("baremetal-cluster", baremetalCluster.Name)

	// Create a helper for managing the baremetal container hosting the machine.
	machineMgr, err := r.ManagerFactory.NewMachineManager(cluster, baremetalCluster, capiMachine, capbmMachine)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to create helper for managing the machineMgr")
	}

	// Create a helper for managing a baremetal container hosting the loadbalancer.
	// NB. the machine controller has to manage the cluster load balancer because the current implementation of the
	// baremetal load balancer does not support auto-discovery of control plane nodes, so CAPD should take care of
	// updating the cluster load balancer configuration when control plane machines are added/removed
	clusterMgr, err := r.ManagerFactory.NewClusterManager(cluster, baremetalCluster)
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
		return r.reconcileDelete(ctx, machineMgr, clusterMgr)
	}

	// Handle non-deleted machines
	return r.reconcileNormal(ctx, machineMgr, clusterMgr, log)
}

func (r *BareMetalMachineReconciler) reconcileNormal(ctx context.Context,
	machineMgr *baremetal.MachineManager,
	clusterMgr *baremetal.ClusterManager, log logr.Logger) (ctrl.Result, error) {
	// If the BareMetalMachine doesn't have finalizer, add it.
	if !util.Contains(machineMgr.BareMetalMachine.Finalizers, capbm.MachineFinalizer) {
		machineMgr.BareMetalMachine.Finalizers = append(machineMgr.BareMetalMachine.Finalizers, capbm.MachineFinalizer)
	}

	if !machineMgr.Cluster.Status.InfrastructureReady {
		log.Info("Cluster infrastructure is not ready yet")
		return ctrl.Result{}, nil
	}

	// if the machine is already provisioned, return
	// if machineMgr.BareMetalMachine.Spec.ProviderID != nil {
	//	return ctrl.Result{}, nil
	// }

	// Make sure bootstrap data is available and populated.
	// if machineMgr.Machine.Spec.Bootstrap.Data == nil {
	// 	log.Info("Waiting for the Bootstrap provider controller to set bootstrap data")
	// 	return ctrl.Result{}, nil
	// }

	//Create the baremetal container hosting the machine
	providerId, err := machineMgr.Create(ctx)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to create worker BareMetalMachine")
	}

	// if the machine is a control plane added, update the load balancer configuration
	if util.IsControlPlaneMachine(machineMgr.Machine) {
		if err := clusterMgr.UpdateConfiguration(); err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to update BareMetalCluster.loadbalancer configuration")
		}
	}

	// exec bootstrap
	// NB. this step is necessary to mimic the behaviour of cloud-init that is embedded in the base images
	// for other cloud providers
	// if err := machineMgr.ExecBootstrap(*machineMgr.Machine.Spec.Bootstrap.Data); err != nil {
	//	return ctrl.Result{}, errors.Wrap(err, "failed to exec BareMetalMachine bootstrap")
	// }

	// Make sure Spec.ProviderID is always set.
	machineMgr.SetProviderID(fmt.Sprintf("metal3:////%s", providerId))

	// Mark the capbmMachine ready
	machineMgr.SetReady()

	return ctrl.Result{}, nil
}

func (r *BareMetalMachineReconciler) reconcileDelete(ctx context.Context,
	machineMgr *baremetal.MachineManager, clusterMgr *baremetal.ClusterManager) (ctrl.Result, error) {
	// if the deleted machine is a control-plane node, exec kubeadm reset so the etcd member hosted
	// on the machine gets removed in a controlled way
	if util.IsControlPlaneMachine(machineMgr.Machine) {
		if err := machineMgr.KubeadmReset(); err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to execute kubeadm reset")
		}
	}

	// delete the machine
	if _, err := machineMgr.Delete(ctx); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to delete BareMetalMachine")
	}

	// if the deleted machine is a control-plane node, remove it from the load balancer configuration;
	if util.IsControlPlaneMachine(machineMgr.Machine) {
		if err := clusterMgr.UpdateConfiguration(); err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to update BareMetalCluster configuration")
		}
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

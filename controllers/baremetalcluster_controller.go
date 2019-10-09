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

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	capbm "sigs.k8s.io/cluster-api-provider-baremetal/api/v1alpha2"
	"sigs.k8s.io/cluster-api-provider-baremetal/baremetal"
	capi "sigs.k8s.io/cluster-api/api/v1alpha2"
	"sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	clusterControllerName = "BareMetalCluster-controller"
)

// BareMetalClusterReconciler reconciles a BareMetalCluster object
type BareMetalClusterReconciler struct {
	Client         client.Client
	ManagerFactory baremetal.ManagerFactory
	Log            logr.Logger
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=baremetalclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=baremetalclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch

// Reconcile reads that state of the cluster for a BareMetalCluster object and makes changes based on the state read
// and what is in the BareMetalCluster.Spec
func (r *BareMetalClusterReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, rerr error) {
	ctx := context.Background()
	log := log.Log.WithName(clusterControllerName).WithValues("baremetal-cluster", req.NamespacedName)

	// Fetch the BareMetalCluster instance
	baremetalCluster := &capbm.BareMetalCluster{}
	if err := r.Client.Get(ctx, req.NamespacedName, baremetalCluster); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Fetch the Cluster.
	cluster, err := util.GetOwnerCluster(ctx, r.Client, baremetalCluster.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if cluster == nil {
		log.Info("Waiting for Cluster Controller to set OwnerRef on BareMetalCluster")
		return ctrl.Result{}, nil
	}

	log = log.WithValues("cluster", cluster.Name)

	// Create a helper for managing a baremetal cluster.
	clusterMgr, err := r.ManagerFactory.NewClusterManager(cluster, baremetalCluster)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to create helper for managing the clusterMgr")
	}

	// Always close the scope when exiting this function so we can persist any BaremetalMachine changes.
	defer func() {
		if err := clusterMgr.Close(); err != nil && rerr == nil {
			rerr = err
		}
	}()

	// Handle deleted clusters
	if !baremetalCluster.DeletionTimestamp.IsZero() {
		return reconcileDelete(clusterMgr)
	}

	// Handle non-deleted clusters
	return reconcileNormal(clusterMgr)
}

func reconcileNormal(clusterMgr *baremetal.ClusterManager) (ctrl.Result, error) {
	// If the BareMetalCluster doesn't have finalizer, add it.
	if !util.Contains(clusterMgr.BareMetalCluster.Finalizers, capbm.ClusterFinalizer) {
		clusterMgr.BareMetalCluster.Finalizers = append(clusterMgr.BareMetalCluster.Finalizers, capbm.ClusterFinalizer)
	}

	//Create the baremetal cluster (no-op)
	if err := clusterMgr.Create(); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to create the cluster")
	}

	// Set APIEndpoints so the Cluster API Cluster Controller can pull it
	endpoints, err := clusterMgr.APIEndpoints()
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to get ip for the API endpoint")
	}

	clusterMgr.BareMetalCluster.Status.APIEndpoints = endpoints

	// Mark the baremetalCluster ready
	clusterMgr.BareMetalCluster.Status.Ready = true

	return ctrl.Result{}, nil
}

func reconcileDelete(clusterMgr *baremetal.ClusterManager) (ctrl.Result, error) {
	// Delete the baremetal container hosting the load balancer
	if err := clusterMgr.Delete(); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to delete load balancer")
	}

	// Cluster is deleted so remove the finalizer.
	clusterMgr.BareMetalCluster.Finalizers = util.Filter(clusterMgr.BareMetalCluster.Finalizers, capbm.ClusterFinalizer)

	return ctrl.Result{}, nil
}

// SetupWithManager will add watches for this controller
func (r *BareMetalClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&capbm.BareMetalCluster{}).
		Watches(
			&source.Kind{Type: &capi.Cluster{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: util.ClusterToInfrastructureMapFunc(capbm.GroupVersion.WithKind("BareMetalCluster")),
			},
		).
		Complete(r)
}

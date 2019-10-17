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
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/pointer"
	capbm "sigs.k8s.io/cluster-api-provider-baremetal/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-baremetal/baremetal"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	clusterControllerName = "BareMetalCluster-controller"
	requeueAfter          = time.Second * 30
)

// BareMetalClusterReconciler reconciles a BareMetalCluster object
type BareMetalClusterReconciler struct {
	Client         client.Client
	ManagerFactory baremetal.ManagerFactoryInterface
	Log            logr.Logger
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=baremetalclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=baremetalclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch

// Reconcile reads that state of the cluster for a BareMetalCluster object and makes changes based on the state read
// and what is in the BareMetalCluster.Spec
func (r *BareMetalClusterReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, rerr error) {

	ctx := context.Background()
	clusterLog := log.Log.WithName(clusterControllerName).WithValues("baremetal-cluster", req.NamespacedName)

	// Fetch the BareMetalCluster instance
	baremetalCluster := &capbm.BareMetalCluster{}

	if err := r.Client.Get(ctx, req.NamespacedName, baremetalCluster); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	helper, err := patch.NewHelper(baremetalCluster, r.Client)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to init patch helper")
	}
	// Always patch baremetalCluster when exiting this function so we can persist any BaremetalCluster changes.
	defer func() {
		err := helper.Patch(ctx, baremetalCluster)
		if err != nil {
			clusterLog.Error(err, "failed to Patch baremetalCluster")
		}
	}()

	// Fetch the Cluster.
	cluster, err := util.GetOwnerCluster(ctx, r.Client, baremetalCluster.ObjectMeta)
	if err != nil {
		error := capierrors.InvalidConfigurationClusterError
		baremetalCluster.Status.FailureReason = &error
		baremetalCluster.Status.FailureMessage = pointer.StringPtr("Unable to get owner cluster")
		return ctrl.Result{}, err
	}
	if cluster == nil {
		clusterLog.Info("Waiting for Cluster Controller to set OwnerRef on BareMetalCluster")
		return ctrl.Result{}, nil
	}

	clusterLog = clusterLog.WithValues("cluster", cluster.Name)
	clusterLog.Info("Reconciling BaremetalCluster")

	// Create a helper for managing a baremetal cluster.
	clusterMgr, err := r.ManagerFactory.NewClusterManager(cluster, baremetalCluster, clusterLog)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to create helper for managing the clusterMgr")
	}
	if clusterMgr == nil {
		return ctrl.Result{}, nil
	}

	// Handle deleted clusters
	if !baremetalCluster.DeletionTimestamp.IsZero() {
		return reconcileDelete(ctx, clusterMgr)
	}

	// Handle non-deleted clusters
	return reconcileNormal(ctx, clusterMgr)
}

func reconcileNormal(ctx context.Context, clusterMgr baremetal.ClusterManagerInterface) (ctrl.Result, error) {
	// If the BareMetalCluster doesn't have finalizer, add it.
	clusterMgr.SetFinalizer()

	//Create the baremetal cluster (no-op)
	if err := clusterMgr.Create(ctx); err != nil {
		return ctrl.Result{}, err
	}

	// Set APIEndpoints so the Cluster API Cluster Controller can pull it
	if err := clusterMgr.UpdateClusterStatus(); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to get ip for the API endpoint")
	}

	return ctrl.Result{}, nil
}

func reconcileDelete(ctx context.Context,
	clusterMgr baremetal.ClusterManagerInterface) (ctrl.Result, error) {

	// Verify that no baremetalmachine depend on the baremetalcluster
	descendants, err := clusterMgr.CountDescendants(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}
	if descendants > 0 {
		// Requeue so we can check the next time to see if there are still any
		// descendants left.
		return ctrl.Result{Requeue: true, RequeueAfter: requeueAfter}, nil
	}

	if err := clusterMgr.Delete(); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to delete BareMetalCluster")
	}

	// Cluster is deleted so remove the finalizer.
	clusterMgr.UnsetFinalizer()

	return ctrl.Result{}, nil
}

// SetupWithManager will add watches for this controller
func (r *BareMetalClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&capbm.BareMetalCluster{}).
		Watches(
			&source.Kind{Type: &capi.Cluster{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: util.ClusterToInfrastructureMapFunc(
					capbm.GroupVersion.WithKind("BareMetalCluster"),
				),
			},
		).
		Complete(r)
}

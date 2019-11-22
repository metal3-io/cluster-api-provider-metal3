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
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
			er := errors.New("Unable to get owner cluster")
			setErrorBMCluster(baremetalCluster, er, capierrors.InvalidConfigurationClusterError)
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
			clusterLog.Info("failed to Patch baremetalCluster")
		}
	}()
	// clear an error if one was previously set
	clearErrorBMCluster(baremetalCluster)
	// Fetch the Cluster.
	cluster, err := util.GetOwnerCluster(ctx, r.Client, baremetalCluster.ObjectMeta)
	if err != nil {
		er := errors.New("Unable to get owner cluster")
		setErrorBMCluster(baremetalCluster, er, capierrors.InvalidConfigurationClusterError)
		return ctrl.Result{}, err
	}
	if cluster == nil {
		clusterLog.Info("Waiting for Cluster Controller to set OwnerRef on BareMetalCluster")
		return ctrl.Result{}, nil
	}

	clusterLog = clusterLog.WithValues("cluster", cluster.Name)

	// Create a helper for managing a baremetal cluster.
	clusterMgr, err := r.ManagerFactory.NewClusterManager(cluster, baremetalCluster, clusterLog)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to create helper for managing the clusterMgr")
	}

	clusterMgr.GetLog().Info("Reconciling BaremetalCluster")

	// Handle deleted clusters
	if !baremetalCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, clusterMgr)
	}

	// Handle non-deleted clusters
	return reconcileNormal(ctx, clusterMgr)
}

func reconcileNormal(ctx context.Context, clusterMgr baremetal.ClusterManagerInterface) (ctrl.Result, error) {
	// If the BareMetalCluster doesn't have finalizer, add it.
	if !util.Contains(clusterMgr.GetBareMetalCluster().Finalizers, capbm.ClusterFinalizer) {
		clusterMgr.GetBareMetalCluster().Finalizers = append(clusterMgr.GetBareMetalCluster().Finalizers, capbm.ClusterFinalizer)
	}

	//Create the baremetal cluster (no-op)
	if err := clusterMgr.Create(ctx); err != nil {
		er := errors.New("failed to create the cluster")
		setErrorBMCluster(clusterMgr.GetBareMetalCluster(), er, capierrors.InvalidConfigurationClusterError)
		return ctrl.Result{}, err
	}

	// Set APIEndpoints so the Cluster API Cluster Controller can pull it
	if err := clusterMgr.UpdateClusterStatus(); err != nil {
		er := errors.New("failed to get ip for the API endpoint")
		setErrorBMCluster(clusterMgr.GetBareMetalCluster(), er, capierrors.InvalidConfigurationClusterError)

		return ctrl.Result{}, errors.Wrap(err, "failed to get ip for the API endpoint")
	}

	return ctrl.Result{}, nil
}

func (r *BareMetalClusterReconciler) reconcileDelete(ctx context.Context,
	clusterMgr baremetal.ClusterManagerInterface) (ctrl.Result, error) {

	// Verify that no baremetalmachine depend on the baremetalcluster
	descendants, err := r.listDescendants(ctx, clusterMgr.GetBareMetalCluster())
	if err != nil {
		clusterMgr.GetLog().Error(err, "Failed to list descendants")

		return ctrl.Result{}, err
	}

	if descendants.length() > 0 {
		clusterMgr.GetLog().Info(
			"BaremetalCluster still has descendants - need to requeue", "descendants",
			descendants.length(),
		)
		// Requeue so we can check the next time to see if there are still any
		// descendants left.
		return ctrl.Result{Requeue: true, RequeueAfter: requeueAfter}, nil
	}

	if err := clusterMgr.Delete(); err != nil {
		er := errors.New("failed to delete BareMetalCluster")
		setErrorBMCluster(clusterMgr.GetBareMetalCluster(), er, capierrors.DeleteClusterError)

		return ctrl.Result{}, errors.Wrap(err, "failed to delete BareMetalCluster")
	}

	// Cluster is deleted so remove the finalizer.
	clusterMgr.GetBareMetalCluster().Finalizers = util.Filter(
		clusterMgr.GetBareMetalCluster().Finalizers, capbm.ClusterFinalizer,
	)

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

type clusterDescendants struct {
	machines capi.MachineList
}

// length returns the number of descendants
func (c *clusterDescendants) length() int {
	return len(c.machines.Items)
}

// listDescendants returns a list of all Machines, for the cluster owning the
// BaremetalCluster.
func (r *BareMetalClusterReconciler) listDescendants(ctx context.Context,
	baremetalCluster *capbm.BareMetalCluster) (clusterDescendants, error) {

	var descendants clusterDescendants
	cluster, err := util.GetOwnerCluster(ctx, r.Client,
		baremetalCluster.ObjectMeta,
	)
	if err != nil {
		return descendants, err
	}

	listOptions := []client.ListOption{
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels(map[string]string{
			capi.MachineClusterLabelName: cluster.Name,
		}),
	}

	if r.Client.List(ctx, &descendants.machines, listOptions...) != nil {

		errMsg := fmt.Sprintf("failed to list BaremetalMachines for cluster %s/%s", cluster.Namespace, cluster.Name)
		return descendants, errors.Wrapf(err, errMsg)
	}

	return descendants, nil
}

// setError sets the ErrorMessage and ErrorReason fields on the baremetalcluster
func setErrorBMCluster(bmc *capbm.BareMetalCluster, message error, reason capierrors.ClusterStatusError) {

	bmc.Status.ErrorMessage = pointer.StringPtr(message.Error())
	bmc.Status.ErrorReason = &reason

}

// clearError removes the ErrorMessage from the baremetalcluster's Status if set.
func clearErrorBMCluster(bmc *capbm.BareMetalCluster) {

	if bmc.Status.ErrorMessage != nil || bmc.Status.ErrorReason != nil {
		bmc.Status.ErrorMessage = nil
		bmc.Status.ErrorReason = nil
	}

}

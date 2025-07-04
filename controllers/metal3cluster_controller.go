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
	"reflect"
	"time"

	"github.com/go-logr/logr"
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	"github.com/metal3-io/cluster-api-provider-metal3/baremetal"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/ptr"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/controllers/clustercache"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	v1beta1conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions"
	v1beta1patch "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	clusterControllerName = "Metal3Cluster-controller"
	requeueAfter          = time.Second * 30
)

// Metal3ClusterReconciler reconciles a Metal3Cluster object.
type Metal3ClusterReconciler struct {
	Client           client.Client
	ClusterCache     clustercache.ClusterCache
	ManagerFactory   baremetal.ManagerFactoryInterface
	Log              logr.Logger
	WatchFilterValue string
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metal3clusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metal3clusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch

// Reconcile reads that state of the cluster for a Metal3Cluster object and makes changes based on the state read
// and what is in the Metal3Cluster.Spec.
func (r *Metal3ClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, rerr error) {
	clusterLog := log.Log.WithName(clusterControllerName).WithValues("metal3-cluster", req.NamespacedName)

	// Fetch the Metal3Cluster instance
	metal3Cluster := &infrav1.Metal3Cluster{}

	if err := r.Client.Get(ctx, req.NamespacedName, metal3Cluster); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	// This is checking if default values are changed or not if the default
	// value of CloudProviderEnabled or NoCloudProvider is changed then update
	// the other value too to avoid conflicts.
	// TODO: Remove this code after v1.10 when NoCloudProvider is completely
	// removed. Ref: https://github.com/metal3-io/cluster-api-provider-metal3/issues/2255
	if metal3Cluster.Spec.CloudProviderEnabled != nil {
		if !*metal3Cluster.Spec.CloudProviderEnabled {
			metal3Cluster.Spec.NoCloudProvider = ptr.To(true)
		}
	} else if metal3Cluster.Spec.NoCloudProvider != nil {
		if *metal3Cluster.Spec.NoCloudProvider {
			metal3Cluster.Spec.CloudProviderEnabled = ptr.To(false)
		}
	}

	patchHelper, err := v1beta1patch.NewHelper(metal3Cluster, r.Client)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to init patch helper")
	}
	// Always patch metal3Cluster when exiting this function so we can persist any metal3Cluster changes.
	defer func() {
		if err := patchMetal3Cluster(ctx, patchHelper, metal3Cluster); err != nil {
			clusterLog.Error(err, "failed to Patch metal3Cluster")
			rerr = err
		}
	}()

	// Fetch the Cluster.
	cluster, err := util.GetOwnerCluster(ctx, r.Client, metal3Cluster.ObjectMeta)
	if err != nil {
		invalidConfigError := capierrors.InvalidConfigurationClusterError
		metal3Cluster.Status.FailureReason = &invalidConfigError
		metal3Cluster.Status.FailureMessage = ptr.To("Unable to get owner cluster")
		v1beta1conditions.MarkFalse(metal3Cluster, infrav1.BaremetalInfrastructureReadyCondition, infrav1.InternalFailureReason, clusterv1beta1.ConditionSeverityError, "%s", err.Error())
		return ctrl.Result{}, err
	}
	if cluster == nil {
		clusterLog.Info("Waiting for Cluster Controller to set OwnerRef on Metal3Cluster")
		return ctrl.Result{}, nil
	}

	clusterLog = clusterLog.WithValues("cluster", cluster.Name)

	// Return early if BMCluster or Cluster is paused.
	if annotations.IsPaused(cluster, metal3Cluster) {
		clusterLog.Info("reconciliation is paused for this object")
		return ctrl.Result{Requeue: true, RequeueAfter: requeueAfter}, nil
	}

	clusterLog.Info("Reconciling metal3Cluster")

	// Create a helper for managing a Metal3 cluster.
	clusterMgr, err := r.ManagerFactory.NewClusterManager(cluster, metal3Cluster, clusterLog)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to create helper for managing the clusterMgr")
	}
	if clusterMgr == nil {
		return ctrl.Result{}, nil
	}

	// Handle deleted clusters
	if !metal3Cluster.DeletionTimestamp.IsZero() {
		res, err := reconcileDelete(ctx, clusterMgr)
		// Requeue if the reconcile failed because the ClusterCache was locked for
		// the current cluster because of concurrent access.
		if errors.Is(err, clustercache.ErrClusterNotConnected) {
			clusterLog.Info("Requeuing because another worker has the lock on the ClusterCache")
			return ctrl.Result{Requeue: true}, nil
		}
		return res, err
	}

	// Handle non-deleted clusters
	res, err := reconcileNormal(ctx, clusterMgr)
	// Requeue if the reconcile failed because the ClusterCache was locked for
	// the current cluster because of concurrent access.
	if errors.Is(err, clustercache.ErrClusterNotConnected) {
		clusterLog.Info("Requeuing because another worker has the lock on the ClusterCache")
		return ctrl.Result{Requeue: true}, nil
	}
	return res, err
}

func patchMetal3Cluster(ctx context.Context, patchHelper *v1beta1patch.Helper, metal3Cluster *infrav1.Metal3Cluster, options ...v1beta1patch.Option) error {
	// Always update the readyCondition by summarizing the state of other conditions.
	v1beta1conditions.SetSummary(metal3Cluster,
		v1beta1conditions.WithConditions(
			infrav1.BaremetalInfrastructureReadyCondition,
		),
	)

	// Patch the object, ignoring conflicts on the conditions owned by this controller.
	options = append(options,
		v1beta1patch.WithOwnedConditions{Conditions: []clusterv1beta1.ConditionType{
			clusterv1beta1.ReadyCondition,
			infrav1.BaremetalInfrastructureReadyCondition,
		}},
		v1beta1patch.WithStatusObservedGeneration{},
	)
	return patchHelper.Patch(ctx, metal3Cluster, options...)
}

func reconcileNormal(ctx context.Context, clusterMgr baremetal.ClusterManagerInterface) (ctrl.Result, error) { //nolint:unparam
	// If the Metal3Cluster doesn't have finalizer, add it.
	clusterMgr.SetFinalizer()

	// Create the Metal3 cluster (no-op)
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
	// Verify that no metal3machine depend on the metal3cluster
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
		return ctrl.Result{}, errors.Wrap(err, "failed to delete Metal3Cluster")
	}

	// Cluster is deleted so remove the finalizer.
	clusterMgr.UnsetFinalizer()

	return ctrl.Result{}, nil
}

// SetupWithManager will add watches for this controller.
func (r *Metal3ClusterReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	clusterToInfraFn := util.ClusterToInfrastructureMapFunc(ctx, infrav1.GroupVersion.WithKind("Metal3Cluster"), mgr.GetClient(), &infrav1.Metal3Cluster{})
	return ctrl.NewControllerManagedBy(mgr).
		For(
			&infrav1.Metal3Cluster{},
			// Predicates can now be set on the for directly, so no need to use a generic event filter and worry about the kind
			builder.WithPredicates(
				predicate.Funcs{
					// Avoid reconciling if the event triggering the reconciliation is related to incremental status updates
					UpdateFunc: func(e event.UpdateEvent) bool {
						oldCluster, ok := e.ObjectOld.(*infrav1.Metal3Cluster)
						if !ok {
							r.Log.Error(nil, "Failed to cast old cluster to Metal3Cluster")
							return false
						}
						newCluster, ok := e.ObjectNew.(*infrav1.Metal3Cluster)
						if !ok {
							r.Log.Error(nil, "Failed to cast new cluster to Metal3Cluster")
							return false
						}
						oldClusterCopy := oldCluster.DeepCopy()
						newClusterCopy := newCluster.DeepCopy()
						oldClusterCopy.Status = infrav1.Metal3ClusterStatus{}
						newClusterCopy.Status = infrav1.Metal3ClusterStatus{}
						oldCluster.ObjectMeta.ResourceVersion = ""
						newClusterCopy.ObjectMeta.ResourceVersion = ""
						return !reflect.DeepEqual(oldClusterCopy, newClusterCopy)
					},
				},
			),
		).
		WithOptions(options).
		// Watches can be defined with predicates in the builder directly now, no need to do `Build()` and then add the watch to the returned controller: https://github.com/kubernetes-sigs/cluster-api/blob/b00bd08d02311919645a4868861d0f9ca0df35ea/util/predicates/cluster_predicates.go#L147-L164
		Watches(
			&clusterv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
				requests := clusterToInfraFn(ctx, o)
				if requests == nil {
					return nil
				}

				c := &infrav1.Metal3Cluster{}
				if err := r.Client.Get(ctx, requests[0].NamespacedName, c); err != nil {
					r.Log.V(baremetal.VerbosityLevelDebug).Error(err, "Failed to get Metal3 cluster")
					return nil
				}

				if annotations.IsExternallyManaged(c) {
					r.Log.V(baremetal.VerbosityLevelDebug).Info("Metal3Cluster is externally managed, skipping mapping.")
					return nil
				}
				return requests
			}),
			// predicates.ClusterUnpaused will handle cluster unpaused logic
			builder.WithPredicates(predicates.ClusterUnpaused(mgr.GetScheme(), ctrl.LoggerFrom(ctx))),
		).
		WithEventFilter(predicates.ResourceIsNotExternallyManaged(mgr.GetScheme(), mgr.GetLogger())).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(mgr.GetScheme(), ctrl.LoggerFrom(ctx), r.WatchFilterValue)).
		Complete(r)
}

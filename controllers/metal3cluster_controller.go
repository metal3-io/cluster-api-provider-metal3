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
	"github.com/pkg/errors"

	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	"github.com/metal3-io/cluster-api-provider-metal3/baremetal"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	clusterControllerName = "Metal3Cluster-controller"
	requeueAfter          = time.Second * 30
)

// Metal3ClusterReconciler reconciles a Metal3Cluster object.
type Metal3ClusterReconciler struct {
	Client           client.Client
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
	metal3Cluster := &capm3.Metal3Cluster{}

	if err := r.Client.Get(ctx, req.NamespacedName, metal3Cluster); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	patchHelper, err := patch.NewHelper(metal3Cluster, r.Client)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to init patch helper")
	}
	// Always patch metal3Cluster when exiting this function so we can persist any metal3Cluster changes.
	defer func() {
		if err := patchMetal3Cluster(ctx, patchHelper, metal3Cluster); err != nil {
			clusterLog.Error(err, "failed to Patch metal3Cluster")
		}
	}()

	// Fetch the Cluster.
	cluster, err := util.GetOwnerCluster(ctx, r.Client, metal3Cluster.ObjectMeta)
	if err != nil {
		invalidConfigError := capierrors.InvalidConfigurationClusterError
		metal3Cluster.Status.FailureReason = &invalidConfigError
		metal3Cluster.Status.FailureMessage = pointer.StringPtr("Unable to get owner cluster")
		conditions.MarkFalse(metal3Cluster, capm3.BaremetalInfrastructureReadyCondition, capm3.InternalFailureReason, clusterv1.ConditionSeverityError, err.Error())
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
		return reconcileDelete(ctx, clusterMgr)
	}

	// Handle non-deleted clusters
	return reconcileNormal(ctx, clusterMgr)
}

func patchMetal3Cluster(ctx context.Context, patchHelper *patch.Helper, metal3Cluster *capm3.Metal3Cluster, options ...patch.Option) error {
	// Always update the readyCondition by summarizing the state of other conditions.
	conditions.SetSummary(metal3Cluster,
		conditions.WithConditions(
			capm3.BaremetalInfrastructureReadyCondition,
		),
	)

	// Patch the object, ignoring conflicts on the conditions owned by this controller.
	options = append(options,
		patch.WithOwnedConditions{Conditions: []clusterv1.ConditionType{
			clusterv1.ReadyCondition,
			capm3.BaremetalInfrastructureReadyCondition,
		}},
		patch.WithStatusObservedGeneration{},
	)
	return patchHelper.Patch(ctx, metal3Cluster, options...)
}

func reconcileNormal(ctx context.Context, clusterMgr baremetal.ClusterManagerInterface) (ctrl.Result, error) {
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
func (r *Metal3ClusterReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	clusterToInfraFn := util.ClusterToInfrastructureMapFunc(capm3.GroupVersion.WithKind("Metal3Cluster"))
	return ctrl.NewControllerManagedBy(mgr).
		For(
			&capm3.Metal3Cluster{},
			// Predicates can now be set on the for directly, so no need to use a generic event filter and worry about the kind
			builder.WithPredicates(
				predicate.Funcs{
					// Avoid reconciling if the event triggering the reconciliation is related to incremental status updates
					UpdateFunc: func(e event.UpdateEvent) bool {
						oldCluster := e.ObjectOld.(*capm3.Metal3Cluster).DeepCopy()
						newCluster := e.ObjectNew.(*capm3.Metal3Cluster).DeepCopy()
						oldCluster.Status = capm3.Metal3ClusterStatus{}
						newCluster.Status = capm3.Metal3ClusterStatus{}
						oldCluster.ObjectMeta.ResourceVersion = ""
						newCluster.ObjectMeta.ResourceVersion = ""
						return !reflect.DeepEqual(oldCluster, newCluster)
					},
				},
			),
		).
		// Watches can be defined with predicates in the builder directly now, no need to do `Build()` and then add the watch to the returned controller: https://github.com/kubernetes-sigs/cluster-api/blob/b00bd08d02311919645a4868861d0f9ca0df35ea/util/predicates/cluster_predicates.go#L147-L164
		Watches(
			&source.Kind{Type: &clusterv1.Cluster{}},
			handler.EnqueueRequestsFromMapFunc(func(o client.Object) []reconcile.Request {
				requests := clusterToInfraFn(o)
				if requests == nil {
					return nil
				}

				c := &capm3.Metal3Cluster{}
				if err := r.Client.Get(ctx, requests[0].NamespacedName, c); err != nil {
					r.Log.V(4).Error(err, "Failed to get Metal3 cluster")
					return nil
				}

				if annotations.IsExternallyManaged(c) {
					r.Log.V(4).Info("Metal3Cluster is externally managed, skipping mapping.")
					return nil
				}
				return requests
			}),
			// predicates.ClusterUnpaused will handle cluster unpaused logic
			builder.WithPredicates(predicates.ClusterUnpaused(ctrl.LoggerFrom(ctx))),
		).
		WithEventFilter(predicates.ResourceIsNotExternallyManaged(mgr.GetLogger())).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(ctrl.LoggerFrom(ctx), r.WatchFilterValue)).
		Complete(r)
}

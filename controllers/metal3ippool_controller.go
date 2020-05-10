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
	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1alpha4"
	"github.com/metal3-io/cluster-api-provider-metal3/baremetal"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	ipPoolControllerName = "Metal3IPPool-controller"
)

// Metal3IPPoolReconciler reconciles a Metal3IPPool object
type Metal3IPPoolReconciler struct {
	Client         client.Client
	ManagerFactory baremetal.ManagerFactoryInterface
	Log            logr.Logger
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metal3ippools,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metal3ippools/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile handles Metal3Machine events
func (r *Metal3IPPoolReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, rerr error) {
	ctx := context.Background()
	metadataLog := r.Log.WithName(ipPoolControllerName).WithValues("metal3-ippool", req.NamespacedName)

	// Fetch the Metal3IPPool instance.
	capm3IPPool := &capm3.Metal3IPPool{}

	if err := r.Client.Get(ctx, req.NamespacedName, capm3IPPool); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	helper, err := patch.NewHelper(capm3IPPool, r.Client)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to init patch helper")
	}
	// Always patch capm3IPPool exiting this function so we can persist any Metal3IPPool changes.
	defer func() {
		err := helper.Patch(ctx, capm3IPPool)
		if err != nil {
			metadataLog.Info("failed to Patch capm3IPPool")
		}
	}()

	cluster := &capi.Cluster{}
	key := client.ObjectKey{
		Name:      capm3IPPool.Spec.ClusterName,
		Namespace: capm3IPPool.Namespace,
	}

	// Fetch the Cluster. Ignore an error if the deletion timestamp is set
	err = r.Client.Get(ctx, key, cluster)
	if capm3IPPool.ObjectMeta.DeletionTimestamp.IsZero() {
		if err != nil {
			metadataLog.Info("Error fetching cluster. It might not exist yet, Requeuing")
			return ctrl.Result{}, nil
		}
	}

	// Create a helper for managing the metadata object.
	ipPoolMgr, err := r.ManagerFactory.NewIPPoolManager(capm3IPPool, metadataLog)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to create helper for managing the IP pool")
	}

	if cluster != nil {
		metadataLog = metadataLog.WithValues("cluster", cluster.Name)
		if err := ipPoolMgr.SetClusterOwnerRef(cluster); err != nil {
			return ctrl.Result{}, err
		}

		// Return early if the Metadata or Cluster is paused.
		if util.IsPaused(cluster, capm3IPPool) {
			metadataLog.Info("reconciliation is paused for this object")
			return ctrl.Result{Requeue: true, RequeueAfter: requeueAfter}, nil
		}
	}

	// Handle deleted metadata
	if !capm3IPPool.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, ipPoolMgr)
	}

	// Handle non-deleted machines
	return r.reconcileNormal(ctx, ipPoolMgr)
}

func (r *Metal3IPPoolReconciler) reconcileNormal(ctx context.Context,
	ipPoolMgr baremetal.IPPoolManagerInterface,
) (ctrl.Result, error) {
	// If the Metal3IPPool doesn't have finalizer, add it.
	ipPoolMgr.SetFinalizer()

	_, err := ipPoolMgr.UpdateAddresses(ctx)
	if err != nil {
		return checkRequeueError(err, "Failed to create the missing data")
	}

	return ctrl.Result{}, nil
}

func (r *Metal3IPPoolReconciler) reconcileDelete(ctx context.Context,
	ipPoolMgr baremetal.IPPoolManagerInterface,
) (ctrl.Result, error) {

	allocationsNb, err := ipPoolMgr.UpdateAddresses(ctx)
	if err != nil {
		return checkRequeueError(err, "Failed to delete the old secrets")
	}

	if allocationsNb == 0 {
		// metal3ippool is marked for deletion and ready to be deleted,
		// so remove the finalizer.
		ipPoolMgr.UnsetFinalizer()
	}

	return ctrl.Result{}, nil
}

// SetupWithManager will add watches for this controller
func (r *Metal3IPPoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&capm3.Metal3IPPool{}).
		Watches(
			&source.Kind{Type: &capm3.Metal3IPClaim{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: handler.ToRequestsFunc(r.Metal3IPClaimToMetal3IPPool),
			},
			// Do not trigger a reconciliation on updates of the claim, as the Spec
			// fields are immutable
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc:  func(e event.UpdateEvent) bool { return false },
				CreateFunc:  func(e event.CreateEvent) bool { return true },
				DeleteFunc:  func(e event.DeleteEvent) bool { return true },
				GenericFunc: func(e event.GenericEvent) bool { return false },
			}),
		).
		Complete(r)
}

// Metal3IPClaimToMetal3IPPool will return a reconcile request for a
// Metal3DataTemplate if the event is for a
// Metal3IPClaim and that Metal3IPClaim references a Metal3DataTemplate
func (r *Metal3IPPoolReconciler) Metal3IPClaimToMetal3IPPool(obj handler.MapObject) []ctrl.Request {
	if m3ipc, ok := obj.Object.(*capm3.Metal3IPClaim); ok {
		if m3ipc.Spec.Pool.Name != "" {
			namespace := m3ipc.Spec.Pool.Namespace
			if namespace == "" {
				namespace = m3ipc.Namespace
			}
			return []ctrl.Request{
				ctrl.Request{
					NamespacedName: types.NamespacedName{
						Name:      m3ipc.Spec.Pool.Name,
						Namespace: namespace,
					},
				},
			}
		}
	}
	return []ctrl.Request{}
}

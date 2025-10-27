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

	"github.com/go-logr/logr"
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	"github.com/metal3-io/cluster-api-provider-metal3/baremetal"
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	capipamv1beta1 "sigs.k8s.io/cluster-api/api/ipam/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/clustercache"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	v1beta1patch "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

const (
	dataControllerName = "Metal3Data-controller"
)

// Metal3DataReconciler reconciles a Metal3Data object.
type Metal3DataReconciler struct {
	Client           client.Client
	ClientReader     client.Reader
	ClusterCache     clustercache.ClusterCache
	ManagerFactory   baremetal.ManagerFactoryInterface
	Log              logr.Logger
	WatchFilterValue string
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metal3datas,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metal3datas/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile handles Metal3Data events.
func (r *Metal3DataReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, rerr error) {
	metadataLog := r.Log.WithName(dataControllerName).WithValues("metal3-data", req.NamespacedName)

	// Fetch the Metal3Data instance.
	metal3Data := &infrav1.Metal3Data{}

	if err := r.Client.Get(ctx, req.NamespacedName, metal3Data); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	helper, err := v1beta1patch.NewHelper(metal3Data, r.Client)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to init patch helper")
	}
	// Always patch Metal3Data exiting this function so we can persist any changes.
	defer func() {
		// Check if the object still exists before attempting to patch
		var currentObj infrav1.Metal3Data
		if err = r.Client.Get(ctx, req.NamespacedName, &currentObj); err != nil {
			if apierrors.IsNotFound(err) {
				metadataLog.Info("Metal3Data no longer exists, skipping patch")
				return
			}
			metadataLog.Info("Failed to check if Metal3Data exists, attempting patch")
		}
		err = helper.Patch(ctx, metal3Data)
		if err != nil {
			metadataLog.Info("failed to Patch Metal3Data")
			rerr = err
		}
	}()

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, metal3Data.ObjectMeta)
	if metal3Data.ObjectMeta.DeletionTimestamp.IsZero() {
		if err != nil {
			metadataLog.Info("Metal3Data is missing cluster label or cluster does not exist")
			return ctrl.Result{}, nil
		}
		if cluster == nil {
			metadataLog.Info("This metadata is not yet associated with a cluster using the label : <name of cluster>", "label", clusterv1.ClusterNameLabel)
			return ctrl.Result{}, nil
		}
	}

	if cluster != nil {
		metadataLog = metadataLog.WithValues("cluster", cluster.Name)

		// Return early if the Metadata or Cluster is paused.
		if annotations.IsPaused(cluster, metal3Data) {
			metadataLog.Info("reconciliation is paused for this object")
			return ctrl.Result{Requeue: true, RequeueAfter: requeueAfter}, nil
		}
	}

	// Create a helper for managing the metadata object.
	metadataMgr, err := r.ManagerFactory.NewDataManager(metal3Data, metadataLog)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to create helper for managing the Metal3Data")
	}

	// Handle deletion of Metal3Data
	if !metal3Data.ObjectMeta.DeletionTimestamp.IsZero() {
		// Check if the Metal3DataClaim is gone. We cannot clean up until it is.
		err := r.Client.Get(ctx, types.NamespacedName{Name: metal3Data.Spec.Claim.Name, Namespace: metal3Data.Spec.Claim.Namespace}, &infrav1.Metal3DataClaim{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return r.reconcileDelete(ctx, metadataMgr)
			}
			return ctrl.Result{}, err
		}
		// Requeue until Metal3DataClaim is gone.
		metadataLog.Info("Metal3Data is being deleted, but the Metal3DataClaim still exists, requeuing")
		return ctrl.Result{Requeue: true, RequeueAfter: requeueAfter}, nil
	}

	// Handle non-deleted machines
	return r.reconcileNormal(ctx, metadataMgr)
}

func (r *Metal3DataReconciler) reconcileNormal(ctx context.Context,
	metadataMgr baremetal.DataManagerInterface,
) (ctrl.Result, error) {
	// If the Metal3Data doesn't have finalizer, add it.
	metadataMgr.SetFinalizer()

	err := metadataMgr.Reconcile(ctx)
	if err != nil {
		return checkReconcileError(err, "Failed to create secrets")
	}
	return ctrl.Result{}, nil
}

func (r *Metal3DataReconciler) reconcileDelete(ctx context.Context,
	metadataMgr baremetal.DataManagerInterface,
) (ctrl.Result, error) {
	err := metadataMgr.ReleaseLeases(ctx)
	if err != nil {
		return checkReconcileError(err, "Failed to release IP address leases")
	}

	metadataMgr.UnsetFinalizer()

	return ctrl.Result{}, nil
}

// SetupWithManager will add watches for this controller.
func (r *Metal3DataReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	builder := ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.Metal3Data{}).
		WithOptions(options)

	// Test for CAPI IPAddressClaim existence
	ipAddressClaimList := &capipamv1beta1.IPAddressClaimList{}
	err := r.ClientReader.List(ctx, ipAddressClaimList, client.Limit(1))
	if err != nil {
		if !meta.IsNoMatchError(err) {
			return err
		}
		ctrl.LoggerFrom(ctx).Info("IPAddressClaim CRD not found, skipping watch")
	} else {
		builder = builder.
			Watches(
				&capipamv1beta1.IPAddressClaim{},
				handler.EnqueueRequestsFromMapFunc(r.IPAddressClaimToMetal3Data),
			)
	}

	// Test for IPClaim existence
	ipClaimList := &ipamv1.IPClaimList{}
	err = r.ClientReader.List(ctx, ipClaimList, client.Limit(1))
	if err != nil {
		if !meta.IsNoMatchError(err) {
			return err
		}
		ctrl.LoggerFrom(ctx).Info("IPClaim CRD not found, skipping watch")
	} else {
		builder = builder.
			Watches(
				&ipamv1.IPClaim{},
				handler.EnqueueRequestsFromMapFunc(r.Metal3IPClaimToMetal3Data),
			)
	}

	return builder.
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(mgr.GetScheme(), ctrl.LoggerFrom(ctx), r.WatchFilterValue)).
		Complete(r)
}

// IPAddressClaimToMetal3Data will return a reconcile request for a Metal3Data if the event is for a
// CAPI IPAddressClaim and that IPAddressClaim references a Metal3Data.
func (r *Metal3DataReconciler) IPAddressClaimToMetal3Data(_ context.Context, obj client.Object) []ctrl.Request {
	requests := []ctrl.Request{}
	if ipAddressClaim, ok := obj.(*capipamv1beta1.IPAddressClaim); ok {
		for _, ownerRef := range ipAddressClaim.OwnerReferences {
			if ownerRef.Kind != "Metal3Data" {
				continue
			}
			aGV, err := schema.ParseGroupVersion(ownerRef.APIVersion)
			if err != nil {
				r.Log.Error(err, "failed to parse the API version")
				continue
			}
			if aGV.Group != infrav1.GroupVersion.Group {
				continue
			}
			requests = append(requests, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      ownerRef.Name,
					Namespace: ipAddressClaim.Namespace,
				},
			})
		}
	}
	return requests
}

// Metal3IPClaimToMetal3Data will return a reconcile request for a Metal3Data if the event is for a
// Metal3IPClaim and that Metal3IPClaim references a Metal3Data.
func (r *Metal3DataReconciler) Metal3IPClaimToMetal3Data(_ context.Context, obj client.Object) []ctrl.Request {
	requests := []ctrl.Request{}
	if ipClaim, ok := obj.(*ipamv1.IPClaim); ok {
		for _, ownerRef := range ipClaim.OwnerReferences {
			if ownerRef.Kind != "Metal3Data" {
				continue
			}
			aGV, err := schema.ParseGroupVersion(ownerRef.APIVersion)
			if err != nil {
				r.Log.Error(err, "failed to parse the API version")
				continue
			}
			if aGV.Group != infrav1.GroupVersion.Group {
				continue
			}
			requests = append(requests, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      ownerRef.Name,
					Namespace: ipClaim.Namespace,
				},
			})
		}
	}
	return requests
}

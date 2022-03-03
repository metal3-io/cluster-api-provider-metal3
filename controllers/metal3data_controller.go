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
	"fmt"

	"github.com/go-logr/logr"
	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	"github.com/metal3-io/cluster-api-provider-metal3/baremetal"
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	dataControllerName = "Metal3Data-controller"
)

// Metal3DataReconciler reconciles a Metal3Data object.
type Metal3DataReconciler struct {
	Client           client.Client
	ManagerFactory   baremetal.ManagerFactoryInterface
	Log              logr.Logger
	WatchFilterValue string
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metal3datas,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metal3datas/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile handles Metal3Machine events.
func (r *Metal3DataReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, rerr error) {
	metadataLog := r.Log.WithName(dataControllerName).WithValues("metal3-data", req.NamespacedName)

	// Fetch the Metal3Data instance.
	capm3Metadata := &capm3.Metal3Data{}

	if err := r.Client.Get(ctx, req.NamespacedName, capm3Metadata); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	helper, err := patch.NewHelper(capm3Metadata, r.Client)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to init patch helper")
	}
	// Always patch capm3Machine exiting this function so we can persist any Metal3Machine changes.
	defer func() {
		err := helper.Patch(ctx, capm3Metadata)
		if err != nil {
			metadataLog.Info("failed to Patch Metal3Data")
		}
	}()

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, capm3Metadata.ObjectMeta)
	if capm3Metadata.ObjectMeta.DeletionTimestamp.IsZero() {
		if err != nil {
			metadataLog.Info("Metal3Data is missing cluster label or cluster does not exist")
			return ctrl.Result{}, nil
		}
		if cluster == nil {
			metadataLog.Info(fmt.Sprintf("This metadata is not yet associated with a cluster using the label %s: <name of cluster>", clusterv1.ClusterLabelName))
			return ctrl.Result{}, nil
		}
	}

	if cluster != nil {
		metadataLog = metadataLog.WithValues("cluster", cluster.Name)

		// Return early if the Metadata or Cluster is paused.
		if annotations.IsPaused(cluster, capm3Metadata) {
			metadataLog.Info("reconciliation is paused for this object")
			return ctrl.Result{Requeue: true, RequeueAfter: requeueAfter}, nil
		}
	}

	// Create a helper for managing the metadata object.
	metadataMgr, err := r.ManagerFactory.NewDataManager(capm3Metadata, metadataLog)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to create helper for managing the Metal3Data")
	}

	// Handle deleted metadata
	if !capm3Metadata.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, metadataMgr)
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
		return checkRequeueError(err, "Failed to create secrets")
	}
	return ctrl.Result{}, nil
}

func (r *Metal3DataReconciler) reconcileDelete(ctx context.Context,
	metadataMgr baremetal.DataManagerInterface,
) (ctrl.Result, error) {
	err := metadataMgr.ReleaseLeases(ctx)
	if err != nil {
		return checkRequeueError(err, "Failed to release IP address leases")
	}

	metadataMgr.UnsetFinalizer()

	return ctrl.Result{}, nil
}

// SetupWithManager will add watches for this controller.
func (r *Metal3DataReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&capm3.Metal3Data{}).
		Watches(
			&source.Kind{Type: &ipamv1.IPClaim{}},
			handler.EnqueueRequestsFromMapFunc(r.Metal3IPClaimToMetal3Data),
		).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(ctrl.LoggerFrom(ctx), r.WatchFilterValue)).
		Complete(r)
}

// Metal3IPClaimToMetal3Data will return a reconcile request for a Metal3Data if the event is for a
// Metal3IPClaim and that Metal3IPClaim references a Metal3Data.
func (r *Metal3DataReconciler) Metal3IPClaimToMetal3Data(obj client.Object) []ctrl.Request {
	requests := []ctrl.Request{}
	if m3dc, ok := obj.(*ipamv1.IPClaim); ok {
		for _, ownerRef := range m3dc.OwnerReferences {
			if ownerRef.Kind != "Metal3Data" {
				continue
			}
			aGV, err := schema.ParseGroupVersion(ownerRef.APIVersion)
			if err != nil {
				r.Log.Error(err, "failed to parse the API version")
				continue
			}
			if aGV.Group != capm3.GroupVersion.Group {
				continue
			}
			requests = append(requests, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      ownerRef.Name,
					Namespace: m3dc.Namespace,
				},
			})
		}
	}
	return requests
}

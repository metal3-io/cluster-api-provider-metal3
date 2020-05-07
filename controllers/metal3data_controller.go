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
	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1alpha4"
	"github.com/metal3-io/cluster-api-provider-metal3/baremetal"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	dataControllerName = "Metal3Data-controller"
)

// Metal3DataReconciler reconciles a Metal3Data object
type Metal3DataReconciler struct {
	Client         client.Client
	ManagerFactory baremetal.ManagerFactoryInterface
	Log            logr.Logger
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metal3datas,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metal3datas/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile handles Metal3Machine events
func (r *Metal3DataReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, rerr error) {
	ctx := context.Background()
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
			metadataLog.Info(fmt.Sprintf("This metadata is not yet associated with a cluster using the label %s: <name of cluster>", capi.ClusterLabelName))
			return ctrl.Result{}, nil
		}
	}

	if cluster != nil {
		metadataLog = metadataLog.WithValues("cluster", cluster.Name)

		// Return early if the Metadata or Cluster is paused.
		if util.IsPaused(cluster, capm3Metadata) {
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

// SetupWithManager will add watches for this controller
func (r *Metal3DataReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&capm3.Metal3Data{}).
		Complete(r)
}

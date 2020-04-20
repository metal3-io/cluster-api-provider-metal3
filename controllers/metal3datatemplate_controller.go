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
	dataTemplateControllerName = "Metal3DataTemplate-controller"
)

// Metal3DataTemplateReconciler reconciles a Metal3DataTemplate object
type Metal3DataTemplateReconciler struct {
	Client         client.Client
	ManagerFactory baremetal.ManagerFactoryInterface
	Log            logr.Logger
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metal3datatemplates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metal3datatemplates/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile handles Metal3Machine events
func (r *Metal3DataTemplateReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, rerr error) {
	ctx := context.Background()
	metadataLog := r.Log.WithName(dataTemplateControllerName).WithValues("metal3-datatemplate", req.NamespacedName)

	// Fetch the Metal3DataTemplate instance.
	capm3Metadata := &capm3.Metal3DataTemplate{}

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
			metadataLog.Info("failed to Patch capm3Metadata")
		}
	}()

	// Fetch the Cluster. Ignore an error if the deletion timestamp is set
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, capm3Metadata.ObjectMeta)
	if capm3Metadata.ObjectMeta.DeletionTimestamp.IsZero() {
		if err != nil {
			metadataLog.Info("Metal3DataTemplate is missing cluster label or cluster does not exist, Requeuing")
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
	metadataMgr, err := r.ManagerFactory.NewDataTemplateManager(capm3Metadata, metadataLog)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to create helper for managing the metadata")
	}

	// Handle deleted metadata
	if !capm3Metadata.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, metadataMgr)
	}

	// Handle non-deleted machines
	return r.reconcileNormal(ctx, metadataMgr)
}

func (r *Metal3DataTemplateReconciler) reconcileNormal(ctx context.Context,
	metadataMgr baremetal.DataTemplateManagerInterface,
) (ctrl.Result, error) {
	// If the Metal3DataTemplate doesn't have finalizer, add it.
	metadataMgr.SetFinalizer()

	// If the lastUpdated is zero, then it might mean that the object was moved.
	// So we need to check if some Metal3Data exist and repopulate the status
	// based on that. This will happen only once after creation. Afterwards, the
	// lastUpdated field is set.
	err := metadataMgr.RecreateStatusConditionally(ctx)
	if err != nil {
		return checkMetadataError(err, "Failed to recreate the status")
	}

	err = metadataMgr.DeleteDatas(ctx)
	if err != nil {
		return checkMetadataError(err, "Failed to delete the old data")
	}

	err = metadataMgr.CreateDatas(ctx)
	if err != nil {
		return checkMetadataError(err, "Failed to create the missing data")
	}

	return ctrl.Result{}, nil
}

func (r *Metal3DataTemplateReconciler) reconcileDelete(ctx context.Context,
	metadataMgr baremetal.DataTemplateManagerInterface,
) (ctrl.Result, error) {

	err := metadataMgr.DeleteDatas(ctx)
	if err != nil {
		return checkMetadataError(err, "Failed to delete the old secrets")
	}

	readyForDeletion, err := metadataMgr.DeleteReady()
	if err != nil {
		return checkMetadataError(err, "Failed to prepare deletion")
	}
	if readyForDeletion {
		// metal3datatemplate is marked for deletion and ready to be deleted,
		// so remove the finalizer.
		metadataMgr.UnsetFinalizer()
	}

	return ctrl.Result{}, nil
}

// SetupWithManager will add watches for this controller
func (r *Metal3DataTemplateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&capm3.Metal3DataTemplate{}).
		Complete(r)
}

func checkMetadataError(err error, errMessage string) (ctrl.Result, error) {
	if err == nil {
		return ctrl.Result{}, nil
	}
	if requeueErr, ok := errors.Cause(err).(baremetal.HasRequeueAfterError); ok {
		return ctrl.Result{Requeue: true, RequeueAfter: requeueErr.GetRequeueAfter()}, nil
	}
	return ctrl.Result{}, errors.Wrap(err, errMessage)
}

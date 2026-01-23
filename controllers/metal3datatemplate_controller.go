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
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	"github.com/metal3-io/cluster-api-provider-metal3/baremetal"
	"github.com/metal3-io/cluster-api-provider-metal3/internal/metrics"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/controllers/clustercache"
	"sigs.k8s.io/cluster-api/util/annotations"
	v1beta1patch "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	dataTemplateControllerName = "Metal3DataTemplate-controller"
)

// Metal3DataTemplateReconciler reconciles a Metal3DataTemplate object.
type Metal3DataTemplateReconciler struct {
	Client           client.Client
	ClusterCache     clustercache.ClusterCache
	ManagerFactory   baremetal.ManagerFactoryInterface
	Log              logr.Logger
	WatchFilterValue string
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metal3datatemplates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metal3datatemplates/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metal3datas,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metal3datas/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metal3dataclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metal3dataclaims/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ipam.metal3.io,resources=ipclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ipam.metal3.io,resources=ipclaims/status,verbs=get;watch
// +kubebuilder:rbac:groups=ipam.metal3.io,resources=ipaddresses,verbs=get;list;watch
// +kubebuilder:rbac:groups=ipam.metal3.io,resources=ipaddresses/status,verbs=get
// +kubebuilder:rbac:groups=ipam.cluster.x-k8s.io,resources=ipaddressclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ipam.cluster.x-k8s.io,resources=ipaddressclaims/status,verbs=get;watch
// +kubebuilder:rbac:groups=ipam.cluster.x-k8s.io,resources=ipaddresses,verbs=get;list;watch
// +kubebuilder:rbac:groups=ipam.cluster.x-k8s.io,resources=ipaddresses/status,verbs=get
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters/status,verbs=get
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile handles Metal3DataTemplate events.
func (r *Metal3DataTemplateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, rerr error) {
	reconcileStart := time.Now()
	log := r.Log.WithName(dataTemplateControllerName).WithValues(
		baremetal.LogFieldDataTemplate, req.NamespacedName,
	)

	// Track metrics for this reconciliation
	defer func() {
		metrics.RecordMetal3DataTemplateReconcile(req.Namespace, reconcileStart, rerr)
		if rerr != nil {
			metrics.RecordReconcileError(dataTemplateControllerName, req.Namespace, false)
		}
	}()

	log.V(baremetal.VerbosityLevelTrace).Info("Reconcile: starting Metal3DataTemplate reconciliation")

	// Fetch the Metal3DataTemplate instance.
	log.V(baremetal.VerbosityLevelTrace).Info("Fetching Metal3DataTemplate")
	metal3DataTemplate := &infrav1.Metal3DataTemplate{}

	if err := r.Client.Get(ctx, req.NamespacedName, metal3DataTemplate); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(baremetal.VerbosityLevelDebug).Info("Metal3DataTemplate not found, may have been deleted")
			return ctrl.Result{}, nil
		}
		log.V(baremetal.VerbosityLevelDebug).Info("Failed to fetch Metal3DataTemplate",
			baremetal.LogFieldError, err.Error())
		return ctrl.Result{}, err
	}
	log.V(baremetal.VerbosityLevelDebug).Info("Metal3DataTemplate fetched successfully",
		"generation", metal3DataTemplate.Generation,
		"clusterName", metal3DataTemplate.Spec.ClusterName)

	log.V(baremetal.VerbosityLevelTrace).Info("Creating patch helper")
	helper, err := v1beta1patch.NewHelper(metal3DataTemplate, r.Client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to init patch helper: %w", err)
	}
	// Always patch the Metal3DataTemplate exiting this function so we can persist any changes.
	defer func() {
		log.V(baremetal.VerbosityLevelTrace).Info("Patching Metal3DataTemplate on exit")
		err = helper.Patch(ctx, metal3DataTemplate)
		if err != nil {
			log.Info("failed to Patch Metal3DataTemplate")
			rerr = err
		}
	}()

	cluster := &clusterv1.Cluster{}
	key := client.ObjectKey{
		Name:      metal3DataTemplate.Spec.ClusterName,
		Namespace: metal3DataTemplate.Namespace,
	}

	// Fetch the Cluster. Ignore an error if the deletion timestamp is set
	log.V(baremetal.VerbosityLevelTrace).Info("Fetching Cluster",
		baremetal.LogFieldCluster, metal3DataTemplate.Spec.ClusterName)
	err = r.Client.Get(ctx, key, cluster)
	if metal3DataTemplate.ObjectMeta.DeletionTimestamp.IsZero() {
		if err != nil {
			log.V(baremetal.VerbosityLevelDebug).Info("Error fetching cluster, may not exist yet")
			log.Info("Error fetching cluster. It might not exist yet, Requeuing")
			return ctrl.Result{}, nil
		}
	}

	// Create a helper for managing the Metal3DataTemplate object.
	log.V(baremetal.VerbosityLevelTrace).Info("Creating DataTemplateManager")
	dataTemplateMgr, err := r.ManagerFactory.NewDataTemplateManager(metal3DataTemplate, log)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create helper for managing the Metal3DataTemplate: %w", err)
	}
	log.V(baremetal.VerbosityLevelDebug).Info("DataTemplateManager created successfully")

	if metal3DataTemplate.Spec.ClusterName != "" && cluster.Name != "" {
		log = log.WithValues(baremetal.LogFieldCluster, cluster.Name)
		log.V(baremetal.VerbosityLevelTrace).Info("Setting cluster owner reference")
		if err := dataTemplateMgr.SetClusterOwnerRef(cluster); err != nil {
			log.V(baremetal.VerbosityLevelDebug).Info("Failed to set cluster owner reference",
				baremetal.LogFieldError, err.Error())
			return ctrl.Result{}, err
		}
		// Return early if the Metal3DataTemplate or Cluster is paused.
		log.V(baremetal.VerbosityLevelTrace).Info("Checking pause status")
		if annotations.IsPaused(cluster, metal3DataTemplate) {
			log.V(baremetal.VerbosityLevelDebug).Info("Reconciliation paused")
			log.Info("reconciliation is paused for this object")
			return ctrl.Result{Requeue: true, RequeueAfter: requeueAfter}, nil
		}
	}

	// Handle deletion of Metal3DataTemplate
	if !metal3DataTemplate.ObjectMeta.DeletionTimestamp.IsZero() {
		log.V(baremetal.VerbosityLevelTrace).Info("Metal3DataTemplate has deletion timestamp, proceeding with deletion")
		return r.reconcileDelete(ctx, dataTemplateMgr, log)
	}

	// Handle non-deleted Metal3DataTemplate
	log.V(baremetal.VerbosityLevelTrace).Info("Proceeding with normal reconciliation")
	return r.reconcileNormal(ctx, dataTemplateMgr, log)
}

func (r *Metal3DataTemplateReconciler) reconcileNormal(ctx context.Context,
	dataTemplateMgr baremetal.DataTemplateManagerInterface, log logr.Logger,
) (ctrl.Result, error) {
	log.V(baremetal.VerbosityLevelTrace).Info("reconcileNormal: starting")

	// If the Metal3DataTemplate doesn't have finalizer, add it.
	log.V(baremetal.VerbosityLevelTrace).Info("Setting finalizer on Metal3DataTemplate")
	dataTemplateMgr.SetFinalizer()
	log.V(baremetal.VerbosityLevelDebug).Info("Finalizer set")

	log.V(baremetal.VerbosityLevelTrace).Info("Updating datas")
	_, _, err := dataTemplateMgr.UpdateDatas(ctx)
	if err != nil {
		log.V(baremetal.VerbosityLevelDebug).Info("Failed to update datas",
			baremetal.LogFieldError, err.Error())
		return checkReconcileError(err, "Failed to recreate the status")
	}
	log.V(baremetal.VerbosityLevelDebug).Info("Datas updated successfully")
	log.V(baremetal.VerbosityLevelTrace).Info("reconcileNormal: completed successfully")
	return ctrl.Result{}, nil
}

func (r *Metal3DataTemplateReconciler) reconcileDelete(ctx context.Context,
	dataTemplateMgr baremetal.DataTemplateManagerInterface, log logr.Logger,
) (ctrl.Result, error) {
	log.V(baremetal.VerbosityLevelTrace).Info("reconcileDelete: starting")

	log.V(baremetal.VerbosityLevelTrace).Info("Updating datas to check for remaining references")
	hasData, hasClaims, err := dataTemplateMgr.UpdateDatas(ctx)
	log.V(baremetal.VerbosityLevelDebug).Info("Datas check result",
		"hasData", hasData,
		"hasClaims", hasClaims)

	if err != nil {
		log.V(baremetal.VerbosityLevelDebug).Info("Failed to update datas",
			baremetal.LogFieldError, err.Error())
		return checkReconcileError(err, "Failed to recreate the status")
	}

	if hasClaims {
		log.V(baremetal.VerbosityLevelDebug).Info("Still has claims, waiting")
		return ctrl.Result{}, nil
	}
	if hasData {
		log.V(baremetal.VerbosityLevelDebug).Info("Still has data, requeuing")
		return ctrl.Result{Requeue: true, RequeueAfter: requeueAfter}, nil
	}

	log.V(baremetal.VerbosityLevelTrace).Info("Removing finalizer from Metal3DataTemplate")
	dataTemplateMgr.UnsetFinalizer()
	log.V(baremetal.VerbosityLevelDebug).Info("Finalizer removed")

	log.V(baremetal.VerbosityLevelTrace).Info("reconcileDelete: completed successfully")
	return ctrl.Result{}, nil
}

// SetupWithManager will add watches for this controller.
func (r *Metal3DataTemplateReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.Metal3DataTemplate{}).
		WithOptions(options).
		Watches(
			&infrav1.Metal3DataClaim{},
			handler.EnqueueRequestsFromMapFunc(r.Metal3DataClaimToMetal3DataTemplate),
		).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(mgr.GetScheme(), ctrl.LoggerFrom(ctx), r.WatchFilterValue)).
		Complete(r)
}

// Metal3DataClaimToMetal3DataTemplate will return a reconcile request for a
// Metal3DataTemplate if the event is for a
// Metal3DataClaim and that Metal3DataClaim references a Metal3DataTemplate.
func (r *Metal3DataTemplateReconciler) Metal3DataClaimToMetal3DataTemplate(_ context.Context, obj client.Object) []ctrl.Request {
	if m3dc, ok := obj.(*infrav1.Metal3DataClaim); ok {
		if m3dc.Spec.Template.Name != "" {
			namespace := m3dc.Spec.Template.Namespace
			if namespace == "" {
				namespace = m3dc.Namespace
			}
			return []ctrl.Request{
				{
					NamespacedName: types.NamespacedName{
						Name:      m3dc.Spec.Template.Name,
						Namespace: namespace,
					},
				},
			}
		}
	}
	return []ctrl.Request{}
}

// checkReconcileError checks if the error is a transient or terminal error.
// If it is transient, it returns a Result with Requeue set to true.
// Non-reconcile errors are returned as-is.
func checkReconcileError(err error, errMessage string) (ctrl.Result, error) {
	if err == nil {
		return ctrl.Result{}, nil
	}
	var reconcileError baremetal.ReconcileError
	if errors.As(err, &reconcileError) {
		if reconcileError.IsTransient() {
			return reconcile.Result{Requeue: true, RequeueAfter: reconcileError.GetRequeueAfter()}, nil
		}
		if reconcileError.IsTerminal() {
			return reconcile.Result{}, nil
		}
	}
	return ctrl.Result{}, fmt.Errorf("%s: %w", errMessage, err)
}

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
	"github.com/pkg/errors"
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
	log := r.Log.WithName(dataTemplateControllerName).WithValues("metal3-datatemplate", req.NamespacedName)

	// Fetch the Metal3DataTemplate instance.
	metal3DataTemplate := &infrav1.Metal3DataTemplate{}

	if err := r.Client.Get(ctx, req.NamespacedName, metal3DataTemplate); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	helper, err := v1beta1patch.NewHelper(metal3DataTemplate, r.Client)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to init patch helper")
	}
	// Always patch the Metal3DataTemplate exiting this function so we can persist any changes.
	defer func() {
		err := helper.Patch(ctx, metal3DataTemplate)
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
	err = r.Client.Get(ctx, key, cluster)
	if metal3DataTemplate.ObjectMeta.DeletionTimestamp.IsZero() {
		if err != nil {
			log.Info("Error fetching cluster. It might not exist yet, Requeuing")
			return ctrl.Result{}, nil
		}
	}

	// Create a helper for managing the Metal3DataTemplate object.
	dataTemplateMgr, err := r.ManagerFactory.NewDataTemplateManager(metal3DataTemplate, log)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to create helper for managing the Metal3DataTemplate")
	}

	if metal3DataTemplate.Spec.ClusterName != "" && cluster.Name != "" {
		log = log.WithValues("cluster", cluster.Name)
		if err := dataTemplateMgr.SetClusterOwnerRef(cluster); err != nil {
			return ctrl.Result{}, err
		}
		// Return early if the Metal3DataTemplate or Cluster is paused.
		if annotations.IsPaused(cluster, metal3DataTemplate) {
			log.Info("reconciliation is paused for this object")
			return ctrl.Result{Requeue: true, RequeueAfter: requeueAfter}, nil
		}
	}

	// Handle deletion of Metal3DataTemplate
	if !metal3DataTemplate.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, dataTemplateMgr)
	}

	// Handle non-deleted Metal3DataTemplate
	return r.reconcileNormal(ctx, dataTemplateMgr)
}

func (r *Metal3DataTemplateReconciler) reconcileNormal(ctx context.Context,
	dataTemplateMgr baremetal.DataTemplateManagerInterface,
) (ctrl.Result, error) {
	// If the Metal3DataTemplate doesn't have finalizer, add it.
	dataTemplateMgr.SetFinalizer()

	_, _, err := dataTemplateMgr.UpdateDatas(ctx)
	if err != nil {
		return checkReconcileError(err, "Failed to recreate the status")
	}
	return ctrl.Result{}, nil
}

func (r *Metal3DataTemplateReconciler) reconcileDelete(ctx context.Context,
	dataTemplateMgr baremetal.DataTemplateManagerInterface,
) (ctrl.Result, error) {
	hasData, hasClaims, err := dataTemplateMgr.UpdateDatas(ctx)
	if err != nil {
		return checkReconcileError(err, "Failed to recreate the status")
	}

	if hasClaims {
		return ctrl.Result{}, nil
	}
	if hasData {
		return ctrl.Result{Requeue: true, RequeueAfter: requeueAfter}, nil
	}

	dataTemplateMgr.UnsetFinalizer()

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
	return ctrl.Result{}, errors.Wrap(err, errMessage)
}

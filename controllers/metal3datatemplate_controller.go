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
	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	"github.com/metal3-io/cluster-api-provider-metal3/baremetal"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	dataTemplateControllerName = "Metal3DataTemplate-controller"
)

// Metal3DataTemplateReconciler reconciles a Metal3DataTemplate object.
type Metal3DataTemplateReconciler struct {
	Client           client.Client
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
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters/status,verbs=get
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile handles Metal3Machine events.
func (r *Metal3DataTemplateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, rerr error) {
	metadataLog := r.Log.WithName(dataTemplateControllerName).WithValues("metal3-datatemplate", req.NamespacedName)

	// Fetch the Metal3DataTemplate instance.
	capm3DataTemplate := &capm3.Metal3DataTemplate{}

	if err := r.Client.Get(ctx, req.NamespacedName, capm3DataTemplate); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	helper, err := patch.NewHelper(capm3DataTemplate, r.Client)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to init patch helper")
	}
	// Always patch capm3Machine exiting this function so we can persist any Metal3Machine changes.
	defer func() {
		err := helper.Patch(ctx, capm3DataTemplate)
		if err != nil {
			metadataLog.Info("failed to Patch capm3DataTemplate")
		}
	}()

	cluster := &clusterv1.Cluster{}
	key := client.ObjectKey{
		Name:      capm3DataTemplate.Spec.ClusterName,
		Namespace: capm3DataTemplate.Namespace,
	}

	// Fetch the Cluster. Ignore an error if the deletion timestamp is set
	err = r.Client.Get(ctx, key, cluster)
	if capm3DataTemplate.ObjectMeta.DeletionTimestamp.IsZero() {
		if err != nil {
			metadataLog.Info("Error fetching cluster. It might not exist yet, Requeuing")
			return ctrl.Result{}, nil
		}
	}

	// Create a helper for managing the metadata object.
	metadataMgr, err := r.ManagerFactory.NewDataTemplateManager(capm3DataTemplate, metadataLog)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to create helper for managing the metadata")
	}

	if capm3DataTemplate.Spec.ClusterName != "" && cluster.Name != "" {
		metadataLog = metadataLog.WithValues("cluster", cluster.Name)
		if err := metadataMgr.SetClusterOwnerRef(cluster); err != nil {
			return ctrl.Result{}, err
		}
		// Return early if the Metadata or Cluster is paused.
		if annotations.IsPaused(cluster, capm3DataTemplate) {
			metadataLog.Info("reconciliation is paused for this object")
			return ctrl.Result{Requeue: true, RequeueAfter: requeueAfter}, nil
		}
	}

	// Handle deleted metadata
	if !capm3DataTemplate.ObjectMeta.DeletionTimestamp.IsZero() {
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

	_, err := metadataMgr.UpdateDatas(ctx)
	if err != nil {
		return checkRequeueError(err, "Failed to recreate the status")
	}
	return ctrl.Result{}, nil
}

func (r *Metal3DataTemplateReconciler) reconcileDelete(ctx context.Context,
	metadataMgr baremetal.DataTemplateManagerInterface,
) (ctrl.Result, error) {
	allocationsNb, err := metadataMgr.UpdateDatas(ctx)
	if err != nil {
		return checkRequeueError(err, "Failed to recreate the status")
	}

	if allocationsNb == 0 {
		// metal3datatemplate is marked for deletion and ready to be deleted,
		// so remove the finalizer.
		metadataMgr.UnsetFinalizer()
	}

	return ctrl.Result{}, nil
}

// SetupWithManager will add watches for this controller.
func (r *Metal3DataTemplateReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&capm3.Metal3DataTemplate{}).
		Watches(
			&source.Kind{Type: &capm3.Metal3DataClaim{}},
			handler.EnqueueRequestsFromMapFunc(r.Metal3DataClaimToMetal3DataTemplate),
		).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(ctrl.LoggerFrom(ctx), r.WatchFilterValue)).
		Complete(r)
}

// Metal3DataClaimToMetal3DataTemplate will return a reconcile request for a
// Metal3DataTemplate if the event is for a
// Metal3DataClaim and that Metal3DataClaim references a Metal3DataTemplate.
func (r *Metal3DataTemplateReconciler) Metal3DataClaimToMetal3DataTemplate(obj client.Object) []ctrl.Request {
	if m3dc, ok := obj.(*capm3.Metal3DataClaim); ok {
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

func checkRequeueError(err error, errMessage string) (ctrl.Result, error) {
	if err == nil {
		return ctrl.Result{}, nil
	}
	if ok := errors.As(err, &hasRequeueAfterError); ok {
		return ctrl.Result{Requeue: true, RequeueAfter: hasRequeueAfterError.GetRequeueAfter()}, nil
	}
	return ctrl.Result{}, errors.Wrap(err, errMessage)
}

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
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta2"
	"github.com/metal3-io/cluster-api-provider-metal3/baremetal"
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
)

const (
	templateControllerName = "Metal3MachineTemplate-controller"
	clonedFromGroupKind    = clusterv1.TemplateClonedFromGroupKindAnnotation
	clonedFromName         = clusterv1.TemplateClonedFromNameAnnotation
)

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metal3machinetemplates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metal3machines,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metal3machines/status,verbs=get

// Metal3MachineTemplateReconciler reconciles a Metal3MachineTemplate object.
type Metal3MachineTemplateReconciler struct {
	Client           client.Client
	ClusterCache     clustercache.ClusterCache
	ManagerFactory   baremetal.ManagerFactoryInterface
	Log              logr.Logger
	WatchFilterValue string
}

// Reconcile handles Metal3MachineTemplate events.
func (r *Metal3MachineTemplateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, rerr error) {
	m3templateLog := r.Log.WithName(templateControllerName).WithValues(
		baremetal.LogFieldMetal3MachineTemplate, req.NamespacedName,
	)

	m3templateLog.V(baremetal.VerbosityLevelTrace).Info("starting reconciliation",
		baremetal.LogFieldController, templateControllerName,
		baremetal.LogFieldNamespace, req.Namespace,
		baremetal.LogFieldName, req.Name,
	)

	// Fetch the Metal3MachineTemplate instance.
	metal3MachineTemplate := &infrav1.Metal3MachineTemplate{}

	if err := r.Client.Get(ctx, req.NamespacedName, metal3MachineTemplate); err != nil {
		if apierrors.IsNotFound(err) {
			m3templateLog.V(baremetal.VerbosityLevelTrace).Info("Metal3MachineTemplate not found, skipping")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("unable to fetch Metal3MachineTemplate: %w", err)
	}

	m3templateLog.V(baremetal.VerbosityLevelDebug).Info("fetched Metal3MachineTemplate",
		baremetal.LogFieldName, metal3MachineTemplate.Name,
	)

	helper, err := v1beta1patch.NewHelper(metal3MachineTemplate, r.Client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to init patch helper: %w", err)
	}

	// Always patch metal3MachineTemplate exiting this function so we can persist any metal3MachineTemplate changes.
	defer func() {
		err = helper.Patch(ctx, metal3MachineTemplate)
		if err != nil {
			m3templateLog.Info("failed to patch Metal3MachineTemplate")
			rerr = err
		}
	}()

	// Fetch the Metal3MachineList
	m3machinelist := &infrav1.Metal3MachineList{}

	if err = r.Client.List(ctx, m3machinelist); err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to fetch Metal3MachineList: %w", err)
	}

	m3templateLog.V(baremetal.VerbosityLevelDebug).Info("fetched Metal3MachineList",
		"count", len(m3machinelist.Items),
	)

	// Create a helper for managing a Metal3MachineTemplate.
	templateMgr, err := r.ManagerFactory.NewMachineTemplateManager(metal3MachineTemplate, m3machinelist, m3templateLog)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create helper for managing the templateMgr: %w", err)
	}

	// Return early if the Metal3MachineTemplate is paused.
	if annotations.HasPaused(metal3MachineTemplate) {
		m3templateLog.Info("Metal3MachineTemplate is currently paused. Remove pause annotation to continue reconciliation.")
		return ctrl.Result{Requeue: true, RequeueAfter: requeueAfter}, nil
	}

	m3templateLog.V(baremetal.VerbosityLevelTrace).Info("proceeding to reconcileNormal")

	// Handle non-deleted machines
	return r.reconcileNormal(ctx, templateMgr, m3templateLog)
}

func (r *Metal3MachineTemplateReconciler) reconcileNormal(ctx context.Context,
	templateMgr baremetal.TemplateManagerInterface,
	log logr.Logger,
) (ctrl.Result, error) { //nolint:unparam
	log.V(baremetal.VerbosityLevelTrace).Info("entering reconcileNormal")

	// Find the Metal3Machines with clonedFromName annotation referencing
	// to the same Metal3MachineTemplate
	if err := templateMgr.UpdateAutomatedCleaningMode(ctx); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update automated cleaning mode: %w", err)
	}

	log.V(baremetal.VerbosityLevelTrace).Info("reconcileNormal completed successfully")

	return ctrl.Result{}, nil
}

// SetupWithManager will add watches for Metal3MachineTemplate controller.
func (r *Metal3MachineTemplateReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.Metal3MachineTemplate{}).
		WithOptions(options).
		Watches(
			&infrav1.Metal3Machine{},
			handler.EnqueueRequestsFromMapFunc(r.Metal3MachinesToMetal3MachineTemplate),
		).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(mgr.GetScheme(), ctrl.LoggerFrom(ctx), r.WatchFilterValue)).
		Complete(r)
}

// Metal3MachinesToMetal3MachineTemplate is a handler.ToRequestsFunc to be used to enqeue
// requests for reconciliation of Metal3MachineTemplates.
func (r *Metal3MachineTemplateReconciler) Metal3MachinesToMetal3MachineTemplate(_ context.Context, o client.Object) []ctrl.Request {
	result := []ctrl.Request{}
	if m3m, ok := o.(*infrav1.Metal3Machine); ok {
		if m3m.Annotations[clonedFromGroupKind] == "" && m3m.Annotations[clonedFromGroupKind] != infrav1.ClonedFromGroupKind {
			return nil
		}
		result = append(result, ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      m3m.Annotations[clonedFromName],
				Namespace: m3m.Namespace,
			},
		})
	} else {
		r.Log.Error(fmt.Errorf("expected a Metal3Machine but got a %T", o),
			"failed to get Metal3Machine for Metal3MachineTemplate",
		)
	}
	return result
}

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
	ManagerFactory   baremetal.ManagerFactoryInterface
	Log              logr.Logger
	WatchFilterValue string
}

// Reconcile handles Metal3MachineTemplate events.
func (r *Metal3MachineTemplateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, rerr error) {
	m3templateLog := r.Log.WithName(templateControllerName).WithValues("metal3-machine-template", req.NamespacedName)

	// Fetch the Metal3MachineTemplate instance.
	metal3MachineTemplate := &capm3.Metal3MachineTemplate{}

	if err := r.Client.Get(ctx, req.NamespacedName, metal3MachineTemplate); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, errors.Wrap(err, "unable to fetch Metal3MachineTemplate")
	}

	helper, err := patch.NewHelper(metal3MachineTemplate, r.Client)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to init patch helper")
	}

	// Always patch metal3MachineTemplate exiting this function so we can persist any metal3MachineTemplate changes.
	defer func() {
		err := helper.Patch(ctx, metal3MachineTemplate)
		if err != nil {
			m3templateLog.Info("failed to patch Metal3MachineTemplate")
		}
	}()

	// Fetch the Metal3MachineList
	m3machinelist := &capm3.Metal3MachineList{}

	if err := r.Client.List(ctx, m3machinelist); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "unable to fetch Metal3MachineList")
	}

	// Create a helper for managing a Metal3MachineTemplate.
	templateMgr, err := r.ManagerFactory.NewMachineTemplateManager(metal3MachineTemplate, m3machinelist, m3templateLog)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to create helper for managing the templateMgr")
	}

	// Return early if the Metal3MachineTemplate is paused.
	if annotations.HasPaused(metal3MachineTemplate) {
		m3templateLog.Info("Metal3MachineTemplate is currently paused. Remove pause annotation to continue reconciliation.")
		return ctrl.Result{Requeue: true, RequeueAfter: requeueAfter}, nil
	}

	// Handle non-deleted machines
	return r.reconcileNormal(ctx, templateMgr)
}

func (r *Metal3MachineTemplateReconciler) reconcileNormal(ctx context.Context,
	templateMgr baremetal.TemplateManagerInterface,
) (ctrl.Result, error) {
	// Find the Metal3Machines with clonedFromName annotation referencing
	// to the same Metal3MachineTemplate
	if err := templateMgr.UpdateAutomatedCleaningMode(ctx); err != nil {
		r.Log.Error(err, "failed to list Metal3Machines with clonedFromName annotation")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager will add watches for Metal3MachineTemplate controller.
func (r *Metal3MachineTemplateReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&capm3.Metal3MachineTemplate{}).
		Watches(
			&source.Kind{Type: &capm3.Metal3Machine{}},
			handler.EnqueueRequestsFromMapFunc(r.Metal3MachinesToMetal3MachineTemplate),
		).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(ctrl.LoggerFrom(ctx), r.WatchFilterValue)).
		Complete(r)
}

// Metal3MachinesToMetal3MachineTemplate is a handler.ToRequestsFunc to be used to enqeue
// requests for reconciliation of Metal3MachineTemplates.
func (r *Metal3MachineTemplateReconciler) Metal3MachinesToMetal3MachineTemplate(o client.Object) []ctrl.Request {
	result := []ctrl.Request{}
	if m3m, ok := o.(*capm3.Metal3Machine); ok {
		if m3m.Annotations[clonedFromGroupKind] == "" && m3m.Annotations[clonedFromGroupKind] != capm3.ClonedFromGroupKind {
			return nil
		}
		result = append(result, ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      m3m.Annotations[clonedFromName],
				Namespace: m3m.Namespace,
			},
		})
	} else {
		r.Log.Error(errors.Errorf("expected a Metal3Machine but got a %T", o),
			"failed to get Metal3Machine for Metal3MachineTemplate",
		)
	}
	return result
}

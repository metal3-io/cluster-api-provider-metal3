/*

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
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "github.com/metal3-io/cluster-api-provider-baremetal/api/v1alpha2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	controllerName = "baremetalcluster-controller"
)

// BareMetalClusterReconciler reconciles a BareMetalCluster object
type BareMetalClusterReconciler struct {
	client.Client
	Log logr.Logger
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=baremetalclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=baremetalclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch

func (r *BareMetalClusterReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, rerr error) {
	ctx := context.Background()
	log := r.Log.WithName(controllerName).
		WithName(fmt.Sprintf("namespace=%s", req.Namespace)).
		WithName(fmt.Sprintf("bmCluster=%s", req.Name))

	// Fetch the BareMetalCluster instance
	bmCluster := &infrav1.BareMetalCluster{}
	err := r.Get(ctx, req.NamespacedName, bmCluster)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	log = log.WithName(bmCluster.APIVersion)

	// Fetch the Cluster.
	cluster, err := util.GetOwnerCluster(ctx, r.Client, bmCluster.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if cluster == nil {
		log.Info("Cluster Controller has not yet set OwnerRef")
		return reconcile.Result{}, nil
	}

	log = log.WithName(fmt.Sprintf("cluster=%s", cluster.Name))

	// TODO(awander): Add Baremetal Cluster Handling

	// Initialize the patch helper
	patchHelper, err := patch.NewHelper(bmCluster, r)
	if err != nil {
		return ctrl.Result{}, err
	}
	// Always attempt to Patch the BareMetalCluster object and status after each reconciliation.
	defer func() {
		if err := patchHelper.Patch(ctx, bmCluster); err != nil {
			log.Error(err, "failed to patch BareMetalCluster")
			if rerr == nil {
				rerr = err
			}
		}
	}()

	// Handle deleted clusters
	if !bmCluster.DeletionTimestamp.IsZero() {
		return reconcileDelete(bmCluster)
	}

	// Handle non-deleted clusters
	return reconcileNormal(bmCluster)
}

func reconcileNormal(bmCluster *infrav1.BareMetalCluster) (ctrl.Result, error) {
	// If the BareMetalCluster doesn't have finalizer, add it.
	if !util.Contains(bmCluster.Finalizers, infrav1.ClusterFinalizer) {
		bmCluster.Finalizers = append(bmCluster.Finalizers, infrav1.ClusterFinalizer)
	}

	// TODO(awander): Add Baremetal Cluster Handling

	// Mark the bmCluster ready
	bmCluster.Status.Ready = true

	return ctrl.Result{}, nil
}

func reconcileDelete(bmCluster *infrav1.BareMetalCluster) (ctrl.Result, error) {
	// Cluster is deleted so remove the finalizer.
	bmCluster.Finalizers = util.Filter(bmCluster.Finalizers, infrav1.ClusterFinalizer)

	return ctrl.Result{}, nil
}

func (r *BareMetalClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.BareMetalCluster{}).
		Complete(r)
}

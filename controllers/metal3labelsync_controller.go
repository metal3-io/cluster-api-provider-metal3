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
	"time"

	"github.com/go-logr/logr"
	bmh "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1alpha4"
	"github.com/metal3-io/cluster-api-provider-metal3/baremetal"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/strings"
	capi "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var (
	bmhSyncInterval = 60 * time.Second
	retryInterval   = 1 * time.Second
)

const (
	labelSyncControllerName = "metal3-label-sync-controller"
)

// Metal3LabelSyncReconciler reconciles label updates to BareMetalHost objects with the corresponding K Node objects in the workload cluster.
type Metal3LabelSyncReconciler struct {
	Client           client.Client
	ManagerFactory   baremetal.ManagerFactoryInterface
	Log              logr.Logger
	CapiClientGetter baremetal.ClientGetter
}

// +kubebuilder:rbac:groups=metal3.io,resources=baremetalhosts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metal3.io,resources=baremetalhosts/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metal3machines,verbs=get;list;watch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metal3machines/status,verbs=get
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile handles label sync events
func (r *Metal3LabelSyncReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, rerr error) {
	ctx := context.Background()
	controllerLog := r.Log.WithName(labelSyncControllerName).WithValues("metal3-baremetalhost", req.NamespacedName)

	// We need to get the NodeRef from the CAPI Machine object:
	// BMH.ConsumerRef --> Metal3Machine.OwnerRef --> Machine.NodeRef

	host := &bmh.BareMetalHost{}
	if err := r.Client.Get(ctx, req.NamespacedName, host); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	helper, err := patch.NewHelper(host, r.Client)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to init patch helper")
	}
	defer func() {
		err := helper.Patch(ctx, host)
		if err != nil {
			controllerLog.Info("failed to Patch BareMetalHost")
		}
	}()

	// Fetch the Metal3Machine instance.
	if host.Spec.ConsumerRef == nil {
		// ignore BMH with no ConsumerRef
		return ctrl.Result{}, nil
	}
	if host.Spec.ConsumerRef.Kind != "Metal3Machine" &&
		host.Spec.ConsumerRef.GroupVersionKind().Group != capm3.GroupVersion.Group {
		controllerLog.Info(fmt.Sprintf("Unkown GroupVersionKind in BareMetalHost Consumer Ref %v", host.Spec.ConsumerRef.GroupVersionKind()))
		return ctrl.Result{}, nil

	}
	capm3Machine := &capm3.Metal3Machine{}
	capm3MachineKey := client.ObjectKey{
		Name:      host.Spec.ConsumerRef.Name,
		Namespace: host.Spec.ConsumerRef.Namespace,
	}
	if err := r.Client.Get(ctx, capm3MachineKey, capm3Machine); err != nil {
		if apierrors.IsNotFound(err) {
			controllerLog.Info(fmt.Sprintf("Could not find associated Metal3Machine %v for BareMetalHost %v/%v, will retry", capm3MachineKey, host.Namespace, host.Name))
			return ctrl.Result{RequeueAfter: retryInterval}, nil
		}
		return ctrl.Result{}, err
	}
	controllerLog.Info(fmt.Sprintf("Found Metal3Machine %v", capm3MachineKey))

	// Fetch the Machine.
	capiMachine, err := util.GetOwnerMachine(ctx, r.Client, capm3Machine.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "Metal3Machine's owner Machine could not be retrieved")
	}
	if capiMachine == nil {
		controllerLog.Info("Could not find Machine object, will retry")
		return ctrl.Result{RequeueAfter: retryInterval}, nil
	}
	controllerLog.Info(fmt.Sprintf("Found Machine %v/%v", capiMachine.Name, capiMachine.Namespace))

	// Fetch the Cluster
	cluster := &capi.Cluster{}
	clusterKey := client.ObjectKey{
		Name:      capiMachine.Spec.ClusterName,
		Namespace: capiMachine.Namespace,
	}
	err = r.Client.Get(ctx, clusterKey, cluster)
	if err != nil {
		controllerLog.Info("Error fetching cluster, will retry")
		return ctrl.Result{RequeueAfter: retryInterval}, err
	}
	controllerLog.Info(fmt.Sprintf("Found Cluster %v/%v", cluster.Name, cluster.Namespace))

	err = r.reconcileBMHLabels(ctx, host, capiMachine, cluster)
	if err != nil {
		controllerLog.Info(fmt.Sprintf("Error reconciling BMH labels to Node, will retry: %v", err))
		return ctrl.Result{RequeueAfter: retryInterval}, err
	}

	// Always requeue to ensure label sync runs periodically for each BMH. This is necessary to catch any label updates to the Node that are synchronized through the BMH.
	return ctrl.Result{RequeueAfter: bmhSyncInterval}, nil

}

func (r *Metal3LabelSyncReconciler) reconcileBMHLabels(ctx context.Context, host *bmh.BareMetalHost, machine *capi.Machine, cluster *capi.Cluster) error {

	prefixSet := map[string]struct{}{
		"my-prefix.metal3.io": struct{}{},
	}

	hostLabelSyncSet := buildLabelSyncSet(prefixSet, host.Labels)
	// Get the Node from the workload cluster
	corev1Remote, err := r.CapiClientGetter(ctx, r.Client, cluster)
	if err != nil {
		return errors.Wrap(err, "Error creating a remote client")
	}
	node, err := corev1Remote.Nodes().Get(ctx, machine.Status.NodeRef.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	nodeLabelSyncSet := buildLabelSyncSet(prefixSet, node.Labels)
	synchronizeLabelSyncSetsOnNode(hostLabelSyncSet, nodeLabelSyncSet, node)
	_, err = corev1Remote.Nodes().Update(ctx, node, metav1.UpdateOptions{})
	if err != nil {
		return errors.Wrap(err, "unable to update the target node")
	}
	return nil
}

func buildLabelSyncSet(prefixSet map[string]struct{}, labels map[string]string) map[string]string {
	labelSyncSet := make(map[string]string)
	for labelKey, labelVal := range labels {
		p, n := strings.SplitQualifiedName(labelKey)
		if p == "" || n == "" {
			continue
		}
		if _, ok := prefixSet[p]; !ok {
			continue
		}
		labelSyncSet[labelKey] = labelVal
	}
	return labelSyncSet
}

func synchronizeLabelSyncSetsOnNode(hostLabelSyncSet, nodeLabelSyncSet map[string]string, node *corev1.Node) {
	if node.Labels == nil {
		node.Labels = map[string]string{}
	}
	for labelKey, labelVal := range nodeLabelSyncSet {
		val, ok := hostLabelSyncSet[labelKey]
		if !ok || val != labelVal {
			delete(node.Labels, labelKey)
		}
	}
	for labelKey, labelVal := range hostLabelSyncSet {
		val, ok := nodeLabelSyncSet[labelKey]
		if !ok || val != labelVal {
			node.Labels[labelKey] = labelVal
		}
	}
}

// SetupWithManager will add watches for this controller
func (r *Metal3LabelSyncReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&bmh.BareMetalHost{}).
		Watches(
			&source.Kind{Type: &capm3.Metal3Cluster{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: handler.ToRequestsFunc(r.Metal3ClusterToBareMetalHosts),
			},
		).
		Complete(r)
}

// Metal3ClusterToBareMetalHosts is a handler.ToRequestsFunc to be used to enqeue
// requests for reconciliation of BareMetalHosts' label updates.
func (r *Metal3LabelSyncReconciler) Metal3ClusterToBareMetalHosts(o handler.MapObject) []ctrl.Request {
	result := []ctrl.Request{}
	c, ok := o.Object.(*capm3.Metal3Cluster)
	if !ok {
		r.Log.Error(errors.Errorf("expected a Metal3Cluster but got a %T", o.Object),
			"failed to get BareMetalHost for Metal3Cluster",
		)
		return nil
	}
	log := r.Log.WithValues("Metal3Cluster", c.Name, "Namespace", c.Namespace)
	cluster, err := util.GetOwnerCluster(context.TODO(), r.Client, c.ObjectMeta)
	switch {
	case apierrors.IsNotFound(err) || cluster == nil:
		return nil
	case err != nil:
		log.Error(err, "failed to get owning cluster")
		return nil
	}

	labels := map[string]string{capi.ClusterLabelName: cluster.Name}
	capiMachineList := &capi.MachineList{}
	if err := r.Client.List(context.TODO(), capiMachineList, client.InNamespace(c.Namespace), client.MatchingLabels(labels)); err != nil {
		log.Error(err, "failed to list Machines")
		return nil
	}
	for _, m := range capiMachineList.Items {
		if m.Spec.InfrastructureRef.Name == "" {
			continue
		}
		name := client.ObjectKey{Namespace: m.Namespace, Name: m.Spec.InfrastructureRef.Name}
		if m.Spec.InfrastructureRef.Namespace != "" {
			name = client.ObjectKey{Namespace: m.Spec.InfrastructureRef.Namespace, Name: m.Spec.InfrastructureRef.Name}
		}
		capm3Machine := &capm3.Metal3Machine{}
		if err := r.Client.Get(context.TODO(), name, capm3Machine); err != nil {
			log.Error(err, "failed to get Metal3Machine", "name", name)
			return nil
		}
		annotations := capm3Machine.ObjectMeta.GetAnnotations()
		if annotations == nil {
			log.Error(err, "no annotations found on Metal3Machine", "name", name)
			return nil
		}
		hostKey, ok := annotations[baremetal.HostAnnotation]
		if !ok {
			log.Error(errors.New("no BareMetalHost annotation on Metal3Machine"), "name", name)
			return nil
		}
		hostNamespace, hostName, err := cache.SplitMetaNamespaceKey(hostKey)
		if err != nil {
			log.Error(err, "could not parase host annotation", "annotation key", hostKey)
			return nil
		}
		hostObjKey := client.ObjectKey{
			Name:      hostName,
			Namespace: hostNamespace,
		}
		log.V(5).Info("found BareMetalHost", "name", hostObjKey)
		result = append(result, ctrl.Request{NamespacedName: hostObjKey})
	}
	return result
}

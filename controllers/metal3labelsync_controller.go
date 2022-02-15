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
	"regexp"
	"strings"
	"time"

	"github.com/go-logr/logr"
	bmh "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	"github.com/metal3-io/cluster-api-provider-metal3/baremetal"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	k8strings "k8s.io/utils/strings"
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

var (
	bmhSyncInterval = 60 * time.Second
)

const (
	labelSyncControllerName = "metal3-label-sync-controller"
	// PrefixAnnotationKey is prefix for annotation key.
	PrefixAnnotationKey = "metal3.io/metal3-label-sync-prefixes"
	// Metal3Machine is name of the Metal3 CRD.
	Metal3Machine = "Metal3Machine"
)

// Metal3LabelSyncReconciler reconciles label updates to BareMetalHost objects with the corresponding K Node objects in the workload cluster.
type Metal3LabelSyncReconciler struct {
	Client           client.Client
	ManagerFactory   baremetal.ManagerFactoryInterface
	Log              logr.Logger
	CapiClientGetter baremetal.ClientGetter
	WatchFilterValue string
}

// +kubebuilder:rbac:groups=metal3.io,resources=baremetalhosts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=metal3.io,resources=baremetalhosts/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metal3machines,verbs=get;list;watch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=metal3machines/status,verbs=get
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile handles label sync events.
func (r *Metal3LabelSyncReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, rerr error) {
	controllerLog := r.Log.WithName(labelSyncControllerName).WithValues("metal3-label-sync", req.NamespacedName)

	// We need to get the NodeRef from the CAPI Machine object:
	// BMH.ConsumerRef --> Metal3Machine.OwnerRef --> Machine.NodeRef

	host := &bmh.BareMetalHost{}
	if err := r.Client.Get(ctx, req.NamespacedName, host); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	if host.Annotations != nil {
		if _, ok := host.Annotations[bmh.PausedAnnotation]; ok {
			controllerLog.Info("BaremetalHost is currently paused. Remove pause to continue reconciliation.")
			return ctrl.Result{RequeueAfter: bmhSyncInterval}, nil
		}
	}

	helper, err := patch.NewHelper(host, r.Client)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to init patch helper")
	}
	defer func() {
		err := helper.Patch(ctx, host)
		if err != nil {
			controllerLog.Info("Failed to Patch BareMetalHost")
		}
	}()

	// Fetch the Metal3Machine instance.
	if host.Spec.ConsumerRef == nil {
		// ignore BMH with no ConsumerRef
		return ctrl.Result{}, nil
	}
	if host.Spec.ConsumerRef.Kind != Metal3Machine &&
		host.Spec.ConsumerRef.GroupVersionKind().Group != capm3.GroupVersion.Group {
		controllerLog.Info(fmt.Sprintf("Unknown GroupVersionKind in BareMetalHost Consumer Ref %v", host.Spec.ConsumerRef.GroupVersionKind()))
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
			return ctrl.Result{RequeueAfter: requeueAfter}, nil
		}
		return ctrl.Result{}, err
	}
	controllerLog.V(5).Info(fmt.Sprintf("Found Metal3Machine %v", capm3MachineKey))

	// Fetch the Machine.
	capiMachine, err := util.GetOwnerMachine(ctx, r.Client, capm3Machine.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "Metal3Machine's owner Machine could not be retrieved")
	}
	if capiMachine == nil {
		controllerLog.Info("Could not find Machine object, will retry")
		return ctrl.Result{RequeueAfter: requeueAfter}, nil
	}
	controllerLog.V(5).Info(fmt.Sprintf("Found Machine %v/%v", capiMachine.Name, capiMachine.Namespace))
	if capiMachine.Status.NodeRef == nil {
		controllerLog.Info("Could not find Node Ref on Machine object, will retry")
		return ctrl.Result{RequeueAfter: requeueAfter}, nil
	}

	// Fetch the Cluster
	cluster := &clusterv1.Cluster{}
	clusterKey := client.ObjectKey{
		Name:      capiMachine.Spec.ClusterName,
		Namespace: capiMachine.Namespace,
	}
	err = r.Client.Get(ctx, clusterKey, cluster)
	if err != nil {
		controllerLog.Info("Error fetching cluster, will retry")
		return ctrl.Result{RequeueAfter: requeueAfter}, err
	}
	controllerLog.V(5).Info(fmt.Sprintf("Found Cluster %v/%v", cluster.Name, cluster.Namespace))

	// Fetch the Metal3 cluster.
	metal3Cluster := &capm3.Metal3Cluster{}
	metal3ClusterName := types.NamespacedName{
		Namespace: capm3Machine.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if err := r.Client.Get(ctx, metal3ClusterName, metal3Cluster); err != nil {
		controllerLog.Info("Error fetching Metal3Cluster, will retry")
		return ctrl.Result{RequeueAfter: requeueAfter}, err
	}
	controllerLog.V(5).Info(fmt.Sprintf("Found Metal3Cluster %v/%v", metal3Cluster.Name, metal3Cluster.Namespace))

	if annotations.IsPaused(cluster, metal3Cluster) {
		controllerLog.Info("Cluster and/or Metal3Cluster are currently paused. Remove pause to continue reconciliation.")
		return ctrl.Result{RequeueAfter: bmhSyncInterval}, nil
	}

	// Get prefix set
	annotations := metal3Cluster.ObjectMeta.GetAnnotations()
	if annotations == nil {
		return ctrl.Result{}, nil
	}
	prefixStr, ok := annotations[PrefixAnnotationKey]
	if !ok {
		controllerLog.V(5).Info("No annotation for prefixes found on Metal3Cluster")
		return ctrl.Result{}, nil
	}

	prefixSet, err := parsePrefixAnnotation(prefixStr)
	if err != nil {
		return ctrl.Result{}, err
	}
	err = r.reconcileBMHLabels(ctx, host, capiMachine, cluster, prefixSet)
	if err != nil {
		controllerLog.Info(fmt.Sprintf("Error reconciling BMH labels to Node, will retry: %v", err))
		return ctrl.Result{RequeueAfter: requeueAfter}, err
	}
	controllerLog.Info("Finished synchronizing labels between BaremetalHost and Node")
	// Always requeue to ensure label sync runs periodically for each BMH. This is necessary to catch any label updates to the Node that are synchronized through the BMH.
	return ctrl.Result{RequeueAfter: bmhSyncInterval}, nil
}

func (r *Metal3LabelSyncReconciler) reconcileBMHLabels(ctx context.Context, host *bmh.BareMetalHost, machine *clusterv1.Machine, cluster *clusterv1.Cluster, prefixSet map[string]struct{}) error {
	hostLabelSyncSet := buildLabelSyncSet(prefixSet, host.Labels)
	// Get the Node from the workload cluster
	corev1Remote, err := r.CapiClientGetter(ctx, r.Client, cluster)
	if err != nil {
		return errors.Wrap(err, "error creating a remote client")
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
		p, n := k8strings.SplitQualifiedName(labelKey)
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

// SetupWithManager will add watches for this controller.
func (r *Metal3LabelSyncReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&bmh.BareMetalHost{}).
		Watches(
			&source.Kind{Type: &capm3.Metal3Cluster{}},
			handler.EnqueueRequestsFromMapFunc(r.Metal3ClusterToBareMetalHosts),
		).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(ctrl.LoggerFrom(ctx), r.WatchFilterValue)).
		Complete(r)
}

// Metal3ClusterToBareMetalHosts is a handler.ToRequestsFunc to be used to enqeue
// requests for reconciliation of BareMetalHosts' label updates.
func (r *Metal3LabelSyncReconciler) Metal3ClusterToBareMetalHosts(o client.Object) []ctrl.Request {
	result := []ctrl.Request{}
	c, ok := o.(*capm3.Metal3Cluster)
	if !ok {
		r.Log.Error(errors.Errorf("expected a Metal3Cluster but got a %T", o),
			"failed to get BareMetalHost for Metal3Cluster",
		)
		return nil
	}
	log := r.Log.WithValues("Metal3ClusterToBareMetalHosts", c.Name, "Namespace", c.Namespace)
	cluster, err := util.GetOwnerCluster(context.TODO(), r.Client, c.ObjectMeta)
	switch {
	case apierrors.IsNotFound(err) || cluster == nil:
		return nil
	case err != nil:
		log.Error(err, "failed to get owning cluster")
		return nil
	}

	labels := map[string]string{clusterv1.ClusterLabelName: cluster.Name}
	capiMachineList := &clusterv1.MachineList{}
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
			log.Error(err, "failed to get Metal3Machine")
			continue
		}
		annotations := capm3Machine.ObjectMeta.GetAnnotations()
		if annotations == nil {
			log.Error(errors.Errorf("no annotations found on Metal3Machine: %v", name), "failed to get annotations in Metal3Machine")
			continue
		}
		hostKey, ok := annotations[baremetal.HostAnnotation]
		if !ok {
			log.Error(errors.Errorf("no %v annotation on Metal3Machine: %v", baremetal.HostAnnotation, name), "failed to get BareMetalHost annotation in Metal3Machine")
			continue
		}
		hostNamespace, hostName, err := cache.SplitMetaNamespaceKey(hostKey)
		if err != nil {
			log.Error(err, "could not parse host annotation")
			continue
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

// parsePrefixAnnotation parses a string for prefixes. The string must be in the format: `prefix-1,prefix-2,...`
// and each prefix must conform to the definition of a subdomain in DNS (RFC 1123).
func parsePrefixAnnotation(prefixStr string) (map[string]struct{}, error) {
	entries := strings.Split(prefixStr, ",")
	prefixSet := make(map[string]struct{})
	for _, prefix := range entries {
		prefix = strings.TrimSpace(prefix)
		if prefix == "" {
			// ignore empty prefix string (e.g. `, ,`)
			continue
		} else if err := IsDNS1123Subdomain(prefix); err != nil {
			return nil, fmt.Errorf("invalid prefix (%v): %w", prefix, err)
		}
		prefixSet[prefix] = struct{}{}
	}
	return prefixSet, nil
}

// The following code is also used by kubectl for label and prefix validation.
// Reference: https://github.com/kubernetes/apimachinery/blob/master/pkg/util/validation/validation.go
const dns1123LabelFmt string = "[a-z0-9]([-a-z0-9]*[a-z0-9])?"
const dns1123SubdomainFmt string = dns1123LabelFmt + "(\\." + dns1123LabelFmt + ")*"
const dns1123SubdomainErrorMsg string = "a DNS-1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character"

// DNS1123SubdomainMaxLength is a subdomain's max length in DNS (RFC 1123).
const DNS1123SubdomainMaxLength int = 253

var dns1123SubdomainRegexp = regexp.MustCompile("^" + dns1123SubdomainFmt + "$")

// IsDNS1123Subdomain tests for a string that conforms to the definition of a
// subdomain in DNS (RFC 1123).
func IsDNS1123Subdomain(value string) error {
	if len(value) > DNS1123SubdomainMaxLength {
		return fmt.Errorf("%v must be no more than %d characters", value, DNS1123SubdomainMaxLength)
	}
	if !dns1123SubdomainRegexp.MatchString(value) {
		return errors.New(RegexError(dns1123SubdomainErrorMsg, dns1123SubdomainFmt, "example.com"))
	}
	return nil
}

// RegexError returns a string explanation of a regex validation failure.
func RegexError(msg string, fmt string, examples ...string) string {
	if len(examples) == 0 {
		return msg + " (regex used for validation is '" + fmt + "')"
	}
	msg += " (e.g. "
	for i := range examples {
		if i > 0 {
			msg += " or "
		}
		msg += "'" + examples[i] + "', "
	}
	msg += "regex used for validation is '" + fmt + "')"
	return msg
}

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
	"regexp"
	"strings"
	"time"

	"github.com/go-logr/logr"
	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta2"
	"github.com/metal3-io/cluster-api-provider-metal3/baremetal"
	"github.com/metal3-io/cluster-api-provider-metal3/internal/metrics"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	k8strings "k8s.io/utils/strings"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/controllers/clustercache"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	v1beta1patch "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

var (
	bmhSyncInterval = 60 * time.Second
)

const (
	labelSyncControllerName = "metal3-label-sync-controller"
	// PrefixAnnotationKey is prefix for annotation key.
	PrefixAnnotationKey = "metal3.io/metal3-label-sync-prefixes"
)

// Metal3LabelSyncReconciler reconciles label updates to BareMetalHost objects with the corresponding K Node objects in the workload cluster.
type Metal3LabelSyncReconciler struct {
	Client           client.Client
	ClusterCache     clustercache.ClusterCache
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
	reconcileStart := time.Now()
	controllerLog := r.Log.WithName(labelSyncControllerName).WithValues(
		baremetal.LogFieldBMH, req.NamespacedName,
	)
	controllerLog.V(baremetal.VerbosityLevelTrace).Info("Reconcile: starting label sync reconciliation")

	// Record metrics at the end of reconciliation
	defer func() {
		metrics.RecordMetal3LabelSyncReconcile(req.Namespace, reconcileStart, rerr)
	}()

	// We need to get the NodeRef from the CAPI Machine object:
	// BareMetalHost.ConsumerRef --> Metal3Machine.OwnerRef --> Machine.NodeRef

	controllerLog.V(baremetal.VerbosityLevelTrace).Info("Fetching BareMetalHost")
	host := &bmov1alpha1.BareMetalHost{}
	if err := r.Client.Get(ctx, req.NamespacedName, host); err != nil {
		if apierrors.IsNotFound(err) {
			controllerLog.V(baremetal.VerbosityLevelDebug).Info("BareMetalHost not found, may have been deleted")
			return ctrl.Result{}, nil
		}
		controllerLog.V(baremetal.VerbosityLevelDebug).Info("Failed to fetch BareMetalHost",
			baremetal.LogFieldError, err.Error())
		return ctrl.Result{}, err
	}
	controllerLog.V(baremetal.VerbosityLevelDebug).Info("BareMetalHost fetched",
		"provisioningState", host.Status.Provisioning.State,
		baremetal.LogFieldPoweredOn, host.Status.PoweredOn)

	if host.Annotations != nil {
		if _, ok := host.Annotations[bmov1alpha1.PausedAnnotation]; ok {
			controllerLog.Info("BaremetalHost is currently paused. Remove pause to continue reconciliation.")
			return ctrl.Result{RequeueAfter: bmhSyncInterval}, nil
		}
	}

	controllerLog.V(baremetal.VerbosityLevelTrace).Info("Creating patch helper")
	helper, err := v1beta1patch.NewHelper(host, r.Client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to init patch helper: %w", err)
	}
	defer func() {
		controllerLog.V(baremetal.VerbosityLevelTrace).Info("Patching BareMetalHost on exit")
		err = helper.Patch(ctx, host)
		if err != nil {
			controllerLog.Info("Failed to Patch BareMetalHost")
			rerr = err
		}
	}()

	// Fetch the Metal3Machine instance.
	controllerLog.V(baremetal.VerbosityLevelTrace).Info("Checking ConsumerRef")
	if host.Spec.ConsumerRef == nil {
		controllerLog.V(baremetal.VerbosityLevelDebug).Info("No ConsumerRef, skipping")
		// ignore BMH with no ConsumerRef
		return ctrl.Result{}, nil
	}
	if host.Spec.ConsumerRef.Kind != metal3MachineKind &&
		host.Spec.ConsumerRef.GroupVersionKind().Group != infrav1.GroupVersion.Group {
		controllerLog.Info("Unknown GroupVersionKind in BareMetalHost Consumer Ref", "groupversion", host.Spec.ConsumerRef.GroupVersionKind())
		return ctrl.Result{}, nil
	}

	controllerLog.V(baremetal.VerbosityLevelTrace).Info("Fetching Metal3Machine from ConsumerRef")
	capm3Machine := &infrav1.Metal3Machine{}
	capm3MachineKey := client.ObjectKey{
		Name:      host.Spec.ConsumerRef.Name,
		Namespace: host.Spec.ConsumerRef.Namespace,
	}
	if err = r.Client.Get(ctx, capm3MachineKey, capm3Machine); err != nil {
		if apierrors.IsNotFound(err) {
			controllerLog.Info("Could not find associated Metal3Machine for BareMetalHost, will retry",
				baremetal.LogFieldMetal3Machine, capm3MachineKey,
				baremetal.LogFieldHostNamespace, host.Namespace,
				baremetal.LogFieldHost, host.Name)
			return ctrl.Result{RequeueAfter: requeueAfter}, nil
		}
		return ctrl.Result{}, err
	}
	controllerLog = controllerLog.WithValues(baremetal.LogFieldMetal3Machine, capm3MachineKey.Name)
	controllerLog.V(baremetal.VerbosityLevelTrace).Info("Found Metal3Machine")

	// Fetch the Machine.
	controllerLog.V(baremetal.VerbosityLevelTrace).Info("Fetching owner Machine")
	capiMachine, err := util.GetOwnerMachine(ctx, r.Client, capm3Machine.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("metal3Machine's owner Machine could not be retrieved: %w", err)
	}
	if capiMachine == nil {
		controllerLog.Info("Could not find Machine object, will retry")
		return ctrl.Result{RequeueAfter: requeueAfter}, nil
	}
	controllerLog = controllerLog.WithValues(baremetal.LogFieldMachine, capiMachine.Name)
	controllerLog.V(baremetal.VerbosityLevelTrace).Info("Found Machine")

	if !capiMachine.Status.NodeRef.IsDefined() {
		controllerLog.Info("Could not find Node Ref on Machine object, will retry")
		return ctrl.Result{RequeueAfter: requeueAfter}, nil
	}
	controllerLog.V(baremetal.VerbosityLevelDebug).Info("NodeRef found",
		baremetal.LogFieldNode, capiMachine.Status.NodeRef.Name)

	// Fetch the Cluster
	controllerLog.V(baremetal.VerbosityLevelTrace).Info("Fetching Cluster")
	cluster := &clusterv1.Cluster{}
	clusterKey := client.ObjectKey{
		Name:      capiMachine.Spec.ClusterName,
		Namespace: capiMachine.Namespace,
	}
	err = r.Client.Get(ctx, clusterKey, cluster)
	if err != nil {
		controllerLog.V(baremetal.VerbosityLevelDebug).Info("Failed to fetch Cluster",
			baremetal.LogFieldError, err.Error())
		controllerLog.Info("Error fetching cluster, will retry")
		return ctrl.Result{RequeueAfter: requeueAfter}, err
	}
	controllerLog = controllerLog.WithValues(baremetal.LogFieldCluster, cluster.Name)
	controllerLog.V(baremetal.VerbosityLevelTrace).Info("Found Cluster")

	// Fetch the Metal3 cluster.
	controllerLog.V(baremetal.VerbosityLevelTrace).Info("Fetching Metal3Cluster")
	metal3Cluster := &infrav1.Metal3Cluster{}
	metal3ClusterName := types.NamespacedName{
		Namespace: capm3Machine.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if err = r.Client.Get(ctx, metal3ClusterName, metal3Cluster); err != nil {
		controllerLog.V(baremetal.VerbosityLevelDebug).Info("Failed to fetch Metal3Cluster",
			baremetal.LogFieldError, err.Error())
		controllerLog.Info("Error fetching Metal3Cluster, will retry")
		return ctrl.Result{RequeueAfter: requeueAfter}, err
	}
	controllerLog = controllerLog.WithValues(baremetal.LogFieldMetal3Cluster, metal3Cluster.Name)
	controllerLog.V(baremetal.VerbosityLevelTrace).Info("Found Metal3Cluster")

	controllerLog.V(baremetal.VerbosityLevelTrace).Info("Checking pause status")
	if annotations.IsPaused(cluster, metal3Cluster) {
		controllerLog.V(baremetal.VerbosityLevelDebug).Info("Cluster or Metal3Cluster is paused")
		controllerLog.Info("Cluster and/or Metal3Cluster are currently paused. Remove pause to continue reconciliation.")
		return ctrl.Result{RequeueAfter: bmhSyncInterval}, nil
	}

	// Get prefix set
	controllerLog.V(baremetal.VerbosityLevelTrace).Info("Getting prefix annotations from Metal3Cluster")
	annotations := metal3Cluster.ObjectMeta.GetAnnotations()
	if annotations == nil {
		controllerLog.V(baremetal.VerbosityLevelDebug).Info("No annotations on Metal3Cluster")
		return ctrl.Result{}, nil
	}
	prefixStr, ok := annotations[PrefixAnnotationKey]
	if !ok {
		controllerLog.V(baremetal.VerbosityLevelDebug).Info("No prefix annotation found on Metal3Cluster")
		return ctrl.Result{}, nil
	}
	controllerLog.V(baremetal.VerbosityLevelDebug).Info("Found prefix annotation",
		"prefixes", prefixStr)

	controllerLog.V(baremetal.VerbosityLevelTrace).Info("Parsing prefix annotation")
	prefixSet, err := parsePrefixAnnotation(prefixStr)
	if err != nil {
		controllerLog.V(baremetal.VerbosityLevelDebug).Info("Failed to parse prefix annotation",
			baremetal.LogFieldError, err.Error())
		return ctrl.Result{}, err
	}
	controllerLog.V(baremetal.VerbosityLevelDebug).Info("Prefix set parsed",
		baremetal.LogFieldCount, len(prefixSet))

	controllerLog.V(baremetal.VerbosityLevelTrace).Info("Reconciling BMH labels to Node")
	err = r.reconcileBMHLabels(ctx, host, capiMachine, cluster, prefixSet, controllerLog)
	if err != nil {
		controllerLog.V(baremetal.VerbosityLevelDebug).Info("Failed to reconcile labels",
			baremetal.LogFieldError, err.Error())
		controllerLog.Info(fmt.Sprintf("Error reconciling BMH labels to Node, will retry: %v", err))
		return ctrl.Result{RequeueAfter: requeueAfter}, err
	}
	controllerLog.Info("Finished synchronizing labels between BaremetalHost and Node")
	controllerLog.V(baremetal.VerbosityLevelTrace).Info("Reconcile: completed successfully")
	// Always requeue to ensure label sync runs periodically for each BareMetalHost. This is necessary to catch any label updates to the Node that are synchronized through the BareMetalHost.
	return ctrl.Result{RequeueAfter: bmhSyncInterval}, nil
}

func (r *Metal3LabelSyncReconciler) reconcileBMHLabels(ctx context.Context, host *bmov1alpha1.BareMetalHost, machine *clusterv1.Machine, cluster *clusterv1.Cluster, prefixSet map[string]struct{}, log logr.Logger) error {
	log.V(baremetal.VerbosityLevelTrace).Info("reconcileBMHLabels: starting label synchronization")

	log.V(baremetal.VerbosityLevelTrace).Info("Building host label sync set")
	hostLabelSyncSet := buildLabelSyncSet(prefixSet, host.Labels)
	log.V(baremetal.VerbosityLevelDebug).Info("Host label sync set built",
		baremetal.LogFieldCount, len(hostLabelSyncSet))

	// Get the Node from the workload cluster
	log.V(baremetal.VerbosityLevelTrace).Info("Getting remote client for workload cluster")
	corev1Remote, err := r.CapiClientGetter(ctx, r.Client, cluster)
	if err != nil {
		return fmt.Errorf("error creating a remote client: %w", err)
	}

	log.V(baremetal.VerbosityLevelTrace).Info("Fetching Node from workload cluster",
		baremetal.LogFieldNode, machine.Status.NodeRef.Name)
	node, err := corev1Remote.Nodes().Get(ctx, machine.Status.NodeRef.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	log.V(baremetal.VerbosityLevelDebug).Info("Node fetched from workload cluster",
		baremetal.LogFieldNode, node.Name)

	log.V(baremetal.VerbosityLevelTrace).Info("Building node label sync set")
	nodeLabelSyncSet := buildLabelSyncSet(prefixSet, node.Labels)
	log.V(baremetal.VerbosityLevelDebug).Info("Node label sync set built",
		baremetal.LogFieldCount, len(nodeLabelSyncSet))

	log.V(baremetal.VerbosityLevelTrace).Info("Synchronizing labels on node")
	synchronizeLabelSyncSetsOnNode(hostLabelSyncSet, nodeLabelSyncSet, node)

	log.V(baremetal.VerbosityLevelTrace).Info("Updating node with synchronized labels")
	_, err = corev1Remote.Nodes().Update(ctx, node, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("unable to update the target node: %w", err)
	}
	log.V(baremetal.VerbosityLevelDebug).Info("Node updated successfully")
	log.V(baremetal.VerbosityLevelTrace).Info("reconcileBMHLabels: completed successfully")
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
func (r *Metal3LabelSyncReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&bmov1alpha1.BareMetalHost{}).
		WithOptions(options).
		Watches(
			&infrav1.Metal3Cluster{},
			handler.EnqueueRequestsFromMapFunc(r.Metal3ClusterToBareMetalHosts),
		).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(mgr.GetScheme(), ctrl.LoggerFrom(ctx), r.WatchFilterValue)).
		Complete(r)
}

// Metal3ClusterToBareMetalHosts is a handler.ToRequestsFunc to be used to enqeue
// requests for reconciliation of BareMetalHosts' label updates.
func (r *Metal3LabelSyncReconciler) Metal3ClusterToBareMetalHosts(ctx context.Context, o client.Object) []ctrl.Request {
	result := []ctrl.Request{}
	c, ok := o.(*infrav1.Metal3Cluster)
	if !ok {
		r.Log.Error(fmt.Errorf("expected a Metal3Cluster but got a %T", o),
			"failed to get BareMetalHost for Metal3Cluster",
		)
		return nil
	}
	log := r.Log.WithValues(
		baremetal.LogFieldMetal3Cluster, c.Name,
		baremetal.LogFieldNamespace, c.Namespace,
	)
	cluster, err := util.GetOwnerCluster(ctx, r.Client, c.ObjectMeta)
	switch {
	case apierrors.IsNotFound(err) || cluster == nil:
		return nil
	case err != nil:
		log.Error(err, "failed to get owning cluster")
		return nil
	}

	labels := map[string]string{clusterv1.ClusterNameLabel: cluster.Name}
	capiMachineList := &clusterv1.MachineList{}
	if err := r.Client.List(ctx, capiMachineList, client.InNamespace(c.Namespace), client.MatchingLabels(labels)); err != nil {
		log.Error(err, "failed to list Machines")
		return nil
	}
	for _, m := range capiMachineList.Items {
		if m.Spec.InfrastructureRef.Name == "" {
			continue
		}
		name := client.ObjectKey{Namespace: m.Namespace, Name: m.Spec.InfrastructureRef.Name}
		if m.Namespace != "" {
			name = client.ObjectKey{Namespace: m.Namespace, Name: m.Spec.InfrastructureRef.Name}
		}
		capm3Machine := &infrav1.Metal3Machine{}
		if err := r.Client.Get(ctx, name, capm3Machine); err != nil {
			log.Error(err, "failed to get Metal3Machine")
			continue
		}
		annotations := capm3Machine.ObjectMeta.GetAnnotations()
		if annotations == nil {
			log.Error(fmt.Errorf("no annotations found on Metal3Machine: %v", name), "failed to get annotations in Metal3Machine")
			continue
		}
		hostKey, ok := annotations[baremetal.HostAnnotation]
		if !ok {
			log.Error(fmt.Errorf("no %v annotation on Metal3Machine: %v", baremetal.HostAnnotation, name), "failed to get BareMetalHost annotation in Metal3Machine")
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
		log.V(baremetal.VerbosityLevelTrace).Info("found BareMetalHost", "name", hostObjKey)
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
	return msg + " (e.g. '" + strings.Join(examples, "' or '") + "', regex used for validation is '" + fmt + "')"
}

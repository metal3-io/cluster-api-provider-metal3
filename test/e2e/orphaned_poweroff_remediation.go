package e2e

import (
	"context"
	"strings"
	"time"

	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// remediationPowerOffAnnotationPrefix mirrors baremetal.PowerOffAnnotationPrefix.
// It is duplicated here because test/e2e is a separate module.
const remediationPowerOffAnnotationPrefix = "reboot.metal3.io/metal3-remediation-"

// nodeHoldFinalizer is put on the workload Node, or on a terminating pod on
// it, to keep the remediation controller in PhaseRunning until the owning
// Machine is deleted.
const nodeHoldFinalizer = "e2e.metal3.io/hold-remediation"

// holdPodImage is the image of the hold pod, pinned by the digest of the
// multi-arch manifest list of registry.k8s.io/pause:3.10 so every CI node runs
// the same content. The pod is deleted right after creation, but the kubelet
// may still pull and start it.
const holdPodImage = "registry.k8s.io/pause:3.10@sha256:ee6521f290b2168b6e0935a181d4cff9be1ac3f505666ef0e3c98fae8199917a"

type OrphanedPowerOffRemediationInput struct {
	E2EConfig             *clusterctl.E2EConfig
	BootstrapClusterProxy framework.ClusterProxy
	TargetCluster         framework.ClusterProxy
	SpecName              string
	ClusterName           string
	Namespace             string
	// NumberOfWorkers is the worker replica count to restore after the test.
	NumberOfWorkers int
	// NumberOfControlplane is used to compute the expected provisioned BMH count
	// once the worker replica count has been restored.
	NumberOfControlplane int
}

/*
 * Orphaned Power Off Annotation Test
 *
 * Reproduces and guards against a BareMetalHost being left permanently powered
 * off when its Machine is deleted in the middle of reboot remediation.
 *
 * The window is between RemediationManager.SetPowerOffAnnotation and
 * RemovePowerOffAnnotation. Inside it the BMH carries
 * reboot.metal3.io/metal3-remediation-<remediation-uid>, which BMO honours by
 * holding the host powered off regardless of spec.online.
 *
 * Deleting the Machine there garbage collects the Metal3Remediation (it is
 * owner-referenced by the Machine). The remediation controller then fails every
 * reconcile at the "owner Machine not set" check, so RemovePowerOffAnnotation is
 * never called. Unless MachineManager.Delete() strips the annotation, it stays
 * on the BMH and the host can never be powered on or reused.
 *
 * This is reachable without an attacker: a MachineHealthCheck may delete a
 * Machine while its own remediation is still running.
 *
 * Holding the window open:
 * Left alone, the remediation controller finishes the Running phase as soon as
 * the workload Node is gone, and PhaseWaiting removes the power off annotation
 * straight away (before TimeoutSeconds is even looked at). That can happen while
 * the test is still waiting for VM shutdown or Machine deletion, which would let
 * the assertions below pass without the fixes. To make the interrupted state
 * deterministic the test holds the last step of PhaseRunning:
 *   - out-of-service taint mode (Kubernetes >= 1.28): a pod bound to the Node,
 *     carrying a test finalizer, is deleted up front and stays Terminating, so
 *     the controller taints the Node but never sees it drained.
 *   - Node deletion mode: a test finalizer on the Node lets the controller
 *     delete it but never observe it gone.
 * Either way the remediation stays in PhaseRunning with the power off
 * annotation and its finalizer in place. CAPI only issues the Node delete and
 * does not wait for it, so Machine deletion still proceeds and "deletion takes
 * over".
 *
 * Workflow:
 * 1. Pick a worker Metal3Machine and resolve its BMH, VM and workload Node.
 * 2. Set up the hold for the active Node handling mode.
 * 3. Create a Reboot-strategy Metal3Remediation owned by the worker Machine.
 * 4. Wait until the power off annotation is set, the VM is shutoff and the
 *    controller has tainted or deleted the Node, then assert the remediation
 *    is still Running with its finalizer and the annotation still present.
 * 5. Mark every other free BMH unhealthy so the replacement Machine can only
 *    reuse this BMH.
 * 6. Delete the owning Machine and wait for it to be gone, then release the
 *    hold.
 * 7. Assert the power off annotation was removed from the BMH.
 * 8. Assert the orphaned Metal3Remediation is really gone, not stuck in
 *    Terminating behind its finalizer.
 * 9. Causal check: assert the replacement Metal3Machine claims this BMH, the
 *    BMH is Provisioned and powered on, the VM is running and the replacement
 *    Node is Ready. Then return the other BMHs to the pool.
 * 10. Wait for the MachineDeployment to be restored.
 *
 * Step 7 (and 9) fail without the MachineManager.Delete() annotation cleanup
 * and step 8 fails without the orphaned finalizer release in the remediation
 * controller, which makes this test a proof of both bugs as well as the fixes.
 */
func OrphanedPowerOffRemediation(ctx context.Context, inputGetter func() OrphanedPowerOffRemediationInput) {
	Logf("Starting orphaned power off annotation remediation test")
	input := inputGetter()
	bootstrapClient := input.BootstrapClusterProxy.GetClient()
	vmStateIntervals := input.E2EConfig.GetIntervals(input.SpecName, "wait-vm-state")

	_, workerM3Machines := GetMetal3Machines(ctx, bootstrapClient, input.ClusterName, input.Namespace)
	Expect(workerM3Machines).ToNot(BeEmpty())

	workerM3Machine := workerM3Machines[0]
	bmhName := Metal3MachineToBmhName(workerM3Machine)
	bmhKey := client.ObjectKey{Namespace: input.Namespace, Name: bmhName}

	workerBmh := bmov1alpha1.BareMetalHost{}
	Expect(bootstrapClient.Get(ctx, bmhKey, &workerBmh)).To(Succeed())
	vmName := BmhToVMName(workerBmh)

	workerMachineName, err := Metal3MachineToMachineName(workerM3Machine)
	Expect(err).ToNot(HaveOccurred())
	workerMachine := GetMachine(ctx, bootstrapClient, client.ObjectKey{Namespace: input.Namespace, Name: workerMachineName})
	nodeName := workerMachine.Status.NodeRef.Name
	Expect(nodeName).ToNot(BeEmpty(), "worker Machine should have a nodeRef")
	nodeKey := client.ObjectKey{Name: nodeName}
	// Fall back to resolving the workload cluster from the bootstrap cluster
	// if the caller did not pass TargetCluster.
	targetCluster := input.TargetCluster
	if targetCluster == nil {
		targetCluster = input.BootstrapClusterProxy.GetWorkloadCluster(ctx, input.Namespace, input.ClusterName)
	}
	targetClient := targetCluster.GetClient()
	useOutOfServiceTaint := isOutOfServiceTaintSupported(input.E2EConfig.MustGetVariable("KUBERNETES_VERSION"))
	holdPodKey := client.ObjectKey{Namespace: metav1.NamespaceDefault, Name: "hold-remediation-" + nodeName}

	// getPowerOffAnnotations returns every remediation power off annotation
	// currently on the BMH. Empty means the host is not held off by remediation.
	// A failed read is returned as an error so that Eventually retries instead of
	// treating it as "no annotations".
	getPowerOffAnnotations := func() ([]string, error) {
		bmh := bmov1alpha1.BareMetalHost{}
		if getErr := bootstrapClient.Get(ctx, bmhKey, &bmh); getErr != nil {
			return nil, getErr
		}
		keys := []string{}
		for key := range bmh.Annotations {
			if strings.HasPrefix(key, remediationPowerOffAnnotationPrefix) {
				keys = append(keys, key)
			}
		}
		return keys, nil
	}

	m3Remediation := &infrav1.Metal3Remediation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "orphaned-poweroff-" + workerMachineName,
			Namespace: input.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "Machine",
					Name:       workerMachineName,
					UID:        workerMachine.GetUID(),
				},
			},
		},
		Spec: infrav1.Metal3RemediationSpec{
			Strategy: &infrav1.RemediationStrategy{
				Type:           "Reboot",
				RetryLimit:     1,
				TimeoutSeconds: int32(1800),
			},
		},
	}

	// Whatever happens below, do not leave the host held off or the remediation
	// wedged: the remaining specs in this suite share this cluster.
	DeferCleanup(func() {
		By("Cleaning up any leftover power off annotation and Metal3Remediation")
		forceClearPowerOffAnnotations(ctx, bootstrapClient, bmhKey)
		forceDeleteMetal3Remediation(ctx, bootstrapClient, client.ObjectKeyFromObject(m3Remediation))
		releaseNodeHold(ctx, targetClient, nodeKey)
		releasePodHold(ctx, targetClient, holdPodKey)
	})

	// The controller uses the out-of-service taint instead of Node deletion on
	// newer Kubernetes, and each path needs a different hold.
	if useOutOfServiceTaint {
		By("Creating a terminating pod on the workload Node so it never counts as drained")
		setPodHold(ctx, targetClient, holdPodKey, nodeName)
	} else {
		By("Holding the workload Node so it never disappears")
		setNodeHold(ctx, targetClient, nodeKey)
	}

	By("Creating a Metal3Remediation resource owned by the worker Machine")
	Expect(bootstrapClient.Create(ctx, m3Remediation)).To(Succeed(), "should create Metal3Remediation CR")

	By("Waiting for the power off annotation to be set on the BareMetalHost")
	Eventually(getPowerOffAnnotations, vmStateIntervals...).ShouldNot(BeEmpty(),
		"remediation controller should have set the power off annotation")

	By("Waiting for VM power off")
	waitForVmsState([]string{vmName}, shutoff, input.SpecName, vmStateIntervals...)

	// Tainting or deleting the Node is the last step of PhaseRunning. After it
	// the controller waits for the Node to be drained or gone, which the hold
	// prevents, so it cannot reach PhaseWaiting and remove the annotation.
	if useOutOfServiceTaint {
		By("Waiting for the remediation controller to add the out-of-service taint")
		waitForOutOfServiceTaint(ctx, targetClient, nodeName, oostAdded, vmStateIntervals...)
	} else {
		By("Waiting for the remediation controller to request Node deletion")
		Eventually(func(g Gomega) {
			node := corev1.Node{}
			g.Expect(targetClient.Get(ctx, nodeKey, &node)).To(Succeed())
			g.Expect(node.DeletionTimestamp).ToNot(BeNil())
		}, vmStateIntervals...).Should(Succeed(), "remediation controller should have deleted the Node")
	}

	By("Confirming the remediation is held inside the power off window")
	rem := infrav1.Metal3Remediation{}
	Expect(bootstrapClient.Get(ctx, client.ObjectKeyFromObject(m3Remediation), &rem)).To(Succeed())
	Expect(rem.Status.Phase).To(Equal(infrav1.PhaseRunning), "remediation should still be in the Running phase")
	Expect(rem.Finalizers).To(ContainElement(infrav1.RemediationFinalizer), "remediation should still hold its finalizer")
	Expect(getPowerOffAnnotations()).ToNot(BeEmpty(), "power off annotation should still be on the BareMetalHost")

	// The MachineSet replaces the deleted Machine. Take every other free host
	// out of the selectable pool so the replacement can only reuse this BMH,
	// which makes the power on check below causal for this host.
	By("Taking the other free BareMetalHosts out of the selectable pool")
	spareBmhs := markOtherFreeBmhsUnhealthy(ctx, bootstrapClient, input.Namespace, bmhName)
	DeferCleanup(func() {
		restoreBmhsHealthy(ctx, bootstrapClient, input.Namespace, spareBmhs)
	})

	By("Deleting the owning Machine mid-remediation")
	Expect(bootstrapClient.Delete(ctx, &workerMachine)).To(Succeed(), "should delete the worker Machine")

	By("Waiting for the Machine to be gone")
	Eventually(func() bool {
		m := clusterv1.Machine{}
		getErr := bootstrapClient.Get(ctx, client.ObjectKey{Namespace: input.Namespace, Name: workerMachineName}, &m)
		return apierrors.IsNotFound(getErr)
	}, input.E2EConfig.GetIntervals(input.SpecName, "wait-machine-remediation")...).Should(BeTrue())

	// Deletion has taken over, the hold is no longer needed. Without an owner
	// Machine the remediation controller cannot make progress either way.
	By("Releasing the remediation hold")
	releaseNodeHold(ctx, targetClient, nodeKey)
	releasePodHold(ctx, targetClient, holdPodKey)

	// The annotation must not outlive the Metal3Machine. This is
	// what MachineManager.Delete() is responsible for.
	By("Asserting the orphaned power off annotation was removed from the BareMetalHost")
	Eventually(getPowerOffAnnotations, 5*time.Minute, 10*time.Second).Should(BeEmpty(),
		"remediation power off annotation outlived the Machine; the BareMetalHost is held powered off forever")

	// The remediation CR holds a finalizer and the controller errors
	// out before deletion handling once its owner Machine is gone.
	By("Asserting the orphaned Metal3Remediation is not stuck in Terminating")
	Eventually(func() bool {
		rem := infrav1.Metal3Remediation{}
		getErr := bootstrapClient.Get(ctx, client.ObjectKeyFromObject(m3Remediation), &rem)
		return apierrors.IsNotFound(getErr)
	}, 5*time.Minute, 10*time.Second).Should(BeTrue(),
		"orphaned Metal3Remediation should be garbage collected, not wedged behind its finalizer")

	// Causal check: the host must actually power on and serve a Node again.
	// Deprovisioning powers the host off anyway, so the annotation being gone
	// says nothing about whether BMO can power it back on.
	By("Waiting for the replacement Machine to reuse the BareMetalHost")
	var replacementM3MName string
	Eventually(func(g Gomega) {
		bmh := bmov1alpha1.BareMetalHost{}
		g.Expect(bootstrapClient.Get(ctx, bmhKey, &bmh)).To(Succeed())
		g.Expect(bmh.Spec.ConsumerRef).ToNot(BeNil(), "BareMetalHost should be claimed by the replacement")
		g.Expect(bmh.Spec.ConsumerRef.Kind).To(Equal("Metal3Machine"))
		g.Expect(bmh.Spec.ConsumerRef.Name).ToNot(Equal(workerM3Machine.Name), "BareMetalHost is still bound to the deleted Metal3Machine")
		g.Expect(bmh.Status.Provisioning.State).To(Equal(bmov1alpha1.StateProvisioned))
		g.Expect(bmh.Status.PoweredOn).To(BeTrue(), "BareMetalHost should be powered on")
		replacementM3MName = bmh.Spec.ConsumerRef.Name
	}, input.E2EConfig.GetIntervals(input.SpecName, "wait-object-provisioned")...).Should(Succeed())

	By("Waiting for the VM to be running")
	waitForVmsState([]string{vmName}, running, input.SpecName, vmStateIntervals...)

	By("Waiting for the replacement Node to become Ready")
	replacementM3M := infrav1.Metal3Machine{}
	Expect(bootstrapClient.Get(ctx, client.ObjectKey{Namespace: input.Namespace, Name: replacementM3MName}, &replacementM3M)).To(Succeed())
	replacementMachineName, err := Metal3MachineToMachineName(replacementM3M)
	Expect(err).ToNot(HaveOccurred())
	var replacementNodeName string
	Eventually(func(g Gomega) {
		m := clusterv1.Machine{}
		g.Expect(bootstrapClient.Get(ctx, client.ObjectKey{Namespace: input.Namespace, Name: replacementMachineName}, &m)).To(Succeed())
		g.Expect(m.Status.NodeRef.Name).ToNot(BeEmpty(), "replacement Machine should have a nodeRef")
		replacementNodeName = m.Status.NodeRef.Name
	}, input.E2EConfig.GetIntervals(input.SpecName, "wait-machine-running")...).Should(Succeed())
	waitForNodeStatus(ctx, targetClient, client.ObjectKey{Name: replacementNodeName}, corev1.ConditionTrue,
		input.SpecName, vmStateIntervals...)

	By("Returning the other free BareMetalHosts to the selectable pool")
	restoreBmhsHealthy(ctx, bootstrapClient, input.Namespace, spareBmhs)

	// Wait until the deployment is restored.
	By("Wait until all metal3machines become ready")
	WaitForNumMetal3MachinesReady(ctx, WaitForNumInput{
		Client:    bootstrapClient,
		Options:   []client.ListOption{client.InNamespace(input.Namespace)},
		Replicas:  input.NumberOfWorkers + input.NumberOfControlplane,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-object-provisioned"),
	})

	By("ORPHANED POWER OFF REMEDIATION TEST PASSED!")
}

// forceClearPowerOffAnnotations strips remediation power off annotations so a
// failed run cannot leave the host wedged for subsequent specs.
func forceClearPowerOffAnnotations(ctx context.Context, cl client.Client, key client.ObjectKey) {
	bmh := bmov1alpha1.BareMetalHost{}
	if err := cl.Get(ctx, key, &bmh); err != nil {
		return
	}
	helper, err := patch.NewHelper(&bmh, cl)
	if err != nil {
		return
	}
	changed := false
	for annotation := range bmh.Annotations {
		if strings.HasPrefix(annotation, remediationPowerOffAnnotationPrefix) {
			delete(bmh.Annotations, annotation)
			changed = true
		}
	}
	if changed {
		Logf("Cleanup: removing leftover power off annotations from BMH %s", key.Name)
		_ = helper.Patch(ctx, &bmh)
	}
}

// setNodeHold adds nodeHoldFinalizer to the workload Node. The remediation
// controller can still delete the Node, but it never sees it disappear.
func setNodeHold(ctx context.Context, cl client.Client, key client.ObjectKey) {
	node := corev1.Node{}
	Expect(cl.Get(ctx, key, &node)).To(Succeed(), "should get the workload Node")
	helper, err := patch.NewHelper(&node, cl)
	Expect(err).ToNot(HaveOccurred())
	controllerutil.AddFinalizer(&node, nodeHoldFinalizer)
	Expect(helper.Patch(ctx, &node)).To(Succeed(), "should add the hold finalizer to the workload Node")
}

// releaseNodeHold removes nodeHoldFinalizer from the workload Node, if present.
// It is safe to call more than once and when the Node is already gone.
func releaseNodeHold(ctx context.Context, cl client.Client, key client.ObjectKey) {
	node := corev1.Node{}
	if err := cl.Get(ctx, key, &node); err != nil {
		return
	}
	helper, err := patch.NewHelper(&node, cl)
	if err != nil {
		return
	}
	if controllerutil.RemoveFinalizer(&node, nodeHoldFinalizer) {
		Logf("Removing hold finalizer from Node %s", key.Name)
		_ = helper.Patch(ctx, &node)
	}
}

// setPodHold creates a pod bound to the workload Node, carrying
// nodeHoldFinalizer, and deletes it right away. The pod then stays in
// Terminating, so the remediation controller never sees the Node as drained.
func setPodHold(ctx context.Context, cl client.Client, key client.ObjectKey, nodeName string) {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:       key.Name,
			Namespace:  key.Namespace,
			Finalizers: []string{nodeHoldFinalizer},
		},
		Spec: corev1.PodSpec{
			// Bypass the scheduler and bind the pod straight to the Node.
			NodeName:                      nodeName,
			TerminationGracePeriodSeconds: ptr.To(int64(0)),
			Containers: []corev1.Container{{
				Name:            "pause",
				Image:           holdPodImage,
				ImagePullPolicy: corev1.PullIfNotPresent,
			}},
		},
	}
	Expect(cl.Create(ctx, &pod)).To(Succeed(), "should create the hold pod")
	Expect(cl.Delete(ctx, &pod)).To(Succeed(), "should delete the hold pod")
	Eventually(func(g Gomega) {
		p := corev1.Pod{}
		g.Expect(cl.Get(ctx, key, &p)).To(Succeed())
		g.Expect(p.DeletionTimestamp).ToNot(BeNil())
	}, time.Minute, 2*time.Second).Should(Succeed(), "hold pod should be terminating")
}

// releasePodHold removes nodeHoldFinalizer from the hold pod so it can go away.
// It is safe to call more than once and when the pod is already gone.
func releasePodHold(ctx context.Context, cl client.Client, key client.ObjectKey) {
	pod := corev1.Pod{}
	if err := cl.Get(ctx, key, &pod); err != nil {
		return
	}
	helper, err := patch.NewHelper(&pod, cl)
	if err != nil {
		return
	}
	if controllerutil.RemoveFinalizer(&pod, nodeHoldFinalizer) {
		Logf("Removing hold finalizer from pod %s/%s", key.Namespace, key.Name)
		_ = helper.Patch(ctx, &pod)
	}
	_ = cl.Delete(ctx, &pod)
}

// markOtherFreeBmhsUnhealthy puts the unhealthy annotation on every BMH that
// is not consumed and is not keepName, so CAPM3 will not pick it. Hosts that
// already carry the annotation are left alone. It returns the names it marked.
func markOtherFreeBmhsUnhealthy(ctx context.Context, cl client.Client, namespace, keepName string) []string {
	bmhs, err := GetAllBmhs(ctx, cl, namespace)
	Expect(err).ToNot(HaveOccurred())
	marked := []string{}
	for _, bmh := range bmhs {
		if bmh.Name == keepName || bmh.Spec.ConsumerRef != nil {
			continue
		}
		if _, ok := bmh.Annotations[infrav1.UnhealthyAnnotation]; ok {
			continue
		}
		AnnotateBmh(ctx, cl, bmh, infrav1.UnhealthyAnnotation, ptr.To(""))
		marked = append(marked, bmh.Name)
	}
	return marked
}

// restoreBmhsHealthy removes the unhealthy annotation from the named BMHs.
// It is safe to call more than once.
func restoreBmhsHealthy(ctx context.Context, cl client.Client, namespace string, names []string) {
	for _, name := range names {
		bmh := bmov1alpha1.BareMetalHost{}
		if err := cl.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &bmh); err != nil {
			continue
		}
		if _, ok := bmh.Annotations[infrav1.UnhealthyAnnotation]; ok {
			AnnotateBmh(ctx, cl, bmh, infrav1.UnhealthyAnnotation, nil)
		}
	}
}

// forceDeleteMetal3Remediation drops the finalizer from a Metal3Remediation that
// the controller can no longer reconcile, then deletes it.
func forceDeleteMetal3Remediation(ctx context.Context, cl client.Client, key client.ObjectKey) {
	rem := infrav1.Metal3Remediation{}
	if err := cl.Get(ctx, key, &rem); err != nil {
		return
	}
	Logf("Cleanup: force removing leftover Metal3Remediation %s", key.Name)
	if len(rem.Finalizers) > 0 {
		helper, err := patch.NewHelper(&rem, cl)
		if err == nil {
			rem.Finalizers = nil
			_ = helper.Patch(ctx, &rem)
		}
	}
	_ = cl.Delete(ctx, &rem)
}

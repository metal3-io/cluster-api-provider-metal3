/*
Copyright 2026 The Kubernetes Authors.

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

package e2e

import (
	"context"
	"sort"

	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// failureDomainLabel is the BMH label CAPM3 uses to match hosts to a failure domain.
const failureDomainLabel = "infrastructure.cluster.x-k8s.io/failure-domain"

// failureDomainControlPlaneCount is the number of control plane machines the
// failure domain spec creates.
const failureDomainControlPlaneCount = 3

type FailureDomainInput struct {
	E2EConfig             *clusterctl.E2EConfig
	BootstrapClusterProxy framework.ClusterProxy
	SpecName              string
	ClusterName           string
	Namespace             string
}

// LabelBmhsForFailureDomain prepares exactly three selectable BMHs: one
// labeled fd-1, one labeled fd-2 and one left unlabeled. fd-1 and fd-2 are the
// declared control-plane failure domains, so KubeadmControlPlane assigns the
// third control plane machine to one of them and finds no free host there.
// The unlabeled host is the only other selectable host: the historical
// fallback would take it, but a machine whose dataTemplate comes from
// failureDomainDataTemplates has to wait instead. Any additional available
// hosts (e.g. when this spec runs in the shared "features" bucket, which
// provisions more than three BMHs) are marked unhealthy so CAPM3's host
// selection skips them and the outcome does not depend on how many BMHs the
// environment provides. Labeling must happen before the cluster is
// provisioned so host selection sees the labels.
func LabelBmhsForFailureDomain(ctx context.Context, c client.Client, namespace string) {
	DeferCleanup(func() {
		CleanupFailureDomainBmhs(ctx, c, namespace)
	})

	bmhs, err := GetAllBmhs(ctx, c, namespace)
	Expect(err).NotTo(HaveOccurred())
	available := FilterBmhsByProvisioningState(bmhs, bmov1alpha1.StateAvailable)
	Expect(len(available)).To(BeNumerically(">=", 3), "failure domain test needs at least 3 available BMHs")

	// Sort by name so the label assignment is deterministic across runs.
	sort.Slice(available, func(i, j int) bool { return available[i].Name < available[j].Name })

	UpdateBmhLabel(ctx, c, available[0], failureDomainLabel, ptr.To("fd-1"))
	UpdateBmhLabel(ctx, c, available[1], failureDomainLabel, ptr.To("fd-2"))
	// available[2] stays unlabeled on purpose.

	// Take any spare hosts out of the selectable pool so the unlabeled host
	// is the only one a fallback could ever pick.
	for i := 3; i < len(available); i++ {
		AnnotateBmh(ctx, c, available[i], infrav1.UnhealthyAnnotation, ptr.To(""))
	}
}

// CleanupFailureDomainBmhs removes the failure-domain label and the unhealthy
// annotation that LabelBmhsForFailureDomain and WaitForFailureDomainControlPlane
// added, returning every BMH to the clean, selectable state that the other
// specs in the shared "features" bucket expect. Those specs reuse the same BMHs
// in the shared namespace rather than recreating them, so this must run even
// when the spec fails.
func CleanupFailureDomainBmhs(ctx context.Context, c client.Client, namespace string) {
	bmhs, err := GetAllBmhs(ctx, c, namespace)
	Expect(err).NotTo(HaveOccurred())
	for i := range bmhs {
		bmh := bmhs[i]
		if _, ok := bmh.Labels[failureDomainLabel]; ok {
			UpdateBmhLabel(ctx, c, bmh, failureDomainLabel, nil)
		}
		if _, ok := bmh.Annotations[infrav1.UnhealthyAnnotation]; ok {
			AnnotateBmh(ctx, c, bmh, infrav1.UnhealthyAnnotation, nil)
		}
	}
}

// WaitForFailureDomainControlPlane returns the clusterctl Waiter the spec uses
// instead of the default control plane readiness wait. KubeadmControlPlane
// brings the control plane machines up one at a time, so the third machine is
// created only after the fd-1 and fd-2 hosts are taken. That machine must then
// be held back with reason WaitingForBareMetalHostInFailureDomain while the
// unlabeled host stays free, even though --enable-failure-domain-fallback is
// true in this bucket. The waiter verifies exactly that, labels the free host
// with the machine's failure domain to unblock it, and then waits for the
// control plane to become ready as the default waiter would.
func WaitForFailureDomainControlPlane(c client.Client, clusterName, namespace string, intervals []any) clusterctl.Waiter {
	return func(ctx context.Context, input clusterctl.ApplyCustomClusterTemplateAndWaitInput, result *clusterctl.ApplyCustomClusterTemplateAndWaitResult) {
		By("Waiting for a control plane Metal3Machine to be held back because its failure domain has no free host")
		spare := unlabeledSelectableBmh(ctx, c, namespace)
		var waitingFD string
		Eventually(func(g Gomega) {
			// Check the unlabeled host first so a fallback regression fails on
			// the first poll with a message that names the cause, instead of
			// timing out while looking for a waiting machine.
			bmh := &bmov1alpha1.BareMetalHost{}
			g.Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: spare.Name}, bmh)).To(Succeed())
			g.Expect(bmh.Spec.ConsumerRef).To(BeNil(),
				"unlabeled host %s was taken by the fallback although the machine's dataTemplate is mapped to its failure domain", spare.Name)

			m3ms := &infrav1.Metal3MachineList{}
			g.Expect(c.List(ctx, m3ms, client.InNamespace(namespace),
				client.MatchingLabels{clusterv1.ClusterNameLabel: clusterName})).To(Succeed())
			waitingFD = ""
			for _, m3m := range m3ms.Items {
				cond := meta.FindStatusCondition(m3m.Status.Conditions, infrav1.AssociateBareMetalHostCondition)
				if cond == nil || cond.Status != metav1.ConditionFalse ||
					cond.Reason != infrav1.WaitingForBareMetalHostInFailureDomainReason {
					continue
				}
				g.Expect(m3m.Spec.FailureDomain).To(BeElementOf("fd-1", "fd-2"),
					"waiting Metal3Machine %s should be assigned a declared failure domain", m3m.Name)
				g.Expect(m3m.Spec.DataTemplate).NotTo(BeNil())
				g.Expect(m3m.Spec.DataTemplate.Name).To(Equal(clusterName+"-controlplane-template-"+m3m.Spec.FailureDomain),
					"waiting Metal3Machine %s should already use the dataTemplate mapped to %s", m3m.Name, m3m.Spec.FailureDomain)
				waitingFD = m3m.Spec.FailureDomain
			}
			g.Expect(waitingFD).NotTo(BeEmpty(), "expected a control plane Metal3Machine with reason %s", infrav1.WaitingForBareMetalHostInFailureDomainReason)
		}, intervals...).Should(Succeed())

		Byf("Labeling host %s with %s to unblock the waiting machine", spare.Name, waitingFD)
		UpdateBmhLabel(ctx, c, spare, failureDomainLabel, ptr.To(waitingFD))

		By("Waiting for the control plane to be ready")
		framework.WaitForControlPlaneAndMachinesReady(ctx, framework.WaitForControlPlaneAndMachinesReadyInput{
			GetLister:    input.ClusterProxy.GetClient(),
			Cluster:      result.Cluster,
			ControlPlane: result.ControlPlane,
		}, input.WaitForControlPlaneIntervals...)
	}
}

// unlabeledSelectableBmh returns the single BMH that LabelBmhsForFailureDomain
// left without the failure-domain label and without the unhealthy annotation.
// It deliberately ignores the provisioning state so the host is still found,
// and its ConsumerRef can be reported, if a fallback regression consumed it.
func unlabeledSelectableBmh(ctx context.Context, c client.Client, namespace string) bmov1alpha1.BareMetalHost {
	bmhs, err := GetAllBmhs(ctx, c, namespace)
	Expect(err).NotTo(HaveOccurred())
	var candidates []bmov1alpha1.BareMetalHost
	for _, bmh := range bmhs {
		if _, labeled := bmh.Labels[failureDomainLabel]; labeled {
			continue
		}
		if _, unhealthy := bmh.Annotations[infrav1.UnhealthyAnnotation]; unhealthy {
			continue
		}
		candidates = append(candidates, bmh)
	}
	Expect(candidates).To(HaveLen(1), "expected exactly one unlabeled BMH without the unhealthy annotation")
	return candidates[0]
}

// FailureDomain verifies that declared failure domains are propagated to status
// and that every control plane machine ended up on a host labeled with its
// assigned failure domain, using the dataTemplate mapped to that domain.
func FailureDomain(ctx context.Context, inputGetter func() FailureDomainInput) {
	Logf("Starting failure domain tests")
	input := inputGetter()
	c := input.BootstrapClusterProxy.GetClient()

	By("Verifying failure domains are propagated to Metal3Cluster and Cluster status")
	Eventually(func(g Gomega) {
		m3cluster := &infrav1.Metal3Cluster{}
		g.Expect(c.Get(ctx, client.ObjectKey{Namespace: input.Namespace, Name: input.ClusterName}, m3cluster)).To(Succeed())
		g.Expect(failureDomainNames(m3cluster.Status.FailureDomains)).To(ConsistOf("fd-1", "fd-2"))

		cluster := &clusterv1.Cluster{}
		g.Expect(c.Get(ctx, client.ObjectKey{Namespace: input.Namespace, Name: input.ClusterName}, cluster)).To(Succeed())
		g.Expect(failureDomainNames(cluster.Status.FailureDomains)).To(ConsistOf("fd-1", "fd-2"))
	}, input.E2EConfig.GetIntervals(input.SpecName, "wait-cluster")...).Should(Succeed())

	By("Verifying every control plane machine is placed on a host labeled with its assigned failure domain")
	Eventually(func(g Gomega) {
		bmhs, err := GetAllBmhs(ctx, c, input.Namespace)
		g.Expect(err).NotTo(HaveOccurred())

		placed := 0
		for _, bmh := range bmhs {
			if bmh.Spec.ConsumerRef == nil {
				continue
			}
			m3m := &infrav1.Metal3Machine{}
			g.Expect(c.Get(ctx, client.ObjectKey{Namespace: input.Namespace, Name: bmh.Spec.ConsumerRef.Name}, m3m)).To(Succeed())
			if m3m.Labels[clusterv1.ClusterNameLabel] != input.ClusterName {
				continue
			}
			g.Expect(bmh.Labels).To(HaveKeyWithValue(failureDomainLabel, m3m.Spec.FailureDomain),
				"host %s consumed by %s should be labeled with the machine's failure domain", bmh.Name, m3m.Name)
			placed++
		}
		g.Expect(placed).To(Equal(failureDomainControlPlaneCount))
	}, input.E2EConfig.GetIntervals(input.SpecName, "wait-control-plane")...).Should(Succeed())

	By("Verifying each control plane Metal3Machine uses the dataTemplate mapped to its assigned failure domain")
	Eventually(func(g Gomega) {
		m3ms := &infrav1.Metal3MachineList{}
		g.Expect(c.List(ctx, m3ms, client.InNamespace(input.Namespace),
			client.MatchingLabels{clusterv1.ClusterNameLabel: input.ClusterName})).To(Succeed())

		// WorkerMachineCount is 0 for this spec, so every Metal3Machine in the
		// cluster is a control plane machine.
		checked := 0
		for _, m3m := range m3ms.Items {
			g.Expect(m3m.Spec.FailureDomain).To(BeElementOf("fd-1", "fd-2"),
				"control plane Metal3Machine %s should have a declared failure domain assigned", m3m.Name)
			g.Expect(m3m.Spec.DataTemplate).NotTo(BeNil())
			g.Expect(m3m.Spec.DataTemplate.Name).To(Equal(input.ClusterName+"-controlplane-template-"+m3m.Spec.FailureDomain),
				"Metal3Machine %s should use the dataTemplate mapped to %s", m3m.Name, m3m.Spec.FailureDomain)
			checked++
		}
		g.Expect(checked).To(Equal(failureDomainControlPlaneCount))
	}, input.E2EConfig.GetIntervals(input.SpecName, "wait-control-plane")...).Should(Succeed())

	By("FAILURE DOMAIN TESTS PASSED!")
}

func failureDomainNames(fds []clusterv1.FailureDomain) []string {
	names := make([]string, 0, len(fds))
	for _, fd := range fds {
		names = append(names, fd.Name)
	}
	return names
}

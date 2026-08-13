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
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// failureDomainLabel is the BMH label CAPM3 uses to match hosts to a failure domain.
const failureDomainLabel = "infrastructure.cluster.x-k8s.io/failure-domain"

type FailureDomainInput struct {
	E2EConfig             *clusterctl.E2EConfig
	BootstrapClusterProxy framework.ClusterProxy
	SpecName              string
	ClusterName           string
	Namespace             string
}

// LabelBmhsForFailureDomain prepares exactly three selectable BMHs so that
// fd-1, fd-2 and fd-3 each own one host. fd-1 and fd-2 are declared
// control-plane failure domains; fd-3 is not declared, so its host is only ever
// reached via the fallback path when a requested FD has no free host. Any
// additional available hosts (e.g. when this spec runs in the shared "features"
// bucket, which provisions more than three BMHs) are marked unhealthy so
// CAPM3's host selection skips them. That leaves fd-3 as the only host the
// fallback machine can land on, keeping the fallback target deterministic
// regardless of how many BMHs the environment provides. Labeling must happen
// before the cluster is provisioned so host selection sees the labels.
func LabelBmhsForFailureDomain(ctx context.Context, c client.Client, namespace string) {
	bmhs, err := GetAllBmhs(ctx, c, namespace)
	Expect(err).NotTo(HaveOccurred())
	available := FilterBmhsByProvisioningState(bmhs, bmov1alpha1.StateAvailable)
	Expect(len(available)).To(BeNumerically(">=", 3), "failure domain test needs at least 3 available BMHs")

	// Sort by name so the label assignment is deterministic across runs.
	sort.Slice(available, func(i, j int) bool { return available[i].Name < available[j].Name })

	UpdateBmhLabel(ctx, c, available[0], failureDomainLabel, ptr.To("fd-1"))
	UpdateBmhLabel(ctx, c, available[1], failureDomainLabel, ptr.To("fd-2"))
	UpdateBmhLabel(ctx, c, available[2], failureDomainLabel, ptr.To("fd-3"))

	// Take any spare hosts out of the selectable pool so the fallback machine
	// can only be placed on the fd-3 host.
	for i := 3; i < len(available); i++ {
		AnnotateBmh(ctx, c, available[i], infrav1.UnhealthyAnnotation, ptr.To(""))
	}
}

// CleanupFailureDomainBmhs removes the failure-domain label and the unhealthy
// annotation that LabelBmhsForFailureDomain added, returning every BMH to the
// clean, selectable state that the other specs in the shared "features" bucket
// expect. Those specs reuse the same BMHs in the shared namespace rather than
// recreating them, so this must run even when the spec fails.
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

// FailureDomain verifies that declared failure domains are propagated to status
// and that control plane machines are placed on hosts according to the failure
// domain labels, including the fallback when the requested FD has no free host.
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

	By("Verifying control plane machines are placed according to failure domain labels")
	// placement maps a host's failure domain label to the failure domain the
	// consuming machine requested (Metal3Machine.Spec.FailureDomain).
	Eventually(func(g Gomega) {
		bmhs, err := GetAllBmhs(ctx, c, input.Namespace)
		g.Expect(err).NotTo(HaveOccurred())

		placement := map[string]string{}
		for _, bmh := range bmhs {
			fd, ok := bmh.Labels[failureDomainLabel]
			if !ok || bmh.Spec.ConsumerRef == nil {
				continue
			}
			m3m := &infrav1.Metal3Machine{}
			g.Expect(c.Get(ctx, client.ObjectKey{Namespace: input.Namespace, Name: bmh.Spec.ConsumerRef.Name}, m3m)).To(Succeed())
			placement[fd] = m3m.Spec.FailureDomain
		}

		// Same failure domain: a labeled host is consumed by a machine that
		// requested that exact failure domain.
		g.Expect(placement).To(HaveKeyWithValue("fd-1", "fd-1"), "host labeled fd-1 should host a machine requesting fd-1")
		g.Expect(placement).To(HaveKeyWithValue("fd-2", "fd-2"), "host labeled fd-2 should host a machine requesting fd-2")

		// Fallback: the undeclared fd-3 host is consumed by the machine that
		// requested a declared FD (fd-1 or fd-2) which had no free host left.
		g.Expect(placement).To(HaveKey("fd-3"), "fd-3 host should host the fallback machine")
		g.Expect(placement["fd-3"]).To(BeElementOf("fd-1", "fd-2"), "fallback machine on fd-3 host should have requested a declared FD")
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

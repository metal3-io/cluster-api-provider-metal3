/*
Copyright 2025 The Metal3 Authors.

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

	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	labelSyncPrefixAnnotation = "metal3.io/metal3-label-sync-prefixes"
	labelSyncTestPrefix       = "e2e.metal3.io"
	labelSyncTestKey          = labelSyncTestPrefix + "/label-sync-test"
	labelSyncTestValue        = "exist"
)

// LabelSyncInput bundles the proxies and names needed.
type LabelSyncInput struct {
	E2EConfig             *clusterctl.E2EConfig
	BootstrapClusterProxy framework.ClusterProxy
	TargetClusterProxy    framework.ClusterProxy
	Namespace             string
	ClusterName           string
	SpecName              string
}

// LabelSync verifies the BMH-to-Node label sync feature.
// It enables a test prefix on the cluster and adds a label
// with the prefix to a BMH. It then confirms that the label shows up on
// the corresponding Node. It removes the label from the BMH and confirms
// that the label is removed from the Node.
func LabelSync(ctx context.Context, inputGetter func() LabelSyncInput) {
	Logf("Starting label sync tests [label_sync]")
	input := inputGetter()

	managementClient := input.BootstrapClusterProxy.GetClient()
	workloadClient := input.TargetClusterProxy.GetClient()
	syncIntervals := input.E2EConfig.GetIntervals(input.SpecName, "wait-label-sync")

	By("Enabling the label sync prefix on the Metal3Cluster")
	metal3Cluster := getMetal3ClusterForCluster(ctx, managementClient, input.Namespace, input.ClusterName)
	setMetal3ClusterAnnotation(ctx, managementClient, metal3Cluster, labelSyncPrefixAnnotation, labelSyncTestPrefix)

	By("Selecting a provisioned BareMetalHost and finding its Node")
	host, nodeName := getProvisionedHostAndNode(ctx, managementClient, input.Namespace)
	Logf("Using BMH %q which maps to Node %q", host.Name, nodeName)

	By("Adding a prefixed label to the BareMetalHost")
	setHostLabel(ctx, managementClient, host, labelSyncTestKey, labelSyncTestValue)

	By("Waiting for the label to appear on the Node")
	Eventually(func(g Gomega) {
		node := &corev1.Node{}
		g.Expect(workloadClient.Get(ctx, client.ObjectKey{Name: nodeName}, node)).To(Succeed())
		g.Expect(node.Labels).To(HaveKeyWithValue(labelSyncTestKey, labelSyncTestValue))
	}, syncIntervals...).Should(Succeed(), "label was not synced onto the Node in time")

	By("Removing the label from the BareMetalHost")
	deleteHostLabel(ctx, managementClient, host, labelSyncTestKey)

	By("Waiting for the label to be removed from the Node")
	Eventually(func(g Gomega) {
		node := &corev1.Node{}
		g.Expect(workloadClient.Get(ctx, client.ObjectKey{Name: nodeName}, node)).To(Succeed())
		g.Expect(node.Labels).ToNot(HaveKey(labelSyncTestKey))
	}, syncIntervals...).Should(Succeed(), "label was not removed from the Node in time")

	By("LABEL SYNC TESTS PASSED!")
}

// getMetal3ClusterForCluster resolves the Metal3Cluster backing the given CAPI Cluster
// by reading the Cluster's infrastructureRef.
func getMetal3ClusterForCluster(ctx context.Context, c client.Client, namespace, clusterName string) *infrav1.Metal3Cluster {
	cluster := &clusterv1.Cluster{}
	Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: clusterName}, cluster)).To(Succeed(),
		"failed to get Cluster %s/%s", namespace, clusterName)

	metal3Cluster := &infrav1.Metal3Cluster{}
	Expect(c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: cluster.Spec.InfrastructureRef.Name}, metal3Cluster)).To(Succeed(),
		"failed to get Metal3Cluster %s/%s", namespace, cluster.Spec.InfrastructureRef.Name)
	return metal3Cluster
}

// setMetal3ClusterAnnotation adds or overwrites a single annotation on a Metal3Cluster.
func setMetal3ClusterAnnotation(ctx context.Context, c client.Client, metal3Cluster *infrav1.Metal3Cluster, key, value string) {
	helper, err := patch.NewHelper(metal3Cluster, c)
	Expect(err).NotTo(HaveOccurred())
	if metal3Cluster.Annotations == nil {
		metal3Cluster.Annotations = map[string]string{}
	}
	metal3Cluster.Annotations[key] = value
	Expect(helper.Patch(ctx, metal3Cluster)).To(Succeed(), "failed to annotate Metal3Cluster %s", metal3Cluster.Name)
}

// getProvisionedHostAndNode returns the first provisioned BareMetalHost in the namespace
// together with the name of the Node it maps to (via ConsumerRef -> Metal3Machine ->
// Machine -> NodeRef).
func getProvisionedHostAndNode(ctx context.Context, c client.Client, namespace string) (bmov1alpha1.BareMetalHost, string) {
	bmhs, err := GetAllBmhs(ctx, c, namespace)
	Expect(err).ToNot(HaveOccurred(), "failed to list BareMetalHosts")
	provisioned := FilterBmhsByProvisioningState(bmhs, bmov1alpha1.StateProvisioned)
	Expect(provisioned).ToNot(BeEmpty(), "expected at least one provisioned BareMetalHost")

	host := provisioned[0]
	Expect(host.Spec.ConsumerRef).ToNot(BeNil(), "provisioned BMH %s has no ConsumerRef", host.Name)

	m3m := &infrav1.Metal3Machine{}
	Expect(c.Get(ctx, client.ObjectKey{Namespace: host.Spec.ConsumerRef.Namespace, Name: host.Spec.ConsumerRef.Name}, m3m)).To(Succeed(),
		"failed to get Metal3Machine %s", host.Spec.ConsumerRef.Name)

	machineName, err := Metal3MachineToMachineName(*m3m)
	Expect(err).ToNot(HaveOccurred(), "failed to resolve owner Machine for Metal3Machine %s", m3m.Name)

	machine := GetMachine(ctx, c, client.ObjectKey{Namespace: namespace, Name: machineName})
	Expect(machine.Status.NodeRef.IsDefined()).To(BeTrue(), "Machine %s has no NodeRef yet", machine.Name)
	return host, machine.Status.NodeRef.Name
}

// setHostLabel adds or overwrites a label on a BareMetalHost.
func setHostLabel(ctx context.Context, c client.Client, host bmov1alpha1.BareMetalHost, key, value string) {
	fresh := &bmov1alpha1.BareMetalHost{}
	Expect(c.Get(ctx, client.ObjectKey{Namespace: host.Namespace, Name: host.Name}, fresh)).To(Succeed())
	helper, err := patch.NewHelper(fresh, c)
	Expect(err).NotTo(HaveOccurred())
	if fresh.Labels == nil {
		fresh.Labels = map[string]string{}
	}
	fresh.Labels[key] = value
	Expect(helper.Patch(ctx, fresh)).To(Succeed(), "failed to label BMH %s", fresh.Name)
}

// deleteHostLabel removes a label from a BareMetalHost.
func deleteHostLabel(ctx context.Context, c client.Client, host bmov1alpha1.BareMetalHost, key string) {
	fresh := &bmov1alpha1.BareMetalHost{}
	Expect(c.Get(ctx, client.ObjectKey{Namespace: host.Namespace, Name: host.Name}, fresh)).To(Succeed())
	helper, err := patch.NewHelper(fresh, c)
	Expect(err).NotTo(HaveOccurred())
	delete(fresh.Labels, key)
	Expect(helper.Patch(ctx, fresh)).To(Succeed(), "failed to remove label from BMH %s", fresh.Name)
}

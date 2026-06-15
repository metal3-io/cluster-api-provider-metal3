package e2e

import (
	"context"
	"fmt"

	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// BMC credentials used for all BMHs.
	bmcUsername = "admin"
	bmcPassword = "password" // #nosec G101 - test credential only
)

// GenerateBMH creates a BareMetalHost and its corresponding Secret programmatically.
func GenerateBMH(vmInfo VMInfo, namespace string) (*bmov1alpha1.BareMetalHost, *corev1.Secret) {
	secretName := vmInfo.Name + "-bmc-secret"

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"username": bmcUsername,
			"password": bmcPassword,
		},
	}

	bmh := &bmov1alpha1.BareMetalHost{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vmInfo.Name,
			Namespace: namespace,
			Labels: map[string]string{
				"infrastructure.cluster.x-k8s.io/clusterctl-move": "metal3",
			},
		},
		Spec: bmov1alpha1.BareMetalHostSpec{
			BMC: bmov1alpha1.BMCDetails{
				Address:         vmInfo.BMCAddress,
				CredentialsName: secretName,
			},
			BootMACAddress: vmInfo.MACAddress,
			Online:         true,
		},
	}

	if vmInfo.RootDeviceHints != nil && vmInfo.RootDeviceHints.DeviceName != "" {
		bmh.Spec.RootDeviceHints = &bmov1alpha1.RootDeviceHints{
			DeviceName: vmInfo.RootDeviceHints.DeviceName,
		}
	}

	return bmh, secret
}

// ApplyBMHs creates BareMetalHost objects and their secrets for the given VMs
// in the specified namespace. This replaces the old ApplyBmh function that read
// pre-generated YAML from metal3-dev-env.
func ApplyBMHs(ctx context.Context, clusterProxy framework.ClusterProxy, vmInfos []VMInfo, namespace string) {
	By(fmt.Sprintf("Creating %d BareMetalHosts in namespace %s", len(vmInfos), namespace))

	// Ensure the namespace exists before creating resources in it.
	clientset := clusterProxy.GetClientSet()
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
	if _, err := clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			Expect(err).ToNot(HaveOccurred(), "Failed to create namespace %s", namespace)
		}
	}

	clusterClient := clusterProxy.GetClient()

	for _, vmInfo := range vmInfos {
		bmh, secret := GenerateBMH(vmInfo, namespace)

		Logf("Creating BMC secret %s/%s", namespace, secret.Name)
		err := clusterClient.Create(ctx, secret)
		if err != nil {
			if apierrors.IsAlreadyExists(err) {
				// Update existing secret.
				existingSecret := &corev1.Secret{}
				Expect(clusterClient.Get(ctx, client.ObjectKeyFromObject(secret), existingSecret)).To(Succeed(), "Failed to get existing secret %s", secret.Name)
				secret.ResourceVersion = existingSecret.ResourceVersion
				Expect(clusterClient.Update(ctx, secret)).To(Succeed(), "Failed to update secret %s", secret.Name)
			} else {
				Expect(err).ToNot(HaveOccurred(), "Failed to create secret %s", secret.Name)
			}
		}

		Logf("Creating BareMetalHost %s/%s (BMC: %s, MAC: %s)", namespace, bmh.Name, bmh.Spec.BMC.Address, bmh.Spec.BootMACAddress)
		err = clusterClient.Create(ctx, bmh)
		if err != nil {
			if apierrors.IsAlreadyExists(err) {
				// Update existing BMH.
				existingBMH := &bmov1alpha1.BareMetalHost{}
				Expect(clusterClient.Get(ctx, client.ObjectKeyFromObject(bmh), existingBMH)).To(Succeed(), "Failed to get existing BMH %s", bmh.Name)
				bmh.ResourceVersion = existingBMH.ResourceVersion
				Expect(clusterClient.Update(ctx, bmh)).To(Succeed(), "Failed to update BMH %s", bmh.Name)
			} else {
				Expect(err).ToNot(HaveOccurred(), "Failed to create BMH %s", bmh.Name)
			}
		}
	}

	ListBareMetalHosts(ctx, clusterClient, client.InNamespace(namespace))
}

// WaitForBMHsAvailable waits for all BMHs to reach the Available state.
func WaitForBMHsAvailable(ctx context.Context, clusterProxy framework.ClusterProxy, namespace string, numNodes int, intervals []any) {
	clusterClient := clusterProxy.GetClient()
	WaitForNumBmhInState(ctx, bmov1alpha1.StateAvailable, WaitForNumInput{
		Client:    clusterClient,
		Options:   []client.ListOption{client.InNamespace(namespace)},
		Replicas:  numNodes,
		Intervals: intervals,
	})
	ListBareMetalHosts(ctx, clusterClient, client.InNamespace(namespace))
}

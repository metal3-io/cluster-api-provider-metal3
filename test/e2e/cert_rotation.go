package e2e

import (
	"context"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
)

type CertRotationInput struct {
	E2EConfig    *clusterctl.E2EConfig
	ClusterProxy framework.ClusterProxy
	SpecName     string
}

func CertRotation(ctx context.Context, inputGetter func() CertRotationInput) {
	Logf("Start the certificate rotation test")
	input := inputGetter()
	clientSet := input.ClusterProxy.GetClientSet()
	clusterClient := input.ClusterProxy.GetClient()
	mariadbEnabled := GetBoolVariable(input.E2EConfig, ironicMariadb)
	ironicNS := input.E2EConfig.MustGetVariable(ironicNamespace)
	By("Check if Ironic is running")
	WaitForIronicReady(ctx, WaitForIronicInput{
		Client:    clusterClient,
		Name:      "ironic",
		Namespace: ironicNS,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-deployment"),
	})

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ironic-service",
			Namespace: ironicNS,
		},
	}

	// Capture deployment annotation and revision before rotation
	preTLSVersion, preTLSVerErr := GetDeploymentTLSSecretVersion(ctx, deployment, input.ClusterProxy)
	if preTLSVerErr != nil {
		Logf("Warning: could not read tls-secret-version pre-rotation: %v", preTLSVerErr)
	}
	preRevision, preRevErr := GetDeploymentRevision(ctx, deployment, input.ClusterProxy)
	if preRevErr != nil {
		Logf("Warning: could not read deployment revision pre-rotation: %v", preRevErr)
	}
	Logf("Pre-rotation: tls-secret-version=%q, deployment.revision=%q", preTLSVersion, preRevision)

	By("Fetch logs from target cluster after pivot")
	err := FetchClusterLogs(input.ClusterProxy, filepath.Join(clusterLogCollectionBasePath, "before-cert-rotation"))
	if err != nil {
		Logf("Error: %v", err)
	}

	By("Force the cert-manager to regenerate the certificate by deleting the secrets")
	secretList := []string{
		"ironic-cert",
	}
	if mariadbEnabled {
		secretList = append(secretList, "mariadb-cert")
	}
	for _, secretName := range secretList {
		secretErr := clientSet.CoreV1().Secrets(ironicNS).Delete(ctx, secretName, metav1.DeleteOptions{})
		Expect(secretErr).ToNot(HaveOccurred(), "cannot delete this secret: %s", secretName)
	}
	By("Check if the secrets are deleted")
	for _, secretName := range secretList {
		_, secretErr := clientSet.CoreV1().Secrets(ironicNS).Get(ctx, secretName, metav1.GetOptions{})
		Expect(secretErr).To(HaveOccurred(), secretName)
		Expect(apierrors.IsNotFound(secretErr)).To(BeTrue())
	}
	By("Wait until secrets are recreated")
	Eventually(func(g Gomega) {
		_, secretErr := clientSet.CoreV1().Secrets(ironicNS).Get(ctx, secretList[0], metav1.GetOptions{})
		g.Expect(secretErr).ToNot(HaveOccurred())
		if mariadbEnabled {
			_, secretErr := clientSet.CoreV1().Secrets(ironicNS).Get(ctx, secretList[1], metav1.GetOptions{})
			g.Expect(secretErr).ToNot(HaveOccurred())
		}
	}, input.E2EConfig.GetIntervals(input.SpecName, "wait-pod-restart")...).Should(Succeed())

	By("Check if Ironic is running after certificate rotation")
	WaitForIronicReady(ctx, WaitForIronicInput{
		Client:    clusterClient,
		Name:      "ironic",
		Namespace: ironicNS,
		Intervals: input.E2EConfig.GetIntervals(input.SpecName, "wait-deployment"),
	})

	// Capture deployment annotation and revision after rotation and assert change
	postTLSVersion, postTLSVerErr := GetDeploymentTLSSecretVersion(ctx, deployment, input.ClusterProxy)
	Expect(postTLSVerErr).ToNot(HaveOccurred())
	postRevision, postRevErr := GetDeploymentRevision(ctx, deployment, input.ClusterProxy)
	Expect(postRevErr).ToNot(HaveOccurred())
	Logf("Post-rotation: tls-secret-version=%q, deployment.revision=%q", postTLSVersion, postRevision)
	Expect(postTLSVersion).ToNot(Equal(preTLSVersion), "tls-secret-version should change after cert rotation. before=%s after=%s", preTLSVersion, postTLSVersion)
	Expect(postRevision).To(Equal(preRevision+1), "deployment revision should increase after cert rotation. before=%s after=%s", preRevision, postRevision)

	By("Fetch logs from target cluster after pivot")
	err = FetchClusterLogs(input.ClusterProxy, filepath.Join(clusterLogCollectionBasePath, "after-cert-rotation"))
	if err != nil {
		Logf("Error: %v", err)
	}
	// TODO(Sunnatillo): Further extend the test with querying ironic endpoint with new certificates

	By("CERTIFICATE ROTATION TESTS PASSED!")
}

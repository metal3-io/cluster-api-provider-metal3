package e2e

import (
	"context"
	"fmt"
	"time"

	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	timeout = 40 * time.Minute
	freq    = 30 * time.Second
)

type HealthCheckInput struct {
	BootstrapClusterProxy framework.ClusterProxy
	ClusterName           string
	Namespace             string
}

func healthcheck(ctx context.Context, inputGetter func() HealthCheckInput) {
	input := inputGetter()
	cli := input.BootstrapClusterProxy.GetClient()
	namespace := input.Namespace
	clusterName := input.ClusterName
	controlplaneM3Machines, workerM3Machines := GetMetal3Machines(ctx, cli, clusterName, namespace)

	// Worker
	By("Healthchecking the workers")
	workerHealthcheck, err := DeployWorkerHealthCheck(ctx, cli, namespace, clusterName)
	Expect(err).ToNot(HaveOccurred())
	workerMachineName, err := Metal3MachineToMachineName(workerM3Machines[0])
	Expect(err).ToNot(HaveOccurred())
	workerMachine := GetMachine(ctx, cli, client.ObjectKey{Name: workerMachineName, Namespace: namespace})
	workerIP, err := MachineToIPAddress(ctx, cli, &workerMachine)
	Expect(err).ToNot(HaveOccurred())
	Expect(runCommand("", "", workerIP, "metal3", "systemctl stop kubelet")).To(Succeed())
	// Wait until node is marked unhealthy and then check that it becomes healthy again
	Logf("Waiting for unhealthy worker...")
	WaitForHealthCheckCurrentHealthyToMatch(ctx, cli, 0, workerHealthcheck, timeout, freq)
	Logf("Waiting for remediationrequest to exist ...")
	WaitForRemediationRequest(ctx, cli, client.ObjectKeyFromObject(&workerMachine), true, timeout, freq)
	Logf("Waiting for worker to get healthy again...")
	WaitForHealthCheckCurrentHealthyToMatch(ctx, cli, 1, workerHealthcheck, timeout, freq)
	Logf("Waiting for remediationrequest to not exist ...")
	WaitForRemediationRequest(ctx, cli, client.ObjectKeyFromObject(&workerMachine), false, timeout, freq)

	// Controlplane
	By("Healthchecking the controlplane")
	controlplaneHealthcheck, err := DeployControlplaneHealthCheck(ctx, cli, namespace, clusterName)
	Expect(err).ToNot(HaveOccurred())
	controlplaneMachineName, err := Metal3MachineToMachineName(controlplaneM3Machines[0])
	Expect(err).ToNot(HaveOccurred())
	controlplaneMachine := GetMachine(ctx, cli, client.ObjectKey{Name: controlplaneMachineName, Namespace: namespace})
	controlplaneIP, err := MachineToIPAddress(ctx, cli, &controlplaneMachine)
	Expect(err).ToNot(HaveOccurred())
	Expect(runCommand("", "", controlplaneIP, "metal3", "systemctl stop kubelet")).To(Succeed())
	// Wait until node is marked unhealthy and then check that it becomes healthy again
	Logf("Waiting for unhealthy controlplane ...")
	WaitForHealthCheckCurrentHealthyToMatch(ctx, cli, 2, controlplaneHealthcheck, timeout, freq)
	Logf("Waiting for remediationrequest to exist ...")
	WaitForRemediationRequest(ctx, cli, client.ObjectKeyFromObject(&controlplaneMachine), true, timeout, freq)
	Logf("Waiting for controlplane to be healthy again...")
	WaitForHealthCheckCurrentHealthyToMatch(ctx, cli, 3, controlplaneHealthcheck, timeout, freq)
	Logf("Waiting for remediationrequest to not exist ...")
	WaitForRemediationRequest(ctx, cli, client.ObjectKeyFromObject(&controlplaneMachine), false, timeout, freq)
}

// DeployControlplaneHealthCheck creates a MachineHealthcheck and Metal3RemediationTemplate for controlplane machines.
func DeployControlplaneHealthCheck(ctx context.Context, cli client.Client, namespace, clusterName string) (*clusterv1.MachineHealthCheck, error) {
	remediationTemplateName := "controlplane-remediation-request"
	healthCheckName := "controlplane-healthcheck"
	matchLabels := map[string]string{
		"cluster.x-k8s.io/control-plane": "",
	}
	healthcheck, err := DeployMachineHealthCheck(ctx, cli, namespace, clusterName, remediationTemplateName, healthCheckName, matchLabels)
	if err != nil {
		return nil, fmt.Errorf("creating controlplane healthcheck failed: %w", err)
	}
	return healthcheck, nil
}

// DeployWorkerHealthCheck creates a MachineHealthcheck and Metal3RemediationTemplate for worker machines.
func DeployWorkerHealthCheck(ctx context.Context, cli client.Client, namespace, clusterName string) (*clusterv1.MachineHealthCheck, error) {
	remediationTemplateName := "worker-remediation-request"
	healthCheckName := "worker-healthcheck"
	matchLabels := map[string]string{
		"nodepool": "nodepool-0",
	}
	healthcheck, err := DeployMachineHealthCheck(ctx, cli, namespace, clusterName, remediationTemplateName, healthCheckName, matchLabels)
	if err != nil {
		return nil, fmt.Errorf("creating worker healthcheck failed: %w", err)
	}
	return healthcheck, nil
}

// DeployMachineHealthCheck creates a MachineHealthcheck and Metal3RemediationTemplate with given values.
func DeployMachineHealthCheck(ctx context.Context, cli client.Client, namespace, clusterName, remediationTemplateName, healthCheckName string, matchLabels map[string]string) (*clusterv1.MachineHealthCheck, error) {
	remediationTemplate := infrav1.Metal3RemediationTemplate{
		TypeMeta: metav1.TypeMeta{
			Kind: "Metal3RemediationTemplate",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      remediationTemplateName,
			Namespace: namespace,
		},
		Spec: infrav1.Metal3RemediationTemplateSpec{
			Template: infrav1.Metal3RemediationTemplateResource{
				Spec: infrav1.Metal3RemediationSpec{
					Strategy: &infrav1.RemediationStrategy{
						Type:       infrav1.RebootRemediationStrategy,
						RetryLimit: 1,
						Timeout:    &metav1.Duration{Duration: time.Second * 300},
					},
				},
			},
		},
	}

	err := cli.Create(ctx, &remediationTemplate)
	if err != nil {
		return nil, fmt.Errorf("couldn't create remediation template: %w", err)
	}

	healthCheck := &clusterv1.MachineHealthCheck{
		TypeMeta: metav1.TypeMeta{
			Kind: "MachineHealthCheck",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      healthCheckName,
			Namespace: namespace,
		},
		Spec: clusterv1.MachineHealthCheckSpec{
			ClusterName: clusterName,
			Selector: metav1.LabelSelector{
				MatchLabels: matchLabels,
			},
			UnhealthyConditions: []clusterv1.UnhealthyCondition{
				{
					Type:   corev1.NodeReady,
					Status: corev1.ConditionUnknown,
					Timeout: metav1.Duration{
						Duration: time.Second * 300,
					},
				},
				{
					Type:   corev1.NodeReady,
					Status: "False",
					Timeout: metav1.Duration{
						Duration: time.Second * 300,
					},
				},
			},
			MaxUnhealthy: &intstr.IntOrString{
				Type:   intstr.String,
				StrVal: "100%",
			},
			NodeStartupTimeout: &clusterv1.ZeroDuration,
			RemediationTemplate: &corev1.ObjectReference{
				Kind:       "Metal3RemediationTemplate",
				APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
				Name:       remediationTemplateName,
			},
		},
	}
	err = cli.Create(ctx, healthCheck)
	if err != nil {
		return nil, fmt.Errorf("couldn't create healthCheck: %w", err)
	}
	return healthCheck, nil
}

// WaitForHealthCheckCurrentHealthyToMatch waits for current healthy machines watched by healthcheck to match the number given.
func WaitForHealthCheckCurrentHealthyToMatch(ctx context.Context, cli client.Client, number int32, healthcheck *clusterv1.MachineHealthCheck, timeout, frequency time.Duration) {
	Eventually(func(g Gomega) int32 {
		g.Expect(cli.Get(ctx, client.ObjectKeyFromObject(healthcheck), healthcheck)).To(Succeed())
		return healthcheck.Status.CurrentHealthy
	}, timeout, frequency).Should(Equal(number))
}

// WaitForRemediationRequest waits until a remediation request created with healthcheck either exists or is deleted.
func WaitForRemediationRequest(ctx context.Context, cli client.Client, healthcheckName types.NamespacedName, toExist bool, timeout, frequency time.Duration) {
	Eventually(func(g Gomega) {
		remediation := &infrav1.Metal3Remediation{}
		if toExist {
			g.Expect(cli.Get(ctx, healthcheckName, remediation)).To(Succeed())
		} else {
			g.Expect(cli.Get(ctx, healthcheckName, remediation)).NotTo(Succeed())
		}
	}, timeout, frequency).Should(Succeed())
}

package e2e

import (
	"context"
	"fmt"
	"time"

	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	timeout = 40 * time.Minute
	freq    = 30 * time.Second
)

type HealthCheckInput struct {
	E2EConfig             *clusterctl.E2EConfig
	BootstrapClusterProxy framework.ClusterProxy
	ClusterName           string
	Namespace             string
	SpecName              string
}

func healthcheck(ctx context.Context, inputGetter func() HealthCheckInput) {
	input := inputGetter()
	bootstrapClusterClient := input.BootstrapClusterProxy.GetClient()
	namespace := input.Namespace
	clusterName := input.ClusterName
	controlplaneM3Machines, workerM3Machines := GetMetal3Machines(ctx, bootstrapClusterClient, clusterName, namespace)

	// get baremetal ip pool for retreiving ip addresses of controlpane and worker nodes
	baremetalv4Pool, _ := GetIPPools(ctx, bootstrapClusterClient, input.ClusterName, input.Namespace)
	Expect(baremetalv4Pool).ToNot(BeEmpty())

	// Worker
	By("Healthchecking the workers")
	workerHealthcheck, workerRemediationTemplate, err := DeployWorkerHealthCheck(ctx, bootstrapClusterClient, namespace, clusterName)
	Expect(err).ToNot(HaveOccurred())

	workerMachineName, err := Metal3MachineToMachineName(workerM3Machines[0])
	Expect(err).ToNot(HaveOccurred())
	workerMachine := GetMachine(ctx, bootstrapClusterClient, client.ObjectKey{Name: workerMachineName, Namespace: namespace})
	workerIP, err := MachineToIPAddress1beta1(ctx, bootstrapClusterClient, &workerMachine, baremetalv4Pool[0])
	Expect(err).ToNot(HaveOccurred())

	Logf("Stopping kubelet on worker machine")
	Eventually(func(g Gomega) {
		g.Expect(runCommand("", "", workerIP, "metal3", "systemctl stop kubelet")).To(Succeed())
	}, input.E2EConfig.GetIntervals(input.SpecName, "wait-command")...).Should(Succeed())

	// Wait until node is marked unhealthy and then check that it becomes healthy again
	Logf("Waiting for unhealthy worker...")
	WaitForHealthCheckCurrentHealthyToMatch(ctx, bootstrapClusterClient, 0, workerHealthcheck, timeout, freq)
	Logf("Waiting for remediationrequest to exist ...")
	WaitForRemediationRequest(ctx, bootstrapClusterClient, client.ObjectKeyFromObject(&workerMachine), true, timeout, freq)
	Logf("Waiting for worker to get healthy again...")
	WaitForHealthCheckCurrentHealthyToMatch(ctx, bootstrapClusterClient, 1, workerHealthcheck, timeout, freq)
	Logf("Waiting for remediationrequest to not exist ...")
	WaitForRemediationRequest(ctx, bootstrapClusterClient, client.ObjectKeyFromObject(&workerMachine), false, timeout, freq)

	// Controlplane
	By("Healthchecking the controlplane")
	controlplaneHealthcheck, controlplaneRemediationTemplate, err := DeployControlplaneHealthCheck(ctx, bootstrapClusterClient, namespace, clusterName)
	Expect(err).ToNot(HaveOccurred())
	controlplaneMachineName, err := Metal3MachineToMachineName(controlplaneM3Machines[0])
	Expect(err).ToNot(HaveOccurred())
	controlplaneMachine := GetMachine(ctx, bootstrapClusterClient, client.ObjectKey{Name: controlplaneMachineName, Namespace: namespace})
	controlplaneIP, err := MachineToIPAddress1beta1(ctx, bootstrapClusterClient, &controlplaneMachine, baremetalv4Pool[0])
	Expect(err).ToNot(HaveOccurred())

	Logf("Stopping kubelet on controlplane machine")
	Eventually(func(g Gomega) {
		g.Expect(runCommand("", "", controlplaneIP, "metal3", "systemctl stop kubelet")).To(Succeed())
	}, input.E2EConfig.GetIntervals(input.SpecName, "wait-command")...).Should(Succeed())

	// Wait until node is marked unhealthy and then check that it becomes healthy again
	Logf("Waiting for unhealthy controlplane ...")
	WaitForHealthCheckCurrentHealthyToMatch(ctx, bootstrapClusterClient, 2, controlplaneHealthcheck, timeout, freq)
	Logf("Waiting for remediationrequest to exist ...")
	WaitForRemediationRequest(ctx, bootstrapClusterClient, client.ObjectKeyFromObject(&controlplaneMachine), true, timeout, freq)
	Logf("Waiting for controlplane to be healthy again...")
	WaitForHealthCheckCurrentHealthyToMatch(ctx, bootstrapClusterClient, 3, controlplaneHealthcheck, timeout, freq)
	Logf("Waiting for remediationrequest to not exist ...")
	WaitForRemediationRequest(ctx, bootstrapClusterClient, client.ObjectKeyFromObject(&controlplaneMachine), false, timeout, freq)

	By("Deleting worker and controlplane Metal3RemediationTemplate CRs")
	Expect(bootstrapClusterClient.Delete(ctx, workerRemediationTemplate)).To(Succeed(), "should delete worker Metal3RemediationTemplate CR")
	Expect(bootstrapClusterClient.Delete(ctx, controlplaneRemediationTemplate)).To(Succeed(), "should delete controlplane Metal3RemediationTemplate CR")

	By("Make sure Metal3RemediationTemplate CRs are actually deleted")
	Eventually(func() bool {
		cpM3MremediationTemplate := &infrav1.Metal3RemediationTemplate{}
		workerM3MremediationTemplate := &infrav1.Metal3RemediationTemplate{}
		cpErr := bootstrapClusterClient.Get(ctx, client.ObjectKeyFromObject(controlplaneRemediationTemplate), cpM3MremediationTemplate)
		workerErr := bootstrapClusterClient.Get(ctx, client.ObjectKeyFromObject(workerRemediationTemplate), workerM3MremediationTemplate)
		return apierrors.IsNotFound(cpErr) && apierrors.IsNotFound(workerErr)
	}, input.E2EConfig.GetIntervals(input.SpecName, "wait-delete-remediation-template")...).Should(BeTrue(), "Metal3RemediationTemplate should have been deleted")
}

// DeployControlplaneHealthCheck creates a MachineHealthcheck and Metal3RemediationTemplate for controlplane machines.
func DeployControlplaneHealthCheck(ctx context.Context, cli client.Client, namespace, clusterName string) (*clusterv1.MachineHealthCheck, *infrav1.Metal3RemediationTemplate, error) {
	remediationTemplateName := "controlplane-remediation-request"
	healthCheckName := "controlplane-healthcheck"
	matchLabels := map[string]string{
		"cluster.x-k8s.io/control-plane": "",
	}
	healthcheck, remediationTemplate, err := DeployMachineHealthCheck(ctx, cli, namespace, clusterName, remediationTemplateName, healthCheckName, matchLabels)
	if err != nil {
		return nil, nil, fmt.Errorf("creating controlplane healthcheck failed: %w", err)
	}
	return healthcheck, remediationTemplate, nil
}

// DeployWorkerHealthCheck creates a MachineHealthcheck and Metal3RemediationTemplate for worker machines.
func DeployWorkerHealthCheck(ctx context.Context, cli client.Client, namespace, clusterName string) (*clusterv1.MachineHealthCheck, *infrav1.Metal3RemediationTemplate, error) {
	remediationTemplateName := "worker-remediation-request"
	healthCheckName := "worker-healthcheck"
	matchLabels := map[string]string{
		"nodepool": "nodepool-0",
	}
	healthcheck, remediationTemplate, err := DeployMachineHealthCheck(ctx, cli, namespace, clusterName, remediationTemplateName, healthCheckName, matchLabels)
	if err != nil {
		return nil, nil, fmt.Errorf("creating worker healthcheck failed: %w", err)
	}
	return healthcheck, remediationTemplate, nil
}

// DeployMachineHealthCheck creates a MachineHealthcheck and Metal3RemediationTemplate with given values.
func DeployMachineHealthCheck(ctx context.Context, cli client.Client, namespace, clusterName, remediationTemplateName, healthCheckName string, matchLabels map[string]string) (*clusterv1.MachineHealthCheck, *infrav1.Metal3RemediationTemplate, error) {
	remediationTemplate := &infrav1.Metal3RemediationTemplate{
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

	err := cli.Create(ctx, remediationTemplate)
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't create remediation template: %w", err)
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
			Checks: clusterv1.MachineHealthCheckChecks{
				UnhealthyNodeConditions: []clusterv1.UnhealthyNodeCondition{
					{
						Type:           corev1.NodeReady,
						Status:         corev1.ConditionUnknown,
						TimeoutSeconds: ptr.To(int32(300)),
					},
					{
						Type:           corev1.NodeReady,
						Status:         "False",
						TimeoutSeconds: ptr.To(int32(300)),
					},
				},
			},
			Remediation: clusterv1.MachineHealthCheckRemediation{
				TemplateRef: clusterv1.MachineHealthCheckRemediationTemplateReference{
					Kind:       "Metal3RemediationTemplate",
					APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
					Name:       remediationTemplateName,
				},
				TriggerIf: clusterv1.MachineHealthCheckRemediationTriggerIf{
					UnhealthyLessThanOrEqualTo: &intstr.IntOrString{
						Type:   intstr.String,
						StrVal: "100%",
					},
				},
			},
		},
	}
	err = cli.Create(ctx, healthCheck)
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't create healthCheck: %w", err)
	}
	return healthCheck, remediationTemplate, nil
}

// WaitForHealthCheckCurrentHealthyToMatch waits for current healthy machines watched by healthcheck to match the number given.
func WaitForHealthCheckCurrentHealthyToMatch(ctx context.Context, cli client.Client, number int32, healthcheck *clusterv1.MachineHealthCheck, timeout, frequency time.Duration) {
	Eventually(func(g Gomega) int32 {
		g.Expect(cli.Get(ctx, client.ObjectKeyFromObject(healthcheck), healthcheck)).To(Succeed())
		return *healthcheck.Status.CurrentHealthy
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

package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("When testing healthcheck [healthcheck] [features] [remediation]", func() {
	BeforeEach(func() {
		osType := strings.ToLower(os.Getenv("OS"))
		Expect(osType).ToNot(Equal(""))
		validateGlobals(specName)

		// We need to override clusterctl apply log folder to avoid getting our credentials exposed.
		clusterctlLogFolder = filepath.Join(os.TempDir(), "clusters", bootstrapClusterProxy.GetName())
	})

	It("Should remediate unhealthy machines", func() {
		By("Fetching cluster configuration")
		k8sVersion := e2eConfig.GetVariable("KUBERNETES_VERSION")
		By("Provision Workload cluster")
		targetCluster, _ = createTargetCluster(k8sVersion)

		cli := bootstrapClusterProxy.GetClient()
		controlplaneM3Machines, workerM3Machines := GetMetal3Machines(ctx, cli, clusterName, namespace)
		timeout := 40 * time.Minute
		freq := 30 * time.Second

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

	})

	AfterEach(func() {
		DumpSpecResourcesAndCleanup(ctx, specName, bootstrapClusterProxy, artifactFolder, namespace, e2eConfig.GetIntervals, clusterName, clusterctlLogFolder, skipCleanup)
	})
})

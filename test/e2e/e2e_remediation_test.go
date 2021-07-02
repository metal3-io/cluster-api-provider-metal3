package e2e

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Remedation Pivoting", func() {
	var (
		ctx                 = context.TODO()
		specName            = "metal3"
		namespace           = "metal3"
		cluster             *clusterv1.Cluster
		clusterName         = "test1"
		clusterctlLogFolder string
	)

	BeforeEach(func() {
		Expect(e2eConfig).ToNot(BeNil(), "Invalid argument. e2eConfig can't be nil when calling %s spec", specName)
		Expect(clusterctlConfigPath).To(BeAnExistingFile(), "Invalid argument. clusterctlConfigPath must be an existing file when calling %s spec", specName)
		Expect(bootstrapClusterProxy).ToNot(BeNil(), "Invalid argument. bootstrapClusterProxy can't be nil when calling %s spec", specName)
		Expect(os.MkdirAll(artifactFolder, 0755)).To(Succeed(), "Invalid argument. artifactFolder can't be created for %s spec", specName)

		Expect(e2eConfig.Variables).To(HaveKey(KubernetesVersion))

		// We need to override clusterctl apply log folder to avoid getting our credentials exposed.
		clusterctlLogFolder = filepath.Join(os.TempDir(), "clusters", bootstrapClusterProxy.GetName())
	})

	AfterEach(func() {
		dumpSpecResourcesAndCleanup(ctx, specName, bootstrapClusterProxy, artifactFolder, namespace, cluster, e2eConfig.GetIntervals, clusterName, clusterctlLogFolder, skipCleanup)
	})

	Context("Creating a highly available control-plane cluster", func() {
		It("Should create a cluster with 3 control-plane and 1 worker nodes", func() {
			By("Creating a high available cluster")
			result := clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
				ClusterProxy: bootstrapClusterProxy,
				ConfigCluster: clusterctl.ConfigClusterInput{
					LogFolder:                clusterctlLogFolder,
					ClusterctlConfigPath:     clusterctlConfigPath,
					KubeconfigPath:           bootstrapClusterProxy.GetKubeconfigPath(),
					InfrastructureProvider:   clusterctl.DefaultInfrastructureProvider,
					Flavor:                   "ha",
					Namespace:                namespace,
					ClusterName:              clusterName,
					KubernetesVersion:        e2eConfig.GetVariable(KubernetesVersion),
					ControlPlaneMachineCount: pointer.Int64Ptr(3),
					WorkerMachineCount:       pointer.Int64Ptr(1),
				},
				CNIManifestPath:              e2eTestsPath + "/data/cni/calico/calico.yaml",
				WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
				WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
				WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
			})
			cluster = result.Cluster
		})

		It("Rebooted node should become Ready", func() {
			// targetCluster := bootstrapClusterProxy.GetWorkloadCluster(ctx, clusterName, namespace)

			By("Check if machines are running.")
			fmt.Println("KubeconfigPath:", bootstrapClusterProxy.GetKubeconfigPath())
			Eventually(func() error {
				machines := &clusterv1.MachineList{}

				if err := bootstrapClusterProxy.GetClient().List(ctx, machines, client.InNamespace(namespace)); err != nil {
					return err
				}
				fmt.Println("Machines:", machines)
				Expect(machines.Items).To((HaveLen(4)))
				for _, machine := range machines.Items {
					if !strings.EqualFold(machine.Status.Phase, "running") { // Case insensitive comparison
						return errors.New("Machine is not in 'running' phase")
					}
				}
				return nil
			}, e2eConfig.GetIntervals(specName, "wait-machine-remediation")...).Should(BeNil())

			// 		  - name: Fetch the target cluster kubeconfig
			//     shell: "kubectl get secrets {{ CLUSTER_NAME }}-kubeconfig -n {{ NAMESPACE }} -o json | jq -r '.data.value'| base64 -d > /tmp/kubeconfig-{{ CLUSTER_NAME }}.yaml"

			//   # Reboot a single worker node
			//   - name: Reboot "{{ WORKER_BMH }}"
			//     shell: |
			//        kubectl annotate bmh "{{ WORKER_BMH }}" -n "{{ NAMESPACE }}" reboot.metal3.io=

			//   - name: List only powered off VMs
			//     virt:
			//       command: list_vms
			//       state: shutdown
			//     register: shutdown_vms
			//     retries: 50
			//     delay: 10
			//     until: WORKER_VM in shutdown_vms.list_vms
			//     become: yes
			//     become_user: root

		})

	})

})

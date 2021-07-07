package e2e

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	bmh "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	"github.com/metal3-io/cluster-api-provider-metal3/api/v1alpha4"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type vmState int

const (
	RUNNING vmState = iota
	PAUSED
	SHUTOFF
	OTHER
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

	It("Apply cluster", func() {
		By("Creating a a cluster with 3 control-plane and 1 worker nodes")
		cluster = clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
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
		}).Cluster

	})

	It("Run test: remediation", func() {

		// log.Logf("Waiting for the cluster infrastructure to be detected")
		// cluster := framework.DiscoveryAndWaitForCluster(ctx, framework.DiscoveryAndWaitForClusterInput{
		// 	Getter:    input.ClusterProxy.GetClient(),
		// 	Namespace: input.ConfigCluster.Namespace,
		// 	Name:      input.ConfigCluster.ClusterName,
		// }, input.WaitForClusterIntervals...)

		By("Checking that rebooted node becomes Ready")
		// targetCluster := bootstrapClusterProxy.GetWorkloadCluster(ctx, clusterName, namespace)

		fmt.Println("KubeconfigPath:", bootstrapClusterProxy.GetKubeconfigPath())
		client := bootstrapClusterProxy.GetClient()
		machines := &clusterv1.MachineList{}
		Eventually(func() error {

			if err := client.List(ctx, machines, byClusterOptions(clusterName, namespace)...); err != nil {
				return err
			}

			Expect(machines.Items).To((HaveLen(4)))
			for _, machine := range machines.Items {
				if !strings.EqualFold(machine.Status.Phase, "running") { // Case insensitive comparison
					return errors.New("Machine is not in 'running' phase")
				}
			}
			return nil
		}, e2eConfig.GetIntervals(specName, "wait-machine-remediation")...).Should(BeNil())

		ctrlM3Machines, workerM3Machines, err := getMetal3Machines(ctx, client, clusterName, namespace)
		Expect(err).NotTo(HaveOccurred())

		for _, m3machine := range ctrlM3Machines {
			fmt.Printf("m3 name: %s bmh name: %s \n", m3machine.ObjectMeta.Name, metal3MachineToBmhName(m3machine))
		}
		for _, m3machine := range workerM3Machines {
			fmt.Printf("m3 name: %s bmh name: %s \n", m3machine.ObjectMeta.Name, metal3MachineToBmhName(m3machine))
		}

		bmhs := getAllBMH(ctx, client, clusterName, namespace, specName)

		// TODO select the worker VM
		bmh := &bmhs.Items[2]
		vmName := bmhToVmName(*bmh)

		By("Rebooting a BareMetalHost")
		rebootBmh(ctx, client, bmh)

		// wait for the rebooted node to show as powered off
		Eventually(func() error {
			vms := listVms(SHUTOFF)
			fmt.Printf("Looking for vm %#v among %#v \n", vmName, vms)
			for _, name := range vms {
				if name == vmName {
					return nil
				}
			}
			return fmt.Errorf("VM '%s' not listed as shut down", bmh.Name)
		}, e2eConfig.GetIntervals(specName, "wait-machine-shutoff")...).Should(BeNil())

		// TODO wait for NonReady and Ready states

		// wait for the rebooted node to show as powered on
		Eventually(func() error {
			vms := listVms(RUNNING)
			fmt.Printf("Looking for vm %#v among %#v \n", vmName, vms)
			for _, name := range vms {
				if name == vmName {
					return nil
				}
			}
			return fmt.Errorf("VM '%s' not listed as shut down", bmh.Name)
		}, e2eConfig.GetIntervals(specName, "wait-machine-remediation")...).Should(BeNil())

		// getAllBMH(ctx, client, clusterName, namespace, specName)

		// hosts := bmh.BareMetalHostList{}

		// err := workloadCluster.GetClient().List(ctx, &hosts, client.InNamespace(namespace))
		// Expect(err).NotTo(HaveOccurred())

		// fmt.Println(hosts)

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

func rebootBmh(ctx context.Context, client client.Client, bmh *bmh.BareMetalHost) {
	helper, err := patch.NewHelper(bmh, client)
	Expect(err).ToNot(HaveOccurred())

	key := "reboot.metal3.io"
	annotations := bmh.GetAnnotations()

	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[key] = ""
	bmh.SetAnnotations(annotations)
	Expect(helper.Patch(ctx, bmh)).To(Succeed())
}

func metal3MachineToBmhName(m3machine v1alpha4.Metal3Machine) string {
	return strings.Replace(m3machine.GetAnnotations()["metal3.io/BareMetalHost"], "metal3/", "", 1)
}

// Derives the name of a VM created by metal3-dev-env from the name of a BareMetalHost object
func bmhToVmName(bmh bmh.BareMetalHost) string {
	return strings.ReplaceAll(bmh.Name, "-", "_")
}

func listVms(state vmState) []string {
	var flag string
	switch state {
	case RUNNING:
		flag = "--state-running"
	case SHUTOFF:
		flag = "--state-shutoff"
	case PAUSED:
		flag = "--state-paused"
	case OTHER:
		flag = "--state-other"
	}

	cmd := exec.Command("virsh", "list", "--name", flag)
	result, err := cmd.Output()
	Expect(err).NotTo(HaveOccurred())

	// virsh may return some empty lines which need to be removed
	lines := strings.Split(string(result), "\n")
	i := 0
	for _, line := range lines {
		if line != "" {
			lines[i] = line
			i++
		}
	}
	return lines[:i]
}

func getAllBMH(ctx context.Context, client client.Client, clusterName, namespace, specName string) bmh.BareMetalHostList {

	bmhs := bmh.BareMetalHostList{}
	Eventually(func() error {
		if err := client.List(ctx, &bmhs, byClusterOptions(clusterName, namespace)...); err != nil {
			fmt.Println(err)
			return err
		}
		return nil
	}, e2eConfig.GetIntervals(specName, "wait-bmh")...).Should(BeNil())

	for _, bmh := range bmhs.Items {
		fmt.Printf("bmh: %+v\n", bmh)
		fmt.Printf("bmh annotations: %+v\n", bmh.GetAnnotations())
	}

	return bmhs
}

func getMetal3Machines(ctx context.Context, client client.Client, cluster, namespace string) (controlplane, workers []v1alpha4.Metal3Machine, err error) {
	allMachines := &v1alpha4.Metal3MachineList{}
	if err = client.List(ctx, allMachines, byClusterOptions(cluster, namespace)...); err != nil {
		return
	}
	for _, machine := range allMachines.Items {
		if strings.Contains(machine.ObjectMeta.Name, "workers") {
			workers = append(workers, machine)
		} else {
			controlplane = append(controlplane, machine)
		}
	}
	return
}

// byClusterOptions returns a set of ListOptions that allows to identify all the objects belonging to a Cluster.
func byClusterOptions(name, namespace string) []client.ListOption {
	return []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels{
			clusterv1.ClusterLabelName: name,
		},
	}
}

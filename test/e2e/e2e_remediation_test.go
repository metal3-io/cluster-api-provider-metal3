package e2e

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	bmh "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"

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

	It("Run test remediation", func() {

		// log.Logf("Waiting for the cluster infrastructure to be detected")
		// cluster := framework.DiscoveryAndWaitForCluster(ctx, framework.DiscoveryAndWaitForClusterInput{
		// 	Getter:    input.ClusterProxy.GetClient(),
		// 	Namespace: input.ConfigCluster.Namespace,
		// 	Name:      input.ConfigCluster.ClusterName,
		// }, input.WaitForClusterIntervals...)

		By("Checking that rebooted node becomes Ready")
		// targetCluster := bootstrapClusterProxy.GetWorkloadCluster(ctx, clusterName, namespace)

		fmt.Println("KubeconfigPath:", bootstrapClusterProxy.GetKubeconfigPath())
		lister := bootstrapClusterProxy.GetClient()
		Eventually(func() error {
			machines := &clusterv1.MachineList{}

			if err := lister.List(ctx, machines, byClusterOptions(clusterName, namespace)...); err != nil {
				return err
			}

			Expect(machines.Items).To((HaveLen(4)))
			for _, machine := range machines.Items {
				if !strings.EqualFold(machine.Status.Phase, "running") { // Case insensitive comparison
					return errors.New("Machine is not in 'running' phase")
				}
				fmt.Println("annotations: ", machine.GetAnnotations())
			}
			return nil
		}, e2eConfig.GetIntervals(specName, "wait-machine-remediation")...).Should(BeNil())

		workloadCluster := bootstrapClusterProxy.GetWorkloadCluster(ctx, namespace, clusterName)
		fmt.Println("workloadCluster", workloadCluster)

		bmhs := &bmh.BareMetalHostList{}
		Eventually(func() error {
			if err := lister.List(ctx, bmhs, byClusterOptions(clusterName, namespace)...); err != nil {
				fmt.Println(err)
				return err
			}
			return nil
		}, e2eConfig.GetIntervals(specName, "wait-bmh")...).Should(BeNil())

		for _, bmh := range bmhs.Items {
			fmt.Printf("bmh: %+v\n", bmh)
			fmt.Printf("bmh annotations: %+v\n", bmh.GetAnnotations())
		}
		fmt.Println("BMHs", bmhs)

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

// func getHost(ctx context.Context, m3Machine *capm3.Metal3Machine, cl client.Client,
// 	mLog logr.Logger,
// ) (*bmh.BareMetalHost, error) {
// 	annotations := m3Machine.ObjectMeta.GetAnnotations()
// 	if annotations == nil {
// 		return nil, nil
// 	}
// 	hostKey, ok := annotations[HostAnnotation]
// 	if !ok {
// 		return nil, nil
// 	}
// 	hostNamespace, hostName, err := cache.SplitMetaNamespaceKey(hostKey)
// 	if err != nil {
// 		mLog.Error(err, "Error parsing annotation value", "annotation key", hostKey)
// 		return nil, err
// 	}

// 	host := bmh.BareMetalHost{}
// 	key := client.ObjectKey{
// 		Name:      hostName,
// 		Namespace: hostNamespace,
// 	}
// 	err = cl.Get(ctx, key, &host)
// 	if apierrors.IsNotFound(err) {
// 		mLog.Info("Annotated host not found", "host", hostKey)
// 		return nil, nil
// 	} else if err != nil {
// 		return nil, err
// 	}
// 	return &host, nil
// }

// // SetPauseAnnotation sets the pause annotations on associated bmh
// func (m *MachineManager) SetPauseAnnotation(ctx context.Context) error {
// 	// look for associated BMH
// 	host, helper, err := m.getHost(ctx)
// 	if err != nil {
// 		m.SetError("Failed to get a BaremetalHost for the Metal3Machine",
// 			capierrors.UpdateMachineError,
// 		)
// 		return err
// 	}
// 	if host == nil {
// 		return nil
// 	}

// 	annotations := host.GetAnnotations()

// 	if annotations != nil {
// 		if _, ok := annotations[bmh.PausedAnnotation]; ok {
// 			m.Log.Info("BaremetalHost is already paused")
// 			return nil
// 		}
// 	} else {
// 		host.Annotations = make(map[string]string)
// 	}
// 	m.Log.Info("Adding PausedAnnotation in BareMetalHost")
// 	host.Annotations[bmh.PausedAnnotation] = PausedAnnotationKey

// 	// Setting annotation with BMH status
// 	newAnnotation, err := json.Marshal(&host.Status)
// 	if err != nil {
// 		m.SetError("Failed to marshal the BareMetalHost status",
// 			capierrors.UpdateMachineError,
// 		)
// 		return errors.Wrap(err, "failed to marshall status annotation")
// 	}
// 	host.Annotations[bmh.StatusAnnotation] = string(newAnnotation)
// 	return helper.Patch(ctx, host)
// }

// byClusterOptions returns a set of ListOptions that allows to identify all the objects belonging to a Cluster.
func byClusterOptions(name, namespace string) []client.ListOption {
	return []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels{
			clusterv1.ClusterLabelName: name,
		},
	}
}

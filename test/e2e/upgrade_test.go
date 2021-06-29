package e2e

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	// "errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/utils/pointer"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"

	// capiCluster "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1alpha4"
	bmo "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	capim3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1alpha4"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/controller-runtime/pkg/client"

	// k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

// TODO: Turn off focus mode (remove "F")
var _ = FDescribe("Upgrade tests", func() {
	var (
		ctx       = context.TODO()
		specName  = "metal3"
		namespace = "metal3"
		// cluster             *clusterv1.Cluster
		clusterName               = "test1"
		ironicDeployName          = "capm3-ironic"
		clusterctlLogFolder       string
		workerNodesSelector       labels.Selector
		workersListOptions        metav1.ListOptions
		controlplaneNodesSelector labels.Selector
		controlplaneListOptions   metav1.ListOptions
	)

	BeforeEach(func() {
		Expect(e2eConfig).ToNot(BeNil(), "Invalid argument. e2eConfig can't be nil when calling %s spec", specName)
		Expect(clusterctlConfigPath).To(BeAnExistingFile(), "Invalid argument. clusterctlConfigPath must be an existing file when calling %s spec", specName)
		Expect(bootstrapClusterProxy).ToNot(BeNil(), "Invalid argument. bootstrapClusterProxy can't be nil when calling %s spec", specName)
		Expect(os.MkdirAll(artifactFolder, 0755)).To(Succeed(), "Invalid argument. artifactFolder can't be created for %s spec", specName)

		Expect(e2eConfig.Variables).To(HaveKey(KubernetesVersion))

		// We need to override clusterctl apply log folder to avoid getting our credentials exposed.
		clusterctlLogFolder = filepath.Join(os.TempDir(), "clusters", bootstrapClusterProxy.GetName())

		// Labels and selectors
		// workerNodesLabel := "node-role.kubernetes.io/control-plane"
		workerNodesRequirement, err := labels.NewRequirement("node-role.kubernetes.io/control-plane", selection.DoesNotExist, []string{})
		if err != nil {
			Fail("Failed to set up worker Node requirements")
		}
		workerNodesSelector = labels.NewSelector().Add(*workerNodesRequirement)
		workersListOptions = metav1.ListOptions{LabelSelector: workerNodesSelector.String()}

		controlplaneNodesRequirement, err := labels.NewRequirement("node-role.kubernetes.io/control-plane", selection.Exists, []string{})
		if err != nil {
			Fail("Failed to set up worker Node requirements")
		}
		controlplaneNodesSelector = labels.NewSelector().Add(*controlplaneNodesRequirement)
		controlplaneListOptions = metav1.ListOptions{LabelSelector: controlplaneNodesSelector.String()}
	})

	AfterEach(func() {
		// dumpSpecResourcesAndCleanup(ctx, specName, bootstrapClusterProxy, artifactFolder, namespace, cluster, e2eConfig.GetIntervals, clusterName, clusterctlLogFolder, skipCleanup)
	})

	Context("Upgrading KCP, Ironic and K8s", func() {
		// TODO: The cluster should be created and pivoting should be done before
		// we start with the upgrade tests.
		It("Should create a cluster with 3 control-plane and 0 worker nodes", func() {
			By("Creating a high available cluster")
			// result :=
			clusterctl.ApplyClusterTemplateAndWait(ctx, clusterctl.ApplyClusterTemplateAndWaitInput{
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
					WorkerMachineCount:       pointer.Int64Ptr(0),
				},
				CNIManifestPath:              e2eTestsPath + "/data/cni/calico/calico.yaml",
				WaitForClusterIntervals:      e2eConfig.GetIntervals(specName, "wait-cluster"),
				WaitForControlPlaneIntervals: e2eConfig.GetIntervals(specName, "wait-control-plane"),
				WaitForMachineDeployments:    e2eConfig.GetIntervals(specName, "wait-worker-nodes"),
			})
			targetCluster := bootstrapClusterProxy.GetWorkloadCluster(ctx, namespace, clusterName)

			clientSet := targetCluster.GetClientSet()
			clusterClient := targetCluster.GetClient()

			// Upgrade control plane
			// =====================
			// Untaint and tweak surge?
			// Scale down workers to 0?
			// Get config and secrets
			// Cleanup CRDs on disk?
			// Create clusterctl settings
			// Build clusterctl binary? -> Use clusterctl library
			// Create local repository?
			// Create next version CRDs
			// Upgrade target cluster
			// Restore config/secrets
			// Upgrade source cluster
			// Restore config/secrets
			// Verify

			// Upgrade ironic
			// ==============
			By("Upgrading ironic image based containers")
			getIronicDeployment := func() (*appsv1.Deployment, error) {
				return clientSet.AppsV1().Deployments(namespace).Get(ironicDeployName, metav1.GetOptions{})
			}
			deploy, err := getIronicDeployment()
			Expect(err).To(BeNil())
			for i := range deploy.Spec.Template.Spec.Containers {
				switch deploy.Spec.Template.Spec.Containers[i].Name {
				case
					"mariadb",
					"ironic-api",
					"ironic-dnsmasq",
					"ironic-conductor",
					"ironic-log-watch",
					"ironic-inspector",
					"ironic-inspector-log-watch":
					deploy.Spec.Template.Spec.Containers[i].Image = "quay.io/metal3-io/ironic:latest"
				}
			}
			clientSet.AppsV1().Deployments(namespace).Update(deploy)

			By("Waiting for ironic update to rollout")
			Eventually(func() bool {
				return deploymentRolledOut(getIronicDeployment, deploy.Status.ObservedGeneration+1)
			},
				e2eConfig.GetIntervals(specName, "wait-deployment")...,
			).Should(Equal(true))

			// Upgrade k8s version and boot image
			// ==================================
			By("Creating new Metal3MachineTemplates")
			m3mtControlplane := &capim3.Metal3MachineTemplate{}
			newM3mt := &capim3.Metal3MachineTemplate{}
			err = clusterClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "test1-controlplane"}, m3mtControlplane)
			Expect(err).To(BeNil(), "Failed to get Metal3MachineTemplate")
			newM3mt.Spec = *m3mtControlplane.Spec.DeepCopy()
			newM3mt.SetName("test1-new-controlplane")
			newM3mt.SetNamespace(namespace)
			err = clusterClient.Create(ctx, newM3mt)
			Expect(err).To(BeNil(), "Failed to create new Metal3MachineTemplate")
			// Repeat for the workers
			m3mtWorkers := &capim3.Metal3MachineTemplate{}
			newM3mt = &capim3.Metal3MachineTemplate{}
			err = clusterClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "test1-workers"}, m3mtWorkers)
			Expect(err).To(BeNil(), "Failed to get Metal3MachineTemplate")
			newM3mt.Spec = *m3mtWorkers.Spec.DeepCopy()
			newM3mt.SetName("test1-new-workers")
			newM3mt.SetNamespace(namespace)
			err = clusterClient.Create(ctx, newM3mt)
			Expect(err).To(BeNil(), "Failed to create new Metal3MachineTemplate")

			// TODO: Would be nice to actually use a different VM image here
			// instead of just setting the k8s version and template name...
			By("Upgrading KubeadmControlPlane image")
			kcp := framework.GetKubeadmControlPlaneByCluster(ctx, framework.GetKubeadmControlPlaneByClusterInput{
				Lister:      targetCluster.GetClient(),
				ClusterName: "test1",
				Namespace:   "metal3",
			})
			patch := []byte(`{"spec": {
				"version": "v1.21.2",
				"infrastructureTemplate": {
					"name": "test1-new-controlplane"}}}`)
			err = clusterClient.Patch(ctx, kcp, client.RawPatch(types.MergePatchType, patch))
			Expect(err).To(BeNil(), "Failed to patch KubeadmControlPlane")

			By("Waiting for the KubeadmControlPlane to upgrade")
			getBmhList := func() (*bmo.BareMetalHostList, error) {
				bmhs := &bmo.BareMetalHostList{}
				err := clusterClient.List(ctx, bmhs, client.InNamespace(namespace))
				return bmhs, err
			}
			Eventually(func() int {
				return bareMetalHostsWithImage(getBmhList, "test1-new-controlplane")
			},
				e2eConfig.GetIntervals(specName, "wait-machine-upgrade")...).Should(Equal(3))

			By("Waiting for the old Nodes to deprovision")
			Eventually(func() int {
				return bareMetalHostsWithImage(getBmhList, "test1-controlplane")
			},
				e2eConfig.GetIntervals(specName, "wait-machine-upgrade")...).Should(Equal(0))

			By("Untainting new control-plane Nodes")
			controlplaneNodes, err := clientSet.CoreV1().Nodes().List(controlplaneListOptions)
			if err != nil {
				Fail("Unable to list Nodes")
			}
			controlplaneTaint := &corev1.Taint{Key: "node-role.kubernetes.io/master", Effect: corev1.TaintEffectNoSchedule}
			untaintNodes(clientSet, controlplaneNodes, controlplaneTaint)

			By("Scaling up workers to 1")
			machineDeployments := framework.GetMachineDeploymentsByCluster(ctx, framework.GetMachineDeploymentsByClusterInput{
				Lister:      targetCluster.GetClient(),
				ClusterName: "test1",
				Namespace:   "metal3",
			})
			Expect(len(machineDeployments)).To(Equal(1), "Expected exactly 1 MachineDeployment")
			machineDeploy := machineDeployments[0]

			patch = []byte(`{"spec": {"replicas": 1}}`)
			err = clusterClient.Patch(ctx, machineDeploy, client.RawPatch(types.MergePatchType, patch))
			Expect(err).To(BeNil(), "Failed to patch workers MachineDeployment")

			By("Waiting for the worker to join the cluster")
			Eventually(func() (int, error) {
				nodes, err := clientSet.CoreV1().Nodes().List(workersListOptions)
				return len(nodes.Items), err
			},
				e2eConfig.GetIntervals(specName, "wait-worker-nodes")...).Should(Equal(1))
			By("Labling worker for scheduling")
			workerNodes, err := clientSet.CoreV1().Nodes().List(workersListOptions)
			Expect(err).To(BeNil(), "Failed to get Nodes")
			patch = []byte(`{"metadata": {"labels": {"type": "worker"}}}`)
			_, err = clientSet.CoreV1().Nodes().Patch(workerNodes.Items[0].Name, types.MergePatchType, patch)
			Expect(err).To(BeNil(), "Failed to label worker Node")

			By("Deploying workload")
			replicas := int32(10)
			workload := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "workload-1",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicas,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "workload-1",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "workload-1",
							},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "nginx",
									Image: "nginx",
								},
							},
							NodeSelector: map[string]string{
								"type": "worker",
							},
						},
					},
				},
			}
			deploy, err = clientSet.AppsV1().Deployments("default").Create(workload)
			Expect(err).To(BeNil())
			By("Verifying workload running")
			getWorkloadDeployment := func() (*appsv1.Deployment, error) {
				return clientSet.AppsV1().Deployments("default").Get("workload-1", metav1.GetOptions{})
			}
			Eventually(func() bool {
				return deploymentRolledOut(getWorkloadDeployment, deploy.Status.ObservedGeneration+1)
			},
				e2eConfig.GetIntervals(specName, "wait-deployment")...,
			).Should(Equal(true))

			By("Upgrading worker image")
			// Note: We have only 4 nodes (3 control-plane and 1 worker) so we
			// must allow maxUnavailable 1 here or it will get stuck.
			patch = []byte(`{"spec": {
				"template": {
					"spec": {
						"version": "v1.21.2",
						"infrastructureRef": {
							"name": "test1-new-workers"
						},
						"strategy": {
							"rollingUpdate": {
								"maxUnavailable": 1
							}
						}
					}
				}}}`)
			err = clusterClient.Patch(ctx, machineDeploy, client.RawPatch(types.MergePatchType, patch))
			Expect(err).To(BeNil(), "Failed to patch workers MachineDeployment")

			getBmhList = func() (*bmo.BareMetalHostList, error) {
				bmhs := &bmo.BareMetalHostList{}
				err := targetCluster.GetClient().List(ctx, bmhs, client.InNamespace(namespace))
				return bmhs, err
			}
			Eventually(func() int {
				return bareMetalHostsWithImage(getBmhList, "test1-new-workers")
			},
				e2eConfig.GetIntervals(specName, "wait-machine-remediation")...,
			).Should(Equal(1))

			By("Verifying new worker joined cluster")
			Eventually(func() (int, error) {
				nodes, err := clientSet.CoreV1().Nodes().List(workersListOptions)
				return len(nodes.Items), err
			},
				e2eConfig.GetIntervals(specName, "wait-worker-nodes")...).Should(Equal(1))

			By("Verifying Machines use new Kubernetes version")
			machines := &clusterv1.MachineList{}
			err = targetCluster.GetClient().List(ctx, machines, client.InNamespace(namespace))
			Expect(err).To(BeNil(), "Failed to get Machines")

			for _, machine := range machines.Items {
				Expect(*machine.Spec.Version).To(Equal("v1.21.2"), "Machine has wrong k8s version")
			}

			// TODO: Should we check status of workload also after upgrade?

			By("Done upgrading!")
		})
	})

})

func deploymentRolledOut(getDeploymentFunc func() (*appsv1.Deployment, error), desiredGeneration int64) bool {
	deploy, err := getDeploymentFunc()
	if err != nil {
		return false
	}
	if (&deploy.Status != nil) && (&deploy.Status.Replicas != nil) {
		// When the number of replicas is equal to the number of available and updated
		// replicas, we know that only "new" pods are running. When we also
		// have the desired number of replicas and a new enough generation, we
		// know that the rollout is complete.
		return (deploy.Status.UpdatedReplicas == *deploy.Spec.Replicas) &&
			(deploy.Status.AvailableReplicas == *deploy.Spec.Replicas) &&
			(deploy.Status.Replicas == *deploy.Spec.Replicas) &&
			(deploy.Status.ObservedGeneration >= desiredGeneration)
	}
	return false
}

func bareMetalHostsWithImage(getBmhListFunc func() (*bmo.BareMetalHostList, error), image string) int {
	// Count the number of BMHs that fullfill the following:
	// 1. status.provisioning.state == "provisioned"
	// 2. spec.consumerRef.name starts with <image>
	bmhs, err := getBmhListFunc()
	if err != nil {
		return 0
	}
	count := 0
	for i := range bmhs.Items {
		if bmhs.Items[i].Spec.ConsumerRef == nil {
			continue
		}
		consumerName := bmhs.Items[i].Spec.ConsumerRef.Name
		state := bmhs.Items[i].Status.Provisioning.State
		if state == bmo.StateProvisioned && strings.HasPrefix(consumerName, image) {
			count += 1
		}
	}
	return count
}

func untaintNodes(clientSet *kubernetes.Clientset, nodes *corev1.NodeList, taint *corev1.Taint) {
	for i := range nodes.Items {
		updatedNode, changed, err := removeTaint(&nodes.Items[i], taint)
		if err != nil {
			Fail("Unable to untaint Node")
		}
		if changed {
			_, err = clientSet.CoreV1().Nodes().Update(updatedNode)
			if err != nil {
				Fail("Unable to untaint Node")
			}
		}
	}
}

// We could use the implementation in kubernetes, but it is not recommended
// to use as a library.
// https://pkg.go.dev/k8s.io/kubernetes/pkg/util/taints#RemoveTaint
func removeTaint(node *corev1.Node, taint *corev1.Taint) (*corev1.Node, bool, error) {
	newNode := node.DeepCopy()
	nodeTaints := newNode.Spec.Taints
	if len(nodeTaints) == 0 {
		return newNode, false, nil
	}

	if !taintExists(nodeTaints, taint) {
		return newNode, false, nil
	}

	newTaints, _ := deleteTaint(nodeTaints, taint)
	newNode.Spec.Taints = newTaints
	return newNode, true, nil
}

func taintExists(taints []corev1.Taint, taintToFind *corev1.Taint) bool {
	for _, taint := range taints {
		if taint.MatchTaint(taintToFind) {
			return true
		}
	}
	return false
}

func deleteTaint(taints []corev1.Taint, taintToDelete *corev1.Taint) ([]corev1.Taint, bool) {
	newTaints := []corev1.Taint{}
	deleted := false
	for i := range taints {
		if taintToDelete.MatchTaint(&taints[i]) {
			deleted = true
			continue
		}
		newTaints = append(newTaints, taints[i])
	}
	return newTaints, deleted
}

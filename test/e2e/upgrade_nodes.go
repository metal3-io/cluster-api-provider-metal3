package e2e

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	bmo "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1alpha5"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func upgrade_nodes() {
	targetClusterClient := targetCluster.GetClient()
	targetClusterClientSet := targetCluster.GetClientSet()
	kubernetesVersion := e2eConfig.GetVariable("KUBERNETES_VERSION")
	upgradedK8sVersion := e2eConfig.GetVariable("UPGRADED_K8S_VERSION")
	controlplaneTaint := &corev1.Taint{Key: "node-role.kubernetes.io/master", Effect: corev1.TaintEffectNoSchedule}
	Logf("KUBERNETES VERSION: %v", kubernetesVersion)
	Logf("UPGRADED K8S VERSION: %v", upgradedK8sVersion)

	By("Download image")
	IMAGE_NAME := fmt.Sprintf("UBUNTU_20.04_NODE_IMAGE_K8S_%s.qcow2", upgradedK8sVersion)
	Logf("IMAGE_NAME: %v", IMAGE_NAME)
	RAW_IMAGE_NAME := fmt.Sprintf("UBUNTU_20.04_NODE_IMAGE_K8S_%s-raw.img", upgradedK8sVersion)
	Logf("RAW_IMAGE_NAME: %v", RAW_IMAGE_NAME)
	IMAGE_LOCATION := fmt.Sprintf("https://artifactory.nordix.org/artifactory/airship/images/k8s_%s/", upgradedK8sVersion)
	Logf("IMAGE_LOCATION: %v", IMAGE_LOCATION)
	IMAGE_URL := fmt.Sprintf("http://172.22.0.1/images/%s", RAW_IMAGE_NAME)
	Logf("IMAGE_URL: %v", IMAGE_URL)
	IMAGE_CHECKSUM := fmt.Sprintf("http://172.22.0.1/images/%s.md5sum", RAW_IMAGE_NAME)
	Logf("IMAGE_CHECKSUM: %v", IMAGE_CHECKSUM)

	// Check if image does not exist download it
	IRONIC_IMAGE_DIR := "/opt/metal3-dev-env/ironic/html/images"
	if _, err := os.Stat(fmt.Sprintf("%s/%s", IRONIC_IMAGE_DIR, RAW_IMAGE_NAME)); err == nil {
		Logf("Local image %v is found", RAW_IMAGE_NAME)
	} else if os.IsNotExist(err) {
		Logf("Local image %v/%v is not found", IRONIC_IMAGE_DIR, RAW_IMAGE_NAME)
		err = downloadFile(fmt.Sprintf("%s/%s", IRONIC_IMAGE_DIR, IMAGE_NAME), fmt.Sprintf("%s/%s", IMAGE_LOCATION, IMAGE_NAME))
		checkError(err)
		cmd := exec.Command("qemu-img", "convert", "-O", "raw", fmt.Sprintf("%s/%s", IRONIC_IMAGE_DIR, IMAGE_NAME), fmt.Sprintf("%s/%s", IRONIC_IMAGE_DIR, RAW_IMAGE_NAME))
		err = cmd.Run()
		checkError(err)
		cmd = exec.Command("md5sum", fmt.Sprintf("%s/%s", IRONIC_IMAGE_DIR, RAW_IMAGE_NAME))
		output, err := cmd.CombinedOutput()
		checkError(err)
		md5sum := strings.Fields(string(output))[0]
		err = ioutil.WriteFile(fmt.Sprintf("%s/%s.md5sum", IRONIC_IMAGE_DIR, RAW_IMAGE_NAME), []byte(md5sum), 0777)
		checkError(err)

	} else {
		checkError(err)
	}

	By("Creating new Metal3MachineTemplates")
	m3mtControlplane := &capm3.Metal3MachineTemplate{}
	newM3mt := &capm3.Metal3MachineTemplate{}

	err := targetClusterClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "test1-controlplane"}, m3mtControlplane)
	Expect(err).To(BeNil(), "Failed to get Metal3MachineTemplate")
	newM3mt.Spec = *m3mtControlplane.Spec.DeepCopy()
	newM3mt.SetName("test1-new-controlplane")
	newM3mt.SetNamespace(namespace)
	newM3mt.Spec.Template.Spec.Image.URL = IMAGE_URL
	newM3mt.Spec.Template.Spec.Image.Checksum = IMAGE_CHECKSUM
	err = targetClusterClient.Create(ctx, newM3mt)
	Expect(err).To(BeNil(), "Failed to create new Metal3MachineTemplate")

	By("Upgrading KubeadmControlPlane image")
	kcp := framework.GetKubeadmControlPlaneByCluster(ctx, framework.GetKubeadmControlPlaneByClusterInput{
		Lister:      targetClusterClient,
		ClusterName: "test1",
		Namespace:   namespace,
	})
	patch := []byte(fmt.Sprintf(`{
		"spec": {
		  "rolloutStrategy": {
			"rollingUpdate": {
			  "maxSurge": 0
			}
		  },
		  "version": "%s",
		  "machineTemplate": {
			"infrastructureRef": {
			  "name": "test1-new-controlplane"
			}
		  }
		}
	  }`, upgradedK8sVersion))
	err = targetClusterClient.Patch(ctx, kcp, client.RawPatch(types.MergePatchType, patch))
	Expect(err).To(BeNil(), "Failed to patch KubeadmControlPlane")

	By("Waiting for the KubeadmControlPlane to upgrade")
	getBmhList := func() (*bmo.BareMetalHostList, error) {
		bmhs := &bmo.BareMetalHostList{}
		err := targetClusterClient.List(ctx, bmhs, client.InNamespace(namespace))
		return bmhs, err
	}
	Eventually(func() int {
		return bareMetalHostsWithTemplateName(getBmhList, "test1-new-controlplane")
	},
		e2eConfig.GetIntervals(specName, "wait-machine-upgrade")...).Should(Equal(3))

	By("Waiting for the old Nodes to deprovision")
	Eventually(func() int {
		return bareMetalHostsWithTemplateName(getBmhList, "test1-controlplane")
	},
		e2eConfig.GetIntervals(specName, "wait-machine-upgrade")...).Should(Equal(0))

	// Repeat for the workers
	By("Creating new Metal3MachineTemplates for worker")
	m3mtWorkers := &capm3.Metal3MachineTemplate{}
	newM3mt = &capm3.Metal3MachineTemplate{}
	err = targetClusterClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "test1-workers"}, m3mtWorkers)
	Expect(err).To(BeNil(), "Failed to get Metal3MachineTemplate")
	newM3mt.Spec = *m3mtWorkers.Spec.DeepCopy()
	newM3mt.SetName("test1-new-workers")
	newM3mt.SetNamespace(namespace)
	err = targetClusterClient.Create(ctx, newM3mt)
	Expect(err).To(BeNil(), "Failed to create new Metal3MachineTemplate")

	By("Labling worker for scheduling")
	// Labels and selectors
	// workerNodesLabel := "node-role.kubernetes.io/control-plane"
	workerNodesRequirement, err := labels.NewRequirement("node-role.kubernetes.io/control-plane", selection.DoesNotExist, []string{})
	if err != nil {
		Fail("Failed to set up worker Node requirements")
	}
	workerNodesSelector := labels.NewSelector().Add(*workerNodesRequirement)
	workersListOptions := metav1.ListOptions{LabelSelector: workerNodesSelector.String()}
	workerNodes, err := targetClusterClientSet.CoreV1().Nodes().List(ctx, workersListOptions)
	Expect(err).To(BeNil(), "Failed to get Nodes")
	patch = []byte(`{"metadata": {"labels": {"type": "worker"}}}`)
	_, err = targetClusterClientSet.CoreV1().Nodes().Patch(ctx, workerNodes.Items[0].Name, types.MergePatchType, patch, metav1.PatchOptions{})
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
	deploy, err := targetClusterClientSet.AppsV1().Deployments("default").Create(ctx, workload, metav1.CreateOptions{})
	Expect(err).To(BeNil())

	By("Verifying workload running")
	getWorkloadDeployment := func() (*appsv1.Deployment, error) {
		return targetClusterClientSet.AppsV1().Deployments("default").Get(ctx, "workload-1", metav1.GetOptions{})
	}
	Eventually(func() bool {
		return deploymentRolledOut(getWorkloadDeployment, deploy.Status.ObservedGeneration+1)
	},
		e2eConfig.GetIntervals(specName, "wait-deployment")...,
	).Should(Equal(true))

	By("Untaint all CP nodes before upgrading machinedeployment")
	controlplaneNodes := getControlplaneNodes(targetClusterClientSet)
	untaintNodes(targetClusterClientSet, controlplaneNodes, controlplaneTaint)

	By("Upgrading worker image")
	// Note: We have only 4 nodes (3 control-plane and 1 worker) so we
	// must allow maxUnavailable 1 here or it will get stuck.
	patch = []byte(fmt.Sprintf(`{
		"spec": {
		  "strategy": {
			"rollingUpdate": {
			  "maxSurge": 0,
			  "maxUnavailable": 1
			}
		  },
		  "template": {
			"spec": {
			  "version": "%s",
			  "infrastructureRef": {
				"name": "test1-new-workers"
			  }
			}
		  }
		}
	  }`, upgradedK8sVersion))
	machineDeployments := framework.GetMachineDeploymentsByCluster(ctx, framework.GetMachineDeploymentsByClusterInput{
		Lister:      targetClusterClient,
		ClusterName: "test1",
		Namespace:   namespace,
	})

	Expect(len(machineDeployments)).To(Equal(1), "Expected exactly 1 MachineDeployment")
	machineDeploy := machineDeployments[0]
	err = targetClusterClient.Patch(ctx, machineDeploy, client.RawPatch(types.MergePatchType, patch))

	Expect(err).To(BeNil(), "Failed to patch workers MachineDeployment")

	getBmhList = func() (*bmo.BareMetalHostList, error) {
		bmhs := &bmo.BareMetalHostList{}
		err := targetClusterClient.List(ctx, bmhs, client.InNamespace(namespace))
		return bmhs, err
	}
	Eventually(func() int {
		return bareMetalHostsWithTemplateName(getBmhList, "test1-new-workers")
	},
		e2eConfig.GetIntervals(specName, "wait-machine-upgrade")...,
	).Should(Equal(1))

	By("Verifying new worker joined cluster")
	Eventually(func() (int, error) {
		nodes, err := targetClusterClientSet.CoreV1().Nodes().List(ctx, workersListOptions)
		return len(nodes.Items), err
	},
		e2eConfig.GetIntervals(specName, "wait-worker-nodes")...).Should(Equal(1))

	By("Verifying Machines use new Kubernetes version")
	machines := &clusterv1.MachineList{}
	err = targetClusterClient.List(ctx, machines, client.InNamespace(namespace))
	Expect(err).To(BeNil(), "Failed to get Machines")

	for _, machine := range machines.Items {
		Expect(*machine.Spec.Version).To(Equal(upgradedK8sVersion), "Machine has wrong k8s version")
	}
}

func bareMetalHostsWithTemplateName(getBmhListFunc func() (*bmo.BareMetalHostList, error), templateName string) int {
	// Count the number of BMHs that fullfill the following:
	// 1. status.provisioning.state == "provisioned"
	// 2. spec.consumerRef.name starts with <templateName>
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
		Logf("DEBUG %v) %v: %v %v", i, bmhs.Items[i].Name, consumerName, state)
		if state == bmo.StateProvisioned && strings.HasPrefix(consumerName, templateName) {
			count += 1
		}
	}
	return count
}
func deploymentRolledOut(getDeploymentFunc func() (*appsv1.Deployment, error), desiredGeneration int64) bool {
	deploy, err := getDeploymentFunc()
	if err != nil {
		return false
	}

	// When the number of replicas is equal to the number of available and updated
	// replicas, we know that only "new" pods are running. When we also
	// have the desired number of replicas and a new enough generation, we
	// know that the rollout is complete.
	return (deploy.Status.UpdatedReplicas == *deploy.Spec.Replicas) &&
		(deploy.Status.AvailableReplicas == *deploy.Spec.Replicas) &&
		(deploy.Status.Replicas == *deploy.Spec.Replicas) &&
		(deploy.Status.ObservedGeneration >= desiredGeneration)

}

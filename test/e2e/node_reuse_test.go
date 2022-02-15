package e2e

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"time"

	bmo "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func nodeReuse() {
	Logf("Starting node reuse tests")
	var (
		targetClusterClient  = targetCluster.GetClient()
		clientSet            = targetCluster.GetClientSet()
		kubernetesVersion    = e2eConfig.GetVariable("KUBERNETES_VERSION")
		upgradedK8sVersion   = e2eConfig.GetVariable("UPGRADED_K8S_VERSION")
		numberOfControlplane = int(*e2eConfig.GetInt32PtrVariable("CONTROL_PLANE_MACHINE_COUNT"))
		numberOfWorkers      = int(*e2eConfig.GetInt32PtrVariable("WORKER_MACHINE_COUNT"))
		numberOfAllBmh       = numberOfControlplane + numberOfWorkers
		controlplaneTaint    = &corev1.Taint{Key: "node-role.kubernetes.io/master", Effect: corev1.TaintEffectNoSchedule}
		imageNamePrefix      string
	)

	const (
		artifactoryURL = "https://artifactory.nordix.org/artifactory/metal3/images/k8s"
		imagesURL      = "http://172.22.0.1/images"
		ironicImageDir = "/opt/metal3-dev-env/ironic/html/images"
		nodeReuseLabel = "infrastructure.cluster.x-k8s.io/node-reuse"
	)

	Logf("KUBERNETES VERSION: %v", kubernetesVersion)
	Logf("UPGRADED K8S VERSION: %v", upgradedK8sVersion)
	Logf("NUMBER OF CONTROLPLANE BMH: %v", numberOfControlplane)
	Logf("NUMBER OF WORKER BMH: %v", numberOfWorkers)

	By("Untaint all CP nodes before scaling down machinedeployment")
	controlplaneNodes := getControlplaneNodes(clientSet)
	untaintNodes(targetClusterClient, controlplaneNodes, controlplaneTaint)

	By("Scale down MachineDeployment to 0")
	scaleMachineDeployment(ctx, targetClusterClient, clusterName, namespace, 0)

	Byf("Wait until the worker is scaled down and %d BMH(s) Available", numberOfWorkers)
	Eventually(
		func(g Gomega) {
			bmhs := bmo.BareMetalHostList{}
			g.Expect(targetClusterClient.List(ctx, &bmhs, client.InNamespace(namespace))).To(Succeed())
			filtered := filterBmhsByProvisioningState(bmhs.Items, bmo.StateAvailable)
			g.Expect(len(filtered)).To(Equal(numberOfWorkers))
		}, e2eConfig.GetIntervals(specName, "wait-bmh-available")...,
	).Should(Succeed())

	By("Get the provisioned BMH names and UUIDs")
	kcpBmhBeforeUpgrade := getProvisionedBmhNamesUuids(targetClusterClient)

	By("Download image")
	osType := strings.ToLower(os.Getenv("OS"))
	Expect(osType).ToNot(Equal(""))
	if osType != "centos" {
		imageNamePrefix = "UBUNTU_20.04_NODE_IMAGE_K8S"
	} else {
		imageNamePrefix = "CENTOS_8_NODE_IMAGE_K8S"
	}
	imageName := fmt.Sprintf("%s_%s.qcow2", imageNamePrefix, upgradedK8sVersion)
	Logf("IMAGE_NAME: %v", imageName)
	rawImageName := fmt.Sprintf("%s_%s-raw.img", imageNamePrefix, upgradedK8sVersion)
	Logf("RAW_IMAGE_NAME: %v", rawImageName)
	imageLocation := fmt.Sprintf("%s_%s/", artifactoryURL, upgradedK8sVersion)
	Logf("IMAGE_LOCATION: %v", imageLocation)
	imageURL := fmt.Sprintf("%s/%s", imagesURL, rawImageName)
	Logf("IMAGE_URL: %v", imageURL)
	imageChecksum := fmt.Sprintf("%s/%s.md5sum", imagesURL, rawImageName)
	Logf("IMAGE_CHECKSUM: %v", imageChecksum)

	// Check if node image with upgraded k8s version exist, if not download it
	if _, err := os.Stat(fmt.Sprintf("%s/%s", ironicImageDir, rawImageName)); err == nil {
		Logf("Local image %v is found", rawImageName)
	} else if os.IsNotExist(err) {
		Logf("Local image %v/%v is not found", ironicImageDir, rawImageName)
		err = downloadFile(fmt.Sprintf("%s/%s", ironicImageDir, imageName), fmt.Sprintf("%s/%s", imageLocation, imageName))
		Expect(err).To(BeNil())
		cmd := exec.Command("qemu-img", "convert", "-O", "raw", fmt.Sprintf("%s/%s", ironicImageDir, imageName), fmt.Sprintf("%s/%s", ironicImageDir, rawImageName))
		err = cmd.Run()
		Expect(err).To(BeNil())
		cmd = exec.Command("md5sum", fmt.Sprintf("%s/%s", ironicImageDir, rawImageName))
		output, err := cmd.CombinedOutput()
		Expect(err).To(BeNil())
		md5sum := strings.Fields(string(output))[0]
		err = os.WriteFile(fmt.Sprintf("%s/%s.md5sum", ironicImageDir, rawImageName), []byte(md5sum), 0777)
		Expect(err).To(BeNil())
	} else {
		fmt.Fprintf(GinkgoWriter, "ERROR: %v\n", err)
		os.Exit(1)
	}
	By("Update KCP Metal3MachineTemplate with upgraded image to boot and set nodeReuse field to 'True'")
	m3machineTemplateName := fmt.Sprintf("%s-controlplane", clusterName)
	updateNodeReuse(true, m3machineTemplateName, targetClusterClient)
	updateBootImage(m3machineTemplateName, targetClusterClient, imageURL, imageChecksum, "raw", "md5")

	Byf("Update KCP to upgrade k8s version and binaries from %s to %s", kubernetesVersion, upgradedK8sVersion)
	kcpObj := framework.GetKubeadmControlPlaneByCluster(ctx, framework.GetKubeadmControlPlaneByClusterInput{
		Lister:      targetClusterClient,
		ClusterName: clusterName,
		Namespace:   namespace,
	})
	patch := []byte(fmt.Sprintf(`{
		"spec": {
			"rolloutStrategy": {
				"rollingUpdate": {
					"maxSurge": 0
				}
			},
			"version": "%s"
		}
	}`, upgradedK8sVersion))
	err := targetClusterClient.Patch(ctx, kcpObj, client.RawPatch(types.MergePatchType, patch))
	Expect(err).To(BeNil(), "Failed to patch KubeadmControlPlane")

	By("Check if only a single machine is in Deleting state and no other new machines are in Provisioning state")
	Eventually(
		func(g Gomega) {
			machines := &clusterv1.MachineList{}
			g.Expect(targetCluster.GetClient().List(ctx, machines, client.InNamespace(namespace))).To(Succeed())
			deletingCount := 0
			for _, machine := range machines.Items {
				Expect(machine.Status.GetTypedPhase() == clusterv1.MachinePhaseProvisioning).To(BeFalse()) // Ensure no machine is provisioning
				if machine.Status.GetTypedPhase() == clusterv1.MachinePhaseDeleting {
					deletingCount++
				}
			}
			g.Expect(deletingCount).To(Equal(1))
		}, e2eConfig.GetIntervals(specName, "wait-machine-deleting")...,
	).Should(Succeed())

	Byf("Wait until 1 BMH is in deprovisioning state")
	deprovisioningBmh := []bmo.BareMetalHost{}
	Eventually(
		func(g Gomega) {
			bmhs := bmo.BareMetalHostList{}
			g.Expect(targetClusterClient.List(ctx, &bmhs, client.InNamespace(namespace))).To(Succeed())
			deprovisioningBmh = filterBmhsByProvisioningState(bmhs.Items, bmo.StateDeprovisioning)
			g.Expect(len(deprovisioningBmh)).To(Equal(1))
		}, e2eConfig.GetIntervals(specName, "wait-bmh-deprovisioning")...,
	).Should(Succeed(), "No BMH is in deprovisioning state")

	By("Wait until above deprovisioning BMH is in available state again")
	Eventually(
		func(g Gomega) {
			bmhs := bmo.BareMetalHostList{}
			g.Expect(targetClusterClient.List(ctx, &bmhs, client.InNamespace(namespace))).To(Succeed())

			bmh, err := getBmhByName(bmhs.Items, deprovisioningBmh[0].Name)
			g.Expect(err).To(BeNil())
			g.Expect(bmh.Status.Provisioning.State).To(Equal(bmo.StateAvailable), "The BMH [%s] is not available yet", deprovisioningBmh[0].Name)
		}, e2eConfig.GetIntervals(specName, "wait-bmh-deprovisioning-available")...,
	).Should(Succeed())

	By("Check if just deprovisioned BMH re-used for the next provisioning")
	Eventually(
		func(g Gomega) {
			bmhs := bmo.BareMetalHostList{}
			g.Expect(targetClusterClient.List(ctx, &bmhs, client.InNamespace(namespace))).To(Succeed())

			bmh, err := getBmhByName(bmhs.Items, deprovisioningBmh[0].Name)
			g.Expect(err).To(BeNil())
			g.Expect(bmh.Status.Provisioning.State).To(Equal(bmo.StateProvisioning), "The BMH [%s]  is not provisioning yet", deprovisioningBmh[0].Name)
		}, e2eConfig.GetIntervals(specName, "wait-bmh-available-provisioning")...,
	).Should(Succeed())

	Byf("Wait until two machines become running and updated with the new %s k8s version", upgradedK8sVersion)
	Eventually(
		func(g Gomega) {
			machines := &clusterv1.MachineList{}
			g.Expect(targetClusterClient.List(ctx, machines, client.InNamespace(namespace))).To(Succeed())

			runningUpgradedLen := 0
			for _, machine := range machines.Items {
				if machine.Status.GetTypedPhase() == clusterv1.MachinePhaseRunning && *machine.Spec.Version == upgradedK8sVersion {
					runningUpgradedLen++
					Logf("Machine [%v] is upgraded to k8s version (%v) and in running state", machine.Name, upgradedK8sVersion)
				}
			}
			g.Expect(runningUpgradedLen).To(Equal(2))
		}, e2eConfig.GetIntervals(specName, "wait-machine-running")...,
	).Should(Succeed())

	By("Untaint CP nodes after upgrade of two controlplane nodes")
	controlplaneNodes = getControlplaneNodes(clientSet)
	untaintNodes(targetClusterClient, controlplaneNodes, controlplaneTaint)

	Byf("Wait until all %v KCP machines become running and updated with new %s k8s version", numberOfControlplane, upgradedK8sVersion)
	Eventually(
		func(g Gomega) {
			machines := &clusterv1.MachineList{}
			g.Expect(targetClusterClient.List(ctx, machines, client.InNamespace(namespace))).To(Succeed())

			runningUpgradedLen := 0
			for _, machine := range machines.Items {
				if machine.Status.GetTypedPhase() == clusterv1.MachinePhaseRunning && *machine.Spec.Version == upgradedK8sVersion {
					runningUpgradedLen++
					Logf("Machine [%v] is upgraded to k8s version (%v) and in running state", machine.Name, upgradedK8sVersion)
				}
			}
			g.Expect(runningUpgradedLen).To(Equal(numberOfControlplane))
		}, e2eConfig.GetIntervals(specName, "wait-machine-running")...,
	).Should(Succeed())

	By("Get the provisioned BMH names and UUIDs after upgrade")
	kcpBmhAfterUpgrade := getProvisionedBmhNamesUuids(targetClusterClient)

	By("Check difference between before and after upgrade mappings")
	equal := reflect.DeepEqual(kcpBmhBeforeUpgrade, kcpBmhAfterUpgrade)
	Expect(equal).To(BeTrue(), "The same BMHs were not reused in KubeadmControlPlane test case")

	By("Put maxSurge field in KubeadmControlPlane back to default value(1)")
	ctrlplane := controlplanev1.KubeadmControlPlane{}
	Expect(targetClusterClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: clusterName}, &ctrlplane)).To(Succeed())
	patch = []byte(`{
		"spec": {
			"rolloutStrategy": {
				"rollingUpdate": {
					"maxSurge": 1
				}
			}
		}
	}`)
	// Retry if failed to patch
	for retry := 0; retry < 3; retry++ {
		err = targetClusterClient.Patch(ctx, &ctrlplane, client.RawPatch(types.MergePatchType, patch))
		if err == nil {
			break
		}
		time.Sleep(30 * time.Second)
	}

	By("Untaint all CP nodes")
	// The rest of CP nodes may take time to be untaintable
	// We have untainted the 2 first CPs
	for untaintedNodeCount := 0; untaintedNodeCount < numberOfControlplane-2; {
		controlplaneNodes = getControlplaneNodes(clientSet)
		untaintedNodeCount = untaintNodes(targetClusterClient, controlplaneNodes, controlplaneTaint)
		time.Sleep(10 * time.Second)
	}

	By("Scale the controlplane down to 1")
	scaleKubeadmControlPlane(ctx, targetClusterClient, client.ObjectKey{Namespace: namespace, Name: clusterName}, 1)

	Byf("Wait until controlplane is scaled down and %d BMHs are Available", numberOfControlplane)
	Eventually(
		func(g Gomega) {
			bmhs := bmo.BareMetalHostList{}
			g.Expect(targetClusterClient.List(ctx, &bmhs, client.InNamespace(namespace))).To(Succeed())

			availableBmhs := filterBmhsByProvisioningState(bmhs.Items, bmo.StateAvailable)
			availableBmhsLength := len(availableBmhs)
			g.Expect(availableBmhsLength).To(Equal(numberOfControlplane), "BMHs available are %d not equal to %d", availableBmhsLength, numberOfControlplane)
		}, e2eConfig.GetIntervals(specName, "wait-cp-available")...,
	).Should(Succeed())

	By("Get MachineDeployment")
	machineDeployments := framework.GetMachineDeploymentsByCluster(ctx, framework.GetMachineDeploymentsByClusterInput{
		Lister:      targetClusterClient,
		ClusterName: clusterName,
		Namespace:   namespace,
	})
	Expect(len(machineDeployments)).To(Equal(1), "Expected exactly 1 MachineDeployment")
	machineDeploy := machineDeployments[0]

	By("Get Metal3MachineTemplate name for MachineDeployment")
	m3machineTemplateName = fmt.Sprintf("%s-workers", clusterName)

	By("Point to proper Metal3MachineTemplate in MachineDeployment")
	pointMDtoM3mt(m3machineTemplateName, machineDeploy.Name, targetClusterClient)

	By("Scale the worker up to 1 to start testing MachineDeployment")
	scaleMachineDeployment(ctx, targetClusterClient, clusterName, namespace, 1)

	Byf("Wait until %d more BMH becomes provisioned", numberOfWorkers)
	Eventually(
		func(g Gomega) int {
			bmhs := bmo.BareMetalHostList{}
			g.Expect(targetClusterClient.List(ctx, &bmhs, client.InNamespace(namespace))).To(Succeed())
			provisionedBmhs := filterBmhsByProvisioningState(bmhs.Items, bmo.StateProvisioned)
			return len(provisionedBmhs)
		}, e2eConfig.GetIntervals(specName, "wait-bmh-provisioned")...,
	).Should(Equal(2))

	Byf("Wait until %d more machine becomes running", numberOfWorkers)
	Eventually(
		func(g Gomega) int {
			machines := &clusterv1.MachineList{}
			g.Expect(targetClusterClient.List(ctx, machines, client.InNamespace(namespace))).To(Succeed())

			runningMachines := filterMachinesByStatusPhase(machines.Items, clusterv1.MachinePhaseRunning)
			return len(runningMachines)
		}, e2eConfig.GetIntervals(specName, "wait-machine-running")...,
	).Should(Equal(2))

	By("Get the provisioned BMH names and UUIDs before starting upgrade in MachineDeployment")
	mdBmhBeforeUpgrade := getProvisionedBmhNamesUuids(targetClusterClient)

	By("List all available BMHs, remove nodeReuse label from them if any")
	bmhs := bmo.BareMetalHostList{}
	Expect(targetClusterClient.List(ctx, &bmhs, client.InNamespace(namespace))).To(Succeed())
	for _, item := range bmhs.Items {
		if item.Status.Provisioning.State == bmo.StateAvailable {
			// We make sure that all available BMHs are choosable by removing nodeReuse label
			// set on them while testing KCP node reuse scenario previously.
			deleteNodeReuseLabelFromHost(ctx, targetClusterClient, item, nodeReuseLabel)
		}
	}

	By("Update MD Metal3MachineTemplate with upgraded image to boot and set nodeReuse field to 'True'")
	updateNodeReuse(true, m3machineTemplateName, targetClusterClient)
	updateBootImage(m3machineTemplateName, targetClusterClient, imageURL, imageChecksum, "raw", "md5")

	Byf("Update MD to upgrade k8s version and binaries from %s to %s", kubernetesVersion, upgradedK8sVersion)
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
					"version": "%s"
				}
			}
		}
	}`, upgradedK8sVersion))

	err = targetClusterClient.Patch(ctx, machineDeploy, client.RawPatch(types.MergePatchType, patch))
	Expect(err).To(BeNil(), "Failed to patch MachineDeployment")

	Byf("Wait until %d BMH(s) in deprovisioning state", numberOfWorkers)
	deprovisioningBmh = []bmo.BareMetalHost{}
	Eventually(
		func(g Gomega) int {
			bmhs := bmo.BareMetalHostList{}
			g.Expect(targetClusterClient.List(ctx, &bmhs, client.InNamespace(namespace))).To(Succeed())
			deprovisioningBmh = filterBmhsByProvisioningState(bmhs.Items, bmo.StateDeprovisioning)
			return len(deprovisioningBmh)
		},
		e2eConfig.GetIntervals(specName, "wait-bmh-deprovisioning")...,
	).Should(Equal(numberOfWorkers), "Deprovisioning BMHs are not equal to %d", numberOfWorkers)

	By("Wait until the above deprovisioning BMH is in available state again")
	Eventually(
		func(g Gomega) {
			bmhs := bmo.BareMetalHostList{}
			g.Expect(targetClusterClient.List(ctx, &bmhs, client.InNamespace(namespace))).To(Succeed())
			bmh, err := getBmhByName(bmhs.Items, deprovisioningBmh[0].Name)
			g.Expect(err).To(BeNil())
			g.Expect(bmh.Status.Provisioning.State).To(Equal(bmo.StateAvailable))
		},
		e2eConfig.GetIntervals(specName, "wait-bmh-deprovisioning-available")...,
	).Should(Succeed())

	By("Check if just deprovisioned BMH re-used for next provisioning")
	Eventually(
		func(g Gomega) {
			bmhs := bmo.BareMetalHostList{}
			g.Expect(targetClusterClient.List(ctx, &bmhs, client.InNamespace(namespace))).To(Succeed())
			bmh, err := getBmhByName(bmhs.Items, deprovisioningBmh[0].Name)
			g.Expect(err).To(BeNil())
			g.Expect(bmh.Status.Provisioning.State).To(Equal(bmo.StateProvisioning))
		},
		e2eConfig.GetIntervals(specName, "wait-bmh-available-provisioning")...,
	).Should(Succeed())

	Byf("Wait until worker machine becomes running and updated with new %s k8s version", upgradedK8sVersion)
	Eventually(
		func(g Gomega) int {
			machines := &clusterv1.MachineList{}
			g.Expect(targetClusterClient.List(ctx, machines, client.InNamespace(namespace))).To(Succeed())
			runningUpgradedLen := 0
			for _, machine := range machines.Items {
				if machine.Status.GetTypedPhase() == clusterv1.MachinePhaseRunning && *machine.Spec.Version == upgradedK8sVersion {
					runningUpgradedLen++
					Logf("Machine [%v] is upgraded to (%v) and running", machine.Name, upgradedK8sVersion)
				}
			}
			return runningUpgradedLen
		}, e2eConfig.GetIntervals(specName, "wait-machine-running")...,
	).Should(Equal(2))

	By("Get provisioned BMH names and UUIDs after upgrade in MachineDeployment")
	mdBmhAfterUpgrade := getProvisionedBmhNamesUuids(targetClusterClient)

	By("Check difference between before and after upgrade mappings in MachineDeployment")
	equal = reflect.DeepEqual(mdBmhBeforeUpgrade, mdBmhAfterUpgrade)
	Expect(equal).To(BeTrue(), "The same BMHs were not reused in MachineDeployment")

	By("Scale controlplane up to 3")
	scaleKubeadmControlPlane(ctx, targetClusterClient, client.ObjectKey{Namespace: namespace, Name: clusterName}, 3)

	Byf("Wait until all %d bmhs are provisioned", numberOfAllBmh)
	Eventually(
		func(g Gomega) {
			bmhs = bmo.BareMetalHostList{}
			g.Expect(targetClusterClient.List(ctx, &bmhs, client.InNamespace(namespace))).To(Succeed())
			provisionedBmh := filterBmhsByProvisioningState(bmhs.Items, bmo.StateProvisioned)
			g.Expect(len(provisionedBmh)).To(Equal(numberOfAllBmh))
		}, e2eConfig.GetIntervals(specName, "wait-bmh-provisioned")...,
	).Should(Succeed())

	Byf("Wait until all %d machine(s) become(s) running", numberOfAllBmh)
	Eventually(
		func(g Gomega) int {
			machines := &clusterv1.MachineList{}
			g.Expect(targetClusterClient.List(ctx, machines, client.InNamespace(namespace))).To(Succeed())
			runningMachines := filterMachinesByStatusPhase(machines.Items, clusterv1.MachinePhaseRunning)
			return len(runningMachines)
		},
		e2eConfig.GetIntervals(specName, "wait-machine-running")...,
	).Should(Equal(numberOfAllBmh))

	By("NODE REUSE TESTS PASSED!")
}

func getControlplaneNodes(clientSet *kubernetes.Clientset) *corev1.NodeList {
	controlplaneNodesRequirement, err := labels.NewRequirement("node-role.kubernetes.io/control-plane", selection.Exists, []string{})
	Expect(err).To(BeNil(), "Failed to set up worker Node requirements")
	controlplaneNodesSelector := labels.NewSelector().Add(*controlplaneNodesRequirement)
	controlplaneListOptions = metav1.ListOptions{LabelSelector: controlplaneNodesSelector.String()}
	controlplaneNodes, err := clientSet.CoreV1().Nodes().List(ctx, controlplaneListOptions)
	Expect(err).To(BeNil(), "Failed to get controlplane nodes")
	return controlplaneNodes
}

func getProvisionedBmhNamesUuids(clusterClient client.Client) []string {
	bmhs := bmo.BareMetalHostList{}
	var nameUUIDList []string
	Expect(clusterClient.List(ctx, &bmhs, client.InNamespace(namespace))).To(Succeed())
	for _, item := range bmhs.Items {
		if item.WasProvisioned() {
			concat := "metal3/" + item.Name + "=metal3://" + (string)(item.UID)
			nameUUIDList = append(nameUUIDList, concat)
		}
	}
	return nameUUIDList
}

func updateNodeReuse(nodeReuse bool, m3machineTemplateName string, clusterClient client.Client) {
	m3machineTemplate := capm3.Metal3MachineTemplate{}
	Expect(clusterClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: m3machineTemplateName}, &m3machineTemplate)).To(Succeed())
	helper, err := patch.NewHelper(&m3machineTemplate, clusterClient)
	Expect(err).NotTo(HaveOccurred())
	m3machineTemplate.Spec.NodeReuse = nodeReuse
	Expect(helper.Patch(ctx, &m3machineTemplate)).To(Succeed())

	// verify that nodeReuse field is updated
	Expect(clusterClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: m3machineTemplateName}, &m3machineTemplate)).To(Succeed())
	Expect(m3machineTemplate.Spec.NodeReuse).To(BeEquivalentTo(nodeReuse))
}

func pointMDtoM3mt(m3mtname, mdName string, clusterClient client.Client) {
	md := clusterv1.MachineDeployment{}
	Expect(clusterClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: mdName}, &md)).To(Succeed())
	helper, err := patch.NewHelper(&md, clusterClient)
	Expect(err).NotTo(HaveOccurred())
	md.Spec.Template.Spec.InfrastructureRef.Name = m3mtname
	Expect(helper.Patch(ctx, &md)).To(Succeed())

	// verify that MachineDeployment is pointing to exact m3mt where nodeReuse is set to 'True'
	Expect(clusterClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: mdName}, &md)).To(Succeed())
	Expect(md.Spec.Template.Spec.InfrastructureRef.Name).To(BeEquivalentTo(fmt.Sprintf("%s-workers", clusterName)))
}

func updateBootImage(m3machineTemplateName string, clusterClient client.Client, imageURL string, imageChecksum string, checksumType string, imageFormat string) {
	m3machineTemplate := capm3.Metal3MachineTemplate{}
	Expect(clusterClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: m3machineTemplateName}, &m3machineTemplate)).To(Succeed())
	helper, err := patch.NewHelper(&m3machineTemplate, clusterClient)
	Expect(err).NotTo(HaveOccurred())
	m3machineTemplate.Spec.Template.Spec.Image.URL = imageURL
	m3machineTemplate.Spec.Template.Spec.Image.Checksum = imageChecksum
	m3machineTemplate.Spec.Template.Spec.Image.DiskFormat = &checksumType
	m3machineTemplate.Spec.Template.Spec.Image.ChecksumType = &imageFormat
	Expect(helper.Patch(ctx, &m3machineTemplate)).To(Succeed())
}

func untaintNodes(targetClusterClient client.Client, nodes *corev1.NodeList, taint *corev1.Taint) (count int) {
	count = 0
	for i := range nodes.Items {
		Logf("Untainting node %v ...", nodes.Items[i].Name)
		newNode, changed := removeTaint(&nodes.Items[i], taint)
		if changed {
			patchHelper, err := patch.NewHelper(&nodes.Items[i], targetClusterClient)
			Expect(err).To(BeNil())
			Expect(patchHelper.Patch(ctx, newNode)).To(Succeed(), "Failed to patch node")
			count++
		}
	}
	return
}

func removeTaint(node *corev1.Node, taint *corev1.Taint) (*corev1.Node, bool) {
	newNode := node.DeepCopy()
	nodeTaints := newNode.Spec.Taints
	if len(nodeTaints) == 0 {
		return newNode, false
	}

	if !taintExists(nodeTaints, taint) {
		return newNode, false
	}

	newTaints, _ := deleteTaint(nodeTaints, taint)
	newNode.Spec.Taints = newTaints
	return newNode, true
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

func filterMachinesByStatusPhase(machines []clusterv1.Machine, phase clusterv1.MachinePhase) (result []clusterv1.Machine) {
	for _, machine := range machines {
		if machine.Status.GetTypedPhase() == phase {
			result = append(result, machine)
		}
	}
	return
}

func getBmhByName(bmhs []bmo.BareMetalHost, name string) (bmo.BareMetalHost, error) {
	for _, bmh := range bmhs {
		if bmh.Name == name {
			return bmh, nil
		}
	}
	return bmo.BareMetalHost{}, errors.New("BMH is not found")
}

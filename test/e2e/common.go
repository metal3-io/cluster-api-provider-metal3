package e2e

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	bmo "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Byf(format string, a ...interface{}) {
	By(fmt.Sprintf(format, a...))
}

func Logf(format string, a ...interface{}) {
	fmt.Fprintf(GinkgoWriter, "INFO: "+format+"\n", a...)
}

func LogFromFile(logFile string) {
	data, err := os.ReadFile(filepath.Clean(logFile))
	Expect(err).To(BeNil(), "No log file found")
	Logf(string(data))
}

func dumpSpecResourcesAndCleanup(ctx context.Context, specName string, clusterProxy framework.ClusterProxy, artifactFolder string, namespace string, intervalsGetter func(spec, key string) []interface{}, clusterName, clusterctlLogFolder string, skipCleanup bool) {
	Expect(os.RemoveAll(clusterctlLogFolder)).Should(Succeed())
	client := clusterProxy.GetClient()

	// Dumps all the resources in the spec namespace, then cleanups the cluster object and the spec namespace itself.
	By(fmt.Sprintf("Dumping all the Cluster API resources in the %q namespace", namespace))
	// Dump all Cluster API related resources to artifacts before deleting them.
	framework.DumpAllResources(ctx, framework.DumpAllResourcesInput{
		Lister:    client,
		Namespace: namespace,
		LogPath:   filepath.Join(artifactFolder, "clusters", clusterProxy.GetName(), "resources"),
	})

	if !skipCleanup {
		By(fmt.Sprintf("Deleting cluster %s/%s", namespace, clusterName))
		// While https://github.com/kubernetes-sigs/cluster-api/issues/2955 is addressed in future iterations, there is a chance
		// that cluster variable is not set even if the cluster exists, so we are calling DeleteAllClustersAndWait
		// instead of DeleteClusterAndWait
		framework.DeleteAllClustersAndWait(ctx, framework.DeleteAllClustersAndWaitInput{
			Client:    client,
			Namespace: namespace,
		}, intervalsGetter(specName, "wait-delete-cluster")...)

		By(fmt.Sprintf("Deleting namespace used for hosting the %q test spec", specName))
		framework.DeleteNamespace(ctx, framework.DeleteNamespaceInput{
			Deleter: client,
			Name:    namespace,
		})
	}
}

// downloadFile will download a url and store it in local filepath.
func downloadFile(filePath string, url string) error {
	// Get the data
	resp, err := http.Get(url) //nolint:noctx // NB: as we're just implementing an external interface we won't be able to get a context here.
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(filepath.Clean(filePath))
	if err != nil {
		return err
	}
	defer func() {
		err := out.Close()
		Expect(err).To(BeNil(), "Error closing file")
	}()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}

// filterBmhsByProvisioningState returns a filtered list of BaremetalHost objects in certain provisioning state.
func filterBmhsByProvisioningState(bmhs []bmo.BareMetalHost, state bmo.ProvisioningState) (result []bmo.BareMetalHost) {
	for _, bmh := range bmhs {
		if bmh.Status.Provisioning.State == state {
			result = append(result, bmh)
		}
	}
	return
}

// filterMachinesByPhase returns a filtered list of CAPI machine objects in certain desired phase.
func filterMachinesByPhase(machines []clusterv1.Machine, phase string) (result []clusterv1.Machine) {
	for _, machine := range machines {
		if machine.Status.Phase == phase {
			result = append(result, machine)
		}
	}
	return
}

// annotateBmh annotates BaremetalHost with a given key and value.
func annotateBmh(ctx context.Context, client client.Client, host bmo.BareMetalHost, key string, value *string) {
	helper, err := patch.NewHelper(&host, client)
	Expect(err).NotTo(HaveOccurred())
	annotations := host.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	if value == nil {
		delete(annotations, key)
	} else {
		annotations[key] = *value
	}
	host.SetAnnotations(annotations)
	Expect(helper.Patch(ctx, &host)).To(Succeed())
}

// deleteNodeReuseLabelFromHost deletes nodeReuseLabelName from the host if it exists.
func deleteNodeReuseLabelFromHost(ctx context.Context, client client.Client, host bmo.BareMetalHost, nodeReuseLabelName string) {
	helper, err := patch.NewHelper(&host, client)
	Expect(err).NotTo(HaveOccurred())
	labels := host.GetLabels()
	if labels != nil {
		if _, ok := labels[nodeReuseLabelName]; ok {
			delete(host.Labels, nodeReuseLabelName)
		}
	}
	Expect(helper.Patch(ctx, &host)).To(Succeed())
}

// scaleMachineDeployment scales up/down MachineDeployment object to desired replicas.
func scaleMachineDeployment(ctx context.Context, clusterClient client.Client, clusterName, namespace string, newReplicas int) {
	machineDeployments := framework.GetMachineDeploymentsByCluster(ctx, framework.GetMachineDeploymentsByClusterInput{
		Lister:      clusterClient,
		ClusterName: clusterName,
		Namespace:   namespace,
	})
	Expect(len(machineDeployments)).To(Equal(1), "Expected exactly 1 MachineDeployment")
	machineDeploy := machineDeployments[0]
	patch := []byte(fmt.Sprintf(`{"spec": {"replicas": %d}}`, newReplicas))
	err := clusterClient.Patch(ctx, machineDeploy, client.RawPatch(types.MergePatchType, patch))
	Expect(err).To(BeNil(), "Failed to patch workers MachineDeployment")
}

// scaleKubeadmControlPlane scales up/down KubeadmControlPlane object to desired replicas.
func scaleKubeadmControlPlane(ctx context.Context, c client.Client, name client.ObjectKey, newReplicaCount int) {
	ctrlplane := controlplanev1.KubeadmControlPlane{}
	Expect(c.Get(ctx, name, &ctrlplane)).To(Succeed())
	helper, err := patch.NewHelper(&ctrlplane, c)
	Expect(err).To(BeNil(), "Failed to create new patch helper")

	ctrlplane.Spec.Replicas = pointer.Int32Ptr(int32(newReplicaCount))
	Expect(helper.Patch(ctx, &ctrlplane)).To(Succeed())
}

func deploymentRolledOut(ctx context.Context, clientSet *kubernetes.Clientset, name string, namespace string, desiredGeneration int64) bool {
	deploy, err := clientSet.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	Expect(err).To(BeNil())
	if deploy != nil {
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

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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	kcp "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1alpha4"
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
	data, err := os.ReadFile(logFile)
	Expect(err).To(BeNil(), "No log file found")
	Logf(string(data))
}

func checkError(err error) {
	if err != nil {
		fmt.Fprintf(GinkgoWriter, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

func dumpSpecResourcesAndCleanup(ctx context.Context, specName string, clusterProxy framework.ClusterProxy, artifactFolder string, namespace string, cluster *clusterv1.Cluster, intervalsGetter func(spec, key string) []interface{}, clusterName, clusterctlLogFolder string, skipCleanup bool) {
	Expect(os.RemoveAll(clusterctlLogFolder)).Should(Succeed())
	client := bootstrapClusterProxy.GetClient()

	// Dumps all the resources in the spec namespace, then cleanups the cluster object and the spec namespace itself.
	By(fmt.Sprintf("Dumping all the Cluster API resources in the %q namespace", namespace))
	// Dump all Cluster API related resources to artifacts before deleting them.
	framework.DumpAllResources(ctx, framework.DumpAllResourcesInput{
		Lister:    client,
		Namespace: namespace,
		LogPath:   filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName(), "resources"),
	})

	if !skipCleanup {
		By(fmt.Sprintf("Deleting cluster %s/%s", cluster.Namespace, cluster.Name))
		// While https://github.com/kubernetes-sigs/cluster-api/issues/2955 is addressed in future iterations, there is a chance
		// that cluster variable is not set even if the cluster exists, so we are calling DeleteAllClustersAndWait
		// instead of DeleteClusterAndWait
		framework.DeleteAllClustersAndWait(ctx, framework.DeleteAllClustersAndWaitInput{
			Client:    client,
			Namespace: namespace,
		}, e2eConfig.GetIntervals(specName, "wait-delete-cluster")...)

		By(fmt.Sprintf("Deleting namespace used for hosting the %q test spec", specName))
		framework.DeleteNamespace(ctx, framework.DeleteNamespaceInput{
			Deleter: client,
			Name:    namespace,
		})
	}
}

func downloadFile(filepath string, url string) error {

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}

func filterBmhsByProvisioningState(bmhs []bmo.BareMetalHost, state bmo.ProvisioningState) (result []bmo.BareMetalHost) {
	for _, bmh := range bmhs {
		if bmh.Status.Provisioning.State == state {
			result = append(result, bmh)
		}
	}
	return
}

func filterMachinesByPhase(machines []clusterv1.Machine, phase string) (result []clusterv1.Machine) {
	for _, machine := range machines {
		if machine.Status.Phase == phase {
			result = append(result, machine)
		}
	}
	return
}

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

func scaleMachineDeployment(ctx context.Context, clusterClient client.Client, newReplicas int) {
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

func scaleControlPlane(ctx context.Context, c client.Client, name client.ObjectKey, newReplicaCount int) {
	ctrlplane := kcp.KubeadmControlPlane{}
	Expect(c.Get(ctx, name, &ctrlplane)).To(Succeed())
	helper, err := patch.NewHelper(&ctrlplane, c)
	Expect(err).To(BeNil(), "Failed to create new patch helper")

	ctrlplane.Spec.Replicas = pointer.Int32Ptr(int32(newReplicaCount))
	Expect(helper.Patch(ctx, &ctrlplane)).To(Succeed())
}

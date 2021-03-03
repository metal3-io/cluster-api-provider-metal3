package e2e
import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/test/framework"
)

func Byf(format string, a ...interface{}) {
	By(fmt.Sprintf(format, a...))
}

func dumpSpecResourcesAndCleanup(ctx context.Context, specName string, clusterProxy framework.ClusterProxy, artifactFolder string, namespace string, cluster *clusterv1.Cluster, intervalsGetter func(spec, key string) []interface{}, clusterName, clusterctlLogFolder string, skipCleanup bool) {
	// Remove clusterctl apply log folder
	Expect(os.RemoveAll(clusterctlLogFolder)).ShouldNot(HaveOccurred())

	// Dumps all the resources in the spec namespace, then cleanups the cluster object and the spec namespace itself.
	By(fmt.Sprintf("Dumping all the Cluster API resources in the %q namespace", namespace))
	// Dump all Cluster API related resources to artifacts before deleting them.
	framework.DumpAllResources(ctx, framework.DumpAllResourcesInput{
		Lister:    bootstrapClusterProxy.GetClient(),
		Namespace: namespace,
		LogPath:   filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName(), "resources"),
	})

	if !skipCleanup {
		By(fmt.Sprintf("Deleting cluster %s/%s", cluster.Namespace, cluster.Name))
		// While https://github.com/kubernetes-sigs/cluster-api/issues/2955 is addressed in future iterations, there is a chance
		// that cluster variable is not set even if the cluster exists, so we are calling DeleteAllClustersAndWait
		// instead of DeleteClusterAndWait
		framework.DeleteAllClustersAndWait(ctx, framework.DeleteAllClustersAndWaitInput{
			Client:    bootstrapClusterProxy.GetClient(),
			Namespace: namespace,
		}, e2eConfig.GetIntervals(specName, "wait-delete-cluster")...)

		By(fmt.Sprintf("Deleting namespace used for hosting the %q test spec", specName))
		framework.DeleteNamespace(ctx, framework.DeleteNamespaceInput{
			Deleter: bootstrapClusterProxy.GetClient(),
			Name:    namespace,
		})
	}
}

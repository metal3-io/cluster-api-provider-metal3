package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	b64 "encoding/base64"
	"strings"
	"net/http"
	"io/ioutil"
	  "bytes"
	"encoding/json"
	"time"
)

/*
 * Apply scaled number of BMHs per batches and wait for the them to become available
 * The test pass when all the BMHs become available
 */
var _ = Describe("When testing scalability with fakeIPA [scalability]", Label("scalability"), func() {
	BeforeEach(func() {
		osType := strings.ToLower(os.Getenv("OS"))+"-fake"
		Expect(osType).ToNot(Equal(""))
		validateGlobals(specName)
		// We need to override clusterctl apply log folder to avoid getting our credentials exposed.
		clusterctlLogFolder = filepath.Join(os.TempDir(), "clusters", bootstrapClusterProxy.GetName())
		createFKASResources()
		caKey, caCert, etcdKey, etcdCert := generateFKASCerts()
		createFKASCerts(caKey, caCert, etcdKey, etcdCert)
	})
	scalabilityTest()
	AfterEach(func() {
		FKASKustomization := e2eConfig.GetVariable("FKAS_RELEASE_LATEST")
		By(fmt.Sprintf("Removing FKAS from kustomization %s from the bootsrap cluster", FKASKustomization))
		BuildAndRemoveKustomization(ctx, FKASKustomization, bootstrapClusterProxy)
		removeFKASCerts()
		DumpSpecResourcesAndCleanup(ctx, specName, bootstrapClusterProxy, artifactFolder, namespace, e2eConfig.GetIntervals, clusterName, clusterctlLogFolder, skipCleanup)
	})
})

func scalabilityTest() {
	It("Should apply per batches BMHs wait for them to become available", func() {
		By("Register fake cluster on the FKAS and get HOST and PORT")
		host, port := registerFKASCluster()
		Logf("CLUSTER_APIENDPOINT_HOST %v CLUSTER_APIENDPOINT_PORT %v", host, port)
		By("Fetching cluster configuration and export template HOST and PORT")
		k8sVersion := e2eConfig.GetVariable("KUBERNETES_VERSION")
		os.Setenv("CLUSTER_APIENDPOINT_HOST", host)
		os.Setenv("CLUSTER_APIENDPOINT_PORT", fmt.Sprintf("%v", port))

		// Apply bmh
		numNodes, _ := strconv.Atoi(e2eConfig.GetVariable("NUM_NODES"))
		batch, _ := strconv.Atoi(e2eConfig.GetVariable("BMH_BATCH_SIZE"))
		Logf("Starting scalability test")
		bootstrapClient := bootstrapClusterProxy.GetClient()
		ListBareMetalHosts(ctx, bootstrapClient, client.InNamespace(namespace))
		applyBatchBmh := func(from int, to int) {
			Logf("Apply BMH batch from node_%d to node_%d", from, to)
			for i := from; i < to+1; i++ {
				resource, err := os.ReadFile(filepath.Join(workDir, fmt.Sprintf("bmhs/node_%d.yaml", i)))
				Expect(err).ShouldNot(HaveOccurred())
				Expect(CreateOrUpdateWithNamespace(ctx, bootstrapClusterProxy, resource, namespace)).ShouldNot(HaveOccurred())
			}
			Logf("Wait for batch from node_%d to node_%d to become available", from, to)
			WaitForNumBmhInState(ctx, bmov1alpha1.StateAvailable, WaitForNumInput{
				Client:    bootstrapClient,
				Options:   []client.ListOption{client.InNamespace(namespace)},
				Replicas:  to + 1,
				Intervals: e2eConfig.GetIntervals(specName, "wait-bmh-available"),
			})
		}

		for i := 0; i < numNodes; i += batch {
			if i+batch > numNodes {
				applyBatchBmh(i, numNodes-1)
				break
			}
			applyBatchBmh(i, i+batch-1)
		}
		// Create the cluster 
		targetCluster, _ = createTargetCluster(k8sVersion)
		By("SCALABILITY TEST PASSED!")
	})
}

func generateFKASCerts() (caKey []byte, caCert []byte, etcdKey []byte, etcdCert []byte) {
	// this function manually generate certs with opessh and saved in the path
	// TODO: generate the certs using certmanager or golang package
	Logf("Generate cluster ca with openssl")
	cmd := exec.Command("bash", "-c", "openssl req -x509 -subj '/CN=Kubernetes API' -new -newkey rsa:2048 -nodes -keyout '/tmp/ca.key' -sha256 -days 3650 -out '/tmp/ca.crt'")
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	Logf("Generating ca certs with openssl:\n %v", string(output))
	Expect(err).ToNot(HaveOccurred())

	Logf("Generate ETCD ca with openssl")
	cmd = exec.Command("bash", "-c", "openssl req -x509 -subj '/CN=ETCD CA' -new -newkey rsa:2048 -nodes -keyout '/tmp/etcd.key' -sha256 -days 3650 -out '/tmp/etcd.crt'")
	cmd.Dir = workDir
	output, err = cmd.CombinedOutput()
	Logf("Generating etcd certs with openssl:\n %v", string(output))
	Expect(err).ToNot(HaveOccurred())

	caKey, err= os.ReadFile("/tmp/ca.key")
	Expect(err).NotTo(HaveOccurred())
	caCert, err= os.ReadFile("/tmp/ca.crt")
	Expect(err).NotTo(HaveOccurred())
	etcdKey, err= os.ReadFile("/tmp/etcd.key")
	Expect(err).NotTo(HaveOccurred())
	etcdCert, err= os.ReadFile("/tmp/etcd.crt")
	Expect(err).NotTo(HaveOccurred())
	os.Remove("/tmp/ca.key")
	os.Remove("/tmp/ca.crt")
	os.Remove("/tmp/etcd.key")
	os.Remove("/tmp/etcd.crt")

	return
}
func createFKASCerts(caKey []byte, caCert []byte, etcdKey []byte, etcdCert []byte) {
	bootstrapClient := bootstrapClusterProxy.GetClient()
	By("Creating a Cluster CA Secret resource")
	secretClusterCA := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName+"-ca",
			Namespace: namespace,
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: clusterName,
			},
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": []byte(caCert),
			"tls.key": []byte(caKey),
		},
	}
	
	Expect(bootstrapClient.Create(ctx, secretClusterCA)).To(Succeed(), "should create Cluster CA Secret CR")

	By("Creating a ETCD CA Secret resource")
	secretClusterETCD := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName+"-ca-etcd",
			Namespace: namespace,
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: clusterName,
			},
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": []byte(etcdCert),
			"tls.key": []byte(etcdKey),
		},
	}

	Expect(bootstrapClient.Create(ctx, secretClusterETCD)).To(Succeed(), "should create ETCD CA Secret CR")

}
func removeFKASCerts() {
	// TDOD should consider the case when the certs do not exist
	bootstrapClient := bootstrapClusterProxy.GetClient()
	By("Deleting FKAS Certs")
	var caCertSecret  corev1.Secret
	var etcdCertSecret  corev1.Secret
	bootstrapClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name:  clusterName+"-ca"}, &caCertSecret)
	bootstrapClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name:  clusterName+"-ca-etcd"}, &etcdCertSecret)
	bootstrapClient.Delete(ctx, &caCertSecret)
	bootstrapClient.Delete(ctx, &etcdCertSecret)
}
func registerFKASCluster() (string, int){
	/*
	* this function reads the certificate from the cluster secrets
	*/
	bootstrapClient := bootstrapClusterProxy.GetClient()
	type FKASCluster struct {
		Resource string `json:"resource"`
		CaKey string `json:"caKey"`
		CaCert string `json:"caCert"`
		EtcdKey string `json:"etcdKey"`
		EtcdCert string `json:"etcdCert"`
	 }
	 type Endpoint struct {
		Host string
		Port int
	}
	 var caCertSecret  corev1.Secret
	 var etcdCertSecret  corev1.Secret
	 Expect(bootstrapClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name:  clusterName+"-ca"}, &caCertSecret)).To(Succeed())
	 Expect(bootstrapClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name:  clusterName+"-ca-etcd"}, &etcdCertSecret)).To(Succeed())
	 
	 fkasCluster := FKASCluster{
		Resource: namespace+"/"+clusterName,
		CaKey: b64.StdEncoding.EncodeToString(caCertSecret.Data["tls.key"]),
		CaCert: b64.StdEncoding.EncodeToString(caCertSecret.Data["tls.crt"]),
		EtcdKey: b64.StdEncoding.EncodeToString(etcdCertSecret.Data["tls.key"]),
		EtcdCert: b64.StdEncoding.EncodeToString(etcdCertSecret.Data["tls.crt"]),
	 }
	 marshalled, err := json.Marshal(fkasCluster)
	 if err != nil {
		 Logf("impossible to marshall fkasCluster: %s", err)
	 }
	// send the request
	cluster_endpoints, err :=http.Post("http://172.22.0.2:3333/register", "application/json", bytes.NewReader(marshalled))
	Expect(err).NotTo(HaveOccurred())
	defer cluster_endpoints.Body.Close()
	body, err := ioutil.ReadAll(cluster_endpoints.Body)
	Expect(err).NotTo(HaveOccurred())
	var response Endpoint
	json.Unmarshal(body, &response)
	return response.Host, response.Port
}
func createFKASResources() {
	FKASKustomization := e2eConfig.GetVariable("FKAS_RELEASE_LATEST")
	By(fmt.Sprintf("Installing FKAS from kustomization %s on the bootsrap cluster", FKASKustomization))
	err := BuildAndApplyKustomization(ctx, &BuildAndApplyKustomizationInput{
		Kustomization:       FKASKustomization,
		ClusterProxy:        bootstrapClusterProxy,
		WaitForDeployment:   false,
		WatchDeploymentLogs: false,
	})
	Expect(err).NotTo(HaveOccurred())
	time.Sleep(240 * time.Second) 

}

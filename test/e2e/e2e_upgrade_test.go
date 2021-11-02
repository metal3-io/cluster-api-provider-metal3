package e2e

import (
	"context"
	"fmt"
	"go/build"
	"html/template"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterctlclient "sigs.k8s.io/cluster-api/cmd/clusterctl/client"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
)

func upgradeComponents() {
	targetClusterClient := targetCluster.GetClient()
	clientSet := targetCluster.GetClientSet()

	KubernetesVersion := e2eConfig.GetVariable("KUBERNETES_VERSION")
	Logf("KubernetesVersion: %v", KubernetesVersion)
	numberOfControlplane := int(*e2eConfig.GetInt32PtrVariable("CONTROL_PLANE_MACHINE_COUNT"))
	Logf("numberOfControlplane: %v", numberOfControlplane)
	numberOfWorker := int(*e2eConfig.GetInt32PtrVariable("WORKER_MACHINE_COUNT"))
	Logf("numberOfWorker: %v", numberOfWorker)
	CAPI_REL_TO_VERSION := e2eConfig.GetVariable("CAPI_REL_TO_VERSION")
	Logf("CAPI_REL_TO_VERSION: %v", CAPI_REL_TO_VERSION)
	CAPM3_REL_TO_VERSION := e2eConfig.GetVariable("CAPM3_REL_TO_VERSION")
	Logf("CAPM3_REL_TO_VERSION: %v", CAPM3_REL_TO_VERSION)
	CAPIRELEASE := e2eConfig.GetVariable("CAPIRELEASE")
	Logf("CAPIRELEASE: %v", CAPIRELEASE)
	CAPM3RELEASE := e2eConfig.GetVariable("CAPM3RELEASE")
	Logf("CAPM3RELEASE: %v", CAPM3RELEASE)
	CAPIRELEASE_HARDCODED := e2eConfig.GetVariable("CAPIRELEASE_HARDCODED")
	Logf("CAPIRELEASE_HARDCODED: %v", CAPIRELEASE_HARDCODED)
	M3PATH := e2eConfig.GetVariable("M3PATH")
	Logf("M3PATH: %v", M3PATH)
	CAPM3PATH := e2eConfig.GetVariable("CAPM3PATH")
	Logf("CAPM3PATH: %v", CAPM3PATH)
	pwd, err := os.Getwd()
	checkError(err)
	Logf("PWD:", pwd)
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		gopath = build.Default.GOPATH
	}
	Logf("GOPATH: %v", gopath)

	// ---------- ---------- ---------- ---------- ---------- ---------- ---------- ----------
	//                       Upgrade controlplane components                                 |
	// ---------- ---------- ---------- ---------- ---------- ---------- ---------- ----------

	By("Backup secrets for re-use when pods are re-created during the upgrade process")
	NAMEPREFIX := "baremetal-operator"
	secrets, err := clientSet.CoreV1().Secrets(fmt.Sprintf("%v-system", NAMEPREFIX)).List(ctx, metav1.ListOptions{})
	checkError(err)

	By("Cleanup - remove existing next versions of controlplane components CRDs")
	home, err := os.UserHomeDir()
	checkError(err)
	working_dir := fmt.Sprintf("%v/.cluster-api/dev-repository", home)

	// folder dev-repository does not exist
	paths := []string{
		fmt.Sprintf("cluster-api/%s/", CAPI_REL_TO_VERSION),
		fmt.Sprintf("bootstrap-kubeadm/%s", CAPI_REL_TO_VERSION),
		fmt.Sprintf("control-plane-kubeadm/%s", CAPI_REL_TO_VERSION),
		fmt.Sprintf("infrastructure-metal3/%s", CAPM3_REL_TO_VERSION),
	}
	for _, path := range paths {
		err = os.RemoveAll(fmt.Sprintf("%s/%s", working_dir, path))
		checkError(err)
	}

	By("Generate clusterctl configuration file")

	type vars struct {
		HOME                          string
		CAPIRELEASE                   string
		CAPM3RELEASE                  string
		CAPI_REL_TO_VERSION           string
		CAPM3_REL_TO_VERSION          string
		KUBERNETES_VERSION            string
		NAMESPACE                     string
		NUM_OF_MASTER_REPLICAS        string
		NUM_OF_WORKER_REPLICAS        string
		NODE_DRAIN_TIMEOUT            string
		MAX_SURGE_VALUE               string
		CLUSTER_APIENDPOINT_HOST      string
		CLUSTER_APIENDPOINT_PORT      string
		POD_CIDR                      string
		SERVICE_CIDR                  string
		PROVISIONING_POOL_RANGE_START string
		PROVISIONING_POOL_RANGE_END   string
		PROVISIONING_CIDR             string
		BAREMETALV4_POOL_RANGE_START  string
		BAREMETALV4_POOL_RANGE_END    string
		BAREMETALV6_POOL_RANGE_START  string
		BAREMETALV6_POOL_RANGE_END    string
		EXTERNAL_SUBNET_V4_PREFIX     string
		EXTERNAL_SUBNET_V4_HOST       string
		EXTERNAL_SUBNET_V6_PREFIX     string
		EXTERNAL_SUBNET_V6_HOST       string
		M3PATH                        string
	}

	NODE_DRAIN_TIMEOUT := "0s"
	MAX_SURGE_VALUE := "0"
	CLUSTER_APIENDPOINT_PORT := "6443"
	CLUSTER_APIENDPOINT_HOST := "192.168.111.249"
	POD_CIDR := "192.168.0.0/18"
	SERVICE_CIDR := "10.96.0.0/12"
	PROVISIONING_POOL_RANGE_START := "172.22.0.100"
	PROVISIONING_POOL_RANGE_END := "172.22.0.200"
	PROVISIONING_CIDR := "24"
	BAREMETALV4_POOL_RANGE_START := "192.168.111.100"
	BAREMETALV4_POOL_RANGE_END := "192.168.111.200"
	BAREMETALV6_POOL_RANGE_START := "fd55::100"
	BAREMETALV6_POOL_RANGE_END := "fd55::200"
	EXTERNAL_SUBNET_V4_PREFIX := "24"
	EXTERNAL_SUBNET_V4_HOST := "192.168.111.1"
	EXTERNAL_SUBNET_V6_PREFIX := "64"
	EXTERNAL_SUBNET_V6_HOST := "fd55::1"

	templateVars := vars{
		home, CAPIRELEASE, CAPM3RELEASE, CAPI_REL_TO_VERSION, CAPM3_REL_TO_VERSION, KubernetesVersion, namespace,
		fmt.Sprint(numberOfControlplane), fmt.Sprint(numberOfWorker), NODE_DRAIN_TIMEOUT, MAX_SURGE_VALUE,
		CLUSTER_APIENDPOINT_HOST, CLUSTER_APIENDPOINT_PORT, POD_CIDR, SERVICE_CIDR, PROVISIONING_POOL_RANGE_START,
		PROVISIONING_POOL_RANGE_END, PROVISIONING_CIDR, BAREMETALV4_POOL_RANGE_START, BAREMETALV4_POOL_RANGE_END,
		BAREMETALV6_POOL_RANGE_START, BAREMETALV6_POOL_RANGE_END, EXTERNAL_SUBNET_V4_PREFIX, EXTERNAL_SUBNET_V4_HOST,
		EXTERNAL_SUBNET_V6_PREFIX, EXTERNAL_SUBNET_V6_HOST, M3PATH}

	clusterctlUpgradeTestTemplate := filepath.Join(pwd, "/config/upgrade/clusterctl-upgrade-test.yaml")
	clusterctlVarsTemplate := filepath.Join(pwd, "/config/upgrade/clusterctl-vars.yaml")
	templates := template.Must(template.New("clusterctl-upgrade-test.yaml").ParseFiles(clusterctlUpgradeTestTemplate, clusterctlVarsTemplate))
	// Create a file to store the template output
	clusterctlFile, err := os.Create(fmt.Sprintf("%s/.cluster-api/clusterctl.yaml", home))
	checkError(err)
	// Execute the templates and write the output to the created file
	err = templates.Execute(clusterctlFile, templateVars)
	checkError(err)
	clusterctlFile.Close()

	By("Get clusterctl repo")
	if _, err := os.Stat("/tmp/cluster-api-clone"); os.IsNotExist(err) {
		_, err = git.PlainClone("/tmp/cluster-api-clone", false, &git.CloneOptions{
			URL:           "https://github.com/kubernetes-sigs/cluster-api.git",
			ReferenceName: plumbing.ReferenceName(fmt.Sprintf("refs/tags/%s", CAPI_REL_TO_VERSION)),
		})
		checkError(err)
	}

	type src_dest struct {
		src  string
		dest string
	}

	By("Creating clusterctl-settings.json for cluster-api and capm3 repo")
	items := []src_dest{
		{
			src:  filepath.Join(pwd, "/../../templates/e2e/upgrade/cluster-api-clusterctl-settings.json"),
			dest: "/tmp/cluster-api-clone/clusterctl-settings.json",
		},
		{
			src:  filepath.Join(pwd, "/../../templates/e2e/upgrade/capm3-clusterctl-settings.json"),
			dest: filepath.Join(CAPM3PATH, "/clusterctl-settings.json"),
		},
	}

	for _, item := range items {
		templates := template.Must(template.New(filepath.Base(item.src)).ParseFiles(item.src))
		settingsFile, err := os.Create(item.dest)
		checkError(err)
		err = templates.Execute(settingsFile, templateVars)
		checkError(err)
		settingsFile.Close()
	}

	By("Build clusterctl binary")
	cmd := exec.Command("make", "clusterctl")
	cmd.Dir = "/tmp/cluster-api-clone/"
	err = cmd.Run()
	checkError(err)

	By("Copy clusterctl to /usr/local/bin")
	copy("/tmp/cluster-api-clone/bin/clusterctl", "/usr/local/bin/clusterctl")

	By("Create local repository")
	cmd = exec.Command("cmd/clusterctl/hack/create-local-repository.py")
	cmd.Dir = "/tmp/cluster-api-clone/"
	output, err := cmd.CombinedOutput()
	Logf("output: %v", string(output))
	checkError(err)

	By("Create folder structure for next version controlplane components")
	working_dir = filepath.Join(home, "/.cluster-api/")
	paths = []string{
		filepath.Join("dev-repository/cluster-api/", CAPIRELEASE),
		filepath.Join("dev-repository/bootstrap-kubeadm/", CAPIRELEASE),
		filepath.Join("dev-repository/control-plane-kubeadm/", CAPIRELEASE),
		filepath.Join("dev-repository/cluster-api/", CAPI_REL_TO_VERSION),
		filepath.Join("dev-repository/bootstrap-kubeadm/", CAPI_REL_TO_VERSION),
		filepath.Join("dev-repository/control-plane-kubeadm/", CAPI_REL_TO_VERSION),
		filepath.Join("overrides/infrastructure-metal3/", CAPM3_REL_TO_VERSION),
		filepath.Join("overrides/infrastructure-metal3/", CAPM3RELEASE),
	}
	for _, path := range paths {
		err = os.MkdirAll(filepath.Join(working_dir, path), 0755)
		if os.IsExist(err) {
			Logf("%v Already exists", path)
		} else {
			checkError(err)
		}
	}

	By("Copy controlplane components files")
	items = []src_dest{
		{
			src:  fmt.Sprintf("dev-repository/cluster-api/%s/core-components.yaml", CAPIRELEASE_HARDCODED),
			dest: fmt.Sprintf("dev-repository/cluster-api/%s/core-components.yaml", CAPIRELEASE),
		},
		{
			src:  fmt.Sprintf("dev-repository/cluster-api/%s/metadata.yaml", CAPIRELEASE_HARDCODED),
			dest: fmt.Sprintf("dev-repository/cluster-api/%s/metadata.yaml", CAPIRELEASE),
		},
		{
			src:  fmt.Sprintf("dev-repository/bootstrap-kubeadm/%s/bootstrap-components.yaml", CAPIRELEASE_HARDCODED),
			dest: fmt.Sprintf("dev-repository/bootstrap-kubeadm/%s/bootstrap-components.yaml", CAPIRELEASE),
		},
		{
			src:  fmt.Sprintf("dev-repository/bootstrap-kubeadm/%s/metadata.yaml", CAPIRELEASE_HARDCODED),
			dest: fmt.Sprintf("dev-repository/bootstrap-kubeadm/%s/metadata.yaml", CAPIRELEASE),
		},
		{
			src:  fmt.Sprintf("dev-repository/control-plane-kubeadm/%s/control-plane-components.yaml", CAPIRELEASE_HARDCODED),
			dest: fmt.Sprintf("dev-repository/control-plane-kubeadm/%s/control-plane-components.yaml", CAPIRELEASE),
		},
		{
			src:  fmt.Sprintf("dev-repository/control-plane-kubeadm/%s/metadata.yaml", CAPIRELEASE_HARDCODED),
			dest: fmt.Sprintf("dev-repository/control-plane-kubeadm/%s/metadata.yaml", CAPIRELEASE),
		},
	}

	for _, item := range items {
		copy(filepath.Join(working_dir, item.src), filepath.Join(working_dir, item.dest))
	}

	By("Remove clusterctl generated next version of controlplane components folders")
	paths = []string{
		fmt.Sprintf("cluster-api/%s/", CAPIRELEASE_HARDCODED),
		fmt.Sprintf("bootstrap-kubeadm/%s", CAPIRELEASE_HARDCODED),
		fmt.Sprintf("control-plane-kubeadm/%s", CAPIRELEASE_HARDCODED),
	}
	for _, path := range paths {
		err = os.RemoveAll(fmt.Sprintf("%s/dev-repository/%s", working_dir, path))
		checkError(err)
	}

	By("Create next version controller CRDs")
	items = []src_dest{
		{
			src:  fmt.Sprintf("dev-repository/cluster-api/%s/core-components.yaml", CAPIRELEASE),
			dest: fmt.Sprintf("dev-repository/cluster-api/%s/core-components.yaml", CAPI_REL_TO_VERSION),
		},
		{
			src:  fmt.Sprintf("dev-repository/cluster-api/%s/metadata.yaml", CAPIRELEASE),
			dest: fmt.Sprintf("dev-repository/cluster-api/%s/metadata.yaml", CAPI_REL_TO_VERSION),
		},
		{
			src:  fmt.Sprintf("dev-repository/bootstrap-kubeadm/%s/bootstrap-components.yaml", CAPIRELEASE),
			dest: fmt.Sprintf("dev-repository/bootstrap-kubeadm/%s/bootstrap-components.yaml", CAPI_REL_TO_VERSION),
		},
		{
			src:  fmt.Sprintf("dev-repository/bootstrap-kubeadm/%s/metadata.yaml", CAPIRELEASE),
			dest: fmt.Sprintf("dev-repository/bootstrap-kubeadm/%s/metadata.yaml", CAPI_REL_TO_VERSION),
		},
		{
			src:  fmt.Sprintf("dev-repository/control-plane-kubeadm/%s/control-plane-components.yaml", CAPIRELEASE),
			dest: fmt.Sprintf("dev-repository/control-plane-kubeadm/%s/control-plane-components.yaml", CAPI_REL_TO_VERSION),
		},
		{
			src:  fmt.Sprintf("dev-repository/control-plane-kubeadm/%s/metadata.yaml", CAPIRELEASE),
			dest: fmt.Sprintf("dev-repository/control-plane-kubeadm/%s/metadata.yaml", CAPI_REL_TO_VERSION),
		},
		{
			src:  fmt.Sprintf("overrides/infrastructure-metal3/%s/infrastructure-components.yaml", CAPM3RELEASE),
			dest: fmt.Sprintf("overrides/infrastructure-metal3/%s/infrastructure-components.yaml", CAPM3_REL_TO_VERSION),
		},
		{
			src:  fmt.Sprintf("overrides/infrastructure-metal3/%s/metadata.yaml", CAPM3RELEASE),
			dest: fmt.Sprintf("overrides/infrastructure-metal3/%s/metadata.yaml", CAPM3_REL_TO_VERSION),
		},
	}

	for _, item := range items {
		copy(filepath.Join(working_dir, item.src), filepath.Join(working_dir, item.dest))
	}

	By("Make changes on CRDs")
	type template_replace struct {
		path    string
		regexp  string
		replace string
	}
	template_items := []template_replace{
		{
			path:    fmt.Sprintf("dev-repository/cluster-api/%s/core-components.yaml", CAPI_REL_TO_VERSION),
			regexp:  "description: Machine",
			replace: "description: upgradedMachine",
		},
		{
			path:    fmt.Sprintf("dev-repository/bootstrap-kubeadm/%s/bootstrap-components.yaml", CAPI_REL_TO_VERSION),
			regexp:  "description: KubeadmConfig",
			replace: "description: upgradedKubeadmConfig",
		},
		{
			path:    fmt.Sprintf("dev-repository/control-plane-kubeadm/%s/control-plane-components.yaml", CAPI_REL_TO_VERSION),
			regexp:  "description: KubeadmControlPlane",
			replace: "description: upgradedKubeadmControlPlane",
		},
		{
			path:    fmt.Sprintf("overrides/infrastructure-metal3/%s/infrastructure-components.yaml", CAPM3_REL_TO_VERSION),
			regexp:  "m3c\n",
			replace: "upgm3c\n",
		},
	}

	for _, item := range template_items {
		r, _ := regexp.Compile(item.regexp)
		data, err := ioutil.ReadFile(filepath.Join(working_dir, item.path))
		checkError(err)
		replacedData := r.ReplaceAllString(string(data), item.replace)
		err = ioutil.WriteFile(filepath.Join(working_dir, item.path), []byte(replacedData), 0644)
		checkError(err)

	}

	By("Perform upgrade on the target cluster")
	upgradeInput := clusterctl.UpgradeManagementClusterAndWaitInput{
		ClusterProxy:         targetCluster,
		LogFolder:            filepath.Join(artifactFolder, "/upgrade"),
		ClusterctlConfigPath: fmt.Sprintf("%s/.cluster-api/clusterctl.yaml", home),
		Contract:             "v1alpha4",
	}
	clusterctl.UpgradeManagementClusterAndWait(ctx, upgradeInput, "25m", "10s")

	By("Restore secrets to fill missing secret fields after performing target cluster upgrade")
	for _, item := range secrets.Items {
		_, err = clientSet.CoreV1().Secrets(fmt.Sprintf("%v-system", NAMEPREFIX)).Update(ctx, &item, metav1.UpdateOptions{})
	}
	if err != nil {
		checkError(err)
	}

	By("Perform upgrade on the source cluster")
	upgradeSourceInput := clusterctl.UpgradeInput{
		KubeconfigPath:       bootstrapClusterProxy.GetKubeconfigPath(),
		LogFolder:            filepath.Join(artifactFolder, "/upgrade"),
		ClusterctlConfigPath: fmt.Sprintf("%s/.cluster-api/clusterctl.yaml", home),
		Contract:             "v1alpha4",
	}

	Upgrade(ctx, upgradeSourceInput)

	By("Verify that CRDs are upgraded")
	type crd_search struct {
		name   string
		search string
	}
	crds_items := []crd_search{
		{
			name:   "machines.cluster.x-k8s.io",
			search: "upgradedMachine",
		},
		{
			name:   "kubeadmcontrolplanes.controlplane.cluster.x-k8s.io",
			search: "upgradedKubeadmControlPlane",
		},
		{
			name:   "kubeadmconfigs.bootstrap.cluster.x-k8s.io",
			search: "upgradedKubeadmConfig",
		},
	}
	for _, item := range crds_items {
		crd := &apiextensionsv1.CustomResourceDefinition{}
		err = targetClusterClient.Get(ctx, types.NamespacedName{Namespace: "", Name: item.name}, crd)
		checkError(err)
		for _, version := range crd.Spec.Versions {
			if version.Name == "v1alpha4" {
				Expect(strings.Contains(version.Schema.OpenAPIV3Schema.Description, item.search)).To(BeTrue(), "CRD was not updated")
			}
		}
	}
	By("Verify upgraded API resource for Metal3Clusters")
	crd := &apiextensionsv1.CustomResourceDefinition{}
	err = targetClusterClient.Get(ctx, types.NamespacedName{Namespace: "", Name: "metal3clusters.infrastructure.cluster.x-k8s.io"}, crd)
	checkError(err)
	found := false
	for _, shortName := range crd.Spec.Names.ShortNames {
		Logf("shortName: %v", shortName)
		if shortName == "upgm3c" {
			found = true
		}
	}
	Expect(found).To(BeTrue(), "Metal3Clusters CRD was not updated")
}

func copy(src string, dst string) {
	data, err := ioutil.ReadFile(src)
	checkError(err)
	err = ioutil.WriteFile(dst, data, 0644)
	checkError(err)
}

// Upgrade calls clusterctl upgrade apply with the list of providers defined in the local repository.
func Upgrade(ctx context.Context, input clusterctl.UpgradeInput) {
	Logf("clusterctl upgrade apply --contract %s",
		input.Contract,
	)

	upgradeOpt := clusterctlclient.ApplyUpgradeOptions{
		Kubeconfig: clusterctlclient.Kubeconfig{
			Path:    input.KubeconfigPath,
			Context: "",
		},
		Contract: input.Contract,
	}
	clusterctlClient := getClusterctlClient(input.ClusterctlConfigPath, "clusterctl-upgrade.log", input.LogFolder)
	err := clusterctlClient.ApplyUpgrade(upgradeOpt)

	for err != nil {
		Logf("Error %v", err)
		err = clusterctlClient.ApplyUpgrade(upgradeOpt)
		time.Sleep(30 * time.Second)
	}
}

func getClusterctlClient(configPath, logName, logFolder string) clusterctlclient.Client {

	c, err := clusterctlclient.New(configPath)
	Expect(err).ToNot(HaveOccurred(), "Failed to create the clusterctl client library")
	return c
}

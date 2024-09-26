package main

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"os"
	"time"

	"go.uber.org/zap/zapcore"
	appsv1 "k8s.io/api/apps/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	rest "k8s.io/client-go/rest"
	cmanager "sigs.k8s.io/cluster-api/test/infrastructure/inmemory/pkg/runtime/manager"
	"sigs.k8s.io/cluster-api/test/infrastructure/inmemory/pkg/server"
	"sigs.k8s.io/cluster-api/util/certs"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	cloudScheme                 = runtime.NewScheme()
	cloudMgr                    cmanager.Manager
	setupLog                    = ctrl.Log.WithName("setup")
	apiServerMux                = &server.WorkloadClustersMux{}
	ctx                         = context.Background()
	bootstrapClient             client.Client
	etcdInfoMap                 = make(map[string]etcdInfo)
	podIP                       string
	workloadListenerActivations = make(map[string]bool)
)

func init() {
	// scheme used for operating on the cloud resource.
	_ = corev1.AddToScheme(cloudScheme)
	_ = appsv1.AddToScheme(cloudScheme)
	_ = rbacv1.AddToScheme(cloudScheme)
	_ = coordinationv1.AddToScheme(cloudScheme)
	cloudMgr = cmanager.New(cloudScheme)
}

// register receives a ClusterName and Namespace from a request
// and generates a fake k8s API server corresponding with the provided name.
func register(w http.ResponseWriter, r *http.Request) {
	setupLog.Info("Received request to /register")
	var requestData struct {
		ClusterName string `json:"cluster"`
		Namespace   string `json:"namespace"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		setupLog.Error(err, "Failed to decode request body")
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		setupLog.Error(err, "Invalid JSON in request body")
		return
	}

	resourceName := fmt.Sprintf("%s/%s", requestData.Namespace, requestData.ClusterName)
	setupLog.Info("Registering new resource", "resource", resourceName)
	resp := &ResourceData{}
	resp.ResourceName = resourceName
	setupLog.Info("Adding resource group", "resourceName", resourceName)
	cloudMgr.AddResourceGroup(resourceName)
	// NOTE: We are using resourceName as listener name for convenience
	listenerName := resourceName
	listener, err := apiServerMux.InitWorkloadClusterListener(listenerName)
	if err != nil {
		setupLog.Error(err, "Failed to initialize listener", "listenerName", listenerName)
		http.Error(w, "Failed to initialize listener", http.StatusInternalServerError)
		return
	}
	if err := apiServerMux.RegisterResourceGroup(listenerName, resourceName); err != nil {
		setupLog.Error(err, "Failed to register resource group to listener", "resourceName", resourceName)
		http.Error(w, "Failed to register resource group to listener", http.StatusInternalServerError)
		setupLog.Error(err, "failed to Register resource group to listener")
		return
	}

	resp.Host = listener.Host()
	resp.Port = listener.Port()

	workloadListenerActivations[listenerName] = false

	data, err := json.Marshal(resp)
	if err != nil {
		setupLog.Error(err, "Failed to marshal the response", "resourceName", resourceName)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err = w.Write(data); err != nil {
		setupLog.Error(err, "failed to write the response data")
		return
	}
}

// activateCluster instantiates the cluster by installing the cluster components that should
// be ready after the first control-plane is started.
func activateCluster(resourceName string, w http.ResponseWriter, k8sVersion string) error {
	listener := cloudMgr.GetResourceGroup(resourceName)
	c := listener.GetClient()

	role := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kubeadm:get-nodes",
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:     []string{"get"},
				APIGroups: []string{"core"},
				Resources: []string{"nodes"},
			},
		},
	}
	if err := c.Create(ctx, role); err != nil {
		setupLog.Error(err, "Failed to create cluster role", "resourceName", resourceName)
		http.Error(w, "", http.StatusInternalServerError)
		return err
	}
	roleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kubeadm:get-nodes",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "kubeadm:get-nodes",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: rbacv1.GroupKind,
				Name: "system:bootstrappers:kubeadm:default-node-token",
			},
		},
	}
	if err := c.Create(ctx, roleBinding); err != nil {
		setupLog.Error(err, "Failed to create cluster rolebinding", "resourceName", resourceName)
		http.Error(w, "", http.StatusInternalServerError)
		return err
	}
	// create kubeadm config map
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubeadm-config",
			Namespace: metav1.NamespaceSystem,
		},
		Data: map[string]string{
			"ClusterConfiguration": "",
		},
	}
	if err := c.Create(ctx, cm); err != nil {
		setupLog.Error(err, "Failed to create kubeadm configmap", "resourceName", resourceName)
		http.Error(w, "", http.StatusInternalServerError)
		return err
	}
	// create kubelet config map
	kubeletCm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kubelet-config",
			Namespace: metav1.NamespaceSystem,
		},
		Data: map[string]string{
			"ClusterConfiguration": "",
		},
	}
	if err := c.Create(ctx, kubeletCm); err != nil {
		setupLog.Error(err, "Failed to create kubelet configmap", "resourceName", resourceName)
		http.Error(w, "", http.StatusInternalServerError)
		return err
	}
	// Create kube-proxy DaemonSet
	kubeProxyDaemonSet := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceSystem,
			Name:      "kube-proxy",
			Labels: map[string]string{
				"component": "kube-proxy",
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "kube-proxy",
							Image: fmt.Sprintf("registry.k8s.io/kube-proxy:%s", k8sVersion),
						},
					},
				},
			},
		},
	}

	if err := c.Create(ctx, kubeProxyDaemonSet); err != nil {
		setupLog.Error(err, "Failed to create kube-proxy DaemonSet", "resourceName", resourceName)
		http.Error(w, "Failed to create kube-proxy DaemonSet", http.StatusInternalServerError)
		return err
	}
	// Create the coredns configMap.
	corednsConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceSystem,
			Name:      "coredns",
		},
		Data: map[string]string{
			"Corefile": "ANG",
		},
	}
	if err := c.Create(ctx, corednsConfigMap); err != nil {
		setupLog.Error(err, "Failed to create corednsConfigMap", "resourceName", resourceName)
		http.Error(w, "Failed to create corednsConfigMap", http.StatusInternalServerError)
		return err
	}
	// Create the coredns deployment.
	corednsDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceSystem,
			Name:      "coredns",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "coredns",
							Image: "registry.k8s.io/coredns/coredns:v1.10.1",
						},
					},
				},
			},
		},
	}
	if err := c.Create(ctx, corednsDeployment); err != nil {
		setupLog.Error(err, "Failed to create corednsDeployment", "resourceName", resourceName)
		http.Error(w, "Failed to create corednsDeployment", http.StatusInternalServerError)
		return err
	}
	return nil
}

// updateNode receives nodeName and providerID, which are provided by CAPI after
// node provisioning, from request, and update the Node object on the fake API server accordingly.
func updateNode(w http.ResponseWriter, r *http.Request) {
	var requestData struct {
		ClusterName string            `json:"cluster"`
		UUID        string            `json:"uuid"`
		NodeName    string            `json:"nodeName"`
		Namespace   string            `json:"namespace"`
		ProviderID  string            `json:"providerID"`
		Labels      map[string]string `json:"labels"`
		K8sVersion  string            `json:"k8sversion"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		setupLog.Error(err, "Invalid JSON in request body")
		return
	}

	setupLog.Info("Decoded request data", "nodeName", requestData.NodeName, "providerID", requestData.ProviderID)
	nodeLabels := requestData.Labels
	nodeLabels["metal3.io/uuid"] = requestData.UUID
	namespace := requestData.Namespace
	clusterName := requestData.ClusterName
	resourceName := fmt.Sprintf("%s/%s", namespace, clusterName)
	nodeName := requestData.NodeName
	_, isControlPlane := nodeLabels["cluster.x-k8s.io/control-plane"]

	workloadActivated, ok := workloadListenerActivations[resourceName]
	if !ok {
		http.Error(w, "Workload Cluster does not exist", http.StatusInternalServerError)
		return
	}

	listener := cloudMgr.GetResourceGroup(resourceName)

	if isControlPlane {
		// Add node role control-plane label
		nodeLabels["node-role.kubernetes.io/control-plane"] = ""
		caSecretName := fmt.Sprintf("%s-ca", clusterName)
		caCertRaw, caKeyRaw, err := getSecretKeyAndCert(ctx, bootstrapClient, requestData.Namespace, caSecretName)
		if err != nil {
			logLine := fmt.Sprintf("Error adding node %s", nodeName)
			http.Error(w, logLine, http.StatusInternalServerError)
			setupLog.Error(err, "Failed to get ca secrets for cluster", "cluster name", resourceName)
			return
		}

		caCert, err := certs.DecodeCertPEM(caCertRaw)
		if err != nil {
			http.Error(w, "Failed to add API server", http.StatusInternalServerError)
			setupLog.Error(err, "failed to decode caCertPEM")
			return
		}

		caKey, err := certs.DecodePrivateKeyPEM(caKeyRaw)
		if err != nil {
			http.Error(w, "Failed to generate etcdKey", http.StatusInternalServerError)
			setupLog.Error(err, "failed to decode caKeyPEM")
			return
		}

		apiServerPodName := nodeName
		err = apiServerMux.AddAPIServer(resourceName, apiServerPodName, caCert, caKey.(*rsa.PrivateKey))
		if err != nil {
			http.Error(w, "Failed to add API server", http.StatusInternalServerError)
			setupLog.Error(err, "failed to add API server")
			return
		}
		// For first CP, we need to install some cluster-wide resources
		if !workloadActivated {
			activateCluster(resourceName, w, requestData.K8sVersion)
		}
		workloadListenerActivations[resourceName] = true
	}

	if activated := workloadListenerActivations[resourceName]; !activated {
		http.Error(w, "Workload Cluster has not been activated", http.StatusInternalServerError)
		return
	}

	c := listener.GetClient()
	timeOutput := metav1.Now()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   nodeName,
			Labels: nodeLabels,
		},
		Spec: corev1.NodeSpec{
			ProviderID: requestData.ProviderID,
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{
					LastHeartbeatTime:  timeOutput,
					LastTransitionTime: timeOutput,
					Type:               corev1.NodeReady,
					Status:             corev1.ConditionTrue,
				},
				{
					LastHeartbeatTime:  timeOutput,
					LastTransitionTime: timeOutput,
					Type:               corev1.NodeMemoryPressure,
					Status:             corev1.ConditionFalse,
					Message:            "kubelet has sufficient memory available",
					Reason:             "KubeletHasSufficientMemory",
				},
				{
					LastHeartbeatTime:  timeOutput,
					LastTransitionTime: timeOutput,
					Message:            "kubelet has no disk pressure",
					Reason:             "KubeletHasNoDiskPressure",
					Status:             corev1.ConditionFalse,
					Type:               corev1.NodeDiskPressure,
				},
				{
					LastHeartbeatTime:  timeOutput,
					LastTransitionTime: timeOutput,
					Message:            "kubelet has sufficient PID available",
					Reason:             "KubeletHasSufficientPID",
					Status:             corev1.ConditionFalse,
					Type:               corev1.NodePIDPressure,
				},
				{
					LastHeartbeatTime:  timeOutput,
					LastTransitionTime: timeOutput,
					Message:            "kubelet is posting ready status",
					Reason:             "KubeletReady",
					Status:             corev1.ConditionTrue,
					Type:               corev1.NodeReady,
				},
			},
		},
	}

	err := c.Create(ctx, node)
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			setupLog.Info("Node already exists", "nodeName", nodeName)
			w.WriteHeader(http.StatusOK)
			return
		}
		logLine := fmt.Sprintf("Error adding node %s: %s", nodeName, err)
		fmt.Println(err, "Error adding node:", nodeName)
		http.Error(w, logLine, http.StatusInternalServerError)
		return
	}
	fmt.Printf("Created node object: %v\n", node)

	// Start a goroutine to update the LastHeartbeatTime every 10 seconds
	go func(nodeName string) {
		setupLog.Info("Starting heartbeat goroutine", "nodeName", nodeName)
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			node := &corev1.Node{}
			if err := c.Get(ctx, client.ObjectKey{Name: nodeName}, node); err != nil {
				setupLog.Error(err, "Failed to get node for heartbeat update", "nodeName", nodeName)
				continue
			}

			timeOutput := metav1.Now()
			for i := range node.Status.Conditions {
				if node.Status.Conditions[i].Type == corev1.NodeReady {
					node.Status.Conditions[i].LastHeartbeatTime = timeOutput
				}
			}

			err := c.Update(ctx, node)
			if err != nil {
				setupLog.Error(err, "Failed to update node heartbeat", "nodeName", nodeName)
			} else {
				setupLog.Info("Updated node heartbeat", "nodeName", nodeName, "timestamp", timeOutput)
			}
		}
	}(nodeName)

	if !isControlPlane {
		return
	}

	// create etcd member
	// Wait for some time after the node is provisioned
	waitForRandomSeconds()
	etcdSecretName := fmt.Sprintf("%s-etcd", clusterName)
	etcdCertRaw, etcdKeyRaw, err := getSecretKeyAndCert(ctx, bootstrapClient, requestData.Namespace, etcdSecretName)
	if err != nil {
		logLine := fmt.Sprintf("Error adding node %s", nodeName)
		http.Error(w, logLine, http.StatusInternalServerError)
		setupLog.Error(err, "Failed to get etcd secrets for cluster", "cluster name", resourceName)
		return
	}

	etcdCert, err := certs.DecodeCertPEM(etcdCertRaw)
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		setupLog.Error(err, "failed to generate etcdCert")
		return
	}
	etcdKey, err := certs.DecodePrivateKeyPEM(etcdKeyRaw)
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		setupLog.Error(err, "failed to generate etcdKey")
		return
	}
	if etcdKey == nil {
		http.Error(w, "", http.StatusInternalServerError)
		setupLog.Error(err, "failed to generate etcdKey")
		return
	}

	// Create the etcd pod
	etcdPodMember := fmt.Sprintf("etcd-%s", nodeName)
	etcdLabels := map[string]string{
		"component": "etcd",
		"tier":      "control-plane",
	}
	etcdPod := getFakePodObject(FakePod{
		PodName:         etcdPodMember,
		Namespace:       metav1.NamespaceSystem,
		NodeName:        nodeName,
		Labels:          etcdLabels,
		TransactionTime: timeOutput,
	})

	if err := c.Get(ctx, client.ObjectKeyFromObject(etcdPod), etcdPod); err != nil {
		if !apierrors.IsNotFound(err) {
			setupLog.Error(err, "failed to get etcd pod")
			http.Error(w, "", http.StatusInternalServerError)
			return
		}

		// Gets info about the current etcd cluster, if any.
		info, ok := etcdInfoMap[resourceName]
		if !ok {
			info = etcdInfo{}
			for {
				info.clusterID = fmt.Sprintf("%d", rand.Uint32()) //nolint:gosec // weak random number generator is good enough here
				if info.clusterID != "0" {
					break
				}
			}
		}

		// Computes a unique memberID.
		var memberID string
		for {
			memberID = fmt.Sprintf("%d", rand.Uint32()) //nolint:gosec // weak random number generator is good enough here
			if !info.members.Has(memberID) && memberID != "0" {
				break
			}
		}

		// Annotate the pod with the info about the etcd cluster.
		etcdPod.Annotations = map[string]string{
			EtcdClusterIDAnnotationName: info.clusterID,
			EtcdMemberIDAnnotationName:  memberID,
		}

		// If the etcd cluster is being created it doesn't have a leader yet, so set this member as a leader.
		if info.leaderID == "" {
			etcdPod.Annotations[EtcdLeaderFromAnnotationName] = time.Now().Format(time.RFC3339)
		}

		etcdInfoMap[resourceName] = info

		if err := c.Create(ctx, etcdPod); err != nil && !apierrors.IsAlreadyExists(err) {
			setupLog.Error(err, "failed to create etcd pod")
			http.Error(w, "Failed to create etcd pod", http.StatusInternalServerError)
			return
		}
	}

	err = apiServerMux.AddEtcdMember(resourceName, nodeName, etcdCert, etcdKey.(*rsa.PrivateKey))
	if err != nil {
		setupLog.Error(err, "failed to add etcd member")
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	// Create the kube-apiserver pod
	if err := createControlPlanePod(ctx, c, "kube-apiserver", FakePod{
		PodName:         fmt.Sprintf("kube-apiserver-%s", nodeName),
		NodeName:        nodeName,
		TransactionTime: timeOutput,
	}); err != nil {
		setupLog.Error(err, "failed to create kube-apiserver pod")
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	// Create the kube-controller-manager
	if err := createControlPlanePod(ctx, c, "kube-controller-manager", FakePod{
		PodName:         fmt.Sprintf("kube-controller-manager-%s", nodeName),
		NodeName:        nodeName,
		TransactionTime: timeOutput,
	}); err != nil {
		setupLog.Error(err, "failed to create kube-controller-manager pod")
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	// Create the kube-scheduler
	if err := createControlPlanePod(ctx, c, "kube-scheduler", FakePod{
		PodName:         fmt.Sprintf("kube-scheduler-%s", nodeName),
		NodeName:        nodeName,
		TransactionTime: timeOutput,
	}); err != nil {
		setupLog.Error(err, "failed to create scheduler Pod")
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
}

func main() {
	debug := os.Getenv("DEBUG")
	logLevel := zapcore.InfoLevel // Default log level
	if debug == "true" {
		logLevel = zapcore.DebugLevel // Set log level to Debug if DEBUG=true
	}
	log.SetLogger(zap.New(zap.UseDevMode(true), zap.Level(logLevel)))
	podIP = os.Getenv("POD_IP")
	apiServerMux, _ = server.NewWorkloadClustersMux(cloudMgr, podIP)
	setupLog.Info("Starting the FKAS server")
	config, err := rest.InClusterConfig()
	if err != nil {
		setupLog.Error(err, "Cannot create bootstrap client config")
		os.Exit(1)
	}

	bootstrapClient, err = client.New(config, client.Options{Scheme: cloudScheme})
	if err != nil {
		setupLog.Error(err, "Failed to create controller-runtime client")
		os.Exit(1)
	}

	http.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		setupLog.Info("Received request to /register", "content", r.Body)
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		register(w, r)
	})
	http.HandleFunc("/updateNode", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		updateNode(w, r)
	})
	server := &http.Server{
		Addr:         ":3333",
		Handler:      nil,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}
	if err := server.ListenAndServe(); err != nil {
		setupLog.Error(err, "Error starting server")
		os.Exit(1)
	}
}

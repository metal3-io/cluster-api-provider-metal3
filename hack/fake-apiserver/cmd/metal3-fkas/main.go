package main

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	cmanager "sigs.k8s.io/cluster-api/test/infrastructure/inmemory/pkg/runtime/manager"
	"sigs.k8s.io/cluster-api/test/infrastructure/inmemory/pkg/server"
	"sigs.k8s.io/cluster-api/util/certs"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	cloudScheme  = runtime.NewScheme()
	cloudMgr     cmanager.Manager
	setupLog     = ctrl.Log.WithName("setup")
	apiServerMux = &server.WorkloadClustersMux{}
	ctx          = context.Background()
)

func init() {
	// scheme used for operating on the cloud resource.
	_ = corev1.AddToScheme(cloudScheme)
	_ = appsv1.AddToScheme(cloudScheme)
	_ = rbacv1.AddToScheme(cloudScheme)
	_ = coordinationv1.AddToScheme(cloudScheme)
	cloudMgr = cmanager.New(cloudScheme)
}

func int32Ptr(i int32) *int32 {
	return &i
}

type ResourceData struct {
	ResourceName string
	Host         string
	Port         int
}

// register receives a resourceName, ca and etcd secrets (key+cert) from a request
// and generates a fake k8s API server corresponding with the provided name and secrets.
func register(w http.ResponseWriter, r *http.Request) {
	setupLog.Info("Received request to /updateNode")
	var requestData struct {
		ResourceName string `json:"resource"`
		CaKey        string `json:"caKey"`
		CaCert       string `json:"caCert"`
		EtcdKey      string `json:"etcdKey"`
		EtcdCert     string `json:"etcdCert"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		setupLog.Error(err, "Failed to decode request body")
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		setupLog.Error(err, "Invalid JSON in request body")
		return
	}

	setupLog.Info("Registering new resource", "resource", requestData.ResourceName)
	resourceName := requestData.ResourceName
	resp := &ResourceData{}
	resp.ResourceName = resourceName
	setupLog.Info("Adding resource group", "resourceName", resourceName)
	cloudMgr.AddResourceGroup(resourceName)
	// NOTE: We are using resourceName as listener name for convenience
	listener, err := apiServerMux.InitWorkloadClusterListener(resourceName)
	if err != nil {
		setupLog.Error(err, "Failed to initialize listener", "resourceName", resourceName)
		http.Error(w, "Failed to initialize listener", http.StatusInternalServerError)
		setupLog.Error(err, "failed to Initialize listener")
		return
	}
	// NOTE: The two params are name of the listener and name of the resource group
	// we are setting both of them to resourceName for convenience, but it is not required.
	if err := apiServerMux.RegisterResourceGroup(resourceName, resourceName); err != nil {
		setupLog.Error(err, "Failed to register resource group to listener", "resourceName", resourceName)
		http.Error(w, "Failed to register resource group to listener", http.StatusInternalServerError)
		setupLog.Error(err, "failed to Register resource group to listener")
		return
	}
	caKeyEncoded := requestData.CaKey
	caKeyRaw, err := base64.StdEncoding.DecodeString(caKeyEncoded)
	if err != nil {
		setupLog.Error(err, "Failed to generate caKey", "resourceName", resourceName)
		http.Error(w, "Failed to generate caKey", http.StatusInternalServerError)
		setupLog.Error(err, "failed to decode caKey")
		return
	}
	caCertEncoded := requestData.CaCert
	caCertRaw, err := base64.StdEncoding.DecodeString(caCertEncoded)
	if err != nil {
		setupLog.Error(err, "Failed to generate caCert", "resourceName", resourceName)
		http.Error(w, "Failed to generate caCert", http.StatusInternalServerError)
		setupLog.Error(err, "failed to decode caCert")
		return
	}

	caCert, err := certs.DecodeCertPEM(caCertRaw)
	if err != nil {
		setupLog.Error(err, "Failed to add API server", "resourceName", resourceName)
		http.Error(w, "Failed to add API server", http.StatusInternalServerError)
		setupLog.Error(err, "failed to decode caCertPEM")
		return
	}
	caKey, err := certs.DecodePrivateKeyPEM(caKeyRaw)
	if err != nil {
		setupLog.Error(err, "Failed to generate etcdKey", "resourceName", resourceName)
		http.Error(w, "Failed to generate etcdKey", http.StatusInternalServerError)
		setupLog.Error(err, "failed to decode caKeyPEM")
		return
	}

	apiServerPod := fmt.Sprintf("kube-apiserver-%s", resourceName)
	err = apiServerMux.AddAPIServer(resourceName, apiServerPod, caCert, caKey.(*rsa.PrivateKey))
	if err != nil {
		setupLog.Error(err, "Failed to generate etcdCert", "resourceName", resourceName)
		http.Error(w, "Failed to generate etcdCert", http.StatusInternalServerError)
		setupLog.Error(err, "failed to add API server")
		return
	}
	etcdKeyEncoded := requestData.EtcdKey
	etcdKeyRaw, err := base64.StdEncoding.DecodeString(etcdKeyEncoded)
	if err != nil {
		setupLog.Error(err, "Failed to add etcd member", "resourceName", resourceName)
		http.Error(w, "Failed to add etcd member", http.StatusInternalServerError)
		setupLog.Error(err, "failed to generate etcdKey")
		return
	}

	etcdCertEncoded := requestData.EtcdCert
	etcdCertRaw, err := base64.StdEncoding.DecodeString(etcdCertEncoded)
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		setupLog.Error(err, "failed to generate etcdKey")
		return
	}
	//
	etcdCert, err := certs.DecodeCertPEM(etcdCertRaw)
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		setupLog.Error(err, "failed to generate etcdKey")
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
	//
	etcdPodMember := fmt.Sprintf("etcd-%s", resourceName)
	err = apiServerMux.AddEtcdMember(resourceName, etcdPodMember, etcdCert, etcdKey.(*rsa.PrivateKey))
	if err != nil {
		setupLog.Error(err, "failed to add etcd member")
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	resp.Host = listener.Host()
	resp.Port = listener.Port()
	data, err := json.Marshal(resp)
	if err != nil {
		setupLog.Error(err, "Failed to marshal the response", "resourceName", resourceName)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	c, err := listener.GetClient()
	if err != nil {
		setupLog.Error(err, "Failed to get listener client", "resourceName", resourceName)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	ctx := context.Background()

	role := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kubeadm:get-nodes",
		},
		Namespace: metav1.NamespaceDefault,
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:     []string{"get"},
				APIGroups: []string{"core"},
				Resources: []string{"nodes"},
			},
		},
	}
	if err = c.Create(ctx, role); err != nil {
		setupLog.Error(err, "Failed to create cluster role", "resourceName", resourceName)
		http.Error(w, "", http.StatusInternalServerError)
		return
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
	if err = c.Create(ctx, roleBinding); err != nil {
		setupLog.Error(err, "Failed to create cluster rolebinding", "resourceName", resourceName)
		http.Error(w, "", http.StatusInternalServerError)
		return
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
	if err = c.Create(ctx, cm); err != nil {
		setupLog.Error(err, "Failed to create kubeadm configmap", "resourceName", resourceName)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err = w.Write(data); err != nil {
		setupLog.Error(err, "failed to write the response data")
		return
	}
}

// updateNode receives nodeName and providerID, which are provided by CAPI after
// node provisioning, from request, and update the Node object on the fake API server accordingly.
func updateNode(w http.ResponseWriter, r *http.Request) {
	var requestData struct {
		ResourceName string            `json:"resource"`
		UUID         string            `json:"uuid"`
		NodeName     string            `json:"nodeName"`
		ProviderID   string            `json:"providerID"`
		Labels       map[string]string `json:"labels"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		setupLog.Error(err, "Invalid JSON in request body")
		return
	}

	setupLog.Info("Decoded request data", "nodeName", requestData.NodeName, "providerID", requestData.ProviderID)
	nodeLabels := requestData.Labels
	nodeLabels["metal3.io/uuid"] = requestData.UUID

	listener := cloudMgr.GetResourceGroup(requestData.ResourceName)
	timeOutput := metav1.Now()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   requestData.NodeName,
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

	c := listener.GetClient()
	err := c.Create(ctx, node)
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			setupLog.Info("Node already exists", "nodeName", requestData.NodeName)
			w.WriteHeader(http.StatusOK)
			return
		}
		logLine := fmt.Sprintf("Error adding node %s: %s", requestData.NodeName, err)
		fmt.Println(err, "Error adding node:", requestData.NodeName)
		http.Error(w, logLine, http.StatusInternalServerError)
		return
	}
	fmt.Printf("Created node object: %v\n", node)
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name: requestData.NodeName,
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       &requestData.NodeName,
			LeaseDurationSeconds: int32Ptr(30),
		},
	}

	err = c.Create(ctx, lease)
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			setupLog.Info("Lease already exists", "leaseName", requestData.NodeName)
		} else {
			setupLog.Error(err, "Error creating lease", "leaseName", requestData.NodeName, "namespace", metav1.NamespaceDefault)
			http.Error(w, "Error creating lease", http.StatusInternalServerError)
			return
		}
	}
	fmt.Printf("Created lease object: %v\n", lease)
}

func main() {
	log.SetLogger(zap.New(zap.UseDevMode(true)))
	podIP := os.Getenv("POD_IP")
	apiServerMux, _ = server.NewWorkloadClustersMux(cloudMgr, podIP)
	setupLog.Info("Starting the FKAS server")
	http.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		setupLog.Info("Received request to /register", "content", r.Body)
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		register(w, r)
	})
	http.HandleFunc("/updateNode", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
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

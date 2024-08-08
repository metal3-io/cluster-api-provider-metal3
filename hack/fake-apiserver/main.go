package main

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	infrav1 "sigs.k8s.io/cluster-api/test/infrastructure/inmemory/api/v1alpha1"
	cmanager "sigs.k8s.io/cluster-api/test/infrastructure/inmemory/pkg/runtime/manager"
	"sigs.k8s.io/cluster-api/test/infrastructure/inmemory/pkg/server"
	"sigs.k8s.io/cluster-api/util/certs"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	cloudScheme  = runtime.NewScheme()
	setupLog     = ctrl.Log.WithName("setup")
	scheme       = runtime.NewScheme()
	cloudMgr     = cmanager.New(cloudScheme)
	apiServerMux = &server.WorkloadClustersMux{}
	ctx          = context.Background()
)

func init() {
	// scheme used for operating on the management cluster.
	_ = clientgoscheme.AddToScheme(scheme)
	_ = clusterv1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)

	// scheme used for operating on the cloud resource.
	_ = corev1.AddToScheme(cloudScheme)
	_ = appsv1.AddToScheme(cloudScheme)
	_ = rbacv1.AddToScheme(cloudScheme)
}

type ResourceData struct {
	ResourceName string
	Host         string
	Port         int
}

func register(w http.ResponseWriter, r *http.Request) {
	resourceName := r.URL.Query().Get("resource")
	resp := &ResourceData{}
	resp.ResourceName = resourceName
	cloudMgr.AddResourceGroup(resourceName)
	// NOTE: We are using resourceName as listener name for convenience
	listener, err := apiServerMux.InitWorkloadClusterListener(resourceName)
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		setupLog.Error(err, "failed to Initialize listener")
		return
	}
	// NOTE: The two params are name of the listener and name of the resource group
	// we are setting both of them to resourceName for convenience, but it is not required.
	if err := apiServerMux.RegisterResourceGroup(resourceName, resourceName); err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		setupLog.Error(err, "failed to Register resource group to listener")
		return
	}
	caKeyEncoded := r.URL.Query().Get("caKey")
	caKeyRaw, err := base64.StdEncoding.DecodeString(caKeyEncoded)
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		setupLog.Error(err, "failed to generate caKey")
		return
	}
	caCertEncoded := r.URL.Query().Get("caCert")
	caCertRaw, err := base64.StdEncoding.DecodeString(caCertEncoded)
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		setupLog.Error(err, "failed to generate caKey")
		return
	}

	caCert, err := certs.DecodeCertPEM(caCertRaw)
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		setupLog.Error(err, "failed to generate caKey")
		return
	}
	caKey, err := certs.DecodePrivateKeyPEM(caKeyRaw)
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		setupLog.Error(err, "failed to generate caKey")
		return
	}

	apiServerPod1 := "kube-apiserver-1"
	err = apiServerMux.AddAPIServer(resourceName, apiServerPod1, caCert, caKey.(*rsa.PrivateKey))
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		setupLog.Error(err, "failed to add API server")
		return
	}
	etcdKeyEncoded := r.URL.Query().Get("etcdKey")
	etcdKeyRaw, err := base64.StdEncoding.DecodeString(etcdKeyEncoded)
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		setupLog.Error(err, "failed to generate etcdKey")
		return
	}

	etcdCertEncoded := r.URL.Query().Get("etcdCert")
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
	etcdPodMember1 := "etcd-1"
	err = apiServerMux.AddEtcdMember(resourceName, etcdPodMember1, etcdCert, etcdKey.(*rsa.PrivateKey))
	if err != nil {
		setupLog.Error(err, "failed to add etcd member")
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	resp.Host = listener.Host()
	resp.Port = listener.Port()
	data, err := json.Marshal(resp)
	if err != nil {
		setupLog.Error(err, "failed to Marshal the response")
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	c, err := listener.GetClient()
	if err != nil {
		setupLog.Error(err, "failed to get listener client")
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	ctx := context.Background()

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
	if err = c.Create(ctx, role); err != nil {
		setupLog.Error(err, "failed to create cluster role")
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
		setupLog.Error(err, "failed to create cluster rolebinding")
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
		setupLog.Error(err, "failed to create kubeadm configmap")
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	if _, err = io.WriteString(w, string(data)); err != nil {
		setupLog.Error(err, "failed to write the response data")
		return
	}
}

func updateNode(w http.ResponseWriter, r *http.Request) {
	resourceName := r.URL.Query().Get("resource")
	listener := cloudMgr.GetResourceGroup(resourceName)
	nodeName := r.URL.Query().Get("nodeName")
	providerID := r.URL.Query().Get("providerID")
	timeOutput := metav1.Now()

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
			Labels: map[string]string{
				"node-role.kubernetes.io/control-plane": "",
			},
		},
		Spec: corev1.NodeSpec{
			ProviderID: providerID,
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
			NodeInfo: corev1.NodeSystemInfo{
				Architecture:    "amd64",
				BootID:          "a4254236-e1e3-4462-97ed-4a25b8b29884",
				OperatingSystem: "linux",
				SystemUUID:      "1ce97e94-730c-42b7-98da-f7dcc0b58e93",
			},
		},
	}
	c := listener.GetClient()
	err := c.Create(ctx, node)
	if err != nil {
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
}

func main() {
	ctrl.SetLogger(klog.Background())
	podIP := os.Getenv("POD_IP")
	apiServerMux, _ = server.NewWorkloadClustersMux(cloudMgr, podIP)
	http.HandleFunc("/register", register)
	http.HandleFunc("/updateNode", updateNode)
	server := &http.Server{
		Addr:         ":3333",
		Handler:      nil,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}
	if err := server.ListenAndServe(); err != nil {
		klog.Error(err.Error())
	}
}

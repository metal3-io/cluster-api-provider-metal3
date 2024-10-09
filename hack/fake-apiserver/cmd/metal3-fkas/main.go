package main

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
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
	cmanager "sigs.k8s.io/cluster-api/test/infrastructure/inmemory/pkg/runtime/manager"
	"sigs.k8s.io/cluster-api/test/infrastructure/inmemory/pkg/server"
	"sigs.k8s.io/cluster-api/util/certs"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	cloudScheme     = runtime.NewScheme()
	bootstrapScheme = runtime.NewScheme()
	cloudMgr        cmanager.Manager
	setupLog        = ctrl.Log.WithName("setup")
	apiServerMux    = &server.WorkloadClustersMux{}
	ctx             = context.Background()
	caCertMap       = make(map[string][]byte)
	caKeyMap        = make(map[string][]byte)
	etcdCertMap     = make(map[string][]byte)
	etcdKeyMap      = make(map[string][]byte)
	etcdInfoMap     = make(map[string]etcdInfo)
)

func init() {
	// scheme used for operating on the cloud resource.
	_ = corev1.AddToScheme(cloudScheme)
	_ = appsv1.AddToScheme(cloudScheme)
	_ = rbacv1.AddToScheme(cloudScheme)
	_ = corev1.AddToScheme(bootstrapScheme)
	_ = appsv1.AddToScheme(bootstrapScheme)
	_ = rbacv1.AddToScheme(bootstrapScheme)
	_ = coordinationv1.AddToScheme(cloudScheme)
	cloudMgr = cmanager.New(cloudScheme)
}

// register receives a resourceName, ca and etcd secrets (key+cert) from a request
// and generates a fake k8s API server corresponding with the provided name and secrets.
func register(w http.ResponseWriter, r *http.Request) {
	setupLog.Info("Received request to /register")
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
		return
	}
	// NOTE: The two params are name of the listener and name of the resource group
	// we are setting both of them to resourceName for convenience, but it is not required.
	listenerName := resourceName
	if err := apiServerMux.RegisterResourceGroup(listenerName, resourceName); err != nil {
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

	// Store certs and keys in global maps
	caCertMap[resourceName] = caCertRaw
	caKeyMap[resourceName] = caKeyRaw
	etcdCertMap[resourceName] = etcdCertRaw
	etcdKeyMap[resourceName] = etcdKeyRaw

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

	// In our workflow, we need to have an API server initialized before we can
	// create the cluster, so we create a server instance whose name equals the cluster
	// name before having any control plane.
	apiServerPod := resourceName
	err = apiServerMux.AddAPIServer(resourceName, apiServerPod, caCert, caKey.(*rsa.PrivateKey))
	if err != nil {
		setupLog.Error(err, "Failed to generate etcdCert", "resourceName", resourceName)
		http.Error(w, "Failed to generate etcdCert", http.StatusInternalServerError)
		setupLog.Error(err, "failed to add API server")
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
	if err = c.Create(ctx, kubeletCm); err != nil {
		setupLog.Error(err, "Failed to create kubelet configmap", "resourceName", resourceName)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	// Create kube-proxy DaemonSet
	daemonSet := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-proxy",
			Namespace: "kube-system",
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"k8s-app": "kube-proxy",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"k8s-app": "kube-proxy",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "kube-proxy",
							Image: "k8s.gcr.io/kube-proxy:v1.21.0", // Use appropriate version
							Command: []string{
								"kube-proxy",
								"--config=/var/lib/kube-proxy/config.conf",
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "kube-proxy",
									MountPath: "/var/lib/kube-proxy",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "kube-proxy",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "kube-proxy",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	if err = c.Create(ctx, daemonSet); err != nil {
		setupLog.Error(err, "Failed to create kube-proxy DaemonSet", "resourceName", resourceName)
		http.Error(w, "Failed to create kube-proxy DaemonSet", http.StatusInternalServerError)
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
		Namespace    string            `json:"namespace"`
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
	resourceName := requestData.ResourceName
	nodeName := requestData.NodeName

	listener := cloudMgr.GetResourceGroup(resourceName)
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

	// Start a goroutine to update the LastHeartbeatTime every 10 seconds
	go func(nodeName string) {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
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

				if err := c.Update(ctx, node); err != nil {
					setupLog.Error(err, "Failed to update node heartbeat", "nodeName", nodeName)
				}
			}
		}
	}(ctx, nodeName)

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
	microTimeNow := metav1.NewMicroTime(time.Now())
	lease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nodeName,
			Namespace: "kube-node-lease",
		},
		Spec: coordinationv1.LeaseSpec{
			HolderIdentity:       &nodeName,
			LeaseDurationSeconds: int32Ptr(600),
			AcquireTime:          &microTimeNow,
			RenewTime:            &microTimeNow,
		},
	}

	err = c.Create(ctx, lease)
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			setupLog.Info("Lease already exists", "leaseName", nodeName)
		} else {
			setupLog.Error(err, "Error creating lease", "leaseName", nodeName, "namespace", metav1.NamespaceDefault)
			http.Error(w, "Error creating lease", http.StatusInternalServerError)
			return
		}
	}
	fmt.Printf("Created lease object: %v\n", lease)

	// If the node is not a control-plane, we're done here
	_, isControlPlane := nodeLabels["cluster.x-k8s.io/control-plane"]

	if !isControlPlane {
		return
	}

	// Create apiServer pod if the node is a control-plane
	caCertRaw, ok := caCertMap[resourceName]
	if !ok {
		logLine := fmt.Sprintf("Error adding node %s: %s", nodeName, err)
		fmt.Printf("caCert not found for resource: %s\n", resourceName)
		http.Error(w, logLine, http.StatusInternalServerError)
		return
	}

	caKeyRaw, ok := caKeyMap[requestData.ResourceName]
	if !ok {
		logLine := fmt.Sprintf("Error adding node %s: %s", nodeName, err)
		fmt.Printf("caKey not found for resource: %s\n", resourceName)
		http.Error(w, logLine, http.StatusInternalServerError)
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

	apiServerPodName := nodeName
	err = apiServerMux.AddAPIServer(resourceName, apiServerPodName, caCert, caKey.(*rsa.PrivateKey))
	if err != nil {
		setupLog.Error(err, "Failed to generate etcdCert", "resourceName", resourceName)
		http.Error(w, "Failed to generate etcdCert", http.StatusInternalServerError)
		setupLog.Error(err, "failed to add API server")
		return
	}

	// Create the apiserver pod
	apiServer := fmt.Sprintf("kube-apiserver-%s", nodeName)

	apiServerPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceSystem,
			Name:      apiServer,
			Labels: map[string]string{
				"component": "kube-apiserver",
				"tier":      "control-plane",
			},
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{
					Type:               corev1.PodReadyToStartContainers,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: timeOutput,
				},
				{
					Type:               corev1.PodInitialized,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: timeOutput,
				},
				{
					Type:               corev1.PodReady,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: timeOutput,
				},
				{
					Type:               corev1.ContainersReady,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: timeOutput,
				},
				{
					Type:               corev1.PodScheduled,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: timeOutput,
				},
			},
		},
	}

	if err := c.Create(ctx, apiServerPod); err != nil && !apierrors.IsAlreadyExists(err) {
		setupLog.Error(err, "failed to create api server pod")
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	// Create the controllerManager pod
	controllerManagerPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceSystem,
			Name:      fmt.Sprintf("kube-controller-manager-%s", nodeName),
			Labels: map[string]string{
				"component": "kube-controller-manager",
				"tier":      "control-plane",
			},
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{
					Type:               corev1.PodReadyToStartContainers,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: timeOutput,
				},
				{
					Type:               corev1.PodInitialized,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: timeOutput,
				},
				{
					Type:               corev1.PodReady,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: timeOutput,
				},
				{
					Type:               corev1.ContainersReady,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: timeOutput,
				},
				{
					Type:               corev1.PodScheduled,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: timeOutput,
				},
			},
		},
	}

	if err := c.Create(ctx, controllerManagerPod); err != nil && !apierrors.IsAlreadyExists(err) {
		setupLog.Error(err, "failed to create controllerManager pod")
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	// Create schedulerPod
	schedulerPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceSystem,
			Name:      fmt.Sprintf("kube-scheduler-%s", nodeName),
			Labels: map[string]string{
				"component": "kube-scheduler",
				"tier":      "control-plane",
			},
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{
					Type:               corev1.PodReadyToStartContainers,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: timeOutput,
				},
				{
					Type:               corev1.PodInitialized,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: timeOutput,
				},
				{
					Type:               corev1.PodReady,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: timeOutput,
				},
				{
					Type:               corev1.ContainersReady,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: timeOutput,
				},
				{
					Type:               corev1.PodScheduled,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: timeOutput,
				},
			},
		},
	}

	if err := c.Create(ctx, schedulerPod); err != nil && !apierrors.IsAlreadyExists(err) {
		setupLog.Error(err, "failed to create scheduler Pod")
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	// Creating Etcd member
	etcdCertRaw, ok := etcdCertMap[resourceName]
	if !ok {
		logLine := fmt.Sprintf("Error adding node %s: %s", nodeName, err)
		fmt.Printf("etcdCert not found for resource: %s\n", resourceName)
		http.Error(w, logLine, http.StatusInternalServerError)
		return
	}

	etcdKeyRaw, ok := etcdKeyMap[resourceName]
	if !ok {
		logLine := fmt.Sprintf("Error adding node %s: %s", nodeName, err)
		fmt.Printf("etcdKey not found for resource: %s\n", resourceName)
		http.Error(w, logLine, http.StatusInternalServerError)
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

	err = apiServerMux.AddEtcdMember(resourceName, nodeName, etcdCert, etcdKey.(*rsa.PrivateKey))
	if err != nil {
		setupLog.Error(err, "failed to add etcd member")
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	// Create the etcd pod
	etcdPodMember := fmt.Sprintf("etcd-%s", nodeName)
	etcdPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceSystem,
			Name:      etcdPodMember,
			Labels: map[string]string{
				"component": "etcd",
				"tier":      "control-plane",
			},
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
	if err := c.Get(ctx, client.ObjectKeyFromObject(etcdPod), etcdPod); err != nil {
		if !apierrors.IsNotFound(err) {
			setupLog.Error(err, "failed to get etcd pod")
			http.Error(w, "", http.StatusInternalServerError)
			return
		}

		// Gets info about the current etcd cluster, if any.
		info, ok := etcdInfoMap[requestData.ResourceName]
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

		etcdInfoMap[requestData.ResourceName] = info

		if err := c.Create(ctx, etcdPod); err != nil && !apierrors.IsAlreadyExists(err) {
			setupLog.Error(err, "failed to create etcd pod")
			http.Error(w, "Failed to create etcd pod", http.StatusInternalServerError)
			return
		}
	}
}

func main() {
	debug := os.Getenv("DEBUG")
	logLevel := zapcore.InfoLevel // Default log level
	if debug == "true" {
		logLevel = zapcore.DebugLevel // Set log level to Debug if DEBUG=true
	}
	log.SetLogger(zap.New(zap.UseDevMode(true), zap.Level(logLevel)))
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

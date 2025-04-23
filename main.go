/*
Copyright 2021 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	"github.com/metal3-io/cluster-api-provider-metal3/baremetal"
	infraremote "github.com/metal3-io/cluster-api-provider-metal3/baremetal/remote"
	"github.com/metal3-io/cluster-api-provider-metal3/controllers"
	webhooks "github.com/metal3-io/cluster-api-provider-metal3/internal/webhooks/v1beta1"
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	"github.com/spf13/pflag"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/component-base/logs"
	logsv1 "k8s.io/component-base/logs/api/v1"
	_ "k8s.io/component-base/logs/json/register"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/clustercache"
	"sigs.k8s.io/cluster-api/controllers/remote"
	caipamv1 "sigs.k8s.io/cluster-api/exp/ipam/api/v1alpha1"
	"sigs.k8s.io/cluster-api/util/flags"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// Constants for TLS versions.
const (
	// out-of-service taint strategy (GA from 1.28).
	minK8sMajorVersionOutOfServiceTaint   = 1
	minK8sMinorVersionGAOutOfServiceTaint = 28
	leaderElectionLeaseTimeout            = 15 * time.Second
	leaderElectionRenewTimeout            = 10 * time.Second
	leaderElectionRetryTimeout            = 2 * time.Second
	defaultMinSyncPeriod                  = 10 * time.Minute
	apiGroupWaitTimeout                   = 10 * time.Second
)

var (
	myscheme                         = runtime.NewScheme()
	setupLog                         = ctrl.Log.WithName("setup")
	controllerName                   = "cluster-api-provider-metal3-manager"
	waitForMetal3Controller          = false
	enableLeaderElection             bool
	leaderElectionLeaseDuration      time.Duration
	leaderElectionRenewDeadline      time.Duration
	leaderElectionRetryPeriod        time.Duration
	syncPeriod                       time.Duration
	clusterCacheClientQPS            float32
	clusterCacheClientBurst          int
	clusterCacheConcurrency          int
	metal3MachineConcurrency         int
	metal3ClusterConcurrency         int
	metal3DataTemplateConcurrency    int
	metal3DataConcurrency            int
	metal3LabelSyncConcurrency       int
	metal3MachineTemplateConcurrency int
	metal3RemediationConcurrency     int
	restConfigQPS                    float32
	restConfigBurst                  int
	webhookPort                      int
	webhookCertDir                   string
	healthAddr                       string
	watchNamespace                   string
	watchFilterValue                 string
	logOptions                       = logs.NewOptions()
	enableBMHNameBasedPreallocation  bool
	managerOptions                   = flags.ManagerOptions{}
)

func init() {
	_ = scheme.AddToScheme(myscheme)
	_ = ipamv1.AddToScheme(myscheme)
	_ = caipamv1.AddToScheme(myscheme)
	_ = infrav1.AddToScheme(myscheme)
	_ = clusterv1.AddToScheme(myscheme)
	_ = bmov1alpha1.AddToScheme(myscheme)
}

// Add RBAC for the authorized diagnostics endpoint.
// +kubebuilder:rbac:groups=authentication.k8s.io,resources=tokenreviews,verbs=create
// +kubebuilder:rbac:groups=authorization.k8s.io,resources=subjectaccessreviews,verbs=create

func main() {
	initFlags(pflag.CommandLine)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()

	if err := logsv1.ValidateAndApply(logOptions, nil); err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	ctrl.SetLogger(klog.Background())
	restConfig := ctrl.GetConfigOrDie()
	restConfig.QPS = restConfigQPS
	restConfig.Burst = restConfigBurst
	restConfig.UserAgent = "controllerName"

	tlsOptions, metricsOptions, err := flags.GetManagerOptions(managerOptions)
	if err != nil {
		setupLog.Error(err, "Unable to start manager: invalid flags")
		os.Exit(1)
	}

	var watchNamespaces map[string]cache.Config
	if watchNamespace != "" {
		watchNamespaces = map[string]cache.Config{
			watchNamespace: {},
		}
	}

	req, _ := labels.NewRequirement(clusterv1.ClusterNameLabel, selection.Exists, nil)
	clusterSecretCacheSelector := labels.NewSelector().Add(*req)

	mgr, err := ctrl.NewManager(restConfig, ctrl.Options{
		Scheme:                     myscheme,
		LeaseDuration:              &leaderElectionLeaseDuration,
		RenewDeadline:              &leaderElectionRenewDeadline,
		RetryPeriod:                &leaderElectionRetryPeriod,
		LeaderElection:             enableLeaderElection,
		LeaderElectionID:           "controller-leader-election-capm3",
		LeaderElectionResourceLock: resourcelock.LeasesResourceLock,
		HealthProbeBindAddress:     healthAddr,
		Metrics:                    *metricsOptions,
		Cache: cache.Options{
			DefaultNamespaces: watchNamespaces,
			SyncPeriod:        &syncPeriod,
			ByObject: map[client.Object]cache.ByObject{
				// Note: Only Secrets with the cluster name label are cached.
				// The default client of the manager won't use the cache for secrets at all (see Client.Cache.DisableFor).
				// The cached secrets will only be used by the secretCachingClient we create below.
				&corev1.Secret{}: {
					Label: clusterSecretCacheSelector,
				},
			},
		},
		Client: client.Options{
			Cache: &client.CacheOptions{
				DisableFor: []client.Object{
					&bmov1alpha1.BareMetalHost{},
					&corev1.ConfigMap{},
					&corev1.Secret{},
				},
			},
		},
		WebhookServer: webhook.NewServer(
			webhook.Options{
				Port:    webhookPort,
				CertDir: webhookCertDir,
				TLSOpts: tlsOptions,
			},
		),
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if waitForMetal3Controller {
		err = waitForAPIs(ctrl.GetConfigOrDie())
		if err != nil {
			setupLog.Error(err, "unable to discover required APIs")
			os.Exit(1)
		}
	}

	// Setup the context that's going to be used in controllers and for the manager.
	ctx := ctrl.SetupSignalHandler()

	baremetal.EnableBMHNameBasedPreallocation = enableBMHNameBasedPreallocation

	setupChecks(mgr)
	setupReconcilers(ctx, mgr)
	setupWebhooks(mgr)

	// +kubebuilder:scaffold:builder
	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func initFlags(fs *pflag.FlagSet) {
	logs.AddFlags(fs, logs.SkipLoggingConfigurationFlags())
	logsv1.AddFlags(logOptions, fs)
	var maxClusterCacheQPS float32 = 20
	maxClusterCacheClientBurst := 30
	defaultWebhookPort := 9443
	defaultConcurrency := 10
	var defaultKubeAPIQPS float32 = 20
	defaultKubeAPIBurst := 30

	fs.BoolVar(
		&enableLeaderElection,
		"leader-elect",
		false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.",
	)

	fs.BoolVar(
		&enableBMHNameBasedPreallocation,
		"enableBMHNameBasedPreallocation",
		false,
		"If set to true, it enables PreAllocation field to use Metal3IPClaim name structured with BaremetalHost and M3IPPool names",
	)

	fs.DurationVar(
		&leaderElectionLeaseDuration,
		"leader-elect-lease-duration",
		leaderElectionLeaseTimeout,
		"Interval at which non-leader candidates will wait to force acquire leadership (duration string)",
	)

	fs.DurationVar(
		&leaderElectionRenewDeadline,
		"leader-elect-renew-deadline",
		leaderElectionRenewTimeout,
		"Duration that the leading controller manager will retry refreshing leadership before giving up (duration string)",
	)

	fs.DurationVar(
		&leaderElectionRetryPeriod,
		"leader-elect-retry-period",
		leaderElectionRetryTimeout,
		"Duration the LeaderElector clients should wait between tries of actions (duration string)",
	)

	fs.StringVar(
		&watchNamespace,
		"namespace",
		"",
		"Namespace that the controller watches to reconcile CAPM3 objects. If unspecified, the controller watches for CAPM3 objects across all namespaces.",
	)

	fs.StringVar(
		&watchFilterValue,
		"watch-filter",
		"",
		fmt.Sprintf("Label value that the controller watches to reconcile cluster-api objects. Label key is always %s. If unspecified, the controller watches for all cluster-api objects.", clusterv1.WatchLabel),
	)

	fs.DurationVar(
		&syncPeriod,
		"sync-period",
		defaultMinSyncPeriod,
		"The minimum interval at which watched resources are reconciled (e.g. 15m)",
	)

	fs.Float32Var(
		&clusterCacheClientQPS,
		"clustercache-client-qps",
		maxClusterCacheQPS,
		"Maximum queries per second from the cluster cache clients to the Kubernetes API server of workload clusters.",
	)

	fs.IntVar(
		&clusterCacheClientBurst,
		"clustercache-client-burst",
		maxClusterCacheClientBurst,
		"Maximum number of queries that should be allowed in one burst from the cluster cache clients to the Kubernetes API server of workload clusters.",
	)

	fs.IntVar(
		&webhookPort,
		"webhook-port",
		defaultWebhookPort,
		"Webhook Server port",
	)

	fs.StringVar(
		&webhookCertDir,
		"webhook-cert-dir",
		"/tmp/k8s-webhook-server/serving-certs/",
		"Webhook cert dir, only used when webhook-port is specified.",
	)

	fs.StringVar(
		&healthAddr,
		"health-addr",
		":9440",
		"The address the health endpoint binds to.",
	)

	fs.IntVar(&metal3MachineConcurrency, "metal3machine-concurrency", defaultConcurrency,
		"Number of metal3machines to process simultaneously. WARNING! Currently not safe to set > 1.")

	fs.IntVar(&metal3ClusterConcurrency, "metal3cluster-concurrency", defaultConcurrency,
		"Number of metal3clusters to process simultaneously")

	fs.IntVar(&metal3DataTemplateConcurrency, "metal3datatemplate-concurrency", defaultConcurrency,
		"Number of metal3datatemplates to process simultaneously")

	fs.IntVar(&metal3DataConcurrency, "metal3data-concurrency", defaultConcurrency,
		"Number of metal3data to process simultaneously")

	fs.IntVar(&metal3LabelSyncConcurrency, "metal3labelsync-concurrency", defaultConcurrency,
		"Number of metal3labelsyncs to process simultaneously")

	fs.IntVar(&metal3MachineTemplateConcurrency, "metal3machinetemplate-concurrency", defaultConcurrency,
		"Number of metal3machinetemplates to process simultaneously")

	fs.IntVar(&metal3RemediationConcurrency, "metal3remediation-concurrency", defaultConcurrency,
		"Number of metal3remediations to process simultaneously")

	fs.Float32Var(&restConfigQPS, "kube-api-qps", defaultKubeAPIQPS,
		"Maximum queries per second from the controller client to the Kubernetes API server. Default 20")

	fs.IntVar(&restConfigBurst, "kube-api-burst", defaultKubeAPIBurst,
		"Maximum number of queries that should be allowed in one burst from the controller client to the Kubernetes API server. Default 30")

	flags.AddManagerOptions(fs, &managerOptions)
}

func waitForAPIs(cfg *rest.Config) error {
	c, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return err
	}

	metal3GV := schema.GroupVersion{
		Group:   "metal3.io",
		Version: "v1alpha1",
	}

	for {
		err = discovery.ServerSupportsVersion(c, metal3GV)
		if err != nil {
			setupLog.Info(fmt.Sprintf("Waiting for API group %v to be available: %v", metal3GV, err))
			time.Sleep(apiGroupWaitTimeout)
			continue
		}
		setupLog.Info(fmt.Sprintf("Found API group %v", metal3GV))
		break
	}

	return nil
}

func setupChecks(mgr ctrl.Manager) {
	if err := mgr.AddReadyzCheck("webhook", mgr.GetWebhookServer().StartedChecker()); err != nil {
		setupLog.Error(err, "unable to create ready check")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("webhook", mgr.GetWebhookServer().StartedChecker()); err != nil {
		setupLog.Error(err, "unable to create health check")
		os.Exit(1)
	}
}

func setupReconcilers(ctx context.Context, mgr ctrl.Manager) {
	secretCachingClient, err := client.New(mgr.GetConfig(), client.Options{
		HTTPClient: mgr.GetHTTPClient(),
		Cache: &client.CacheOptions{
			Reader: mgr.GetCache(),
		},
	})
	if err != nil {
		setupLog.Error(err, "unable to create secret caching client")
		os.Exit(1)
	}

	// Set up a ClusterCache and ClusterCacheReconciler to provide to controllers
	// requiring a connection to a remote cluster

	clusterCache, err := clustercache.SetupWithManager(ctx, mgr, clustercache.Options{
		SecretClient: secretCachingClient,
		Cache: clustercache.CacheOptions{
			Indexes: []clustercache.CacheOptionsIndex{clustercache.NodeProviderIDIndex},
		},
		Client: clustercache.ClientOptions{
			QPS:       clusterCacheClientQPS,
			Burst:     clusterCacheClientBurst,
			UserAgent: remote.DefaultClusterAPIUserAgent(controllerName),
			Cache: clustercache.ClientCacheOptions{
				DisableFor: []client.Object{
					// Don't cache ConfigMaps & Secrets.
					&corev1.ConfigMap{},
					&corev1.Secret{},
					// Don't cache Pods & DaemonSets (we get/list them e.g. during drain).
					&corev1.Pod{},
					&appsv1.DaemonSet{},
					// Don't cache PersistentVolumes and VolumeAttachments (we get/list them e.g. during wait for volumes to detach)
					&storagev1.VolumeAttachment{},
					&corev1.PersistentVolume{},
				},
			},
		},
	}, concurrency(clusterCacheConcurrency))
	if err != nil {
		setupLog.Error(err, "Unable to create ClusterCache")
		os.Exit(1)
	}
	if err := (&controllers.Metal3MachineReconciler{
		Client:           mgr.GetClient(),
		ClusterCache:     clusterCache,
		ManagerFactory:   baremetal.NewManagerFactory(mgr.GetClient()),
		Log:              ctrl.Log.WithName("controllers").WithName("Metal3Machine"),
		CapiClientGetter: infraremote.NewClusterClient,
		WatchFilterValue: watchFilterValue,
	}).SetupWithManager(ctx, mgr, concurrency(metal3MachineConcurrency)); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Metal3MachineReconciler")
		os.Exit(1)
	}

	if err := (&controllers.Metal3ClusterReconciler{
		Client:           mgr.GetClient(),
		ClusterCache:     clusterCache,
		ManagerFactory:   baremetal.NewManagerFactory(mgr.GetClient()),
		Log:              ctrl.Log.WithName("controllers").WithName("Metal3Cluster"),
		WatchFilterValue: watchFilterValue,
	}).SetupWithManager(ctx, mgr, concurrency(metal3ClusterConcurrency)); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Metal3ClusterReconciler")
		os.Exit(1)
	}

	if err := (&controllers.Metal3DataTemplateReconciler{
		Client:           mgr.GetClient(),
		ClusterCache:     clusterCache,
		ManagerFactory:   baremetal.NewManagerFactory(mgr.GetClient()),
		Log:              ctrl.Log.WithName("controllers").WithName("Metal3DataTemplate"),
		WatchFilterValue: watchFilterValue,
	}).SetupWithManager(ctx, mgr, concurrency(metal3DataTemplateConcurrency)); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Metal3DataTemplateReconciler")
		os.Exit(1)
	}

	if err := (&controllers.Metal3DataReconciler{
		Client:           mgr.GetClient(),
		ClusterCache:     clusterCache,
		ManagerFactory:   baremetal.NewManagerFactory(mgr.GetClient()),
		Log:              ctrl.Log.WithName("controllers").WithName("Metal3Data"),
		WatchFilterValue: watchFilterValue,
	}).SetupWithManager(ctx, mgr, concurrency(metal3DataConcurrency)); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Metal3DataReconciler")
		os.Exit(1)
	}

	if err := (&controllers.Metal3LabelSyncReconciler{
		Client:           mgr.GetClient(),
		ClusterCache:     clusterCache,
		ManagerFactory:   baremetal.NewManagerFactory(mgr.GetClient()),
		Log:              ctrl.Log.WithName("controllers").WithName("Metal3LabelSync"),
		CapiClientGetter: infraremote.NewClusterClient,
	}).SetupWithManager(ctx, mgr, concurrency(metal3LabelSyncConcurrency)); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Metal3LabelSyncReconciler")
		os.Exit(1)
	}

	if err := (&controllers.Metal3MachineTemplateReconciler{
		Client:         mgr.GetClient(),
		ClusterCache:   clusterCache,
		ManagerFactory: baremetal.NewManagerFactory(mgr.GetClient()),
		Log:            ctrl.Log.WithName("controllers").WithName("Metal3MachineTemplate"),
	}).SetupWithManager(ctx, mgr, concurrency(metal3MachineTemplateConcurrency)); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Metal3MachineTemplateReconciler")
		os.Exit(1)
	}

	isOOSTSupported, err := isOutOfServiceTaintSupported(mgr.GetConfig())
	if err != nil {
		setupLog.Error(err, "unable to detect support for Out-of-service taint")
	}
	if err := (&controllers.Metal3RemediationReconciler{
		Client:                     mgr.GetClient(),
		ClusterCache:               clusterCache,
		ManagerFactory:             baremetal.NewManagerFactory(mgr.GetClient()),
		Log:                        ctrl.Log.WithName("controllers").WithName("Metal3Remediation"),
		IsOutOfServiceTaintEnabled: isOOSTSupported,
	}).SetupWithManager(ctx, mgr, concurrency(metal3RemediationConcurrency)); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Metal3Remediation")
		os.Exit(1)
	}
}

func setupWebhooks(mgr ctrl.Manager) {
	if err := (&webhooks.Metal3Cluster{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "Metal3Cluster")
		os.Exit(1)
	}

	if err := (&webhooks.Metal3Machine{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "Metal3Machine")
		os.Exit(1)
	}

	if err := (&webhooks.Metal3MachineTemplate{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "Metal3MachineTemplate")
		os.Exit(1)
	}

	if err := (&webhooks.Metal3DataTemplate{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "Metal3DataTemplate")
		os.Exit(1)
	}

	if err := (&webhooks.Metal3Data{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "Metal3Data")
		os.Exit(1)
	}

	if err := (&webhooks.Metal3DataClaim{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "Metal3DataClaim")
		os.Exit(1)
	}

	if err := (&webhooks.Metal3Remediation{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "Metal3Remediation")
		os.Exit(1)
	}

	if err := (&webhooks.Metal3RemediationTemplate{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "Metal3RemediationTemplate")
		os.Exit(1)
	}

	if err := (&webhooks.Metal3ClusterTemplate{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "Metal3ClusterTemplate")
		os.Exit(1)
	}
}

func concurrency(c int) controller.Options {
	return controller.Options{MaxConcurrentReconciles: c}
}

func isOutOfServiceTaintSupported(config *rest.Config) (bool, error) {
	cs, err := kubernetes.NewForConfig(config)
	if err != nil || cs == nil {
		if cs == nil {
			err = errors.New("k8s client set is nil")
		}
		setupLog.Error(err, "unable to get k8s client")
		return false, err
	}

	k8sVersion, err := cs.Discovery().ServerVersion()
	if err != nil || k8sVersion == nil {
		if k8sVersion == nil {
			err = errors.New("k8s server version is nil")
		}
		setupLog.Error(err, "unable to get k8s server version")
		return false, err
	}

	major, err := strconv.Atoi(k8sVersion.Major)
	if err != nil {
		setupLog.Error(err, "could not parse k8s server major version", "major version", k8sVersion.Major)
		return false, err
	}
	minor, err := strconv.Atoi(k8sVersion.Minor)
	if err != nil {
		setupLog.Error(err, "could not convert k8s server minor version", "minor version", k8sVersion.Minor)
		return false, err
	}

	isSupported := major > minK8sMajorVersionOutOfServiceTaint ||
		(major == minK8sMajorVersionOutOfServiceTaint &&
			minor >= minK8sMinorVersionGAOutOfServiceTaint)

	setupLog.Info("out-of-service taint", "supported", isSupported)
	return isSupported, nil
}

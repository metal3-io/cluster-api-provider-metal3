// Borrowed and adapted from sigs.k8s.io/cluster-api/test/extension/main.go

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	goruntime "runtime"
	"time"

	infrav1beta1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	"github.com/metal3-io/cluster-api-provider-metal3/test/extension/handlers/inplaceupdate"
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	logsv1 "k8s.io/component-base/logs/api/v1"
	_ "k8s.io/component-base/logs/json/register"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"
	controlplanev1 "sigs.k8s.io/cluster-api/api/controlplane/kubeadm/v1beta2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	runtimehooksv1 "sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	runtimecatalog "sigs.k8s.io/cluster-api/exp/runtime/catalog"
	"sigs.k8s.io/cluster-api/exp/runtime/server"
	"sigs.k8s.io/cluster-api/feature"
	"sigs.k8s.io/cluster-api/util/flags"
	"sigs.k8s.io/cluster-api/version"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
)

const (
	// Leader election defaults.
	defaultLeaderElectionLeaseDuration = 15 * time.Second
	defaultLeaderElectionRenewDeadline = 10 * time.Second
	defaultLeaderElectionRetryPeriod   = 2 * time.Second
	defaultSyncPeriod                  = 10 * time.Minute
	defaultKubeAPIQPS                  = float32(20)
	defaultKubeAPIBurst                = 30
	defaultWebhookPort                 = 9443
)

var (
	// catalog contains all information about RuntimeHooks.
	catalog = runtimecatalog.New()

	// scheme is a Kubernetes runtime scheme containing all the information about API types used by the test extension.
	// NOTE: it is not mandatory to use scheme in custom RuntimeExtension, but working with typed API objects makes code
	// easier to read and less error-prone than using unstructured or working with raw json/yaml.
	scheme = runtime.NewScheme()
	// Creates a logger to be used during the main func using controller runtime utilities
	// NOTE: it is not mandatory to use controller runtime utilities in custom RuntimeExtension, but it is recommended
	// because it makes log from those components similar to log from controllers.
	setupLog       = ctrl.Log.WithName("setup")
	controllerName = "capm3-test-extension-manager"

	// flags.
	enableLeaderElection        bool
	leaderElectionLeaseDuration time.Duration
	leaderElectionRenewDeadline time.Duration
	leaderElectionRetryPeriod   time.Duration
	profilerAddress             string
	enableContentionProfiling   bool
	syncPeriod                  time.Duration
	restConfigQPS               float32
	restConfigBurst             int
	webhookPort                 int
	webhookCertDir              string
	webhookCertName             string
	webhookKeyName              string
	healthAddr                  string
	managerOptions              = flags.ManagerOptions{}
	logOptions                  = logs.NewOptions()
)

func init() {
	// Adds to the scheme all the API types used by the test extension.
	_ = clientgoscheme.AddToScheme(scheme)
	_ = apiextensionsv1.AddToScheme(scheme)
	_ = infrav1beta1.AddToScheme(scheme)
	_ = ipamv1.AddToScheme(scheme)
	_ = bootstrapv1.AddToScheme(scheme)
	_ = controlplanev1.AddToScheme(scheme)
	_ = clusterv1.AddToScheme(scheme)

	// Register the RuntimeHook types into the catalog.
	_ = runtimehooksv1.AddToCatalog(catalog)
}

// InitFlags initializes the flags.
func InitFlags(fs *pflag.FlagSet) {
	// Initialize logs flags using Kubernetes component-base machinery.
	// NOTE: it is not mandatory to use Kubernetes component-base machinery in custom RuntimeExtension, but it is
	// recommended because it helps in ensuring consistency across different components in the cluster.
	logsv1.AddFlags(logOptions, fs)

	fs.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")

	fs.DurationVar(&leaderElectionLeaseDuration, "leader-elect-lease-duration", defaultLeaderElectionLeaseDuration,
		"Interval at which non-leader candidates will wait to force acquire leadership (duration string)")

	fs.DurationVar(&leaderElectionRenewDeadline, "leader-elect-renew-deadline", defaultLeaderElectionRenewDeadline,
		"Duration that the leading controller manager will retry refreshing leadership before giving up (duration string)")

	fs.DurationVar(&leaderElectionRetryPeriod, "leader-elect-retry-period", defaultLeaderElectionRetryPeriod,
		"Duration the LeaderElector clients should wait between tries of actions (duration string)")

	fs.StringVar(&profilerAddress, "profiler-address", "",
		"Bind address to expose the pprof profiler (e.g. localhost:6060)")

	fs.BoolVar(&enableContentionProfiling, "contention-profiling", false,
		"Enable block profiling")

	fs.DurationVar(&syncPeriod, "sync-period", defaultSyncPeriod,
		"The minimum interval at which watched resources are reconciled (e.g. 15m)")

	fs.Float32Var(&restConfigQPS, "kube-api-qps", defaultKubeAPIQPS,
		"Maximum queries per second from the controller client to the Kubernetes API server.")

	fs.IntVar(&restConfigBurst, "kube-api-burst", defaultKubeAPIBurst,
		"Maximum number of queries that should be allowed in one burst from the controller client to the Kubernetes API server.")

	fs.IntVar(&webhookPort, "webhook-port", defaultWebhookPort,
		"Webhook Server port")

	fs.StringVar(&webhookCertDir, "webhook-cert-dir", "/tmp/k8s-webhook-server/serving-certs/",
		"Webhook cert dir.")

	fs.StringVar(&webhookCertName, "webhook-cert-name", "tls.crt",
		"Webhook cert name.")

	fs.StringVar(&webhookKeyName, "webhook-key-name", "tls.key",
		"Webhook key name.")

	fs.StringVar(&healthAddr, "health-addr", ":9440",
		"The address the health endpoint binds to.")

	flags.AddManagerOptions(fs, &managerOptions)

	feature.MutableGates.AddFlag(fs)

	// Add test-extension specific flags
	// NOTE: it is not mandatory to use the same flag names in all RuntimeExtension, but it is recommended when
	// addressing common concerns like profiler-address, webhook-port, webhook-cert-dir etc. because it helps in ensuring
	// consistency across different components in the cluster.
}

// Add RBAC for the authorized diagnostics endpoint.
// +kubebuilder:rbac:groups=authentication.k8s.io,resources=tokenreviews,verbs=create
// +kubebuilder:rbac:groups=authorization.k8s.io,resources=subjectaccessreviews,verbs=create

func main() {
	InitFlags(pflag.CommandLine)
	pflag.CommandLine.SetNormalizeFunc(cliflag.WordSepNormalizeFunc)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	// Set log level 2 as default.
	if err := pflag.CommandLine.Set("v", "2"); err != nil {
		klog.Errorf("Failed to set default log level: %v", err)
		os.Exit(1)
	}
	pflag.Parse()

	// Validates logs flags using Kubernetes component-base machinery and apply them
	// so klog will automatically use the right logger.
	// NOTE: klog is the log of choice of component-base machinery.
	if err := logsv1.ValidateAndApply(logOptions, nil); err != nil {
		klog.Errorf("Unable to start manager: %v", err)
		os.Exit(1)
	}

	// Add the klog logger in the context.
	// NOTE: it is not mandatory to use contextual logging in custom RuntimeExtension, but it is recommended
	// because it allows to use a log stored in the context across the entire chain of calls (without
	// requiring an addition log parameter in all the functions).
	ctrl.SetLogger(klog.Background())

	// Note: setupLog can only be used after ctrl.SetLogger was called
	setupLog.Info(fmt.Sprintf("Version: %s (git commit: %s)", version.Get().String(), version.Get().GitCommit))

	restConfig := ctrl.GetConfigOrDie()
	restConfig.QPS = restConfigQPS
	restConfig.Burst = restConfigBurst
	restConfig.UserAgent = remote.DefaultClusterAPIUserAgent(controllerName)

	tlsOptions, metricsOptions, err := flags.GetManagerOptions(managerOptions)
	if err != nil {
		setupLog.Error(err, "Unable to start manager: invalid flags")
		os.Exit(1)
	}

	if enableContentionProfiling {
		goruntime.SetBlockProfileRate(1)
	}

	// Create an HTTP server for serving Runtime Extensions.
	runtimeExtensionWebhookServer, err := server.New(server.Options{
		Port:     webhookPort,
		CertDir:  webhookCertDir,
		CertName: webhookCertName,
		KeyName:  webhookKeyName,
		TLSOpts:  tlsOptions,
		Catalog:  catalog,
	})
	if err != nil {
		setupLog.Error(err, "Error creating runtime extension webhook server")
		os.Exit(1)
	}

	ctrlOptions := ctrl.Options{
		Controller: config.Controller{
			UsePriorityQueue: ptr.To[bool](feature.Gates.Enabled(feature.PriorityQueue)),
		},
		Scheme:                     scheme,
		LeaderElection:             enableLeaderElection,
		LeaderElectionID:           "controller-leader-election-capm3-test-extension",
		LeaseDuration:              &leaderElectionLeaseDuration,
		RenewDeadline:              &leaderElectionRenewDeadline,
		RetryPeriod:                &leaderElectionRetryPeriod,
		LeaderElectionResourceLock: resourcelock.LeasesResourceLock,
		HealthProbeBindAddress:     healthAddr,
		PprofBindAddress:           profilerAddress,
		Metrics:                    *metricsOptions,
		Cache: cache.Options{
			SyncPeriod: &syncPeriod,
		},
		Client: client.Options{
			Cache: &client.CacheOptions{
				DisableFor: []client.Object{
					&corev1.ConfigMap{},
					&corev1.Secret{},
				},
				// Use the cache for all Unstructured get/list calls.
				Unstructured: true,
			},
		},
		WebhookServer: runtimeExtensionWebhookServer,
	}

	// Start the manager
	mgr, err := ctrl.NewManager(restConfig, ctrlOptions)
	if err != nil {
		setupLog.Error(err, "Unable to start manager")
		os.Exit(1)
	}

	// Set up a context listening for SIGINT.
	ctx := ctrl.SetupSignalHandler()

	// Setup Runtime Extensions.
	setupInPlaceUpdateHookHandlers(mgr, runtimeExtensionWebhookServer)

	// Setup checks, indexes, reconcilers and webhooks.
	setupChecks(mgr)
	setupIndexes(ctx, mgr)
	setupReconcilers(ctx, mgr)
	setupWebhooks(mgr)

	setupLog.Info("Starting manager", "version", version.Get().String())
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "Problem running manager")
		os.Exit(1)
	}
}

// setupInPlaceUpdateHookHandlers sets up In-Place Update Hooks.
func setupInPlaceUpdateHookHandlers(mgr ctrl.Manager, runtimeExtensionWebhookServer *server.Server) {
	// Create the ExtensionHandlers for the in-place update hooks
	// NOTE: it is not mandatory to group all the ExtensionHandlers using a struct, what is important
	// is to have HandlerFunc with the signature defined in sigs.k8s.io/cluster-api/api/runtime/hooks/v1alpha1.
	inPlaceUpdateExtensionHandlers := inplaceupdate.NewExtensionHandlers(mgr.GetClient())

	if err := runtimeExtensionWebhookServer.AddExtensionHandler(server.ExtensionHandler{
		Hook:        runtimehooksv1.CanUpdateMachine,
		Name:        "can-update-machine",
		HandlerFunc: inPlaceUpdateExtensionHandlers.DoCanUpdateMachine,
	}); err != nil {
		setupLog.Error(err, "Error adding CanUpdateMachine handler")
		os.Exit(1)
	}

	if err := runtimeExtensionWebhookServer.AddExtensionHandler(server.ExtensionHandler{
		Hook:        runtimehooksv1.CanUpdateMachineSet,
		Name:        "can-update-machineset",
		HandlerFunc: inPlaceUpdateExtensionHandlers.DoCanUpdateMachineSet,
	}); err != nil {
		setupLog.Error(err, "Error adding CanUpdateMachineSet handler")
		os.Exit(1)
	}

	if err := runtimeExtensionWebhookServer.AddExtensionHandler(server.ExtensionHandler{
		Hook:        runtimehooksv1.UpdateMachine,
		Name:        "update-machine",
		HandlerFunc: inPlaceUpdateExtensionHandlers.DoUpdateMachine,
	}); err != nil {
		setupLog.Error(err, "Error adding UpdateMachine handler")
		os.Exit(1)
	}
}

func setupChecks(mgr ctrl.Manager) {
	if err := mgr.AddReadyzCheck("webhook", mgr.GetWebhookServer().StartedChecker()); err != nil {
		setupLog.Error(err, "Unable to create ready check")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("webhook", mgr.GetWebhookServer().StartedChecker()); err != nil {
		setupLog.Error(err, "Unable to create health check")
		os.Exit(1)
	}
}

func setupIndexes(_ context.Context, _ ctrl.Manager) {
}

func setupReconcilers(_ context.Context, _ ctrl.Manager) {
}

func setupWebhooks(_ ctrl.Manager) {
}

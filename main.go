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
	"flag"
	"fmt"
	"os"
	"time"

	bmoapis "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	infrav1alpha4 "github.com/metal3-io/cluster-api-provider-metal3/api/v1alpha4"
	infrav1alpha5 "github.com/metal3-io/cluster-api-provider-metal3/api/v1alpha5"
	"github.com/metal3-io/cluster-api-provider-metal3/baremetal"
	capm3remote "github.com/metal3-io/cluster-api-provider-metal3/baremetal/remote"
	"github.com/metal3-io/cluster-api-provider-metal3/controllers"
	ipamv1 "github.com/metal3-io/ip-address-manager/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	"k8s.io/klog/klogr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	// +kubebuilder:scaffold:imports
)

var (
	myscheme                = runtime.NewScheme()
	setupLog                = ctrl.Log.WithName("setup")
	waitForMetal3Controller = false
	metricsAddr             string
	enableLeaderElection    bool
	syncPeriod              time.Duration
	webhookPort             int
	healthAddr              string
	watchNamespace          string
)

func init() {
	_ = scheme.AddToScheme(myscheme)
	_ = ipamv1.AddToScheme(myscheme)
	_ = infrav1alpha5.AddToScheme(myscheme)
	_ = infrav1alpha4.AddToScheme(myscheme)
	_ = clusterv1.AddToScheme(myscheme)
	_ = bmoapis.AddToScheme(myscheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	klog.InitFlags(nil)
	flag.StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&watchNamespace, "namespace", "",
		"Namespace that the controller watches to reconcile CAPM3 objects. If unspecified, the controller watches for CAPM3 objects across all namespaces.")
	flag.DurationVar(&syncPeriod, "sync-period", 10*time.Minute,
		"The minimum interval at which watched resources are reconciled (e.g. 15m)")
	flag.IntVar(&webhookPort, "webhook-port", 0,
		"Webhook Server port (set to 0 to disable)")
	flag.StringVar(&healthAddr, "health-addr", ":9440",
		"The address the health endpoint binds to.")
	flag.Parse()

	ctrl.SetLogger(klogr.New())

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 myscheme,
		MetricsBindAddress:     metricsAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "controller-leader-election-capm3",
		SyncPeriod:             &syncPeriod,
		Port:                   webhookPort,
		HealthProbeBindAddress: healthAddr,
		Namespace:              watchNamespace,
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

	setupChecks(mgr)
	setupReconcilers(mgr)
	setupWebhooks(mgr)

	// +kubebuilder:scaffold:builder
	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
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
			time.Sleep(time.Second * 10)
			continue
		}
		setupLog.Info(fmt.Sprintf("Found API group %v", metal3GV))
		break
	}

	return nil
}

func setupChecks(mgr ctrl.Manager) {
	if err := mgr.AddReadyzCheck("ping", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to create ready check")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("ping", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to create health check")
		os.Exit(1)
	}
}

func setupReconcilers(mgr ctrl.Manager) {
	if webhookPort != 0 {
		return
	}
	if err := (&controllers.Metal3MachineReconciler{
		Client:           mgr.GetClient(),
		ManagerFactory:   baremetal.NewManagerFactory(mgr.GetClient()),
		Log:              ctrl.Log.WithName("controllers").WithName("Metal3Machine"),
		CapiClientGetter: capm3remote.NewClusterClient,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Metal3MachineReconciler")
		os.Exit(1)
	}

	if err := (&controllers.Metal3ClusterReconciler{
		Client:         mgr.GetClient(),
		ManagerFactory: baremetal.NewManagerFactory(mgr.GetClient()),
		Log:            ctrl.Log.WithName("controllers").WithName("Metal3Cluster"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Metal3ClusterReconciler")
		os.Exit(1)
	}

	if err := (&controllers.Metal3DataTemplateReconciler{
		Client:         mgr.GetClient(),
		ManagerFactory: baremetal.NewManagerFactory(mgr.GetClient()),
		Log:            ctrl.Log.WithName("controllers").WithName("Metal3DataTemplate"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Metal3DataTemplateReconciler")
		os.Exit(1)
	}

	if err := (&controllers.Metal3DataReconciler{
		Client:         mgr.GetClient(),
		ManagerFactory: baremetal.NewManagerFactory(mgr.GetClient()),
		Log:            ctrl.Log.WithName("controllers").WithName("Metal3Data"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Metal3DataReconciler")
		os.Exit(1)
	}

	if err := (&controllers.Metal3LabelSyncReconciler{
		Client:           mgr.GetClient(),
		ManagerFactory:   baremetal.NewManagerFactory(mgr.GetClient()),
		Log:              ctrl.Log.WithName("controllers").WithName("Metal3LabelSync"),
		CapiClientGetter: capm3remote.NewClusterClient,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Metal3LabelSyncReconciler")
		os.Exit(1)
	}
}

func setupWebhooks(mgr ctrl.Manager) {
	if webhookPort == 0 {
		return
	}

	if err := (&infrav1alpha5.Metal3Cluster{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "Metal3Cluster")
		os.Exit(1)
	}

	if err := (&infrav1alpha5.Metal3Machine{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "Metal3Machine")
		os.Exit(1)
	}

	if err := (&infrav1alpha5.Metal3MachineTemplate{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "Metal3MachineTemplate")
		os.Exit(1)
	}

	if err := (&infrav1alpha5.Metal3DataTemplate{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "Metal3DataTemplate")
		os.Exit(1)
	}

	if err := (&infrav1alpha5.Metal3Data{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "Metal3Data")
		os.Exit(1)
	}

	if err := (&infrav1alpha5.Metal3DataClaim{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "Metal3DataClaim")
		os.Exit(1)
	}
}

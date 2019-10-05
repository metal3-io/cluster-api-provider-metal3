/*
Copyright 2019 The Kubernetes Authors.

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

	bmoapis "github.com/metal3-io/baremetal-operator/pkg/apis"
	capbm "sigs.k8s.io/cluster-api-provider-baremetal/api/v1alpha2"
	"sigs.k8s.io/cluster-api-provider-baremetal/pkg/cloud/baremetal/actuators/machine"
	"sigs.k8s.io/cluster-api-provider-baremetal/pkg/manager/wrapper"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/klog"
	clusterapis "sigs.k8s.io/cluster-api/api/v1alpha2"
	capimachine "sigs.k8s.io/cluster-api/controllers"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"
)

func main() {
	klog.InitFlags(nil)

	watchNamespace := flag.String("namespace", "", "Namespace that the controller watches to reconcile machine-api objects. If unspecified, the controller watches for machine-api objects across all namespaces.")
	metricsAddr := flag.String("metrics-addr", ":8080", "The address the metric endpoint binds to.")
	flag.Parse()

	log := logf.Log.WithName("baremetal-controller-manager")
	logf.SetLogger(logf.ZapLogger(false))
	entryLog := log.WithName("entrypoint")

	cfg := config.GetConfigOrDie()
	if cfg == nil {
		panic(fmt.Errorf("GetConfigOrDie didn't die"))
	}

	err := waitForAPIs(cfg)
	if err != nil {
		entryLog.Error(err, "unable to discover required APIs")
		os.Exit(1)
	}

	// Setup a Manager
	opts := manager.Options{
		MetricsBindAddress: *metricsAddr,
	}
	if *watchNamespace != "" {
		opts.Namespace = *watchNamespace
		klog.Infof("Watching machine-api objects only in namespace %q for reconciliation.", opts.Namespace)
	}

	mgr, err := manager.New(cfg, opts)
	if err != nil {
		entryLog.Error(err, "unable to set up overall controller manager")
		os.Exit(1)
	}

	machineActuator, err := machine.NewActuator(machine.ActuatorParams{
		Client: mgr.GetClient(),
	})
	if err != nil {
		panic(err)
	}

	if err := capbm.AddToScheme(mgr.GetScheme()); err != nil {
		panic(err)
	}

	if err := clusterapis.AddToScheme(mgr.GetScheme()); err != nil {
		panic(err)
	}

	if err := bmoapis.AddToScheme(mgr.GetScheme()); err != nil {
		panic(err)
	}

	// the manager wrapper will add an extra Watch to the controller
	capimachine.AddWithActuator(wrapper.New(mgr), machineActuator)

	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		entryLog.Error(err, "unable to run manager")
		os.Exit(1)
	}
}

func waitForAPIs(cfg *rest.Config) error {
	log := logf.Log.WithName("baremetal-controller-manager")

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
			log.Info(fmt.Sprintf("Waiting for API group %v to be available: %v", metal3GV, err))
			time.Sleep(time.Second * 10)
			continue
		}
		log.Info(fmt.Sprintf("Found API group %v", metal3GV))
		break
	}

	return nil
}

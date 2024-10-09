package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	bmov1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"

	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
	rest "k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func main() {
	// Set up logger
	debug := os.Getenv("DEBUG")
	logLevel := zapcore.InfoLevel // Default log level
	if debug == "true" {
		logLevel = zapcore.DebugLevel // Set log level to Debug if DEBUG=true
	}
	log.SetLogger(zap.New(zap.UseDevMode(true), zap.Level(logLevel)))

	// Create a Kubernetes client
	config, err := rest.InClusterConfig()
	setupLog := ctrl.Log.WithName("setup")
	if err != nil {
		setupLog.Error(err, "Error getting context kubeconfig")
	}

	// Add BareMetalHost to scheme
	scheme := runtime.NewScheme()
	if err := bmov1alpha1.AddToScheme(scheme); err != nil {
		setupLog.Error(err, "Error adding BareMetalHost to scheme")
	}

	if err := infrav1.AddToScheme(scheme); err != nil {
		setupLog.Error(err, "Error adding Metal3Machine to scheme")
	}

	// Create a Kubernetes client
	mgr, err := manager.New(config, manager.Options{Scheme: scheme})
	if err != nil {
		setupLog.Error(err, "Error creating manager")
	}

	// Set up the BareMetalHost controller
	if err := ctrl.NewControllerManagedBy(mgr).
		For(&bmov1alpha1.BareMetalHost{}).
		Complete(reconcile.Func(func(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
			setupLog.Info("Detected change in BMH", "namespace", req.Namespace, "name", req.Name)
			bmh := &bmov1alpha1.BareMetalHost{}
			if err := mgr.GetClient().Get(ctx, req.NamespacedName, bmh); err != nil {
				setupLog.Error(err, "Error fetching BareMetalHost")
				return reconcile.Result{}, err
			}

			// Check if the state has changed from "available" to "provisioning"
			if bmh.Status.Provisioning.State != "provisioning" && bmh.Status.Provisioning.State != "provisioned" {
				setupLog.V(4).Info(fmt.Sprintf("BMH %s/%s state is not in 'provisioning' or 'provisioned' state.", req.Namespace, req.Name))
				return reconcile.Result{}, nil
			}
			uuid := bmh.ObjectMeta.UID
			if bmh.Spec.ConsumerRef == nil {
				return reconcile.Result{}, err
			}
			m3m := &infrav1.Metal3Machine{}
			m3mKey := client.ObjectKey{
				Namespace: bmh.Spec.ConsumerRef.Namespace,
				Name:      bmh.Spec.ConsumerRef.Name,
			}
			if err := mgr.GetClient().Get(ctx, m3mKey, m3m); err != nil {
				setupLog.Error(err, "Error fetching Metal3Machine", "namespace", bmh.Spec.ConsumerRef.Namespace, "name", bmh.Spec.ConsumerRef.Name)
				return reconcile.Result{}, err
			}
			labels := m3m.Labels
			clusterName, ok := labels["cluster.x-k8s.io/cluster-name"]
			if !ok {
				return reconcile.Result{}, err
			}
			providerID := fmt.Sprintf("metal3://%s/%s/%s", m3m.Namespace, bmh.Name, m3m.Name)
			url := "http://localhost:3333/updateNode"
			requestData := map[string]interface{}{
				"resource":   fmt.Sprintf("%s/%s", m3m.Namespace, clusterName),
				"nodeName":   m3m.Name,
				"namespace":  m3m.Namespace,
				"providerID": providerID,
				"uuid":       string(uuid),
				"labels":     labels,
			}
			jsonData, err := json.Marshal(requestData)
			if err != nil {
				setupLog.Error(err, "Error marshalling JSON")
				return reconcile.Result{}, err
			}
			setupLog.Info("Making POST request", "content", string(jsonData))
			resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
			if err != nil {
				setupLog.Error(err, "Error making POST request")
				return reconcile.Result{}, err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				setupLog.Info(fmt.Sprintf("POST request failed with status: %s", resp.Status))
				return reconcile.Result{}, fmt.Errorf("POST request failed with status: %s", resp.Status)
			}

			return reconcile.Result{}, nil
		})); err != nil {
		setupLog.Error(err, "Error setting up controller")
	}

	// Start the manager
	setupLog.Info("Starting controller...")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "Error starting manager")
	}
}

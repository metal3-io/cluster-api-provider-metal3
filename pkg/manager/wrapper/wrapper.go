package wrapper

import (
	"fmt"
	"log"

	bmh "github.com/metal3-io/baremetal-operator/pkg/apis/metal3/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"
	admtypes "sigs.k8s.io/controller-runtime/pkg/webhook/admission/types"
)

// New returns a new manager wrapper. It intercepts the controller when it gets
// added and causes that controller to Watch BareMetalHost objects.
func New(mgr manager.Manager) manager.Manager {
	return &managerWrapper{
		manager: mgr,
	}
}

// managerWrapper is a wrapper around a "real manager". It intercepts the
// Controller when it gets added and causes that controller to Watch
// BareMetalHost objects.
type managerWrapper struct {
	manager manager.Manager
}

// Add causes the controller to Watch for BareMetalHost events, and then calls
// the wrapped manager's Add function.
func (m *managerWrapper) Add(r manager.Runnable) error {
	err := m.manager.Add(r)
	if err != nil {
		return err
	}

	c, ok := r.(controller.Controller)
	if !ok {
		return fmt.Errorf("Runnable was not a Controller")
	}

	if c == nil {
		return fmt.Errorf("Controller was nil")
	}

	err = c.Watch(&source.Kind{Type: &bmh.BareMetalHost{}}, &handler.EnqueueRequestsFromMapFunc{ToRequests: &mapper{}})
	if err != nil {
		log.Printf("Error watching BareMetalHosts: %s", err.Error())
		return err
	}
	return nil
}

// SetFields will set any dependencies on an object for which the object has implemented the inject
// interface - e.g. inject.Client.
func (m *managerWrapper) SetFields(i interface{}) error {
	return m.manager.SetFields(i)
}

// Start starts all registered Controllers and blocks until the Stop channel is closed.
// Returns an error if there is an error starting any controller.
func (m *managerWrapper) Start(c <-chan struct{}) error {
	return m.manager.Start(c)
}

// GetConfig returns an initialized Config
func (m *managerWrapper) GetConfig() *rest.Config {
	return m.manager.GetConfig()
}

// GetScheme returns and initialized Scheme
func (m *managerWrapper) GetScheme() *runtime.Scheme {
	return m.manager.GetScheme()
}

// GetAdmissionDecoder returns the runtime.Decoder based on the scheme.
func (m *managerWrapper) GetAdmissionDecoder() admtypes.Decoder {
	return m.manager.GetAdmissionDecoder()
}

// GetClient returns a client configured with the Config
func (m *managerWrapper) GetClient() client.Client {
	return m.manager.GetClient()
}

// GetFieldIndexer returns a client.FieldIndexer configured with the client
func (m *managerWrapper) GetFieldIndexer() client.FieldIndexer {
	return m.manager.GetFieldIndexer()
}

// GetCache returns a cache.Cache
func (m *managerWrapper) GetCache() cache.Cache {
	return m.manager.GetCache()
}

// GetRecorder returns a new EventRecorder for the provided name
func (m *managerWrapper) GetRecorder(name string) record.EventRecorder {
	return m.manager.GetRecorder(name)
}

// GetRESTMapper returns a RESTMapper
func (m *managerWrapper) GetRESTMapper() meta.RESTMapper {
	return m.manager.GetRESTMapper()
}

package wrapper

import (
	bmh "github.com/metal3-io/baremetal-operator/pkg/apis/metal3/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type mapper struct{}

// Map will return a reconcile request for a Machine if the event is for a
// BareMetalHost and that BareMetalHost references a Machine.
func (m *mapper) Map(obj handler.MapObject) []reconcile.Request {
	if host, ok := obj.Object.(*bmh.BareMetalHost); ok {
		if host.Spec.ConsumerRef != nil {
			return []reconcile.Request{
				reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      host.Spec.ConsumerRef.Name,
						Namespace: host.Spec.ConsumerRef.Namespace,
					},
				},
			}
		}
	}
	return []reconcile.Request{}
}

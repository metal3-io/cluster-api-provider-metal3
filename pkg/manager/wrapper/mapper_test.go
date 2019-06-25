package wrapper

import (
	"testing"

	bmh "github.com/metal3-io/baremetal-operator/pkg/apis/metal3/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

func TestMap(t *testing.T) {
	m := mapper{}

	for _, tc := range []struct {
		Host          *bmh.BareMetalHost
		ExpectRequest bool
	}{
		{
			Host: &bmh.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "host1",
					Namespace: "myns",
				},
				Spec: bmh.BareMetalHostSpec{
					ConsumerRef: &corev1.ObjectReference{
						Name:       "someothermachine",
						Namespace:  "myns",
						Kind:       "Machine",
						APIVersion: "v1alpha1",
					},
				},
			},
			ExpectRequest: true,
		},
		{
			Host: &bmh.BareMetalHost{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "host1",
					Namespace: "myns",
				},
				Spec: bmh.BareMetalHostSpec{},
			},
			ExpectRequest: false,
		},
	} {
		obj := handler.MapObject{
			Object: tc.Host,
		}
		reqs := m.Map(obj)

		if tc.ExpectRequest {
			if len(reqs) != 1 {
				t.Errorf("Expected 1 request, found %d", len(reqs))
			}
			req := reqs[0]
			if req.NamespacedName.Name != tc.Host.Spec.ConsumerRef.Name {
				t.Errorf("Expected name %s, found %s", tc.Host.Spec.ConsumerRef.Name, req.NamespacedName.Name)
			}
			if req.NamespacedName.Namespace != tc.Host.Spec.ConsumerRef.Namespace {
				t.Errorf("Expected namespace %s, found %s", tc.Host.Spec.ConsumerRef.Namespace, req.NamespacedName.Namespace)
			}

		} else {
			if len(reqs) != 0 {
				t.Errorf("Expected 0 request, found %d", len(reqs))
			}
		}
	}
}

/*
Copyright 2021 The Kubernetes Authors.
Copyright 2025 The Metal3 Authors.
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

package webhooks

import (
	"testing"

	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
)

var ctx = ctrl.SetupSignalHandler()

func TestMetal3ClusterDefault(t *testing.T) {
	g := NewWithT(t)
	webhook := &Metal3Cluster{}

	m3c := &infrav1.Metal3Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "fooboo",
		},
		Spec: infrav1.Metal3ClusterSpec{
			ControlPlaneEndpoint: infrav1.APIEndpoint{},
		},
	}

	g.Expect(webhook.Default(ctx, m3c)).To(Succeed())

	g.Expect(m3c.Spec.ControlPlaneEndpoint.Port).To(BeEquivalentTo(6443))
}

func TestMetal3ClusterValidation(t *testing.T) {
	valid := &infrav1.Metal3Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
		},
		Spec: infrav1.Metal3ClusterSpec{
			ControlPlaneEndpoint: infrav1.APIEndpoint{
				Host: "abc.com",
				Port: 443,
			},
		},
	}
	invalidHost := valid.DeepCopy()
	invalidHost.Spec.ControlPlaneEndpoint.Host = ""

	tests := []struct {
		name              string
		expectErrOnCreate bool
		expectErrOnUpdate bool
		newCluster        *infrav1.Metal3Cluster
		oldCluster        *infrav1.Metal3Cluster
	}{
		{
			name:              "should return error when endpoint empty",
			expectErrOnCreate: true,
			expectErrOnUpdate: true,
			newCluster:        invalidHost,
			oldCluster:        valid,
		},
		{
			name:              "should succeed when endpoint correct",
			expectErrOnCreate: false,
			expectErrOnUpdate: false,
			newCluster:        valid,
			oldCluster:        valid,
		},
		{
			name:              "should succeed when cloudProviderEnabled and noCloudProvider are not set",
			expectErrOnCreate: false,
			expectErrOnUpdate: false,
			newCluster:        valid,
			oldCluster:        valid,
		},
		{
			name:              "should succeed when cloudProviderEnabled is set and noCloudProvider not set",
			expectErrOnCreate: false,
			expectErrOnUpdate: false,
			newCluster: &infrav1.Metal3Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
				},
				Spec: infrav1.Metal3ClusterSpec{
					ControlPlaneEndpoint: infrav1.APIEndpoint{
						Host: "abc.com",
						Port: 443,
					},
					CloudProviderEnabled: ptr.To(true),
				},
			},
			oldCluster: valid,
		},
		{
			name:              "should succeed when noCloudProvider is set and cloudProviderEnabled not set",
			expectErrOnCreate: false,
			expectErrOnUpdate: false,
			newCluster: &infrav1.Metal3Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
				},
				Spec: infrav1.Metal3ClusterSpec{
					ControlPlaneEndpoint: infrav1.APIEndpoint{
						Host: "abc.com",
						Port: 443,
					},
					NoCloudProvider: ptr.To(true),
				},
			},
			oldCluster: valid,
		},
		{
			name:              "should succeed when cloudProviderEnabled and noCloudProvider do not conflict",
			expectErrOnCreate: false,
			expectErrOnUpdate: false,
			newCluster: &infrav1.Metal3Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
				},
				Spec: infrav1.Metal3ClusterSpec{
					ControlPlaneEndpoint: infrav1.APIEndpoint{
						Host: "abc.com",
						Port: 443,
					},
					CloudProviderEnabled: ptr.To(true),
				},
			},
			oldCluster: &infrav1.Metal3Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
				},
				Spec: infrav1.Metal3ClusterSpec{
					ControlPlaneEndpoint: infrav1.APIEndpoint{
						Host: "abc.com",
						Port: 443,
					},
					NoCloudProvider: ptr.To(false),
				},
			},
		},
		{
			name:              "should not succeed when cloudProviderEnabled and noCloudProvider do conflict on update",
			expectErrOnCreate: false,
			expectErrOnUpdate: true,
			newCluster: &infrav1.Metal3Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
				},
				Spec: infrav1.Metal3ClusterSpec{
					ControlPlaneEndpoint: infrav1.APIEndpoint{
						Host: "abc.com",
						Port: 443,
					},
					CloudProviderEnabled: ptr.To(false),
				},
			},
			oldCluster: &infrav1.Metal3Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
				},
				Spec: infrav1.Metal3ClusterSpec{
					ControlPlaneEndpoint: infrav1.APIEndpoint{
						Host: "abc.com",
						Port: 443,
					},
					NoCloudProvider: ptr.To(false),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			webhook := &Metal3Cluster{}

			if tt.expectErrOnCreate {
				_, err := webhook.ValidateCreate(ctx, tt.newCluster)
				g.Expect(err).To(HaveOccurred())
			} else {
				_, err := webhook.ValidateCreate(ctx, tt.newCluster)
				g.Expect(err).NotTo(HaveOccurred())
			}
			if tt.expectErrOnUpdate {
				_, err := webhook.ValidateUpdate(ctx, tt.oldCluster, tt.newCluster)
				g.Expect(err).To(HaveOccurred())
			} else {
				_, err := webhook.ValidateUpdate(ctx, tt.oldCluster, tt.newCluster)
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

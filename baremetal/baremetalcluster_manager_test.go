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

package baremetal

import (
	"context"
	"testing"

	_ "github.com/go-logr/logr"
	bmh "github.com/metal3-io/baremetal-operator/pkg/apis/metal3/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/klogr"
	infrav1 "sigs.k8s.io/cluster-api-provider-baremetal/api/v1alpha2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var bmcSpec = &infrav1.BareMetalClusterSpec{
	APIEndpoint: "http://192.168.111.249:6443",
}

var bmcSpecApiEmpty = &infrav1.BareMetalClusterSpec{
	APIEndpoint: "",
}

var bmcOwnerRef = &metav1.OwnerReference{
	APIVersion: clusterv1.GroupVersion.String(),
	Kind:       "Cluster",
	Name:       clusterName,
}

const (
	clusterName          = "testCluster"
	baremetalClusterName = "testBaremetalCluster"
	namespaceName        = "testNameSpace"
)

type tcTest struct {
	c             client.Client
	BMCluster     *infrav1.BareMetalCluster
	Cluster       *clusterv1.Cluster
	ExpectSuccess bool
}

var testCases = map[string]struct {
	c             client.Client
	BMCluster     *infrav1.BareMetalCluster
	Cluster       *clusterv1.Cluster
	ExpectSuccess bool
}{
	"Cluster and BMCluster exist": {
		Cluster:       newCluster(clusterName),
		BMCluster:     newBareMetalCluster(baremetalClusterName, bmcOwnerRef, bmcSpec, nil),
		ExpectSuccess: true,
	},
	"Cluster exists, BMCluster empty": {
		Cluster:       newCluster(clusterName),
		BMCluster:     &infrav1.BareMetalCluster{},
		ExpectSuccess: false,
	},
	"Cluster empty, BMCluster exists": {
		Cluster:       &clusterv1.Cluster{},
		BMCluster:     newBareMetalCluster(baremetalClusterName, bmcOwnerRef, bmcSpec, nil),
		ExpectSuccess: true,
	},
	"Cluster and BMCluster are nil": {
		Cluster:       nil,
		BMCluster:     nil,
		ExpectSuccess: false,
	},
	"Cluster exists and BMCluster is nil": {
		Cluster:       newCluster(clusterName),
		BMCluster:     nil,
		ExpectSuccess: false,
	},
	"Cluster and BMCluster exist, BMC spec API empty": {
		Cluster:       newCluster(clusterName),
		BMCluster:     newBareMetalCluster(baremetalClusterName, bmcOwnerRef, bmcSpecApiEmpty, nil),
		ExpectSuccess: false,
	},
}

func TestNewClusterManagerCreate(t *testing.T) {
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {

			clusterMgr, err := newSetup(t, tc)
			if clusterMgr == nil {
				if tc.ExpectSuccess {
					t.Error(err)
				}
			} else {
				if err != nil {
					t.Error(err)
					return
				}
				err = clusterMgr.Create(context.TODO())
				if err != nil {
					if tc.ExpectSuccess {
						t.Error(err)
					}
				} else {
					if !tc.ExpectSuccess {
						t.Error(err)
					}
				}
			}
		})
	}
}

func TestNewClusterManagerUpdateClusterStatus(t *testing.T) {
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {

			clusterMgr, err := newSetup(t, tc)
			if clusterMgr == nil {
				if tc.ExpectSuccess {
					t.Error(err)
				}
			} else {
				if err != nil {
					t.Error(err)
					return
				}
				err = clusterMgr.UpdateClusterStatus()
				if err != nil {
					if tc.ExpectSuccess {
						t.Error(err)
					}
				} else {
					apiEndPoints, err := clusterMgr.APIEndpoints()
					if err != nil {
						if tc.ExpectSuccess {
							t.Error(err)
						}
					} else {
						if !tc.ExpectSuccess {
							if apiEndPoints[0].Host != "" {
								t.Errorf("APIEndPoints Host not empty %s", apiEndPoints[0].Host)
							}
						} else {
							if apiEndPoints[0].Host != "192.168.111.249" || apiEndPoints[0].Port != 6443 {
								t.Errorf("APIEndPoints mismatch %s:%d", apiEndPoints[0].Host, apiEndPoints[0].Port)
							}
						}
					}
				}
			}
		})
	}
}

//-----------------------------------
//------ Helper functions -----------
//-----------------------------------
func setupScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	if err := clusterv1.AddToScheme(s); err != nil {
		panic(err)
	}
	if err := infrav1.AddToScheme(s); err != nil {
		panic(err)
	}
	if err := corev1.AddToScheme(s); err != nil {
		panic(err)
	}
	if err := bmh.SchemeBuilder.AddToScheme(s); err != nil {
		panic(err)
	}
	return s
}

func newSetup(t *testing.T, tc tcTest) (ClusterManagerInterface, error) {
	objects := []runtime.Object{
		tc.Cluster, tc.BMCluster,
	}

	if tc.Cluster == nil || tc.BMCluster == nil {
		tc.c = fakeclient.NewFakeClientWithScheme(setupScheme())
	} else {
		tc.c = fakeclient.NewFakeClientWithScheme(setupScheme(), objects...)
	}

	clusterMgr, err := NewClusterManager(tc.c, tc.Cluster, tc.BMCluster, klogr.New())
	if err != nil {
		if tc.ExpectSuccess {
			t.Error(err)
		}
	}
	return clusterMgr, err
}

func newCluster(clusterName string) *clusterv1.Cluster {
	return &clusterv1.Cluster{
		TypeMeta: metav1.TypeMeta{
			Kind: "Cluster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: namespaceName,
		},
		Spec: clusterv1.ClusterSpec{
			InfrastructureRef: &v1.ObjectReference{
				Name:       baremetalClusterName,
				Namespace:  namespaceName,
				Kind:       "InfrastructureConfig",
				APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha2",
			},
		},
		Status: clusterv1.ClusterStatus{
			InfrastructureReady: true,
		},
	}
}

func newBareMetalCluster(baremetalName string, ownerRef *metav1.OwnerReference,
	spec *infrav1.BareMetalClusterSpec, status *infrav1.BareMetalClusterStatus) *infrav1.BareMetalCluster {
	if spec == nil {
		spec = &infrav1.BareMetalClusterSpec{}
	}
	if status == nil {
		status = &infrav1.BareMetalClusterStatus{}
	}
	ownerRefs := []metav1.OwnerReference{}
	if ownerRef != nil {
		ownerRefs = []metav1.OwnerReference{*ownerRef}
	}

	return &infrav1.BareMetalCluster{
		TypeMeta: metav1.TypeMeta{
			Kind: "BareMetalCluster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            baremetalClusterName,
			Namespace:       namespaceName,
			OwnerReferences: ownerRefs,
		},
		Spec:   *spec,
		Status: *status,
	}
}

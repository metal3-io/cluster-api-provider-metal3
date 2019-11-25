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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/klogr"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-baremetal/api/v1alpha2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha2"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util"
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

type tcTest struct {
	BMCluster     *infrav1.BareMetalCluster
	Cluster       *clusterv1.Cluster
	ExpectSuccess bool
}

func TestNewClusterManager(t *testing.T) {
	testCases := map[string]tcTest{
		"defined": {
			Cluster:       &clusterv1.Cluster{},
			BMCluster:     &infrav1.BareMetalCluster{},
			ExpectSuccess: true,
		},
		"BMCluster undefined": {
			Cluster:       &clusterv1.Cluster{},
			BMCluster:     nil,
			ExpectSuccess: false,
		},
		"Cluster undefined": {
			Cluster:       nil,
			BMCluster:     &infrav1.BareMetalCluster{},
			ExpectSuccess: false,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			c := fakeclient.NewFakeClientWithScheme(setupScheme())
			_, err := NewClusterManager(c, tc.Cluster, tc.BMCluster,
				klogr.New(),
			)
			if err != nil {
				if tc.ExpectSuccess {
					t.Errorf("Unexpected error : %v", err)
				}
			} else {
				if !tc.ExpectSuccess {
					t.Error("Expected error")
				}
			}
		})
	}
}

func TestFinalizers(t *testing.T) {
	testCases := map[string]tcTest{
		"No finalizers": {
			Cluster: newCluster(clusterName),
			BMCluster: newBareMetalCluster(baremetalClusterName,
				bmcOwnerRef, nil, nil,
			),
		},
		"finalizers": {
			Cluster: newCluster(clusterName),
			BMCluster: &infrav1.BareMetalCluster{
				TypeMeta: metav1.TypeMeta{
					Kind: "BareMetalCluster",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            baremetalClusterName,
					Namespace:       namespaceName,
					OwnerReferences: []metav1.OwnerReference{*bmcOwnerRef},
					Finalizers:      []string{infrav1.ClusterFinalizer},
				},
				Spec:   infrav1.BareMetalClusterSpec{},
				Status: infrav1.BareMetalClusterStatus{},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			clusterMgr, err := newBMClusterSetup(t, tc)
			if err != nil {
				t.Error(err)
				return
			}

			clusterMgr.SetFinalizer()

			if !util.Contains(tc.BMCluster.ObjectMeta.Finalizers,
				infrav1.ClusterFinalizer,
			) {
				t.Errorf("Expected finalizer %v in %v", infrav1.ClusterFinalizer,
					tc.BMCluster.ObjectMeta.Finalizers,
				)
			}

			clusterMgr.UnsetFinalizer()

			if util.Contains(tc.BMCluster.ObjectMeta.Finalizers,
				infrav1.ClusterFinalizer,
			) {
				t.Errorf("Did not expect finalizer %v in %v", infrav1.ClusterFinalizer,
					tc.BMCluster.ObjectMeta.Finalizers,
				)
			}
		})
	}
}

func TestErrors(t *testing.T) {
	testCases := map[string]tcTest{
		"No errors": {
			Cluster: newCluster(clusterName),
			BMCluster: newBareMetalCluster(baremetalClusterName,
				bmcOwnerRef, nil, nil,
			),
		},
		"Error message": {
			Cluster: newCluster(clusterName),
			BMCluster: newBareMetalCluster(baremetalClusterName,
				bmcOwnerRef, nil, &infrav1.BareMetalClusterStatus{
					ErrorMessage: pointer.StringPtr("cba"),
				},
			),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			clusterMgr, err := newBMClusterSetup(t, tc)
			if err != nil {
				t.Error(err)
				return
			}

			clusterMgr.setError("abc", capierrors.InvalidConfigurationClusterError)

			if *tc.BMCluster.Status.ErrorReason != capierrors.InvalidConfigurationClusterError {
				t.Errorf("Expected error reason %v instead of %v",
					capierrors.InvalidConfigurationClusterError,
					tc.BMCluster.Status.ErrorReason,
				)
			}
			if *tc.BMCluster.Status.ErrorMessage != "abc" {
				t.Errorf("Expected error message abc instead of %v",
					tc.BMCluster.Status.ErrorMessage,
				)
			}

			clusterMgr.clearError()

			if tc.BMCluster.Status.ErrorReason != nil {
				t.Error("Did not expect an error reason")
			}
			if tc.BMCluster.Status.ErrorMessage != nil {
				t.Error("Did not expect an error message")
			}
		})
	}
}

func TestBMClusterDelete(t *testing.T) {
	var testCases = map[string]tcTest{
		"delete": {
			Cluster:   &clusterv1.Cluster{},
			BMCluster: &infrav1.BareMetalCluster{},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {

			clusterMgr, err := newBMClusterSetup(t, tc)
			if err != nil {
				if tc.ExpectSuccess {
					t.Error(err)
					return
				}
			}

			err = clusterMgr.Delete()
			if err != nil {
				if tc.ExpectSuccess {
					t.Error(err)
					return
				}
			}
		})
	}
}

var testCases = map[string]tcTest{
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
	"Cluster empty, BMCluster exists without owner": {
		Cluster:       &clusterv1.Cluster{},
		BMCluster:     newBareMetalCluster(baremetalClusterName, nil, bmcSpec, nil),
		ExpectSuccess: true,
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

			clusterMgr, err := newBMClusterSetup(t, tc)
			if err != nil {
				if tc.ExpectSuccess {
					t.Error(err)
				}
			} else {
				if clusterMgr == nil {
					return
				}
				err = clusterMgr.Create(context.TODO())
				if err != nil {
					if tc.ExpectSuccess {
						t.Error(err)
					}
				} else {
					if !tc.ExpectSuccess {
						t.Error("Expected an error")
					}
				}
			}
		})
	}
}

func TestNewClusterManagerUpdateClusterStatus(t *testing.T) {
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {

			clusterMgr, err := newBMClusterSetup(t, tc)
			if err != nil {
				if tc.ExpectSuccess {
					t.Error(err)
				}
			} else {
				if clusterMgr == nil {
					return
				}
				err = clusterMgr.UpdateClusterStatus()
				if err != nil {
					if tc.ExpectSuccess {
						t.Error(err)
					}
				} else {
					apiEndPoints := tc.BMCluster.Status.APIEndpoints
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
		})
	}
}

type descendantsTestCase struct {
	Machines            []*clusterv1.Machine
	ExpectError         bool
	ExpectedDescendants int
}

var descendantsTestCases = map[string]descendantsTestCase{
	"No Descendants": {
		Machines:            []*clusterv1.Machine{},
		ExpectError:         false,
		ExpectedDescendants: 0,
	},
	"One Descendant": {
		Machines: []*clusterv1.Machine{
			&clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespaceName,
					Labels: map[string]string{
						clusterv1.MachineClusterLabelName: clusterName,
					},
				},
			},
		},
		ExpectError:         false,
		ExpectedDescendants: 1,
	},
}

func TestListDescendants(t *testing.T) {

	for name, tc := range descendantsTestCases {
		t.Run(name, func(t *testing.T) {

			clusterMgr := descendantsSetup(tc)

			descendants, err := clusterMgr.listDescendants(context.TODO())
			if err != nil {
				if !tc.ExpectError {
					t.Error(err)
					return
				}
			} else {
				if tc.ExpectError {
					t.Error("Expected an error")
				}
			}
			if len(descendants.Items) != tc.ExpectedDescendants {
				t.Errorf("Expected %v descendants, got %v", tc.ExpectedDescendants,
					len(descendants.Items),
				)
			}
		})
	}
}

func TestCountDescendants(t *testing.T) {

	for name, tc := range descendantsTestCases {
		t.Run(name, func(t *testing.T) {

			clusterMgr := descendantsSetup(tc)
			nbDescendants, err := clusterMgr.CountDescendants(context.TODO())

			if err != nil {
				if !tc.ExpectError {
					t.Error(err)
					return
				}
			} else {
				if tc.ExpectError {
					t.Error("Expected an error")
				}
			}
			if nbDescendants != tc.ExpectedDescendants {
				t.Errorf("Expected %v descendants, got %v", tc.ExpectedDescendants,
					nbDescendants,
				)
			}
		})
	}
}

func newBMClusterSetup(t *testing.T, tc tcTest) (*ClusterManager, error) {
	objects := []runtime.Object{}

	if tc.Cluster != nil {
		objects = append(objects, tc.Cluster)
	}
	if tc.BMCluster != nil {
		objects = append(objects, tc.BMCluster)
	}
	c := fakeclient.NewFakeClientWithScheme(setupScheme(), objects...)

	return &ClusterManager{
		client:           c,
		BareMetalCluster: tc.BMCluster,
		Cluster:          tc.Cluster,
		Log:              klogr.New(),
	}, nil
}

func descendantsSetup(tc descendantsTestCase) *ClusterManager {
	cluster := newCluster(clusterName)
	bmCluster := newBareMetalCluster(baremetalClusterName, bmcOwnerRef,
		nil, nil,
	)
	objects := []runtime.Object{
		cluster,
		bmCluster,
	}
	for _, machine := range tc.Machines {
		objects = append(objects, machine)
	}
	c := fakeclient.NewFakeClientWithScheme(setupScheme(), objects...)

	return &ClusterManager{
		client:           c,
		BareMetalCluster: bmCluster,
		Cluster:          cluster,
		Log:              klogr.New(),
	}
}

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

package controllers

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	bmh "github.com/metal3-io/baremetal-operator/pkg/apis/metal3/v1alpha1"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/klogr"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-baremetal/api/v1alpha2"
	"sigs.k8s.io/cluster-api-provider-baremetal/baremetal"
	baremetal_mocks "sigs.k8s.io/cluster-api-provider-baremetal/baremetal/mocks"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

func setReconcileNormalExpectations(ctrl *gomock.Controller,
	tc reconcileNormalTestCase, r *BareMetalMachineReconciler,
) *baremetal_mocks.MockMachineManagerInterface {

	m := baremetal_mocks.NewMockMachineManagerInterface(ctrl)

	m.EXPECT().SetFinalizer()

	m.EXPECT().IsProvisioned().Return(tc.Provisioned)
	if tc.Provisioned {
		m.EXPECT().Update(context.TODO())
		m.EXPECT().IsBootstrapReady().MaxTimes(0)
		m.EXPECT().HasAnnotation().MaxTimes(0)
		m.EXPECT().GetBaremetalHostID(context.TODO()).MaxTimes(0)
		return m
	}

	m.EXPECT().IsBootstrapReady().Return(!tc.BootstrapNotReady)
	if tc.BootstrapNotReady {
		m.EXPECT().HasAnnotation().MaxTimes(0)
		m.EXPECT().GetBaremetalHostID(context.TODO()).MaxTimes(0)
		m.EXPECT().Update(context.TODO()).MaxTimes(0)
		return m
	}

	m.EXPECT().HasAnnotation().Return(tc.Annotated)
	if !tc.Annotated {
		if tc.AssociateFails {
			m.EXPECT().Associate(context.TODO()).Return(errors.New("Failed"))
			m.EXPECT().GetBaremetalHostID(context.TODO()).MaxTimes(0)
			m.EXPECT().Update(context.TODO()).MaxTimes(0)
			return m
		} else {
			m.EXPECT().Associate(context.TODO()).Return(nil)
		}
	}

	if tc.GetBMHIDFails {
		m.EXPECT().GetBaremetalHostID(context.TODO()).Return(nil,
			errors.New("Failed"),
		)
		m.EXPECT().Update(context.TODO()).MaxTimes(0)
		m.EXPECT().SetProviderID("abc").MaxTimes(0)
		return m
	}

	if tc.BMHIDSet {
		m.EXPECT().GetBaremetalHostID(context.TODO()).Return(
			pointer.StringPtr("abc"), nil,
		)

		if tc.SetNodeProviderIDFails {
			m.EXPECT().
				SetNodeProviderID("abc", "metal3://abc", nil).
				Return(errors.New("Failed"))
			m.EXPECT().SetProviderID("abc").MaxTimes(0)
			m.EXPECT().Update(context.TODO()).MaxTimes(0)
			return m
		}

		m.EXPECT().
			SetNodeProviderID("abc", "metal3://abc", nil).
			Return(nil)
		m.EXPECT().SetProviderID("metal3://abc")

	} else {
		m.EXPECT().GetBaremetalHostID(context.TODO()).Return(nil, nil)

		m.EXPECT().
			SetNodeProviderID("abc", "metal3://abc", nil).
			MaxTimes(0)
	}

	m.EXPECT().Update(context.TODO())
	return m
}

type reconcileNormalTestCase struct {
	ExpectError            bool
	ExpectRequeue          bool
	Provisioned            bool
	BootstrapNotReady      bool
	Annotated              bool
	AssociateFails         bool
	GetBMHIDFails          bool
	BMHIDSet               bool
	SetNodeProviderIDFails bool
}

func TestMachineReconcileNormal(t *testing.T) {
	testCases := map[string]reconcileNormalTestCase{
		"Provisioned": {
			ExpectError:   false,
			ExpectRequeue: false,
			Provisioned:   true,
		},
		"Bootstrap not ready": {
			ExpectError:       false,
			ExpectRequeue:     false,
			BootstrapNotReady: true,
		},
		"Not Annotated": {
			ExpectError:   false,
			ExpectRequeue: false,
			Annotated:     false,
		},
		"Not Annotated, Associate fails": {
			ExpectError:    true,
			ExpectRequeue:  false,
			Annotated:      false,
			AssociateFails: true,
		},
		"Annotated": {
			ExpectError:   false,
			ExpectRequeue: false,
			Annotated:     true,
		},
		"GetBMHID Fails": {
			ExpectError:   true,
			ExpectRequeue: false,
			GetBMHIDFails: true,
		},
		"BMH ID set": {
			ExpectError:   false,
			ExpectRequeue: false,
			BMHIDSet:      true,
		},
		"BMH ID set, SetNodeProviderID fails": {
			ExpectError:            true,
			ExpectRequeue:          false,
			BMHIDSet:               true,
			SetNodeProviderIDFails: true,
		},
	}
	for name, tc := range testCases {
		ctrl := gomock.NewController(t)

		defer ctrl.Finish()

		c := fake.NewFakeClientWithScheme(setupScheme())

		r := &BareMetalMachineReconciler{
			Client:           c,
			ManagerFactory:   baremetal.NewManagerFactory(c),
			Log:              klogr.New(),
			CapiClientGetter: nil,
		}

		m := setReconcileNormalExpectations(ctrl, tc, r)

		t.Run(name, func(t *testing.T) {
			res, err := r.reconcileNormal(context.TODO(), m)

			if tc.ExpectError {
				if err == nil {
					t.Error("Expected an error")
				}
			} else {
				if err != nil {
					t.Error("Did not expect an error")
				}
			}
			if tc.ExpectRequeue {
				if res.Requeue == false {
					t.Error("Expected a requeue")
				}
			} else {
				if res.Requeue != false {
					t.Error("Did not expect a requeue")
				}
			}
		})
	}
}

type reconcileDeleteTestCase struct {
	ExpectError   bool
	ExpectRequeue bool
	DeleteFails   bool
	DeleteRequeue bool
}

func setReconcileDeleteExpectations(ctrl *gomock.Controller,
	tc reconcileDeleteTestCase, r *BareMetalMachineReconciler,
) *baremetal_mocks.MockMachineManagerInterface {

	m := baremetal_mocks.NewMockMachineManagerInterface(ctrl)

	if tc.DeleteFails {
		m.EXPECT().Delete(context.TODO()).Return(errors.New("failed"))
		m.EXPECT().UnsetFinalizer().MaxTimes(0)
		return m
	} else if tc.DeleteRequeue {
		m.EXPECT().Delete(context.TODO()).Return(&baremetal.RequeueAfterError{})
		m.EXPECT().UnsetFinalizer().MaxTimes(0)
		return m
	} else {
		m.EXPECT().Delete(context.TODO()).Return(nil)
	}

	m.EXPECT().UnsetFinalizer()
	return m
}

func TestMachineReconcileDelete(t *testing.T) {
	testCases := map[string]reconcileDeleteTestCase{
		"Deletion success": {
			ExpectError:   false,
			ExpectRequeue: false,
		},
		"Deletion failure": {
			ExpectError:   true,
			ExpectRequeue: false,
			DeleteFails:   true,
		},
		"Deletion requeue": {
			ExpectError:   false,
			ExpectRequeue: true,
			DeleteRequeue: true,
		},
	}
	for name, tc := range testCases {
		ctrl := gomock.NewController(t)

		defer ctrl.Finish()

		c := fake.NewFakeClientWithScheme(setupScheme())

		r := &BareMetalMachineReconciler{
			Client:           c,
			ManagerFactory:   baremetal.NewManagerFactory(c),
			Log:              klogr.New(),
			CapiClientGetter: nil,
		}

		m := setReconcileDeleteExpectations(ctrl, tc, r)

		t.Run(name, func(t *testing.T) {
			res, err := r.reconcileDelete(context.TODO(), m)

			if tc.ExpectError {
				if err == nil {
					t.Error("Expected an error")
				}
			} else {
				if err != nil {
					t.Error("Did not expect an error")
				}
			}
			if tc.ExpectRequeue {
				if res.Requeue == false {
					t.Error("Expected a requeue")
				}
			} else {
				if res.Requeue != false {
					t.Error("Did not expect a requeue")
				}
			}
		})
	}
}

var _ = Describe("BareMetalMachine manager", func() {

	// Legacy tests
	It("TestBareMetalMachineReconciler_BareMetalClusterToBareMetalMachines", func() {
		baremetalCluster := newBareMetalCluster("my-baremetal-cluster",
			bmcOwnerRef, bmcSpec, nil,
		)
		objects := []runtime.Object{
			newCluster(clusterName, nil, nil),
			baremetalCluster,
			newMachine(clusterName, "my-machine-0", "my-baremetal-machine-0"),
			newMachine(clusterName, "my-machine-1", "my-baremetal-machine-1"),
			// Intentionally omitted
			newMachine(clusterName, "my-machine-2", ""),
		}
		c := fake.NewFakeClientWithScheme(setupScheme(), objects...)
		r := BareMetalMachineReconciler{
			Client: c,
			Log:    klogr.New(),
		}
		mo := handler.MapObject{
			Object: baremetalCluster,
		}
		out := r.BareMetalClusterToBareMetalMachines(mo)
		machineNames := make([]string, len(out))
		for i := range out {
			machineNames[i] = out[i].Name
		}
		Expect(len(out)).To(Equal(2), "Expected 2 baremetal machines to reconcile but got %d", len(out))

		for _, expectedName := range []string{"my-machine-0", "my-machine-1"} {
			Expect(contains(machineNames, expectedName)).To(BeTrue(), "expected %q in slice %v", expectedName, machineNames)
		}
	})

	It("TestBareMetalMachineReconciler_BareMetalClusterToBareMetalMachines_with_no_cluster", func() {
		baremetalCluster := newBareMetalCluster("my-baremetal-cluster",
			bmcOwnerRef, bmcSpec, nil,
		)
		objects := []runtime.Object{
			baremetalCluster,
			newMachine(clusterName, "my-machine-0", "my-baremetal-machine-0"),
			newMachine(clusterName, "my-machine-1", "my-baremetal-machine-1"),
			// Intentionally omitted
			newMachine(clusterName, "my-machine-2", ""),
		}
		c := fake.NewFakeClientWithScheme(setupScheme(), objects...)
		r := BareMetalMachineReconciler{
			Client: c,
			Log:    klogr.New(),
		}
		mo := handler.MapObject{
			Object: baremetalCluster,
		}
		out := r.BareMetalClusterToBareMetalMachines(mo)
		fmt.Printf("%v", out)
		Expect(len(out)).To(Equal(0), "Expected 0 request, found %d", len(out))
	})

	It("TestBareMetalMachineReconciler_BareMetalClusterToBareMetalMachines_with_no_bareMetalcluster", func() {
		objects := []runtime.Object{}
		c := fake.NewFakeClientWithScheme(setupScheme(), objects...)
		r := BareMetalMachineReconciler{
			Client: c,
			Log:    klogr.New(),
		}
		mo := handler.MapObject{}
		out := r.BareMetalClusterToBareMetalMachines(mo)
		fmt.Printf("%v", out)
		Expect(len(out)).To(Equal(0), "Expected 0 request, found %d", len(out))
	})

	type TestCaseBMHToBMM struct {
		Host          *bmh.BareMetalHost
		ExpectRequest bool
	}

	DescribeTable("BareMetalHost To BareMetalMachines tests",
		func(tc TestCaseBMHToBMM) {
			r := BareMetalMachineReconciler{}
			obj := handler.MapObject{
				Object: tc.Host,
			}
			reqs := r.BareMetalHostToBareMetalMachines(obj)

			if tc.ExpectRequest {
				Expect(len(reqs)).To(Equal(1), "Expected 1 request, found %d", len(reqs))

				req := reqs[0]
				Expect(req.NamespacedName.Name).To(Equal(tc.Host.Spec.ConsumerRef.Name),
					"Expected name %s, found %s", tc.Host.Spec.ConsumerRef.Name, req.NamespacedName.Name)
				Expect(req.NamespacedName.Namespace).To(Equal(tc.Host.Spec.ConsumerRef.Namespace),
					"Expected namespace %s, found %s", tc.Host.Spec.ConsumerRef.Namespace, req.NamespacedName.Namespace)

			} else {
				Expect(len(reqs)).To(Equal(0), "Expected 0 request, found %d", len(reqs))

			}
		},
		//Given machine, but no baremetalMachine resource
		Entry("BareMetalHost To BareMetalMachines",
			TestCaseBMHToBMM{
				Host: &bmh.BareMetalHost{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "host1",
						Namespace: "myns",
					},
					Spec: bmh.BareMetalHostSpec{
						ConsumerRef: &corev1.ObjectReference{
							Name:       "someothermachine",
							Namespace:  "myns",
							Kind:       "BareMetalMachine",
							APIVersion: infrav1.GroupVersion.String(),
						},
					},
				},
				ExpectRequest: true,
			},
		),
		//Given machine, but no baremetalMachine resource
		Entry("BareMetalHost To BareMetalMachines, no ConsumerRef",
			TestCaseBMHToBMM{
				Host: &bmh.BareMetalHost{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "host1",
						Namespace: "myns",
					},
					Spec: bmh.BareMetalHostSpec{},
				},
				ExpectRequest: false,
			},
		),
	)

})

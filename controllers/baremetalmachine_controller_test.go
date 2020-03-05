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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"github.com/golang/mock/gomock"
	bmh "github.com/metal3-io/baremetal-operator/pkg/apis/metal3/v1alpha1"
	infrav1 "github.com/metal3-io/cluster-api-provider-metal3/api/v1alpha3"
	"github.com/metal3-io/cluster-api-provider-metal3/baremetal"
	baremetal_mocks "github.com/metal3-io/cluster-api-provider-metal3/baremetal/mocks"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/klogr"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

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

func setReconcileNormalExpectations(ctrl *gomock.Controller,
	tc reconcileNormalTestCase,
) *baremetal_mocks.MockMachineManagerInterface {

	m := baremetal_mocks.NewMockMachineManagerInterface(ctrl)

	m.EXPECT().SetFinalizer()

	// provisioned, we should only call Update, nothing else
	m.EXPECT().IsProvisioned().Return(tc.Provisioned)
	if tc.Provisioned {
		m.EXPECT().Update(context.TODO())
		m.EXPECT().IsBootstrapReady().MaxTimes(0)
		m.EXPECT().HasAnnotation().MaxTimes(0)
		m.EXPECT().GetBaremetalHostID(context.TODO()).MaxTimes(0)
		return m
	}

	// Bootstrap data not ready, we'll requeue, not call anything else
	m.EXPECT().IsBootstrapReady().Return(!tc.BootstrapNotReady)
	if tc.BootstrapNotReady {
		m.EXPECT().HasAnnotation().MaxTimes(0)
		m.EXPECT().GetBaremetalHostID(context.TODO()).MaxTimes(0)
		m.EXPECT().Update(context.TODO()).MaxTimes(0)
		return m
	}

	// Bootstrap data is ready and node is not annotated, i.e. not associated
	m.EXPECT().HasAnnotation().Return(tc.Annotated)
	if !tc.Annotated {
		// if associate fails, we do not go further
		if tc.AssociateFails {
			m.EXPECT().Associate(context.TODO()).Return(errors.New("Failed"))
			m.EXPECT().GetBaremetalHostID(context.TODO()).MaxTimes(0)
			m.EXPECT().Update(context.TODO()).MaxTimes(0)
			return m
		} else {
			m.EXPECT().Associate(context.TODO()).Return(nil)
		}
	}

	// if node is now associated, if getting the ID fails, we do not go further
	if tc.GetBMHIDFails {
		m.EXPECT().GetBaremetalHostID(context.TODO()).Return(nil,
			errors.New("Failed"),
		)
		m.EXPECT().Update(context.TODO()).MaxTimes(0)
		m.EXPECT().SetProviderID("abc").MaxTimes(0)
		return m
	}

	// The ID is available (GetBaremetalHostID did not return nil)
	if tc.BMHIDSet {
		m.EXPECT().GetBaremetalHostID(context.TODO()).Return(
			pointer.StringPtr("abc"), nil,
		)

		// if we fail to set it on the node, we do not go further
		if tc.SetNodeProviderIDFails {
			m.EXPECT().
				SetNodeProviderID(context.TODO(), "abc", "metal3://abc", nil).
				Return(errors.New("Failed"))
			m.EXPECT().SetProviderID("abc").MaxTimes(0)
			m.EXPECT().Update(context.TODO()).MaxTimes(0)
			return m
		}

		// we successfully set it on the node
		m.EXPECT().
			SetNodeProviderID(context.TODO(), "abc", "metal3://abc", nil).
			Return(nil)
		m.EXPECT().SetProviderID("metal3://abc")

		// We did not get an id (got nil), so we'll requeue and not go further
	} else {
		m.EXPECT().GetBaremetalHostID(context.TODO()).Return(nil, nil)

		m.EXPECT().
			SetNodeProviderID(context.TODO(), "abc", "metal3://abc", nil).
			MaxTimes(0)
	}

	// last call
	m.EXPECT().Update(context.TODO())
	return m
}

type reconcileDeleteTestCase struct {
	ExpectError   bool
	ExpectRequeue bool
	DeleteFails   bool
	DeleteRequeue bool
}

func setReconcileDeleteExpectations(ctrl *gomock.Controller,
	tc reconcileDeleteTestCase,
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

var _ = Describe("BareMetalMachine manager", func() {

	Describe("Test MachineReconcileNormal", func() {

		var gomockCtrl *gomock.Controller
		var bmReconcile *BareMetalMachineReconciler

		BeforeEach(func() {
			gomockCtrl = gomock.NewController(GinkgoT())

			c := fake.NewFakeClientWithScheme(setupScheme())

			bmReconcile = &BareMetalMachineReconciler{
				Client:           c,
				ManagerFactory:   baremetal.NewManagerFactory(c),
				Log:              klogr.New(),
				CapiClientGetter: nil,
			}
		})

		AfterEach(func() {
			gomockCtrl.Finish()
		})

		DescribeTable("Deletion tests",
			func(tc reconcileNormalTestCase) {
				m := setReconcileNormalExpectations(gomockCtrl, tc)
				res, err := bmReconcile.reconcileNormal(context.TODO(), m)

				if tc.ExpectError {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).NotTo(HaveOccurred())
				}
				if tc.ExpectRequeue {
					Expect(res.Requeue).To(BeTrue())
				} else {
					Expect(res.Requeue).To(BeFalse())
				}
			},
			Entry("Provisioned", reconcileNormalTestCase{
				ExpectError:   false,
				ExpectRequeue: false,
				Provisioned:   true,
			}),
			Entry("Bootstrap not ready", reconcileNormalTestCase{
				ExpectError:       false,
				ExpectRequeue:     false,
				BootstrapNotReady: true,
			}),
			Entry("Not Annotated", reconcileNormalTestCase{
				ExpectError:   false,
				ExpectRequeue: false,
				Annotated:     false,
			}),
			Entry("Not Annotated, Associate fails", reconcileNormalTestCase{
				ExpectError:    true,
				ExpectRequeue:  false,
				Annotated:      false,
				AssociateFails: true,
			}),
			Entry("Annotated", reconcileNormalTestCase{
				ExpectError:   false,
				ExpectRequeue: false,
				Annotated:     true,
			}),
			Entry("GetBMHID Fails", reconcileNormalTestCase{
				ExpectError:   true,
				ExpectRequeue: false,
				GetBMHIDFails: true,
			}),
			Entry("BMH ID set", reconcileNormalTestCase{
				ExpectError:   false,
				ExpectRequeue: false,
				BMHIDSet:      true,
			}),
			Entry("BMH ID set, SetNodeProviderID fails", reconcileNormalTestCase{
				ExpectError:            true,
				ExpectRequeue:          false,
				BMHIDSet:               true,
				SetNodeProviderIDFails: true,
			}),
		)
	})

	Describe("Test MachineReconcileDelete", func() {

		var gomockCtrl *gomock.Controller
		var bmReconcile *BareMetalMachineReconciler

		BeforeEach(func() {
			gomockCtrl = gomock.NewController(GinkgoT())

			c := fake.NewFakeClientWithScheme(setupScheme())

			bmReconcile = &BareMetalMachineReconciler{
				Client:           c,
				ManagerFactory:   baremetal.NewManagerFactory(c),
				Log:              klogr.New(),
				CapiClientGetter: nil,
			}
		})

		AfterEach(func() {
			gomockCtrl.Finish()
		})

		DescribeTable("Deletion tests",
			func(tc reconcileDeleteTestCase) {
				m := setReconcileDeleteExpectations(gomockCtrl, tc)
				res, err := bmReconcile.reconcileDelete(context.TODO(), m)

				if tc.ExpectError {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).NotTo(HaveOccurred())
				}
				if tc.ExpectRequeue {
					Expect(res.Requeue).To(BeTrue())
				} else {
					Expect(res.Requeue).To(BeFalse())
				}
			},
			Entry("Deletion success", reconcileDeleteTestCase{
				ExpectError:   false,
				ExpectRequeue: false,
			}),
			Entry("Deletion failure", reconcileDeleteTestCase{
				ExpectError:   true,
				ExpectRequeue: false,
				DeleteFails:   true,
			}),
			Entry("Deletion requeue", reconcileDeleteTestCase{
				ExpectError:   false,
				ExpectRequeue: true,
				DeleteRequeue: true,
			}),
		)
	})

	// Legacy tests
	It("TestBareMetalMachineReconciler_BareMetalClusterToBareMetalMachines", func() {
		baremetalCluster := newBareMetalCluster("my-baremetal-cluster",
			bmcOwnerRef(), bmcSpec(), nil, false,
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
			bmcOwnerRef(), bmcSpec(), nil, false,
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

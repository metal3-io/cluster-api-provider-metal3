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
	"fmt"

	bmh "github.com/metal3-io/baremetal-operator/pkg/apis/metal3/v1alpha1"
	corev1 "k8s.io/api/core/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/klogr"
	infrav1 "sigs.k8s.io/cluster-api-provider-baremetal/api/v1alpha2"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

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

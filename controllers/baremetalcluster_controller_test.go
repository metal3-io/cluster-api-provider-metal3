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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/cluster-api-provider-baremetal/baremetal"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/klogr"
	infrav1 "sigs.k8s.io/cluster-api-provider-baremetal/api/v1alpha2"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Reconcile Baremetalcluster", func() {

	type TestCaseReconcileBMC struct {
		Objects        []runtime.Object
		ErrorExpected  bool
		RequeeExpected bool
	}

	DescribeTable("Reconcile tests BaremetalCluster",
		func(tc TestCaseReconcileBMC) {
			c := fake.NewFakeClientWithScheme(setupScheme(), tc.Objects...)

			r := &BareMetalClusterReconciler{
				Client:         c,
				ManagerFactory: baremetal.NewManagerFactory(c),
				Log:            klogr.New(),
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      baremetalClusterName,
					Namespace: namespaceName,
				},
			}

			res, err := r.Reconcile(req)
			if tc.ErrorExpected {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			if tc.RequeeExpected {
				Expect(res.Requeue).NotTo(BeFalse())
			} else {
				Expect(res.Requeue).To(BeFalse())
			}
		},
		// Given cluster, but no baremetalcluster resource
		Entry("Should not return an error when baremetalcluster is not found",
			TestCaseReconcileBMC{
				Objects: []runtime.Object{
					newCluster(clusterName),
				},
				ErrorExpected:  false,
				RequeeExpected: false,
			},
		),
		// Given no cluster resource
		Entry("Should return en error when cluster is not found",
			TestCaseReconcileBMC{
				Objects: []runtime.Object{
					newBareMetalCluster(baremetalClusterName, bmcOwnerRef, bmcSpec, nil),
				},
				ErrorExpected:  true,
				RequeeExpected: false,
			},
		),
		// Given cluster and baremetalcluster with no owner reference
		Entry("Should not return an error if OwnerRef is not set on BareMetalCluster",
			TestCaseReconcileBMC{
				Objects: []runtime.Object{
					newBareMetalCluster(baremetalClusterName, nil, nil, nil),
					newCluster(clusterName),
				},
				ErrorExpected:  false,
				RequeeExpected: false,
			},
		),
		// Given cluster and BareMetalCluster with no APIEndpoint
		Entry("Should return an error if APIEndpoint is not set",
			TestCaseReconcileBMC{
				Objects: []runtime.Object{
					newBareMetalCluster(baremetalClusterName, bmcOwnerRef, nil, nil),
					newCluster(clusterName),
				},
				ErrorExpected:  true,
				RequeeExpected: false,
			},
		),
		// Given cluster and BareMetalCluster with mandatory fields
		Entry("Should not return an error when mandatory fields are provided",
			TestCaseReconcileBMC{
				Objects: []runtime.Object{
					newBareMetalCluster(baremetalClusterName, bmcOwnerRef, bmcSpec, nil),
					newCluster(clusterName),
				},
				ErrorExpected:  false,
				RequeeExpected: false,
			},
		),
		// Reconcile Deletion
		Entry("Should reconcileDelete when deletion timestamp is set.",
			TestCaseReconcileBMC{
				Objects: []runtime.Object{
					&infrav1.BareMetalCluster{
						TypeMeta: metav1.TypeMeta{
							Kind: "BareMetalCluster",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:              baremetalClusterName,
							Namespace:         namespaceName,
							DeletionTimestamp: &deletionTimestamp,
							OwnerReferences:   []metav1.OwnerReference{*bmcOwnerRef},
						},
						Spec: *bmcSpec,
					},
					newCluster(clusterName),
				},
				ErrorExpected:  false,
				RequeeExpected: false,
			},
		),
	)

})

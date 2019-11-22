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
	"reflect"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	capierrors "sigs.k8s.io/cluster-api/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/klogr"
	infrav1 "sigs.k8s.io/cluster-api-provider-baremetal/api/v1alpha2"
	fakebm "sigs.k8s.io/cluster-api-provider-baremetal/baremetal/fake"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Reconcile Baremetalcluster", func() {

	type TestCaseReconcileBMC struct {
		Objects             []runtime.Object
		ErrorType           error
		ErrorExpected       bool
		RequeueExpected     bool
		ErrorReasonExpected bool
		ErrorReason         capierrors.ClusterStatusError
	}

	DescribeTable("Reconcile tests BaremetalCluster",
		func(tc TestCaseReconcileBMC) {
			testclstr := &infrav1.BareMetalCluster{}
			c := fake.NewFakeClientWithScheme(setupScheme(), tc.Objects...)

			r := &BareMetalClusterReconciler{
				Client:         c,
				ManagerFactory: fakebm.NewFakeManagerFactory(c),
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
				if tc.ErrorType != nil {
					Expect(reflect.TypeOf(tc.ErrorType)).To(Equal(reflect.TypeOf(errors.Cause(err))))
				}

			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			if tc.RequeueExpected {
				Expect(res.Requeue).NotTo(BeFalse())
				Expect(res.RequeueAfter).To(Equal(requeueAfter))
			} else {
				Expect(res.Requeue).To(BeFalse())
			}
			if tc.ErrorReasonExpected {
				_ = c.Get(context.TODO(), *getKey(baremetalClusterName), testclstr)
				Expect(testclstr.Status.ErrorReason).NotTo(BeNil())
				Expect(tc.ErrorReason).To(Equal(*testclstr.Status.ErrorReason))
			}
		},
		// Given cluster, but no baremetalcluster resource
		Entry("Should not return an error when baremetalcluster is not found",
			TestCaseReconcileBMC{
				Objects: []runtime.Object{
					newCluster(clusterName, nil, nil),
				},
				ErrorExpected:   false,
				RequeueExpected: false,
			},
		),
		// Given no cluster resource
		Entry("Should return en error when cluster is not found",
			TestCaseReconcileBMC{
				Objects: []runtime.Object{
					newBareMetalCluster(baremetalClusterName, bmcOwnerRef, bmcSpec, nil),
				},
				ErrorExpected:       true,
				ErrorReasonExpected: true,
				ErrorReason:         capierrors.InvalidConfigurationClusterError,
				RequeueExpected:     false,
			},
		),
		// Given cluster and baremetalcluster with no owner reference
		Entry("Should not return an error if OwnerRef is not set on BareMetalCluster",
			TestCaseReconcileBMC{
				Objects: []runtime.Object{
					newBareMetalCluster(baremetalClusterName, nil, nil, nil),
					newCluster(clusterName, nil, nil),
				},
				ErrorExpected:   false,
				RequeueExpected: false,
			},
		),
		// Given cluster and BareMetalCluster with no APIEndpoint
		Entry("Should return an error if APIEndpoint is not set",
			TestCaseReconcileBMC{
				Objects: []runtime.Object{
					newBareMetalCluster(baremetalClusterName, bmcOwnerRef, nil, nil),
					newCluster(clusterName, nil, nil),
				},
				ErrorExpected:       true,
				ErrorType:           &infrav1.APIEndPointError{},
				RequeueExpected:     false,
				ErrorReasonExpected: true,
				ErrorReason:         capierrors.InvalidConfigurationClusterError,
			},
		),
		// Given cluster and BareMetalCluster with mandatory fields
		Entry("Should not return an error when mandatory fields are provided",
			TestCaseReconcileBMC{
				Objects: []runtime.Object{
					newBareMetalCluster(baremetalClusterName, bmcOwnerRef, bmcSpec, nil),
					newCluster(clusterName, nil, nil),
				},
				ErrorExpected:   false,
				RequeueExpected: false,
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
					newCluster(clusterName, nil, nil),
				},
				ErrorExpected:   false,
				RequeueExpected: false,
			},
		),
		// Reconcile Deletion, wait for baremetalmachine
		Entry("reconcileDelete should wait for baremetalmachine",
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
					newCluster(clusterName, nil, nil),
					newMachine(clusterName, machineName, ""),
				},
				ErrorExpected:   false,
				RequeueExpected: true,
			},
		),
	)

})

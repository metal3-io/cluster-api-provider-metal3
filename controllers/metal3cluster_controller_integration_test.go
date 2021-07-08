/*
Copyright 2021 The Kubernetes Authors.
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

	infrav1alpha5 "github.com/metal3-io/cluster-api-provider-metal3/api/v1alpha5"
	"github.com/metal3-io/cluster-api-provider-metal3/baremetal"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/klogr"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Reconcile metal3Cluster", func() {

	type TestCaseReconcileBMC struct {
		Objects             []runtime.Object
		ErrorType           error
		ErrorExpected       bool
		RequeueExpected     bool
		ErrorReasonExpected bool
		ErrorReason         capierrors.ClusterStatusError
	}

	DescribeTable("Reconcile tests metal3Cluster",
		func(tc TestCaseReconcileBMC) {
			testclstr := &infrav1alpha5.Metal3Cluster{}
			c := fake.NewFakeClientWithScheme(setupScheme(), tc.Objects...)

			r := &Metal3ClusterReconciler{
				Client:         c,
				ManagerFactory: baremetal.NewManagerFactory(c),
				Log:            klogr.New(),
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      metal3ClusterName,
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
				_ = c.Get(context.TODO(), *getKey(metal3ClusterName), testclstr)
				Expect(testclstr.Status.FailureReason).NotTo(BeNil())
				Expect(tc.ErrorReason).To(Equal(*testclstr.Status.FailureReason))
			}
		},
		// Given cluster, but no metal3cluster resource
		Entry("Should not return an error when metal3cluster is not found",
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
					newMetal3Cluster(metal3ClusterName, bmcOwnerRef(), bmcSpec(), nil, false),
				},
				ErrorExpected:       true,
				ErrorReasonExpected: true,
				ErrorReason:         capierrors.InvalidConfigurationClusterError,
				RequeueExpected:     false,
			},
		),
		// Given cluster and metal3cluster with no owner reference
		Entry("Should not return an error if OwnerRef is not set on Metal3Cluster",
			TestCaseReconcileBMC{
				Objects: []runtime.Object{
					newMetal3Cluster(metal3ClusterName, nil, nil, nil, false),
					newCluster(clusterName, nil, nil),
				},
				ErrorExpected:   false,
				RequeueExpected: false,
			},
		),
		// Given cluster and Metal3Cluster with no APIEndpoint
		Entry("Should return an error if APIEndpoint is not set",
			TestCaseReconcileBMC{
				Objects: []runtime.Object{
					newMetal3Cluster(metal3ClusterName, bmcOwnerRef(), nil, nil, false),
					newCluster(clusterName, nil, nil),
				},
				ErrorExpected: true,
				//ErrorType:           &infrav1alpha5.APIEndPointError{},
				RequeueExpected:     false,
				ErrorReasonExpected: true,
				ErrorReason:         capierrors.InvalidConfigurationClusterError,
			},
		),

		// Given cluster and Metal3Cluster with mandatory fields
		Entry("Should not return an error when mandatory fields are provided",
			TestCaseReconcileBMC{
				Objects: []runtime.Object{
					newMetal3Cluster(metal3ClusterName, bmcOwnerRef(), bmcSpec(), nil, false),
					newCluster(clusterName, nil, nil),
				},
				ErrorExpected:   false,
				RequeueExpected: false,
			},
		),

		//Given: Cluster, Metal3Cluster.
		//Cluster.Spec.Paused=true
		//Expected: Requeue Expected
		Entry("Should requeue when owner Cluster is paused",
			TestCaseReconcileBMC{
				Objects: []runtime.Object{
					newCluster(clusterName, clusterPauseSpec(), nil),
					newMetal3Cluster(metal3ClusterName, bmcOwnerRef(), bmcSpec(), nil, false),
				},
				ErrorExpected:   false,
				RequeueExpected: true,
			},
		),

		//Given: Cluster, Metal3Cluster.
		//Metal3Cluster has cluster.x-k8s.io/paused annotation
		//Expected: Requeue Expected
		Entry("Should requeue when Metal3Cluster has paused annotation",
			TestCaseReconcileBMC{
				Objects: []runtime.Object{
					newCluster(clusterName, nil, nil),
					newMetal3Cluster(metal3ClusterName, nil, nil, nil, true),
				},
				ErrorExpected:   false,
				RequeueExpected: true,
			},
		),
		// Reconcile Deletion
		Entry("Should reconcileDelete when deletion timestamp is set.",
			TestCaseReconcileBMC{
				Objects: []runtime.Object{
					&infrav1alpha5.Metal3Cluster{
						TypeMeta: metav1.TypeMeta{
							Kind: "Metal3Cluster",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:              metal3ClusterName,
							Namespace:         namespaceName,
							DeletionTimestamp: &deletionTimestamp,
							OwnerReferences:   []metav1.OwnerReference{*bmcOwnerRef()},
						},
						Spec: *bmcSpec(),
					},
					newCluster(clusterName, nil, nil),
				},
				ErrorExpected:   false,
				RequeueExpected: false,
			},
		),
		// Reconcile Deletion, wait for metal3machine
		Entry("reconcileDelete should wait for metal3machine",
			TestCaseReconcileBMC{
				Objects: []runtime.Object{
					&infrav1alpha5.Metal3Cluster{
						TypeMeta: metav1.TypeMeta{
							Kind: "Metal3Cluster",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:              metal3ClusterName,
							Namespace:         namespaceName,
							DeletionTimestamp: &deletionTimestamp,
							OwnerReferences:   []metav1.OwnerReference{*bmcOwnerRef()},
						},
						Spec: *bmcSpec(),
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

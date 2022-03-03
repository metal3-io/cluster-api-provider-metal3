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

	"github.com/go-logr/logr"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util/conditions"

	capm3 "github.com/metal3-io/cluster-api-provider-metal3/api/v1beta1"
	"github.com/metal3-io/cluster-api-provider-metal3/baremetal"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Reconcile metal3Cluster", func() {

	type TestCaseReconcileBMC struct {
		Objects             []client.Object
		ErrorType           error
		ErrorExpected       bool
		RequeueExpected     bool
		ErrorReasonExpected bool
		ErrorReason         capierrors.ClusterStatusError
		ConditionsExpected  clusterv1.Conditions
	}

	DescribeTable("Reconcile tests metal3Cluster",
		func(tc TestCaseReconcileBMC) {
			testclstr := &capm3.Metal3Cluster{}
			fakeClient := fake.NewClientBuilder().WithScheme(setupScheme()).WithObjects(tc.Objects...).Build()

			r := &Metal3ClusterReconciler{
				Client:           fakeClient,
				ManagerFactory:   baremetal.NewManagerFactory(fakeClient),
				Log:              logr.Discard(),
				WatchFilterValue: "",
			}

			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      metal3ClusterName,
					Namespace: namespaceName,
				},
			}
			ctx := context.Background()
			res, err := r.Reconcile(ctx, req)
			_ = fakeClient.Get(ctx, *getKey(metal3ClusterName), testclstr)

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
				Expect(testclstr.Status.FailureReason).NotTo(BeNil())
				Expect(tc.ErrorReason).To(Equal(*testclstr.Status.FailureReason))
			}
			for _, condExp := range tc.ConditionsExpected {
				condGot := conditions.Get(testclstr, condExp.Type)
				Expect(condGot).NotTo(BeNil())
				Expect(condGot.Status).To(Equal(condExp.Status))
				if condExp.Reason != "" {
					Expect(condGot.Reason).To(Equal(condExp.Reason))
				}
			}
		},
		// Given cluster, but no metal3cluster resource
		Entry("Should not return an error when metal3cluster is not found",
			TestCaseReconcileBMC{
				Objects: []client.Object{
					newCluster(clusterName, nil, nil),
				},
				ErrorExpected:   false,
				RequeueExpected: false,
			},
		),
		// Given no cluster resource
		Entry("Should return en error when cluster is not found",
			TestCaseReconcileBMC{
				Objects: []client.Object{
					newMetal3Cluster(metal3ClusterName, bmcOwnerRef(), bmcSpec(), nil, nil, false),
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
				Objects: []client.Object{
					newMetal3Cluster(metal3ClusterName, nil, nil, nil, nil, false),
					newCluster(clusterName, nil, nil),
				},
				ErrorExpected:   false,
				RequeueExpected: false,
			},
		),
		// Given cluster and Metal3Cluster with no APIEndpoint
		Entry("Should return an error if APIEndpoint is not set",
			TestCaseReconcileBMC{
				Objects: []client.Object{
					newMetal3Cluster(metal3ClusterName, bmcOwnerRef(), nil, nil, nil, false),
					newCluster(clusterName, nil, nil),
				},
				ErrorExpected: true,
				//ErrorType:           &capm3.APIEndPointError{},
				RequeueExpected:     false,
				ErrorReasonExpected: true,
				ErrorReason:         capierrors.InvalidConfigurationClusterError,
			},
		),

		// Given cluster and Metal3Cluster with mandatory fields
		Entry("Should not return an error when mandatory fields are provided",
			TestCaseReconcileBMC{
				Objects: []client.Object{
					newMetal3Cluster(metal3ClusterName, bmcOwnerRef(), bmcSpec(), nil, nil, false),
					newCluster(clusterName, nil, nil),
				},
				ErrorExpected:   false,
				RequeueExpected: false,
				ConditionsExpected: clusterv1.Conditions{
					clusterv1.Condition{
						Type:   capm3.BaremetalInfrastructureReadyCondition,
						Status: corev1.ConditionTrue,
					},
					clusterv1.Condition{
						Type:   clusterv1.ReadyCondition,
						Status: corev1.ConditionTrue,
					},
				},
			},
		),

		// Given: Cluster, Metal3Cluster.
		// Cluster.Spec.Paused=true
		// Expected: Requeue Expected
		Entry("Should requeue when owner Cluster is paused",
			TestCaseReconcileBMC{
				Objects: []client.Object{
					newCluster(clusterName, clusterPauseSpec(), nil),
					newMetal3Cluster(metal3ClusterName, bmcOwnerRef(), bmcSpec(), nil, nil, false),
				},
				ErrorExpected:   false,
				RequeueExpected: true,
			},
		),

		//Given: Cluster, Metal3Cluster.
		// Metal3Cluster has cluster.x-k8s.io/paused annotation
		//Expected: Requeue Expected
		Entry("Should requeue when Metal3Cluster has paused annotation",
			TestCaseReconcileBMC{
				Objects: []client.Object{
					newCluster(clusterName, nil, nil),
					newMetal3Cluster(metal3ClusterName, nil, nil, nil, nil, true),
				},
				ErrorExpected:   false,
				RequeueExpected: true,
			},
		),
		// Reconcile Deletion
		Entry("Should reconcileDelete when deletion timestamp is set.",
			TestCaseReconcileBMC{
				Objects: []client.Object{
					&capm3.Metal3Cluster{
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
				Objects: []client.Object{
					&capm3.Metal3Cluster{
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
					newMachine(clusterName, machineName, "", ""),
				},
				ErrorExpected:   false,
				RequeueExpected: true,
			},
		),
	)

})

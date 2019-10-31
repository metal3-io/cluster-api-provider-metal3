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
	. "github.com/onsi/gomega"
	"sigs.k8s.io/cluster-api-provider-baremetal/baremetal"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/klogr"
	infrav1 "sigs.k8s.io/cluster-api-provider-baremetal/api/v1alpha2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha2"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Reconcile Baremetalcluster", func() {

	testCluster := &clusterv1.Cluster{

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
	}

	BeforeEach(func() {

	})
	// Given cluster, but no baremetalcluster resource
	It("Should not return an error when baremetalcluster is not found", func() {

		c := fake.NewFakeClientWithScheme(setupScheme(), testCluster)

		r := &BareMetalClusterReconciler{
			Client: c,
			Log:    klogr.New(),
		}

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      baremetalClusterName,
				Namespace: namespaceName,
			},
		}

		res, err := r.Reconcile(req)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())

	})

	// Given no cluster resource
	It("Should return en error when cluster is not found ", func() {
		baremetalCluster := &infrav1.BareMetalCluster{
			TypeMeta: metav1.TypeMeta{
				Kind: "BareMetalCluster",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      baremetalClusterName,
				Namespace: namespaceName,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       clusterName,
					},
				},
			},
			Spec: infrav1.BareMetalClusterSpec{
				APIEndpoint: "http://192.168.111.249:6443",
			},
		}
		c := fake.NewFakeClientWithScheme(setupScheme(), baremetalCluster)

		r := &BareMetalClusterReconciler{
			Client: c,
			Log:    klogr.New(),
		}

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      baremetalClusterName,
				Namespace: namespaceName,
			},
		}

		res, err := r.Reconcile(req)
		Expect(err).To(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())

	})

	// Given cluster and baremetalcluster with no owner reference
	It("Should not return an error if OwnerRef is not set on BareMetalCluster", func() {
		baremetalCluster := &infrav1.BareMetalCluster{
			TypeMeta: metav1.TypeMeta{
				Kind: "BareMetalCluster",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      baremetalClusterName,
				Namespace: namespaceName,
			},
		}

		c := fake.NewFakeClientWithScheme(setupScheme(), testCluster, baremetalCluster)

		r := &BareMetalClusterReconciler{
			Client: c,
			Log:    klogr.New(),
		}

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      baremetalClusterName,
				Namespace: namespaceName,
			},
		}

		res, err := r.Reconcile(req)
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())

	})

	// Given cluster and BareMetalCluster with no APIEndpoint
	It("Should return an error if APIEndpoint is not set", func() {
		baremetalCluster := &infrav1.BareMetalCluster{
			TypeMeta: metav1.TypeMeta{
				Kind: "BareMetalCluster",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      baremetalClusterName,
				Namespace: namespaceName,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       clusterName,
					},
				},
			},
		}

		c := fake.NewFakeClientWithScheme(setupScheme(), testCluster, baremetalCluster)

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
		Expect(err).To(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())

	})
	// Given cluster and BareMetalCluster with mandatory fields
	It("Should not return an error when mandatory fields are provided", func() {
		baremetalCluster := &infrav1.BareMetalCluster{
			TypeMeta: metav1.TypeMeta{
				Kind: "BareMetalCluster",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      baremetalClusterName,
				Namespace: namespaceName,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       clusterName,
					},
				},
			},
			Spec: infrav1.BareMetalClusterSpec{
				APIEndpoint: "http://192.168.111.249:6443",
			},
		}
		c := fake.NewFakeClientWithScheme(setupScheme(), testCluster, baremetalCluster)

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
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())

	})

	// Reconcile Deletion
	It("Should reconcileDelete when deletion timestamp is set.", func() {
		baremetalCluster := &infrav1.BareMetalCluster{
			TypeMeta: metav1.TypeMeta{
				Kind: "BareMetalCluster",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      baremetalClusterName,
				Namespace: namespaceName,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       clusterName,
					},
				},
			},
			Spec: infrav1.BareMetalClusterSpec{
				APIEndpoint: "http://192.168.111.249:6443",
			},
		}
		deletionTimestamp := metav1.Now()
		baremetalCluster.SetDeletionTimestamp(&deletionTimestamp)
		c := fake.NewFakeClientWithScheme(setupScheme(), testCluster, baremetalCluster)

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
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Requeue).To(BeFalse())

	})

})

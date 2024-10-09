package main

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	inmemoryclient "sigs.k8s.io/cluster-api/test/infrastructure/inmemory/pkg/runtime/client"
)

type etcdInfo struct {
	clusterID string
	leaderID  string
	members   sets.Set[string]
}

type ResourceData struct {
	ResourceName string
	Host         string
	Port         int
}

func int32Ptr(i int32) *int32 {
	return &i
}

const (
	// EtcdClusterIDAnnotationName defines the name of the annotation applied to in memory etcd
	// pods to track the cluster ID of the etcd member each pod represent.
	EtcdClusterIDAnnotationName = "etcd.inmemory.infrastructure.cluster.x-k8s.io/cluster-id"

	// EtcdMemberIDAnnotationName defines the name of the annotation applied to in memory etcd
	// pods to track the member ID of the etcd member each pod represent.
	EtcdMemberIDAnnotationName = "etcd.inmemory.infrastructure.cluster.x-k8s.io/member-id"

	// EtcdLeaderFromAnnotationName defines the name of the annotation applied to in memory etcd
	// pods to track leadership status of the etcd member each pod represent.
	// Note: We are tracking the time from an etcd member is leader; if more than one pod has this
	// annotation, the last etcd member that became leader is the current leader.
	// By using this mechanism leadership can be forwarded to another pod with an atomic operation
	// (add/update of the annotation to the pod/etcd member we are forwarding leadership to).
	EtcdLeaderFromAnnotationName = "etcd.inmemory.infrastructure.cluster.x-k8s.io/leader-from"

	// EtcdMemberRemoved is added to etcd pods which have been removed from the etcd cluster.
	EtcdMemberRemoved = "etcd.inmemory.infrastructure.cluster.x-k8s.io/member-removed"
)

func createControlPlanePod(ctx context.Content, c inmemoryclient.Client, podName, nodeName string, timestamp metav1.Time) error {
	// Check if the namespace exists
	controllerManagerPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceSystem,
			Name:      podName,
			Labels: map[string]string{
				"component": "kube-controller-manager",
				"tier":      "control-plane",
			},
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{
					Type:               corev1.PodReadyToStartContainers,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: timestamp,
				},
				{
					Type:               corev1.PodInitialized,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: timestamp,
				},
				{
					Type:               corev1.PodReady,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: timestamp,
				},
				{
					Type:               corev1.ContainersReady,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: timestamp,
				},
				{
					Type:               corev1.PodScheduled,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: timestamp,
				},
			},
		},
	}

	if err := c.Create(ctx, controllerManagerPod); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

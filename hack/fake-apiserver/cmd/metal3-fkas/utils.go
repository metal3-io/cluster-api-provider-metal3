package main

import (
	"context"
	"errors"
	"math/rand"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	inmemoryclient "sigs.k8s.io/cluster-api/test/infrastructure/inmemory/pkg/runtime/client"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

type FakePod struct {
	PodName         string
	Namespace       string
	NodeName        string
	TransactionTime metav1.Time
	Labels          map[string]string
}

func getFakePodObject(f FakePod) *corev1.Pod {
	podObj := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: f.Namespace,
			Name:      f.PodName,
			Labels:    f.Labels,
		},
		Spec: corev1.PodSpec{
			NodeName: f.NodeName,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{
					Type:               corev1.PodReadyToStartContainers,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: f.TransactionTime,
				},
				{
					Type:               corev1.PodInitialized,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: f.TransactionTime,
				},
				{
					Type:               corev1.PodReady,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: f.TransactionTime,
				},
				{
					Type:               corev1.ContainersReady,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: f.TransactionTime,
				},
				{
					Type:               corev1.PodScheduled,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: f.TransactionTime,
				},
			},
		},
	}
	return podObj
}

func createFakePod(ctx context.Context, c inmemoryclient.Client, f FakePod) error {
	// Check if the namespace exists
	ns := &corev1.Namespace{}
	err := c.Get(ctx, client.ObjectKey{Name: f.Namespace}, ns)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.New("failed to get namespace: " + err.Error())
		}

		// Namespace does not exist, create it
		ns = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: f.Namespace,
			},
		}
		if err := c.Create(ctx, ns); err != nil {
			return errors.New("failed to create namespace: " + err.Error())
		}
	}

	podObj := getFakePodObject(f)

	if err := c.Create(ctx, podObj); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func createControlPlanePod(ctx context.Context, c inmemoryclient.Client, component string, f FakePod) error {
	podLabels := map[string]string{
		"component": component,
		"tier":      "control-plane",
	}
	f.Labels = podLabels
	f.Namespace = metav1.NamespaceSystem
	return createFakePod(ctx, c, f)
}

func getSecretKeyAndCert(
	ctx context.Context,
	k8sClient client.Client,
	namespace, secretName string,
) ([]byte, []byte, error) {
	// Get the secret
	secret := &corev1.Secret{}
	err := k8sClient.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      secretName,
	}, secret)
	if err != nil {
		return nil, nil, errors.New("failed to get secret: " + err.Error())
	}

	// Extract tls.crt and tls.key
	tlsCrt, ok := secret.Data["tls.crt"]
	if !ok {
		return nil, nil, errors.New("tls.crt not found in secret")
	}

	tlsKey, ok := secret.Data["tls.key"]
	if !ok {
		return nil, nil, errors.New("tls.key not found in secret")
	}

	return tlsCrt, tlsKey, nil
}

func waitForRandomSeconds() {
	// Generate a random number of seconds between 1 and timeoutDuration
	randomSeconds := rand.Intn(int(timeoutDuration.Seconds())) + 1 //nolint:gosec // weak random number generator is good enough here

	setupLog.Info("Waiting for " + strconv.Itoa(randomSeconds) + " seconds...\n")

	// Wait for the random number of seconds
	time.Sleep(time.Duration(randomSeconds) * time.Second)

	setupLog.Info("Done waiting!")
}

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type GoldpingerInput struct {
	E2EConfig     *clusterctl.E2EConfig
	TargetCluster framework.ClusterProxy
	SpecName      string
	// SkipCleanup, when true, leaves the Goldpinger resources in place so they
	// can be inspected for debugging (mirrors the suite's -e2e.skip-resource-cleanup).
	SkipCleanup bool
}

// The JSON tags use the exact PascalCase field names emitted by the
// upstream Goldpinger /check_all endpoint, so tagliatelle rules do not
// apply here.
//
//nolint:tagliatelle
type goldpingerCheckAllResponse struct {
	Hosts     []goldpingerHost `json:"hosts"`
	Responses map[string]struct {
		OK       bool `json:"OK"`
		Response struct {
			PodResults map[string]struct {
				OK bool `json:"OK"`
			} `json:"podResults"`
		} `json:"response"`
	} `json:"responses"`
}

type goldpingerHost struct {
	HostIP  string `json:"hostIP"`
	PodIP   string `json:"podIP"`
	PodName string `json:"podName"`
}

// GoldpingerCheck deploys Goldpinger as a DaemonSet on the target (workload)
// cluster and verifies pod-to-pod network connectivity between all nodes.
//
// Goldpinger runs one pod per node and pings every other instance. After the
// DaemonSet has rolled out we query the /check_all endpoint through the API
// server service proxy and assert that every peer reports healthy connectivity.
func GoldpingerCheck(ctx context.Context, inputGetter func() GoldpingerInput) {
	input := inputGetter()
	Expect(input.E2EConfig).ToNot(BeNil(), "E2EConfig is required for GoldpingerCheck")
	Expect(input.TargetCluster).ToNot(BeNil(), "TargetCluster is required for GoldpingerCheck")

	image := input.E2EConfig.MustGetVariable("GOLDPINGER_IMAGE")
	namespace := input.E2EConfig.MustGetVariable("GOLDPINGER_NAMESPACE")
	cli := input.TargetCluster.GetClient()
	clientSet := input.TargetCluster.GetClientSet()

	By("Deploying Goldpinger to the workload cluster")
	objects := goldpingerObjects(namespace, image)

	// Track only objects we actually created so cleanup never touches
	// pre-existing resources.
	created := make([]client.Object, 0, len(objects))

	for _, obj := range objects {
		// Fail on collisions rather than adopting a pre-existing object
		Expect(cli.Create(ctx, obj)).To(Succeed(),
			"failed to create Goldpinger %T %s (it may already exist from a previous run or a shared namespace)",
			obj, obj.GetName())
		created = append(created, obj)
	}

	By("Waiting for the Goldpinger DaemonSet to be fully rolled out")
	var expectedPods int
	Eventually(func(g Gomega) {
		ds := &appsv1.DaemonSet{}
		g.Expect(cli.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "goldpinger"}, ds)).To(Succeed())
		desired := ds.Status.DesiredNumberScheduled
		g.Expect(desired).To(BeNumerically(">", 0), "Goldpinger DaemonSet has no scheduled pods yet")
		g.Expect(ds.Status.NumberReady).To(Equal(desired),
			"Goldpinger DaemonSet not ready: %d/%d", ds.Status.NumberReady, desired)
		expectedPods = int(desired)
	}, input.E2EConfig.GetIntervals(input.SpecName, "wait-goldpinger")...).Should(Succeed())

	By("Querying the Goldpinger /check_all endpoint for pod-to-pod connectivity")
	var checkAll goldpingerCheckAllResponse
	Eventually(func(g Gomega) {
		raw, err := clientSet.CoreV1().RESTClient().Get().
			Namespace(namespace).
			Resource("services").
			Name("goldpinger:http").
			SubResource("proxy").
			Suffix("check_all").
			DoRaw(ctx)
		g.Expect(err).ToNot(HaveOccurred(), "failed to query Goldpinger /check_all")

		var resp goldpingerCheckAllResponse
		g.Expect(json.Unmarshal(raw, &resp)).To(Succeed(), "failed to parse Goldpinger response: %s", string(raw))

		// A DaemonSet with N ready pods must produce a complete NxN mesh
		g.Expect(resp.Hosts).To(HaveLen(expectedPods),
			"Goldpinger discovered %d/%d hosts; expected all pods to be discovered\nraw response: %s",
			len(resp.Hosts), expectedPods, string(raw))
		g.Expect(resp.Responses).To(HaveLen(expectedPods),
			"Goldpinger has %d/%d source pods reporting\nraw response: %s",
			len(resp.Responses), expectedPods, string(raw))

		problems := goldpingerMeshProblems(resp, expectedPods)
		g.Expect(problems).To(BeEmpty(),
			"Goldpinger connectivity check failed: %v\nraw response: %s", problems, string(raw))
		checkAll = resp
	}, input.E2EConfig.GetIntervals(input.SpecName, "wait-goldpinger")...).Should(Succeed())

	Logf("Goldpinger pod-to-pod connectivity check passed: full mesh of %d peers report healthy connectivity", len(checkAll.Hosts))

	// The check passed, so clean up now while the target API is still alive.
	// When SkipCleanup is set, leave everything in place for debugging; the
	// workload-cluster teardown will remove it regardless.
	if !input.SkipCleanup {
		cleanupGoldpinger(ctx, cli, created, namespace)
	}
}

// goldpingerMeshProblems verifies a complete, healthy connectivity mesh and
// returns a list of human-readable problems.
func goldpingerMeshProblems(resp goldpingerCheckAllResponse, expectedPods int) []string {
	var problems []string
	for source, r := range resp.Responses {
		if !r.OK {
			problems = append(problems, source+" (source unhealthy)")
			continue
		}
		if len(r.Response.PodResults) != expectedPods {
			problems = append(problems, fmt.Sprintf("%s reported %d/%d target results (incomplete)",
				source, len(r.Response.PodResults), expectedPods))
		}
		for target, pod := range r.Response.PodResults {
			if !pod.OK {
				problems = append(problems, fmt.Sprintf("%s -> %s", source, target))
			}
		}
	}
	return problems
}

func goldpingerObjects(namespace, image string) []client.Object {
	labels := map[string]string{"app": "goldpinger"}

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: namespace},
	}

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: "goldpinger", Namespace: namespace},
	}

	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{Name: "goldpinger", Namespace: namespace},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"list", "get"},
			},
		},
	}

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "goldpinger", Namespace: namespace},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     "goldpinger",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "goldpinger",
				Namespace: namespace,
			},
		},
	}

	daemonSet := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "goldpinger",
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: appsv1.DaemonSetSpec{
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{Type: appsv1.RollingUpdateDaemonSetStrategyType},
			Selector:       &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					ServiceAccountName: "goldpinger",
					// Tolerate every taint so a Goldpinger pod is scheduled on
					// every node.
					Tolerations: []corev1.Toleration{
						{Operator: corev1.TolerationOpExists},
					},
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: ptr.To(true),
						RunAsUser:    ptr.To(int64(1000)),
						FSGroup:      ptr.To(int64(2000)),
					},
					Containers: []corev1.Container{
						{
							Name:            "goldpinger",
							Image:           image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Env: []corev1.EnvVar{
								{Name: "HOST", Value: "0.0.0.0"},
								{Name: "PORT", Value: "8080"},
								{Name: "HOSTNAME", ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"},
								}},
								{Name: "POD_IP", ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.podIP"},
								}},
								{Name: "LABEL_SELECTOR", Value: "app=goldpinger"},
								{Name: "NAMESPACE", Value: namespace},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: ptr.To(false),
								ReadOnlyRootFilesystem:   ptr.To(true),
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("80Mi"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1m"),
									corev1.ResourceMemory: resource.MustParse("40Mi"),
								},
							},
							Ports: []corev1.ContainerPort{
								{ContainerPort: 8080, Name: "http"},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/healthz",
										Port: intstr.FromInt32(8080),
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       5,
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/healthz",
										Port: intstr.FromInt32(8080),
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       5,
							},
						},
					},
				},
			},
		},
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "goldpinger",
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: labels,
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       8080,
					TargetPort: intstr.FromInt32(8080),
				},
			},
		},
	}

	// Order matters on creation: namespace and RBAC before workloads.
	return []client.Object{ns, sa, role, roleBinding, daemonSet, service}
}

// cleanupGoldpinger deletes the given objects (which must be only those the
// test created) in reverse creation order. Deletion errors are logged but never
// fatal so cleanup cannot fail the spec.
func cleanupGoldpinger(ctx context.Context, cli client.Client, created []client.Object, namespace string) {
	if len(created) == 0 {
		return
	}
	By("Cleaning up Goldpinger resources")
	createdNamespace := false
	for i := len(created) - 1; i >= 0; i-- {
		obj := created[i]
		if _, ok := obj.(*corev1.Namespace); ok {
			createdNamespace = true
		}
		if err := cli.Delete(ctx, obj); err != nil && !apierrors.IsNotFound(err) {
			Logf("Warning: failed to delete Goldpinger %T %s: %v", obj, obj.GetName(), err)
		}
	}

	// Only wait on the namespace if this test created it. Waiting avoids leaking
	// resources into subsequent operations.
	if createdNamespace {
		deadline := time.Now().Add(2 * time.Minute)
		for {
			ns := &corev1.Namespace{}
			err := cli.Get(ctx, client.ObjectKey{Name: namespace}, ns)
			if apierrors.IsNotFound(err) {
				return
			}
			if time.Now().After(deadline) {
				Logf("Warning: Goldpinger namespace %q was not deleted before timeout (last error: %v)", namespace, err)
				return
			}
			select {
			case <-ctx.Done():
				Logf("Warning: stopped waiting for Goldpinger namespace %q deletion: %v", namespace, ctx.Err())
				return
			case <-time.After(5 * time.Second):
			}
		}
	}
}

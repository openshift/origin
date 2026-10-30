// Package workload contains e2e tests for the workload-aware scheduling (WAS) feature
// gates that have no upstream e2e coverage.
//
// Upstream covers GenericWorkload and DRAWorkloadResourceClaims with e2e tests that
// OpenShift already runs on TechPreview clusters. TopologyAwareWorkloadScheduling is only
// covered by integration tests in test/integration/scheduler/..., which run against an
// in-process apiserver and so give no signal about OpenShift. The specs here port those
// scenarios to a real cluster.
package workload

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	schedulingv1beta1 "k8s.io/api/scheduling/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"
	"k8s.io/utils/ptr"

	"github.com/openshift/origin/test/extended/util/image"
)

const (
	// gangResource is an extended resource the tests add to specific nodes. Contention is
	// created by handing out a fixed amount of this rather than by squeezing real CPU or
	// memory, which keeps the specs runnable on a default worker pool and avoids flakes
	// from unrelated load. Upstream's workload_aware_preemption e2e uses the same trick.
	gangResource corev1.ResourceName = "example.com/gang-slot"
)

func one() resource.Quantity {
	return *resource.NewQuantity(1, resource.DecimalSI)
}

// schedulableWorkers returns the worker nodes the tests may place pods on, in a stable
// order so that a spec picking "the first two" is deterministic across its own steps.
func schedulableWorkers(ctx context.Context, client kubernetes.Interface) ([]corev1.Node, error) {
	nodes, err := e2enode.GetReadySchedulableNodes(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to list schedulable nodes: %w", err)
	}
	workers := make([]corev1.Node, 0, len(nodes.Items))
	for _, node := range nodes.Items {
		// Control-plane nodes are schedulable on compact clusters, but giving them
		// extended resources and gang workloads is not what these specs are about.
		if _, isMaster := node.Labels["node-role.kubernetes.io/master"]; isMaster {
			continue
		}
		if _, isControlPlane := node.Labels["node-role.kubernetes.io/control-plane"]; isControlPlane {
			continue
		}
		workers = append(workers, node)
	}
	return workers, nil
}

// grantGangSlots adds count units of the gang extended resource to a node and registers
// removal. Capacity is what decides where the scheduler can place the gang, so each spec
// states it explicitly rather than relying on the node's real allocatable.
func grantGangSlots(ctx context.Context, client kubernetes.Interface, nodeName string, count int) {
	e2enode.AddExtendedResource(ctx, client, nodeName, gangResource, *resource.NewQuantity(int64(count), resource.DecimalSI))
	g.DeferCleanup(func(ctx context.Context) {
		e2enode.RemoveExtendedResource(ctx, client, nodeName, gangResource)
	})
}

// labelTopologyDomain puts a node into a topology domain and registers restoration of the
// previous value, so a failed spec does not leave the cluster labelled.
func labelTopologyDomain(ctx context.Context, client kubernetes.Interface, nodeName, key, domain string) {
	node, err := client.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	framework.ExpectNoError(err, "failed to read node %s before labelling", nodeName)
	previous, had := node.Labels[key]

	e2enode.AddOrUpdateLabelOnNode(client, nodeName, key, domain)
	g.DeferCleanup(func() {
		if had {
			e2enode.AddOrUpdateLabelOnNode(client, nodeName, key, previous)
			return
		}
		e2enode.RemoveLabelOffNode(client, nodeName, key)
	})
}

// newPodGroup builds a gang-scheduled PodGroup. Callers mutate the returned object to add
// the field their gate guards before creating it.
func newPodGroup(name, namespace, priorityClassName string, minCount int32) *schedulingv1beta1.PodGroup {
	return &schedulingv1beta1.PodGroup{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: schedulingv1beta1.PodGroupSpec{
			PriorityClassName: priorityClassName,
			SchedulingPolicy: schedulingv1beta1.PodGroupSchedulingPolicy{
				Gang: &schedulingv1beta1.GangSchedulingPolicy{MinCount: minCount},
			},
		},
	}
}

// createPodGroup creates a PodGroup and registers its deletion.
func createPodGroup(ctx context.Context, client kubernetes.Interface, pg *schedulingv1beta1.PodGroup) {
	_, err := client.SchedulingV1beta1().PodGroups(pg.Namespace).Create(ctx, pg, metav1.CreateOptions{})
	framework.ExpectNoError(err, "failed to create PodGroup %s/%s", pg.Namespace, pg.Name)
	g.DeferCleanup(func(ctx context.Context) {
		err := client.SchedulingV1beta1().PodGroups(pg.Namespace).Delete(ctx, pg.Name, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			framework.Logf("failed to delete PodGroup %s/%s: %v", pg.Namespace, pg.Name, err)
		}
	})
}

// newGangPod builds a pod that consumes exactly one gang slot. podGroupName may be empty
// for a pod that is not part of a group.
func newGangPod(name, namespace, podGroupName, priorityClassName string) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: corev1.PodSpec{
			RestartPolicy:     corev1.RestartPolicyNever,
			PriorityClassName: priorityClassName,
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot:   ptr.To(true),
				SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
			},
			Containers: []corev1.Container{{
				Name:    "workload",
				Image:   image.ShellImage(),
				Command: []string{"sh", "-c", "sleep infinity"},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{gangResource: one()},
					Limits:   corev1.ResourceList{gangResource: one()},
				},
				SecurityContext: &corev1.SecurityContext{
					AllowPrivilegeEscalation: ptr.To(false),
					Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
				},
			}},
		},
	}
	if podGroupName != "" {
		pod.Spec.SchedulingGroup = &corev1.PodSchedulingGroup{PodGroupName: ptr.To(podGroupName)}
	}
	return pod
}

// createPods creates the given pods and returns their names.
func createPods(ctx context.Context, client kubernetes.Interface, namespace string, pods []*corev1.Pod) []string {
	names := make([]string, 0, len(pods))
	for _, pod := range pods {
		_, err := client.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
		framework.ExpectNoError(err, "failed to create pod %s/%s", namespace, pod.Name)
		names = append(names, pod.Name)
	}
	return names
}

// nodeOf returns the node a pod is assigned to, or "" when it is still unscheduled or gone.
func nodeOf(ctx context.Context, client kubernetes.Interface, namespace, name string) string {
	pod, err := client.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return ""
	}
	return pod.Spec.NodeName
}

// expectNoneScheduled asserts that none of the named pods got a node. It is used by the
// negative specs, where the interesting failure is a pod being placed at all.
func expectNoneScheduled(ctx context.Context, client kubernetes.Interface, namespace string, names []string) {
	for _, name := range names {
		o.Expect(nodeOf(ctx, client, namespace, name)).To(o.BeEmpty(),
			"pod %s should not have been scheduled", name)
	}
}

// domainOf returns the topology domain of the node a pod landed on.
func domainOf(ctx context.Context, client kubernetes.Interface, namespace, podName, topologyKey string) string {
	nodeName := nodeOf(ctx, client, namespace, podName)
	o.Expect(nodeName).NotTo(o.BeEmpty(), "pod %s should be scheduled before reading its domain", podName)
	node, err := client.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	framework.ExpectNoError(err, "failed to read node %s", nodeName)
	return node.Labels[topologyKey]
}

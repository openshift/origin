package node

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	ote "github.com/openshift-eng/openshift-tests-extension/pkg/ginkgo"

	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/utils/ptr"

	nodeutils "github.com/openshift/origin/test/extended/node"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/operator"
)

var _ = g.Describe("[Suite:openshift/disruptive-longrunning][sig-node][Disruptive] PodDisruptionBudget", func() {
	var (
		oc = exutil.NewCLIWithoutNamespace("pdb-drain")
	)

	g.BeforeEach(func(ctx context.Context) {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("Skipping test on MicroShift cluster")
		}

		nodeutils.EnsureNodesReady(ctx, oc)
	})

	//author: bgudi@redhat.com
	g.It("[OTP] Node's drain should block when PodDisruptionBudget minAvailable equals 100 percentage and selector is empty [OCP-67564]", ote.Informing(), func() {
		ctx := context.Background()

		// Skip on SNO/External topologies where there might not be dedicated worker nodes
		infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to get cluster infrastructure")
		if infra.Status.ControlPlaneTopology == "SingleReplica" || infra.Status.ControlPlaneTopology == "External" {
			g.Skip("Skipping on SNO/External topology - requires dedicated worker nodes")
		}

		oc.SetupProject()
		namespace := oc.Namespace()

		g.By("Get a worker node to schedule pods on")
		workers, err := exutil.GetReadySchedulableWorkerNodes(ctx, oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to get worker nodes")
		o.Expect(workers).NotTo(o.BeEmpty(), "no ready schedulable worker nodes found")
		workerNode := workers[0].Name
		e2e.Logf("Selected worker node: %s", workerNode)

		g.By("Create 6 pods on the selected worker node")
		numPods := 6
		podBaseName := "pdb-drain-test-pod"
		for i := 0; i < numPods; i++ {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-%d", podBaseName, i),
					Namespace: namespace,
					Labels: map[string]string{
						"app": "pdb-drain-test",
					},
				},
				Spec: corev1.PodSpec{
					NodeSelector: map[string]string{
						"kubernetes.io/hostname": workerNode,
					},
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: ptr.To(true),
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "test-container",
							Image: "quay.io/openshifttest/hello-openshift@sha256:4200f438cf2e9446f6bcff9d67ceea1f69ed07a2f83363b7fb52529f7ddd8a83",
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: ptr.To(false),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
							},
						},
					},
				},
			}
			_, err = oc.KubeClient().CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("failed to create pod %d", i))
		}

		g.By("Wait for all pods to be ready")
		err = wait.PollUntilContextTimeout(ctx, 3*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
			podList, pollErr := oc.KubeClient().CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
				LabelSelector: "app=pdb-drain-test",
			})
			if pollErr != nil {
				e2e.Logf("Error getting pods: %v", pollErr)
				return false, nil
			}
			readyPods := 0
			for _, pod := range podList.Items {
				for _, cond := range pod.Status.Conditions {
					if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
						readyPods++
						break
					}
				}
			}
			if readyPods == numPods {
				e2e.Logf("All %d pods are ready", readyPods)
				return true, nil
			}
			e2e.Logf("Waiting for pods to be ready: %d/%d", readyPods, numPods)
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "pods did not become ready")

		g.By("Create PodDisruptionBudget with 100% minAvailable and empty selector")
		pdb := &policyv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pdb-drain-test",
				Namespace: namespace,
			},
			Spec: policyv1.PodDisruptionBudgetSpec{
				MinAvailable: &intstr.IntOrString{
					Type:   intstr.String,
					StrVal: "100%",
				},
				Selector: &metav1.LabelSelector{},
			},
		}
		_, err = oc.KubeClient().PolicyV1().PodDisruptionBudgets(namespace).Create(ctx, pdb, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create PodDisruptionBudget")
		g.DeferCleanup(oc.KubeClient().PolicyV1().PodDisruptionBudgets(namespace).Delete, ctx, "pdb-drain-test", metav1.DeleteOptions{})

		g.By("Verify all test pods are on the selected worker node")
		podList, err := oc.KubeClient().CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: "app=pdb-drain-test",
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to get pods")
		podsOnWorker := 0
		for _, pod := range podList.Items {
			if pod.Spec.NodeName == workerNode {
				podsOnWorker++
			}
		}
		o.Expect(podsOnWorker).To(o.Equal(numPods), "not all pods are on the selected worker node")

		g.By("Make sure that PDB's DisruptionAllowed condition is False")
		var pdbStatus string
		err = wait.PollUntilContextTimeout(ctx, 2*time.Second, 30*time.Second, true, func(pollCtx context.Context) (bool, error) {
			var pollErr error
			pdbStatus, pollErr = oc.AsAdmin().WithoutNamespace().Run("get").Args("poddisruptionbudget", "pdb-drain-test", "-n", namespace, "-o=jsonpath={.status.conditions[?(@.type==\"DisruptionAllowed\")].status}").Output()
			if pollErr != nil {
				e2e.Logf("Error getting PDB status: %v", pollErr)
				return false, nil
			}
			if pdbStatus != "" {
				return true, nil
			}
			e2e.Logf("Waiting for PDB DisruptionAllowed condition to appear")
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "PDB DisruptionAllowed condition not found")
		o.Expect(pdbStatus).Should(o.Equal("False"), "PDB DisruptionAllowed should be False")

		g.By("Drain the selected worker node")
		g.DeferCleanup(func() {
			err := operator.WaitForOperatorsToSettle(ctx, oc.AdminConfigClient(), 10)
			o.Expect(err).NotTo(o.HaveOccurred(), "cluster operators failed to return to available state after node drain")
		})
		g.DeferCleanup(oc.AsAdmin().WithoutNamespace().Run("adm").Args("uncordon", workerNode).Execute)

		out, err := oc.AsAdmin().WithoutNamespace().Run("adm").Args("drain", workerNode, "--ignore-daemonsets", "--delete-emptydir-data", "--force", "--timeout=30s").Output()
		o.Expect(err).To(o.HaveOccurred(), "drain operation should have been blocked but it wasn't")
		o.Expect(strings.Contains(out, "Cannot evict pod as it would violate the pod's disruption budget")).Should(o.BeTrue(), "drain output missing PDB violation error message")
		o.Expect(strings.Contains(out, "There are pending nodes to be drained")).Should(o.BeTrue(), "drain output missing pending nodes error message")

		g.By("Verify that test pods remain on the node after failed drain")
		podsAfterDrain, err := oc.KubeClient().CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: "app=pdb-drain-test",
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to get pods after drain attempt")
		podsStillOnWorker := 0
		for _, pod := range podsAfterDrain.Items {
			if pod.Spec.NodeName == workerNode {
				podsStillOnWorker++
			}
		}
		o.Expect(podsStillOnWorker).To(o.Equal(numPods), "all test pods should still be on the worker node")
	})
})

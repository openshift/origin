package two_node

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	v1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

var _ = g.Describe("[sig-etcd][apigroup:config.openshift.io][OCPFeatureGate:HighlyAvailableArbiter][Suite:openshift/two-node][Disruptive] Single node outage is handled seamlessly", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("").AsAdmin()

	g.BeforeEach(func() {
		skipIfNotTopology(oc, v1.HighlyAvailableArbiterMode)
	})

	g.It("should maintain etcd quorum and workloads with one master node down", func() {
		ctx := context.Background()

		g.By("Identifying one master node to simulate failure")
		masterNodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{
			LabelSelector: labelNodeRoleMaster,
		})
		o.Expect(err).To(o.BeNil())
		o.Expect(len(masterNodes.Items)).To(o.Equal(2))
		targetNode := masterNodes.Items[0].Name

		g.By(fmt.Sprintf("Gracefully rebooting %s to simulate failure", targetNode))
		shutdownOrRebootNode(oc, targetNode, "openshift-etcd", "shutdown", "-r", "+1")

		g.By("Waiting for the node to become NotReady")
		waitForNodeCondition(oc, targetNode, corev1.NodeReady, corev1.ConditionFalse, "NotReady", 10*time.Minute)

		g.By("Validating etcd quorum is met while the node is still NotReady")
		err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
			node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, targetNode, metav1.GetOptions{})
			if err != nil {
				return false, nil
			}
			stillNotReady := false
			for _, cond := range node.Status.Conditions {
				if cond.Type == corev1.NodeReady && cond.Status != corev1.ConditionTrue {
					stillNotReady = true
					break
				}
			}
			if !stillNotReady {
				return false, nil
			}

			operator, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().Get(ctx, "etcd", metav1.GetOptions{})
			if err != nil {
				return false, nil
			}
			return isClusterOperatorAvailable(operator), nil
		})
		o.Expect(err).To(o.BeNil(), "Expected etcd operator to remain healthy while one master node is NotReady")
	})
	g.AfterEach(func() {
		ctx := context.Background()
		g.By("Ensuring all cluster nodes are back to Ready state")

		nodeList, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		o.Expect(err).To(o.BeNil(), "Failed to list cluster nodes")

		for _, node := range nodeList.Items {
			waitForNodeCondition(oc, node.Name, corev1.NodeReady, corev1.ConditionTrue, "Ready", 15*time.Minute)
		}
	})
})

var _ = g.Describe("[sig-etcd][apigroup:config.openshift.io][OCPFeatureGate:HighlyAvailableArbiter][Suite:openshift/two-node][Disruptive] Recovery after arbiter down and master node restart", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithoutNamespace("").AsAdmin()
	var arbiterNodeName string
	g.BeforeEach(func() {
		skipIfNotTopology(oc, v1.HighlyAvailableArbiterMode)
	})
	g.It("should regain quorum after arbiter down and master nodes restart", func() {
		ctx := context.Background()

		g.By("Getting arbiter node")
		arbiterNodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{
			LabelSelector: labelNodeRoleArbiter,
		})
		o.Expect(err).To(o.BeNil())
		o.Expect(len(arbiterNodes.Items)).To(o.Equal(1))
		arbiterNode := arbiterNodes.Items[0]
		arbiterNodeName = arbiterNode.Name

		g.By("Triggering 15-minute simulated shutdown on arbiter node by stopping kubelet")
		_, err = exutil.DebugNodeRetryWithOptionsAndChroot(oc, arbiterNodeName, "openshift-etcd",
			"bash", "-c", `systemd-run --on-active=10s --unit=delayed-reboot.service bash -c "sleep 5; systemctl stop kubelet; sleep 900; reboot"`)
		o.Expect(err).To(o.BeNil(), "Expected arbiter shutdown simulation to succeed")

		g.By("Waiting for arbiter to become status uknown due to kubelet stopped")
		waitForNodeCondition(oc, arbiterNodeName, corev1.NodeReady, corev1.ConditionUnknown, "Unknown", 5*time.Minute)

		g.By("Rebooting both master nodes")
		masterNodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{
			LabelSelector: labelNodeRoleMaster,
		})
		o.Expect(err).To(o.BeNil())
		for _, node := range masterNodes.Items {
			shutdownOrRebootNode(oc, node.Name, "openshift-etcd", "shutdown", "-r", "+1")
		}

		g.By("Waiting for master nodes to become NotReady")
		for _, node := range masterNodes.Items {
			waitForNodeCondition(oc, node.Name, corev1.NodeReady, corev1.ConditionFalse, "NotReady", 5*time.Minute)
		}

		g.By("Waiting for master nodes to become Ready")
		for _, node := range masterNodes.Items {
			waitForNodeCondition(oc, node.Name, corev1.NodeReady, corev1.ConditionFalse, "Ready", 15*time.Minute)
		}

		g.By("Waiting for etcd quorum to be restored")
		err = wait.PollUntilContextTimeout(ctx, 15*time.Second, 15*time.Minute, true, func(ctx context.Context) (bool, error) {
			operator, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().Get(ctx, "etcd", metav1.GetOptions{})
			if err != nil {
				return false, nil
			}
			return isClusterOperatorAvailable(operator), nil
		})
		o.Expect(err).To(o.BeNil(), "Expected etcd operator to become available again")
	})
	g.AfterEach(func() {
		g.By("Ensuring arbiter node becomes ready again")
		waitForNodeCondition(oc, arbiterNodeName, corev1.NodeReady, corev1.ConditionFalse, "NotReady", 15*time.Minute)
	})
})

func shutdownOrRebootNode(oc *exutil.CLI, nodeName, component string, args ...string) {
	_, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, component, args...)
	action := strings.Join(args, " ")
	o.Expect(err).To(o.BeNil(), fmt.Sprintf("Expected node %s to execute '%s' successfully", nodeName, action))
}

func waitForNodeCondition(oc *exutil.CLI, nodeName string, conditionType corev1.NodeConditionType, expectStatus corev1.ConditionStatus, statusName string, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	err := wait.PollUntilContextTimeout(ctx, 10*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		for _, cond := range node.Status.Conditions {
			if cond.Type == conditionType && cond.Status == expectStatus {
				return true, nil
			}
		}
		return false, nil
	})
	o.Expect(err).To(o.BeNil(), fmt.Sprintf("Expected node %s to become %s", nodeName, statusName))
}

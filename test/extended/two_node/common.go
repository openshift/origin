package two_node

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	v1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/test/extended/util"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
)

const (
	labelNodeRoleMaster       = "node-role.kubernetes.io/master"
	labelNodeRoleControlPlane = "node-role.kubernetes.io/control-plane"
	labelNodeRoleWorker       = "node-role.kubernetes.io/worker"
	labelNodeRoleArbiter      = "node-role.kubernetes.io/arbiter"
)

func skipIfNotTopology(oc *exutil.CLI, wanted v1.TopologyMode) {
	current, err := exutil.GetControlPlaneTopology(oc)
	if err != nil {
		e2eskipper.Skip(fmt.Sprintf("Could not get current topology, skipping test: error %v", err))
	}
	if *current != wanted {
		e2eskipper.Skip(fmt.Sprintf("Cluster is not in %v topology, skipping test", wanted))
	}
}

func isClusterOperatorAvailable(operator *v1.ClusterOperator) bool {
	for _, cond := range operator.Status.Conditions {
		if cond.Type == v1.OperatorAvailable && cond.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}

func isClusterOperatorDegraded(operator *v1.ClusterOperator) bool {
	for _, cond := range operator.Status.Conditions {
		if cond.Type == v1.OperatorDegraded && cond.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}

func hasNodeRebooted(oc *util.CLI, node *corev1.Node) (bool, error) {
	if n, err := oc.AdminKubeClient().CoreV1().Nodes().Get(context.Background(), node.Name, metav1.GetOptions{}); err != nil {
		return false, err
	} else {
		return n.Status.NodeInfo.BootID != node.Status.NodeInfo.BootID, nil
	}
}

func ensureTNFDegradedOrSkip(oc *exutil.CLI) {
	skipIfNotTopology(oc, v1.DualReplicaTopologyMode)

	ctx := context.Background()
	kubeClient := oc.AdminKubeClient()

	masters, err := listControlPlaneNodes(ctx, kubeClient)
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to list control-plane nodes")

	if len(masters) != 2 {
		g.Skip(fmt.Sprintf(
			"TNF degraded tests expect exactly 2 control-plane nodes, found %d",
			len(masters),
		))
	}

	readyCount := countReadyNodes(masters)
	if readyCount != 1 {
		g.Skip(fmt.Sprintf(
			"cluster is not in TNF degraded mode (expected exactly 1 Ready master, got %d)",
			readyCount,
		))
	}
}

func listControlPlaneNodes(ctx context.Context, client kubernetes.Interface) ([]corev1.Node, error) {
	nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: "node-role.kubernetes.io/master",
	})
	if err != nil {
		return nil, err
	}
	if len(nodes.Items) > 0 {
		return nodes.Items, nil
	}

	nodes, err = client.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: "node-role.kubernetes.io/control-plane",
	})
	if err != nil {
		return nil, err
	}
	return nodes.Items, nil
}

func countReadyNodes(nodes []corev1.Node) int {
	ready := 0
	for _, n := range nodes {
		for _, cond := range n.Status.Conditions {
			if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
				ready++
				break
			}
		}
	}
	return ready
}

func getReadyMasterNode(
	ctx context.Context,
	client kubernetes.Interface,
) (*corev1.Node, error) {
	nodes, err := client.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: "node-role.kubernetes.io/master=",
	})
	if err != nil {
		return nil, err
	}

	for i := range nodes.Items {
		node := &nodes.Items[i]
		if isNodeReady(node) {
			return node, nil
		}
	}
	return nil, fmt.Errorf("no Ready master node found")
}

func isNodeReady(node *corev1.Node) bool {
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

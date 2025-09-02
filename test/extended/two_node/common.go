package two_node

import (
	"context"
	"fmt"

	v1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
)

const (
	// Node filtering constants
	allNodes = ""
	labelNodeRoleControlPlane = "node-role.kubernetes.io/control-plane"
	labelNodeRoleWorker       = "node-role.kubernetes.io/worker"
	labelNodeRoleArbiter      = "node-role.kubernetes.io/arbiter"

	// CLI privilege levels
	nonAdmin = false
	admin    = true
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

// createCLI creates a new CLI instance with optional admin privileges
func createCLI(requireAdmin bool) *exutil.CLI {
	if requireAdmin {
		return exutil.NewCLIWithoutNamespace("").AsAdmin()
	}
	return exutil.NewCLIWithoutNamespace("")
}

// getNodes returns a list of nodes, optionally filtered by role label
// When roleLabel is allNodes (""), returns all nodes
// When roleLabel is specified, filters nodes by that label
func getNodes(oc *exutil.CLI, roleLabel string) (*corev1.NodeList, error) {
	if roleLabel == "" {
		return oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	}
	return oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
		LabelSelector: roleLabel,
	})
}

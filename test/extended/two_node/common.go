package two_node

import (
	"context"
	"fmt"
	"strings"

	v1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/test/extended/util"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
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

// addConstraint adds constraint that avoids having resource as part of the cluster
func addConstraint(oc *exutil.CLI, nodeName string, resourceName string, targetNode string) error {
	framework.Logf("Adding constraint for %s resource to avoid %s", resourceName, targetNode)

	cmd := []string{
		"bash", "-c",
		fmt.Sprintf("pcs constraint location %s avoids %s", resourceName, targetNode),
	}

	_, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "openshift-etcd", cmd...)
	if err != nil {
		framework.Logf("Failed to add constraint using debug node")
		return fmt.Errorf("failed to add constraint for resource %s to avoid %s: %v", resourceName, targetNode, err)
	}

	framework.Logf("Successfully added constraint for resource %s to avoid %s", resourceName, targetNode)
	return nil
}

// removeConstraint removes constraint that avoids having resource as part of the cluster
func removeConstraint(oc *exutil.CLI, nodeName string, constraintId string) error {
	framework.Logf("Removing constraint with ID %s on %s", constraintId, nodeName)

	cmd := []string{
		"bash", "-c",
		fmt.Sprintf("pcs constraint remove %s", constraintId),
	}

	_, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "openshift-etcd", cmd...)
	if err != nil {
		framework.Logf("Failed to remove constraint using debug node")
		return fmt.Errorf("failed to remove constraint %s: %v", constraintId, err)
	}

	framework.Logf("Successfully removed constraint %s", constraintId)
	return nil
}

// discoverConstraintId discovers the constraint ID for a specific resource and node combination using JSON output and jq
func discoverConstraintId(oc *exutil.CLI, nodeName string, resourceName string, targetNode string) (string, error) {
	framework.Logf("Discovering constraint ID for resource %s avoiding node %s using node %s", resourceName, targetNode, nodeName)

	// Use jq to extract the constraint ID from JSON output directly
	jqQuery := fmt.Sprintf(`'.location[] | select(.resource_id == "%s" and .attributes.node == "%s" and .attributes.score == "-INFINITY") | .attributes.constraint_id'`, resourceName, targetNode)

	cmd := []string{
		"bash", "-c",
		fmt.Sprintf(`pcs constraint config --output-format json | jq -r %s`, jqQuery),
	}

	constraintId, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "openshift-etcd", cmd...)
	if err != nil {
		framework.Logf("Failed to discover constraint ID from node %s", nodeName)
		return "", fmt.Errorf("failed to discover constraint ID using jq on node %s: %v", nodeName, err)
	}

	constraintId = strings.TrimSpace(constraintId)
	if constraintId == "" || constraintId == "null" {
		framework.Logf("Constraint ID not found for resource %s avoiding node %s", resourceName, targetNode)
		return "", fmt.Errorf("constraint ID not found for resource %s avoiding node %s", resourceName, targetNode)
	}

	framework.Logf("Found constraint ID: %s for resource %s avoiding node %s", constraintId, resourceName, targetNode)
	return constraintId, nil
}

// isResourceStopped checks if a Pacemaker resource is stopped on a specific node
func isResourceStopped(oc *exutil.CLI, nodeName string, resourceName string) (bool, error) {
	framework.Logf("Checking if resource %s is stopped on node %s", resourceName, nodeName)

	cmd := []string{
		"pcs", "status", "resources", resourceName, fmt.Sprintf("node=%s", nodeName),
	}

	output, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "openshift-etcd", cmd...)
	if err != nil {
		return false, fmt.Errorf("failed to get pcs status resources on node %s: %v", nodeName, err)
	}

	framework.Logf("PCS status resources output:\n%s", output)

	// Resource is stopped if output contains "no active resources"
	if strings.Contains(strings.ToLower(output), "no active resources") {
		framework.Logf("Resource %s is stopped on node %s (no active resources)", resourceName, nodeName)
		return true, nil
	}

	// Parse each line to check the specific node status
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Look for lines containing the resource name
		if strings.Contains(line, resourceName) {
			// Check if this line shows the target node as Started
			if strings.Contains(line, "Started") && strings.Contains(line, nodeName) {
				framework.Logf("Resource %s is running on node %s (Started)", resourceName, nodeName)
				return false, nil
			}

			// Check if this line shows the target node as Stopped
			if strings.Contains(line, "Stopped") && strings.Contains(line, nodeName) {
				framework.Logf("Resource %s is stopped on node %s (Stopped)", resourceName, nodeName)
				return true, nil
			}
		}
	}

	// Unknown state - log the output for debugging
	framework.Logf("Unknown resource state for %s on node %s. Output: %s", resourceName, nodeName, output)
	return false, fmt.Errorf("could not determine resource %s state on node %s", resourceName, nodeName)
}

// stopKubeletService stops the kubelet service on the specified node
func stopKubeletService(oc *exutil.CLI, nodeName string) error {
	framework.Logf("Stopping kubelet service on node %s", nodeName)

	cmd := []string{
		"systemctl", "stop", "kubelet",
	}
	_, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "openshift-etcd", cmd...)
	if err != nil {
		return fmt.Errorf("failed to stop kubelet service on node %s: %v", nodeName, err)
	}

	framework.Logf("Successfully stopped kubelet service on node %s", nodeName)
	return nil
}

// isServiceRunning checks if the specified service is running on the specified node
func isServiceRunning(oc *exutil.CLI, nodeName string, serviceName string) bool {
	cmd := []string{
		"systemctl", "is-active", "--quiet", serviceName,
	}

	_, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "openshift-etcd", cmd...)
	return err == nil
}

// isNodeReady checks if a node is in Ready state
func isNodeReady(oc *exutil.CLI, nodeName string) bool {
	node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		framework.Logf("Error getting node %s: %v", nodeName, err)
		return false
	}

	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

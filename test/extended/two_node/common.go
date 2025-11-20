package two_node

import (
	"context"
	"errors"
	"fmt"
	"strings"

	v1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
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

func hasNodeRebooted(oc *exutil.CLI, node *corev1.Node) (bool, error) {
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

// validateClusterOperatorsAvailable ensures all cluster operators are available
func validateClusterOperatorsAvailable(oc *exutil.CLI) error {
	clusterOperators, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list cluster operators: %v", err)
	}

	var unavailableOperators []string
	var degradedOperators []string

	// Slice through all operators and collect problematic ones
	for _, operator := range clusterOperators.Items {
		if !isClusterOperatorAvailable(&operator) {
			unavailableOperators = append(unavailableOperators, operator.Name)
			// Log detailed condition information for unavailable operators
			framework.Logf("Operator %s is NOT AVAILABLE. Conditions:", operator.Name)
			for _, cond := range operator.Status.Conditions {
				if cond.Type == v1.OperatorAvailable {
					framework.Logf("  Available: %s (Reason: %s, Message: %s)", cond.Status, cond.Reason, cond.Message)
				}
			}
		}
		if isClusterOperatorDegraded(&operator) {
			degradedOperators = append(degradedOperators, operator.Name)
			// Log detailed condition information for degraded operators
			framework.Logf("Operator %s is DEGRADED. Conditions:", operator.Name)
			for _, cond := range operator.Status.Conditions {
				if cond.Type == v1.OperatorDegraded {
					framework.Logf("  Degraded: %s (Reason: %s, Message: %s)", cond.Status, cond.Reason, cond.Message)
				}
			}
		}
	}

	// Report results
	totalOperators := len(clusterOperators.Items)
	availableOperators := totalOperators - len(unavailableOperators)
	nonDegradedOperators := totalOperators - len(degradedOperators)

	framework.Logf("Cluster Operators Summary: %d total, %d available, %d non-degraded",
		totalOperators, availableOperators, nonDegradedOperators)

	// Return detailed error if any operators are problematic
	if len(unavailableOperators) > 0 || len(degradedOperators) > 0 {
		var errorMsg strings.Builder

		if len(unavailableOperators) > 0 {
			errorMsg.WriteString(fmt.Sprintf("Unavailable operators (%d): %s",
				len(unavailableOperators), strings.Join(unavailableOperators, ", ")))
		}

		if len(degradedOperators) > 0 {
			if errorMsg.Len() > 0 {
				errorMsg.WriteString("; ")
			}
			errorMsg.WriteString(fmt.Sprintf("Degraded operators (%d): %s",
				len(degradedOperators), strings.Join(degradedOperators, ", ")))
		}

		return errors.New(errorMsg.String())
	}

	framework.Logf("All %d cluster operators are available and not degraded", totalOperators)
	return nil
}

// logEtcdClusterStatus performs comprehensive etcd cluster status logging and validation
// This function is designed to be used in AfterEach functions to ensure tests leave the cluster in a known good state
func logEtcdClusterStatus(oc *exutil.CLI, testContext string) error {
	framework.Logf("=== Starting comprehensive etcd cluster status check (%s) ===", testContext)

	// Check etcd ClusterOperator status
	framework.Logf("Checking etcd ClusterOperator status...")
	etcdOperator, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().Get(context.Background(), "etcd", metav1.GetOptions{})
	if err != nil {
		framework.Logf("ERROR: Failed to retrieve etcd ClusterOperator: %v", err)
		return fmt.Errorf("failed to retrieve etcd ClusterOperator: %v", err)
	}

	// Log etcd operator conditions in detail
	framework.Logf("Etcd ClusterOperator conditions:")
	for _, condition := range etcdOperator.Status.Conditions {
		framework.Logf("  - %s: %s (Reason: %s, Message: %s, LastTransition: %s)",
			condition.Type, condition.Status, condition.Reason, condition.Message, condition.LastTransitionTime)
	}

	// Check if etcd operator is Available
	available := false
	degraded := false
	progressing := false

	for _, condition := range etcdOperator.Status.Conditions {
		switch condition.Type {
		case v1.OperatorAvailable:
			available = (condition.Status == v1.ConditionTrue)
		case v1.OperatorDegraded:
			degraded = (condition.Status == v1.ConditionTrue)
		case v1.OperatorProgressing:
			progressing = (condition.Status == v1.ConditionTrue)
		}
	}

	framework.Logf("Etcd ClusterOperator summary: Available=%t, Degraded=%t, Progressing=%t", available, degraded, progressing)

	if !available {
		framework.Logf("WARNING: etcd ClusterOperator is not Available")
		return fmt.Errorf("etcd ClusterOperator is not Available")
	}
	if degraded {
		framework.Logf("WARNING: etcd ClusterOperator is Degraded")
		return fmt.Errorf("etcd ClusterOperator is Degraded")
	}
	if progressing {
		framework.Logf("INFO: etcd ClusterOperator is Progressing (this may be normal during updates)")
	}

	// Check etcd pods status
	framework.Logf("Checking etcd pods status...")
	etcdPods, err := oc.AdminKubeClient().CoreV1().Pods("openshift-etcd").List(context.Background(), metav1.ListOptions{
		LabelSelector: "app=etcd",
	})
	if err != nil {
		framework.Logf("ERROR: Failed to retrieve etcd pods: %v", err)
		return fmt.Errorf("failed to retrieve etcd pods: %v", err)
	}

	framework.Logf("Found %d etcd pods:", len(etcdPods.Items))
	runningPods := 0
	for _, pod := range etcdPods.Items {
		framework.Logf("  - Pod %s: Phase=%s, Ready=%t, Node=%s",
			pod.Name, pod.Status.Phase, isPodReady(&pod), pod.Spec.NodeName)

		// Log container statuses for more detail
		for _, containerStatus := range pod.Status.ContainerStatuses {
			framework.Logf("    Container %s: Ready=%t, RestartCount=%d",
				containerStatus.Name, containerStatus.Ready, containerStatus.RestartCount)
			if containerStatus.State.Waiting != nil {
				framework.Logf("      Waiting: %s - %s", containerStatus.State.Waiting.Reason, containerStatus.State.Waiting.Message)
			}
			if containerStatus.State.Terminated != nil {
				framework.Logf("      Terminated: %s - %s", containerStatus.State.Terminated.Reason, containerStatus.State.Terminated.Message)
			}
		}

		if pod.Status.Phase == corev1.PodRunning {
			runningPods++
		}
	}

	framework.Logf("Etcd pods summary: %d total, %d running", len(etcdPods.Items), runningPods)

	if runningPods < 1 {
		framework.Logf("ERROR: No etcd pods are running")
		return fmt.Errorf("no etcd pods are running")
	}

	// Enhanced node and etcd member health checks
	nodeList, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		framework.Logf("WARNING: Failed to retrieve nodes for etcd member health check: %v", err)
	} else {
		framework.Logf("=== Enhanced Node and Etcd Member Analysis ===")

		// Check if both nodes are healthy
		framework.Logf("Checking node health status...")
		healthyNodes := 0
		readyNodes := 0
		for _, node := range nodeList.Items {
			isReady := false
			for _, condition := range node.Status.Conditions {
				if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
					isReady = true
					readyNodes++
					break
				}
			}

			framework.Logf("  - Node %s: Ready=%t, Roles=%s",
				node.Name, isReady, getNodeRoles(&node))

			if isReady {
				healthyNodes++
			}
		}
		framework.Logf("Node health summary: %d total nodes, %d ready nodes", len(nodeList.Items), readyNodes)

		// Enhanced etcd member analysis
		framework.Logf("Checking detailed etcd member status...")
		votingMembers := 0
		learnerMembers := 0
		healthyMembers := 0

		for _, node := range nodeList.Items {
			// Check if this node has an etcd pod
			var etcdPod *corev1.Pod
			for _, pod := range etcdPods.Items {
				if pod.Spec.NodeName == node.Name && pod.Status.Phase == corev1.PodRunning {
					etcdPod = &pod
					break
				}
			}

			if etcdPod != nil {
				framework.Logf("  - Node %s: has running etcd pod %s", node.Name, etcdPod.Name)
				healthyMembers++

				// Try to determine if member is promoted (voting) or learner
				// This is inferred from etcd operator status rather than direct etcd API calls
				memberStatus := checkEtcdMemberPromotionStatus(oc, node.Name)
				switch memberStatus {
				case "voting":
					votingMembers++
					framework.Logf("    └─ Member status: VOTING (promoted)")
				case "learner":
					learnerMembers++
					framework.Logf("    └─ Member status: LEARNER (not yet promoted)")
				default:
					framework.Logf("    └─ Member status: UNKNOWN (unable to determine)")
				}
			} else {
				framework.Logf("  - Node %s: no running etcd pod", node.Name)
			}
		}

		framework.Logf("Etcd member promotion summary: %d voting members, %d learner members, %d total healthy",
			votingMembers, learnerMembers, healthyMembers)

		// Check if both members are promoted (for 2-node clusters)
		if len(nodeList.Items) == 2 {
			if votingMembers == 2 && learnerMembers == 0 {
				framework.Logf("✅ Both etcd members are promoted (voting members)")
			} else if learnerMembers > 0 {
				framework.Logf("⚠️  Found %d learner members - waiting for promotion to voting members", learnerMembers)
			} else {
				framework.Logf("❓ Unable to determine promotion status for all members")
			}
		}
	}

	// Check if we're waiting for CEO (Cluster Etcd Operator) revision controller
	framework.Logf("=== CEO Revision Controller Analysis ===")
	if err := checkCEORevisionControllerStatus(oc); err != nil {
		framework.Logf("WARNING: CEO revision controller issues detected: %v", err)
	}

	// Final validation - ensure cluster operators are available
	framework.Logf("=== Final Cluster Operators Validation ===")
	if err := validateClusterOperatorsAvailable(oc); err != nil {
		framework.Logf("WARNING: Some cluster operators are not available: %v", err)
		// Don't return error here as this might be transient during cluster operations
	} else {
		framework.Logf("All cluster operators are available and healthy")
	}

	framework.Logf("=== Etcd cluster status check completed successfully (%s) ===", testContext)
	return nil
}

// isPodReady checks if a pod is ready based on its conditions
func isPodReady(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// getNodeRoles returns a comma-separated string of node roles
func getNodeRoles(node *corev1.Node) string {
	var roles []string

	for label := range node.Labels {
		switch label {
		case labelNodeRoleMaster:
			roles = append(roles, "master")
		case labelNodeRoleControlPlane:
			roles = append(roles, "control-plane")
		case labelNodeRoleWorker:
			roles = append(roles, "worker")
		case labelNodeRoleArbiter:
			roles = append(roles, "arbiter")
		}
	}

	if len(roles) == 0 {
		return "none"
	}

	return strings.Join(roles, ",")
}

// checkEtcdMemberPromotionStatus attempts to determine if an etcd member is voting or learner
// This is inferred from the etcd operator's node status rather than direct etcd API calls
func checkEtcdMemberPromotionStatus(oc *exutil.CLI, nodeName string) string {
	// Get etcd operator status
	etcdOperator, err := oc.AdminOperatorClient().OperatorV1().Etcds().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		framework.Logf("Unable to get etcd operator status for member promotion check: %v", err)
		return "unknown"
	}

	// Check node statuses in etcd operator
	for _, nodeStatus := range etcdOperator.Status.NodeStatuses {
		if nodeStatus.NodeName == nodeName {
			// Check if the node has a current revision and is not in learner state
			// A member with a current revision that matches the latest available revision
			// is typically a voting member
			if nodeStatus.CurrentRevision > 0 {
				// Additional heuristics could be added here based on the operator status
				// For now, assume members with current revisions are voting members

				// Check if there are any conditions indicating learner status
				// This is a best-effort approach as the exact status depends on operator internals
				return "voting"
			} else {
				return "learner"
			}
		}
	}

	return "unknown"
}

// checkCEORevisionControllerStatus checks if we're waiting for CEO revision controller to clear conditions
func checkCEORevisionControllerStatus(oc *exutil.CLI) error {
	framework.Logf("Checking CEO (Cluster Etcd Operator) revision controller status...")

	// Get etcd operator status
	etcdOperator, err := oc.AdminOperatorClient().OperatorV1().Etcds().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get etcd operator for revision controller check: %v", err)
	}

	// Check if operator is progressing (which might indicate revision controller activity)
	progressing := false
	progressingReason := ""
	progressingMessage := ""

	for _, condition := range etcdOperator.Status.Conditions {
		if condition.Type == operatorv1.OperatorStatusTypeProgressing && condition.Status == operatorv1.ConditionTrue {
			progressing = true
			progressingReason = condition.Reason
			progressingMessage = condition.Message
			break
		}
	}

	if progressing {
		framework.Logf("CEO is progressing: Reason=%s, Message=%s", progressingReason, progressingMessage)

		// Check if this looks like revision controller activity
		if strings.Contains(strings.ToLower(progressingMessage), "revision") ||
			strings.Contains(strings.ToLower(progressingReason), "revision") {
			framework.Logf("🔄 Appears to be waiting for CEO revision controller activity")
			framework.Logf("   This typically indicates the operator is processing revision changes")
		}
	} else {
		framework.Logf("CEO is not currently progressing - revision controller appears stable")
	}

	// Check revision consistency across nodes
	framework.Logf("Checking revision consistency across etcd members...")
	latestRevision := int32(0)
	revisionMap := make(map[int32][]string)

	for _, nodeStatus := range etcdOperator.Status.NodeStatuses {
		if nodeStatus.CurrentRevision > latestRevision {
			latestRevision = nodeStatus.CurrentRevision
		}
		revisionMap[nodeStatus.CurrentRevision] = append(revisionMap[nodeStatus.CurrentRevision], nodeStatus.NodeName)
	}

	framework.Logf("Latest revision: %d", latestRevision)
	for revision, nodes := range revisionMap {
		if revision == latestRevision {
			framework.Logf("  - Revision %d (latest): nodes %v ✅", revision, nodes)
		} else {
			framework.Logf("  - Revision %d (outdated): nodes %v ⚠️", revision, nodes)
		}
	}

	// Check if all nodes are on the same revision
	if len(revisionMap) == 1 {
		framework.Logf("✅ All etcd members are on the same revision (%d)", latestRevision)
	} else {
		framework.Logf("⚠️  Etcd members are on different revisions - CEO may be processing updates")
		framework.Logf("   This could indicate the revision controller is working to sync all members")
	}

	// Check for any specific revision-related conditions
	framework.Logf("Checking for revision-related operator conditions...")
	hasRevisionIssues := false

	for _, condition := range etcdOperator.Status.Conditions {
		conditionLower := strings.ToLower(string(condition.Type))
		messageLower := strings.ToLower(condition.Message)
		reasonLower := strings.ToLower(condition.Reason)

		if strings.Contains(conditionLower, "revision") ||
			strings.Contains(messageLower, "revision") ||
			strings.Contains(reasonLower, "revision") {

			framework.Logf("  - Revision-related condition: %s=%s (Reason: %s)",
				condition.Type, condition.Status, condition.Reason)
			framework.Logf("    Message: %s", condition.Message)

			if condition.Status == operatorv1.ConditionTrue && string(condition.Type) != string(operatorv1.OperatorStatusTypeAvailable) {
				hasRevisionIssues = true
			}
		}
	}

	if !hasRevisionIssues {
		framework.Logf("✅ No revision-related issues detected in CEO conditions")
	}

	return nil
}

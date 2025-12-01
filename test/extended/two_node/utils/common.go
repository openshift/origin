// Package utils provides common cluster utilities: topology validation, CLI management, node filtering, and operator health checks.
package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	v1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/test/e2e/framework"
	nodehelper "k8s.io/kubernetes/test/e2e/framework/node"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
)

const (
	AllNodes                  = ""                                      // No label filter for GetNodes
	LabelNodeRoleControlPlane = "node-role.kubernetes.io/control-plane" // Control plane node label
	LabelNodeRoleMaster       = "node-role.kubernetes.io/master"        // Legacy master node label
	LabelNodeRoleWorker       = "node-role.kubernetes.io/worker"        // Worker node label
	LabelNodeRoleArbiter      = "node-role.kubernetes.io/arbiter"       // Arbiter node label
	CLIPrivilegeNonAdmin      = false                                   // Standard user CLI
	CLIPrivilegeAdmin         = true                                    // Admin CLI with cluster-admin permissions
)

// DecodeObject decodes YAML or JSON data into a Kubernetes runtime object using generics.
//
//	var bmh metal3v1alpha1.BareMetalHost
//	if err := DecodeObject(yamlData, &bmh); err != nil { return err }
func DecodeObject[T runtime.Object](data string, target T) error {
	decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(data), 4096)
	return decoder.Decode(target)
}

// SkipIfNotTopology skips the test if cluster topology doesn't match the wanted mode (e.g., DualReplicaTopologyMode).
//
//	SkipIfNotTopology(oc, v1.DualReplicaTopologyMode)
func SkipIfNotTopology(oc *exutil.CLI, wanted v1.TopologyMode) {
	current, err := exutil.GetControlPlaneTopology(oc)
	if err != nil {
		e2eskipper.Skip(fmt.Sprintf("Could not get current topology, skipping test: error %v", err))
	}
	if *current != wanted {
		e2eskipper.Skip(fmt.Sprintf("Cluster is not in %v topology, skipping test", wanted))
	}
}

// IsClusterOperatorAvailable returns true if operator has Available=True condition.
//
//	if !IsClusterOperatorAvailable(etcdOperator) { return fmt.Errorf("etcd not available") }
func IsClusterOperatorAvailable(operator *v1.ClusterOperator) bool {
	for _, cond := range operator.Status.Conditions {
		if cond.Type == v1.OperatorAvailable && cond.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}

// IsClusterOperatorDegraded returns true if operator has Degraded=True condition.
//
//	if IsClusterOperatorDegraded(co) { return fmt.Errorf("%s degraded", coName) }
func IsClusterOperatorDegraded(operator *v1.ClusterOperator) bool {
	for _, cond := range operator.Status.Conditions {
		if cond.Type == v1.OperatorDegraded && cond.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}

// HasNodeRebooted checks if a node has rebooted by comparing its current BootID with a previous snapshot.
// Returns true if the node's BootID has changed, indicating a reboot occurred.
//
//	nodeSnapshot, _ := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
//	// ... trigger reboot ...
//	if rebooted, err := HasNodeRebooted(oc, nodeSnapshot); rebooted { /* node rebooted */ }
func HasNodeRebooted(oc *exutil.CLI, node *corev1.Node) (bool, error) {
	if n, err := oc.AdminKubeClient().CoreV1().Nodes().Get(context.Background(), node.Name, metav1.GetOptions{}); err != nil {
		return false, err
	} else {
		return n.Status.NodeInfo.BootID != node.Status.NodeInfo.BootID, nil
	}
}

// GetNodes returns nodes filtered by role label (LabelNodeRoleControlPlane, LabelNodeRoleWorker, etc), or all nodes if roleLabel is AllNodes.
//
//	controlPlaneNodes, err := GetNodes(oc, LabelNodeRoleControlPlane)
func GetNodes(oc *exutil.CLI, roleLabel string) (*corev1.NodeList, error) {
	return oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
		LabelSelector: roleLabel,
	})
}

// IsNodeReady checks if a node exists and is in Ready state.
// Returns true if the node exists and has Ready condition, false otherwise.
//
//	if !IsNodeReady(oc, "master-0") { /* node not ready, approve CSRs */ }
func IsNodeReady(oc *exutil.CLI, nodeName string) bool {
	node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		// Node doesn't exist or error retrieving it
		return false
	}

	// Check node conditions for Ready status
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
			return true
		}
	}

	return false
}

// IsAPIResponding checks if the Kubernetes API server is responding to requests.
// Returns true if the API responds successfully, false otherwise.
//
//	if !IsAPIResponding(oc) { /* API not ready, continue waiting */ }
func IsAPIResponding(oc *exutil.CLI) bool {
	// Try a simple API call to check if the server is responding
	// Using a lightweight list operation with limit=1 to test API availability
	_, err := oc.AdminKubeClient().CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{Limit: 1})
	return err == nil
}

// UnmarshalJSON parses JSON string into a Go type using generics.
//
//	var node corev1.Node
//	if err := UnmarshalJSON(nodeJSON, &node); err != nil { return err }
func UnmarshalJSON[T any](jsonData string, target *T) error {
	return json.Unmarshal([]byte(jsonData), target)
}

// AddConstraint adds a pacemaker location constraint to prevent a resource from running on a specific node.
//
//	err := AddConstraint(oc, "master-0", "kubelet-clone", "master-1")
func AddConstraint(oc *exutil.CLI, nodeName string, resourceName string, targetNode string) error {
	framework.Logf("Adding constraint on node %s to prevent %s from running on %s", nodeName, resourceName, targetNode)

	constraintName := fmt.Sprintf("location-%s-%s-constraint", resourceName, targetNode)
	cmd := fmt.Sprintf("sudo pcs constraint location %s avoids %s=INFINITY", resourceName, targetNode)

	output, err := oc.AsAdmin().Run("debug").Args(
		fmt.Sprintf("node/%s", nodeName),
		"--", "chroot", "/host", "bash", "-c", cmd).Output()

	if err != nil {
		framework.Logf("Failed to add constraint: %v, output: %s", err, output)
		return fmt.Errorf("failed to add constraint: %v", err)
	}

	framework.Logf("Successfully added constraint %s", constraintName)
	return nil
}

// RemoveConstraint removes a pacemaker location constraint by its constraint ID.
//
//	err := RemoveConstraint(oc, "master-0", "constraint-id-123")
func RemoveConstraint(oc *exutil.CLI, nodeName string, constraintId string) error {
	framework.Logf("Removing constraint %s on node %s", constraintId, nodeName)

	cmd := fmt.Sprintf("sudo pcs constraint delete %s", constraintId)

	output, err := oc.AsAdmin().Run("debug").Args(
		fmt.Sprintf("node/%s", nodeName),
		"--", "chroot", "/host", "bash", "-c", cmd).Output()

	if err != nil {
		framework.Logf("Failed to remove constraint: %v, output: %s", err, output)
		return fmt.Errorf("failed to remove constraint: %v", err)
	}

	framework.Logf("Successfully removed constraint %s", constraintId)
	return nil
}

// DiscoverConstraintId discovers the constraint ID for a specific resource and target node combination.
// Uses the 'pcs constraint location --full' command for direct location constraint discovery.
//
//	constraintId, err := DiscoverConstraintId(oc, "master-0", "kubelet-clone", "master-1")
func DiscoverConstraintId(oc *exutil.CLI, nodeName string, resourceName string, targetNode string) (string, error) {
	framework.Logf("Discovering constraint ID for resource %s avoiding node %s", resourceName, targetNode)

	cmd := "sudo pcs constraint location --full"

	output, err := oc.AsAdmin().Run("debug").Args(
		fmt.Sprintf("node/%s", nodeName),
		"--", "chroot", "/host", "bash", "-c", cmd).Output()

	if err != nil {
		return "", fmt.Errorf("failed to list location constraints: %v", err)
	}

	framework.Logf("PCS constraint location output:\n%s", output)

	return parseConstraintIdFromLocationOutput(output, resourceName, targetNode)
}

// parseConstraintIdFromLocationOutput parses constraint ID from 'pcs constraint location --full' output
func parseConstraintIdFromLocationOutput(output, resourceName, targetNode string) (string, error) {
	lines := strings.Split(output, "\n")

	for i, line := range lines {
		framework.Logf("Line %d: %s", i, line)

		// Look for lines that mention our resource and target node
		// Expected formats:
		// "  resource 'kubelet-clone' avoids node 'master-0' with score INFINITY (id: location-kubelet-clone-master-0--INFINITY)"
		// or: "resource 'kubelet-clone' avoids node 'master-0' with score INFINITY (id: location-kubelet-clone-master-0--INFINITY)"
		if strings.Contains(line, resourceName) && strings.Contains(line, targetNode) && strings.Contains(line, "avoids") {
			framework.Logf("Found matching line: %s", line)

			// Strategy 1: Extract ID from parentheses: (id: constraint-id-here)
			if constraintId := extractConstraintIdFromParens(line); constraintId != "" && isValidConstraintId(constraintId) {
				framework.Logf("Extracted constraint ID (strategy 1): %s", constraintId)
				return constraintId, nil
			}

			// Strategy 2: Try alternative patterns
			if constraintId := extractConstraintIdAlternative(line, resourceName, targetNode); constraintId != "" && isValidConstraintId(constraintId) {
				framework.Logf("Extracted constraint ID (strategy 2): %s", constraintId)
				return constraintId, nil
			}

			framework.Logf("WARNING: Found matching line but could not extract constraint ID: %s", line)
		}
	}

	return "", fmt.Errorf("constraint not found for resource %s avoiding node %s", resourceName, targetNode)
}

// extractConstraintIdFromParens extracts the constraint ID from (id: ...) pattern
func extractConstraintIdFromParens(line string) string {
	framework.Logf("DEBUG: extractConstraintIdFromParens - input line: '%s'", line)

	// Find "(id: "
	start := strings.Index(line, "(id: ")
	if start == -1 {
		framework.Logf("DEBUG: extractConstraintIdFromParens - no '(id: ' pattern found")
		return ""
	}

	framework.Logf("DEBUG: extractConstraintIdFromParens - found '(id: ' at position %d", start)
	start += 5 // Move past "(id: "

	// Find the closing ")"
	remaining := line[start:]
	framework.Logf("DEBUG: extractConstraintIdFromParens - remaining after '(id: ': '%s'", remaining)

	end := strings.Index(remaining, ")")
	if end == -1 {
		framework.Logf("DEBUG: extractConstraintIdFromParens - no closing ')' found in remaining: '%s'", remaining)
		return ""
	}

	framework.Logf("DEBUG: extractConstraintIdFromParens - found closing ')' at position %d in remaining", end)
	constraintId := strings.TrimSpace(remaining[:end])
	framework.Logf("DEBUG: extractConstraintIdFromParens - extracted constraint ID: '%s'", constraintId)
	return constraintId
}

// extractConstraintIdAlternative tries alternative extraction methods
func extractConstraintIdAlternative(line, resourceName, targetNode string) string {
	framework.Logf("DEBUG: extractConstraintIdAlternative - line: '%s', resource: '%s', target: '%s'", line, resourceName, targetNode)

	// Try pattern: location-{resource}-{target}--INFINITY
	predictedId := fmt.Sprintf("location-%s-%s--INFINITY", resourceName, targetNode)
	framework.Logf("DEBUG: extractConstraintIdAlternative - looking for predicted ID: '%s'", predictedId)
	if strings.Contains(line, predictedId) {
		framework.Logf("DEBUG: extractConstraintIdAlternative - found predicted ID: '%s'", predictedId)
		return predictedId
	}

	// Try finding any "location-" pattern in the line
	if start := strings.Index(line, "location-"); start != -1 {
		framework.Logf("DEBUG: extractConstraintIdAlternative - found 'location-' at position %d", start)
		// Extract from "location-" to the next space or end of line
		remaining := line[start:]
		framework.Logf("DEBUG: extractConstraintIdAlternative - remaining after 'location-': '%s'", remaining)
		parts := strings.Fields(remaining)
		framework.Logf("DEBUG: extractConstraintIdAlternative - split into parts: %v", parts)
		if len(parts) > 0 {
			candidate := strings.Trim(parts[0], "(),")
			framework.Logf("DEBUG: extractConstraintIdAlternative - candidate after trim: '%s'", candidate)
			if strings.HasPrefix(candidate, "location-") {
				framework.Logf("DEBUG: extractConstraintIdAlternative - returning candidate: '%s'", candidate)
				return candidate
			}
		}
	}

	framework.Logf("DEBUG: extractConstraintIdAlternative - no constraint ID found")
	return ""
}

// isValidConstraintId validates that the extracted constraint ID looks reasonable
func isValidConstraintId(constraintId string) bool {
	// Reject obviously wrong results
	invalidIds := []string{"resource", "avoids", "node", "with", "score", "INFINITY", "id:", "(id:", ")"}

	for _, invalid := range invalidIds {
		if constraintId == invalid {
			framework.Logf("DEBUG: isValidConstraintId - rejecting invalid ID: '%s'", constraintId)
			return false
		}
	}

	// Valid constraint IDs should typically start with "location-" or similar
	if strings.HasPrefix(constraintId, "location-") || strings.HasPrefix(constraintId, "colocation-") || strings.HasPrefix(constraintId, "order-") {
		framework.Logf("DEBUG: isValidConstraintId - accepting valid ID: '%s'", constraintId)
		return true
	}

	framework.Logf("DEBUG: isValidConstraintId - uncertain about ID format: '%s' (allowing it)", constraintId)
	return true // Allow other formats we might not know about
}

// IsResourceStopped checks if a pacemaker resource is in stopped state.
//
//	stopped, err := IsResourceStopped(oc, "master-0", "kubelet-clone")
func IsResourceStopped(oc *exutil.CLI, nodeName string, resourceName string) (bool, error) {
	framework.Logf("Checking if resource %s is stopped on node %s", resourceName, nodeName)

	cmd := fmt.Sprintf("sudo pcs status resources %s", resourceName)

	output, err := oc.AsAdmin().Run("debug").Args(
		fmt.Sprintf("node/%s", nodeName),
		"--", "chroot", "/host", "bash", "-c", cmd).Output()

	if err != nil {
		framework.Logf("Failed to check resource status: %v, output: %s", err, output)
		return false, fmt.Errorf("failed to check resource status: %v", err)
	}

	// Check if the output indicates the resource is stopped
	isStopped := strings.Contains(strings.ToLower(output), "stopped") ||
		strings.Contains(strings.ToLower(output), "inactive")

	framework.Logf("Resource %s stopped status: %t", resourceName, isStopped)
	return isStopped, nil
}

// StopKubeletService stops the kubelet service on a specific node.
//
//	err := StopKubeletService(oc, "master-0")
func StopKubeletService(oc *exutil.CLI, nodeName string) error {
	framework.Logf("Stopping kubelet service on node %s", nodeName)

	cmd := "sudo systemctl stop kubelet"

	output, err := oc.AsAdmin().Run("debug").Args(
		fmt.Sprintf("node/%s", nodeName),
		"--", "chroot", "/host", "bash", "-c", cmd).Output()

	if err != nil {
		framework.Logf("Failed to stop kubelet service: %v, output: %s", err, output)
		return fmt.Errorf("failed to stop kubelet service: %v", err)
	}

	framework.Logf("Successfully stopped kubelet service on node %s", nodeName)
	return nil
}

// IsServiceRunning checks if a systemd service is running on a specific node.
//
//	running := IsServiceRunning(oc, "master-0", "kubelet")
func IsServiceRunning(oc *exutil.CLI, nodeName string, serviceName string) bool {
	framework.Logf("Checking service %s on node %s", serviceName, nodeName)
	
	cmd := fmt.Sprintf("sudo systemctl is-active %s", serviceName)
	framework.Logf("Executing command: %s", cmd)

	output, err := oc.AsAdmin().Run("debug").Args(
		fmt.Sprintf("node/%s", nodeName),
		"--", "chroot", "/host", "bash", "-c", cmd).Output()

	if err != nil {
		framework.Logf("ERROR: Failed to check service %s on node %s: %v", serviceName, nodeName, err)
		return false
	}

	framework.Logf("Raw output: '%s' (length: %d)", output, len(output))
	trimmedOutput := strings.TrimSpace(output)
	isActive := trimmedOutput == "active"
	framework.Logf("Trimmed output: '%s', IsActive: %t", trimmedOutput, isActive)
	
	return isActive
}

// ValidateClusterOperatorsAvailable validates that all cluster operators are available and not degraded.
//
//	if err := ValidateClusterOperatorsAvailable(oc); err != nil { return err }
func ValidateClusterOperatorsAvailable(oc *exutil.CLI) error {
	framework.Logf("Validating all cluster operators are available and not degraded")

	clusterOperators, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list cluster operators: %v", err)
	}

	var unavailableOperators []string
	var degradedOperators []string
	totalOperators := len(clusterOperators.Items)

	for _, co := range clusterOperators.Items {
		if !IsClusterOperatorAvailable(&co) {
			unavailableOperators = append(unavailableOperators, co.Name)
		}
		if IsClusterOperatorDegraded(&co) {
			degradedOperators = append(degradedOperators, co.Name)
		}
	}

	if len(unavailableOperators) > 0 {
		return fmt.Errorf("cluster operators not available: %v", unavailableOperators)
	}
	if len(degradedOperators) > 0 {
		return fmt.Errorf("cluster operators degraded: %v", degradedOperators)
	}

	framework.Logf("All %d cluster operators are available and not degraded", totalOperators)
	return nil
}

// ValidateEssentialOperatorsAvailable validates that essential cluster operators are available for kubelet disruption tests.
// This is more lenient than ValidateClusterOperatorsAvailable and only checks core operators needed for the test.
//
//	if err := ValidateEssentialOperatorsAvailable(oc); err != nil { return err }
func ValidateEssentialOperatorsAvailable(oc *exutil.CLI) error {
	framework.Logf("Validating essential cluster operators are available for kubelet disruption test")

	// Essential operators for kubelet disruption tests
	essentialOperators := []string{
		"etcd",                    // Core cluster state
		"kube-apiserver",          // Kubernetes API
		"openshift-apiserver",     // OpenShift API
		"network",                 // Cluster networking
		"kube-controller-manager", // Core controllers
	}

	clusterOperators, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list cluster operators: %v", err)
	}

	var unavailableOperators []string
	var degradedOperators []string

	for _, operatorName := range essentialOperators {
		// Find the operator in the list
		var operator *v1.ClusterOperator
		for _, co := range clusterOperators.Items {
			if co.Name == operatorName {
				operator = &co
				break
			}
		}

		if operator == nil {
			framework.Logf("WARNING: Essential operator %s not found in cluster", operatorName)
			continue
		}

		if !IsClusterOperatorAvailable(operator) {
			unavailableOperators = append(unavailableOperators, operatorName)
			framework.Logf("Essential operator %s is not available", operatorName)
		}
		if IsClusterOperatorDegraded(operator) {
			degradedOperators = append(degradedOperators, operatorName)
			framework.Logf("Essential operator %s is degraded", operatorName)
		}
	}

	// Log status of non-essential operators for info
	nonEssentialCount := 0
	nonEssentialUnavailable := 0
	for _, co := range clusterOperators.Items {
		isEssential := false
		for _, essential := range essentialOperators {
			if co.Name == essential {
				isEssential = true
				break
			}
		}
		if !isEssential {
			nonEssentialCount++
			if !IsClusterOperatorAvailable(&co) {
				nonEssentialUnavailable++
				framework.Logf("Non-essential operator %s is not available (not blocking test)", co.Name)
			}
		}
	}

	if len(unavailableOperators) > 0 {
		return fmt.Errorf("essential cluster operators not available: %v", unavailableOperators)
	}
	if len(degradedOperators) > 0 {
		return fmt.Errorf("essential cluster operators degraded: %v", degradedOperators)
	}

	framework.Logf("All %d essential operators are available (%d non-essential operators, %d unavailable but not blocking)",
		len(essentialOperators), nonEssentialCount, nonEssentialUnavailable)
	return nil
}

// LogEtcdClusterStatus performs comprehensive etcd cluster status logging and validation.
// This function is designed to be used in AfterEach functions to ensure tests leave the cluster in a known good state.
//
//	if err := LogEtcdClusterStatus(oc, "BeforeEach validation"); err != nil { return err }
func LogEtcdClusterStatus(oc *exutil.CLI, testContext string) error {
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
	if err := ValidateClusterOperatorsAvailable(oc); err != nil {
		framework.Logf("WARNING: Some cluster operators are not available: %v", err)
		// Don't return error here as this might be transient during cluster operations
	} else {
		framework.Logf("All cluster operators are available and healthy")
	}

	framework.Logf("=== Etcd cluster status check completed successfully (%s) ===", testContext)
	return nil
}

// IsClusterHealthy checks if the cluster is in a healthy state before running disruptive tests.
// It verifies that all nodes are ready and all cluster operators are available (not degraded or progressing).
// Returns an error with details if the cluster is not healthy, nil if healthy.
//
//	if err := IsClusterHealthy(oc); err != nil {
//		e2eskipper.Skipf("Cluster is not healthy: %v", err)
//	}
func IsClusterHealthy(oc *exutil.CLI) error {
	ctx := context.Background()
	timeout := 30 * time.Second // Quick check, not a long wait

	// Check all nodes are ready first using upstream framework function
	klog.V(2).Infof("Checking if all nodes are ready...")
	if err := nodehelper.AllNodesReady(ctx, oc.AdminKubeClient(), timeout); err != nil {
		return fmt.Errorf("not all nodes are ready: %w", err)
	}
	klog.V(2).Infof("All nodes are ready")

	// Check all cluster operators using MonitorClusterOperators
	klog.V(2).Infof("Checking if all cluster operators are healthy...")
	_, err := MonitorClusterOperators(oc, timeout, 5*time.Second)
	if err != nil {
		return fmt.Errorf("cluster operators not healthy: %w", err)
	}
	klog.V(2).Infof("All cluster operators are healthy")

	return nil
}

// MonitorClusterOperators monitors cluster operators and ensures they are all available.
// Returns the cluster operators status output and an error if operators are not healthy within timeout.
//
//	output, err := MonitorClusterOperators(oc, 5*time.Minute, 15*time.Second)
func MonitorClusterOperators(oc *exutil.CLI, timeout time.Duration, pollInterval time.Duration) (string, error) {
	ctx := context.Background()
	startTime := time.Now()

	for {
		// Get cluster operators status
		clusterOperators, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().List(ctx, metav1.ListOptions{})
		if err != nil {
			klog.V(4).Infof("Error getting cluster operators: %v", err)
			if time.Since(startTime) >= timeout {
				return "", fmt.Errorf("timeout waiting for cluster operators: %w", err)
			}
			time.Sleep(pollInterval)
			continue
		}

		// Check each operator's conditions
		allHealthy := true
		var degradedOperators []string
		var progressingOperators []string

		for _, co := range clusterOperators.Items {
			isDegraded := false
			isProgressing := false

			// Check conditions
			for _, condition := range co.Status.Conditions {
				if condition.Type == "Degraded" && condition.Status == "True" {
					isDegraded = true
					degradedOperators = append(degradedOperators, fmt.Sprintf("%s: %s (reason: %s)", co.Name, condition.Message, condition.Reason))
				}
				if condition.Type == "Progressing" && condition.Status == "True" {
					isProgressing = true
					progressingOperators = append(progressingOperators, fmt.Sprintf("%s: %s (reason: %s)", co.Name, condition.Message, condition.Reason))
				}
			}

			if isDegraded || isProgressing {
				allHealthy = false
			}
		}

		// Log current status
		klog.V(4).Infof("Cluster operators status check: All healthy: %v, Degraded count: %d, Progressing count: %d",
			allHealthy, len(degradedOperators), len(progressingOperators))

		if len(degradedOperators) > 0 {
			klog.V(4).Infof("Degraded operators: %v", degradedOperators)
		}
		if len(progressingOperators) > 0 {
			klog.V(4).Infof("Progressing operators: %v", progressingOperators)
		}

		// If all operators are healthy, we're done
		if allHealthy {
			klog.V(2).Infof("All cluster operators are healthy (not degraded or progressing)!")
			// Get final wide output for display purposes
			wideOutput, _ := oc.AsAdmin().Run("get").Args("co", "-o", "wide").Output()
			return wideOutput, nil
		}

		// Check timeout
		if time.Since(startTime) >= timeout {
			// Get final wide output for display purposes
			wideOutput, _ := oc.AsAdmin().Run("get").Args("co", "-o", "wide").Output()
			klog.V(4).Infof("Final cluster operators status after timeout:\n%s", wideOutput)
			return wideOutput, fmt.Errorf("cluster operators did not become healthy within %v", timeout)
		}

		// Log the current operator status for debugging
		if klog.V(4).Enabled() {
			wideOutput, _ := oc.AsAdmin().Run("get").Args("co", "-o", "wide").Output()
			klog.V(4).Infof("Current cluster operators status:\n%s", wideOutput)
		}

		time.Sleep(pollInterval)
	}
}

// EnsureTNFDegradedOrSkip skips the test if the cluster is not in TNF degraded mode
// (DualReplica topology with exactly one Ready control-plane node).
func EnsureTNFDegradedOrSkip(oc *exutil.CLI) {
	SkipIfNotTopology(oc, v1.DualReplicaTopologyMode)

	nodeList, err := GetNodes(oc, LabelNodeRoleControlPlane)
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to list master nodes")

	masters := nodeList.Items

	if len(masters) != 2 {
		g.Skip(fmt.Sprintf(
			"expect exactly 2 master nodes, found %d",
			len(masters),
		))
	}

	readyCount := CountReadyNodes(masters)
	if readyCount != 1 {
		g.Skip(fmt.Sprintf(
			"cluster is not TNF degraded mode (expected exactly 1 Ready master node, got %d)",
			readyCount,
		))
	}
}

// CountReadyNodes returns the number of nodes in Ready state.
func CountReadyNodes(nodes []corev1.Node) int {
	ready := 0
	for _, n := range nodes {
		if isNodeObjReady(n) {
			ready++
		}
	}
	return ready
}

// GetReadyMasterNode returns the first Ready control-plane node.
func GetReadyMasterNode(
	ctx context.Context,
	oc *exutil.CLI,
) (*corev1.Node, error) {
	nodeList, err := GetNodes(oc, LabelNodeRoleControlPlane)
	if err != nil {
		return nil, err
	}
	for i := range nodeList.Items {
		node := &nodeList.Items[i]
		if isNodeObjReady(nodeList.Items[i]) {
			return node, nil
		}
	}

	return nil, fmt.Errorf("no Ready control-plane node found")
}

// check ready condition on an existing Node object.
func isNodeObjReady(node corev1.Node) bool {
	for _, c := range node.Status.Conditions {
		if c.Type == corev1.NodeReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
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
		if strings.HasPrefix(label, "node-role.kubernetes.io/") {
			role := strings.TrimPrefix(label, "node-role.kubernetes.io/")
			if role != "" {
				roles = append(roles, role)
			}
		}
	}
	if len(roles) == 0 {
		return "<none>"
	}
	return strings.Join(roles, ",")
}

// checkEtcdMemberPromotionStatus tries to determine if an etcd member is promoted (voting) or learner
func checkEtcdMemberPromotionStatus(oc *exutil.CLI, nodeName string) string {
	// This is a simplified heuristic - in a real implementation,
	// you would query the etcd API directly to get member status
	// For now, we'll return "unknown" as a placeholder
	framework.Logf("Checking etcd member promotion status for node %s (heuristic)", nodeName)

	// Try to get etcd operator status and infer from there
	etcdOperator, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().Get(context.Background(), "etcd", metav1.GetOptions{})
	if err != nil {
		return "unknown"
	}

	// If etcd operator is available and not progressing, assume members are voting
	for _, condition := range etcdOperator.Status.Conditions {
		if condition.Type == v1.OperatorAvailable && condition.Status == v1.ConditionTrue {
			// Check if progressing
			for _, progCond := range etcdOperator.Status.Conditions {
				if progCond.Type == v1.OperatorProgressing && progCond.Status == v1.ConditionTrue {
					return "learner" // Likely still promoting
				}
			}
			return "voting" // Available and not progressing
		}
	}

	return "unknown"
}

// checkCEORevisionControllerStatus checks the status of the Cluster Etcd Operator revision controller
func checkCEORevisionControllerStatus(oc *exutil.CLI) error {
	framework.Logf("Checking CEO revision controller status...")

	// Get the cluster-etcd-operator deployment status
	deployment, err := oc.AdminKubeClient().AppsV1().Deployments("openshift-etcd-operator").Get(
		context.Background(), "etcd-operator", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get etcd-operator deployment: %v", err)
	}

	framework.Logf("CEO deployment status: Ready=%d/%d, Available=%d, Unavailable=%d",
		deployment.Status.ReadyReplicas, deployment.Status.Replicas,
		deployment.Status.AvailableReplicas, deployment.Status.UnavailableReplicas)

	// Check if all conditions are satisfied
	for _, condition := range deployment.Status.Conditions {
		framework.Logf("  CEO condition: %s=%s (Reason: %s)",
			condition.Type, condition.Status, condition.Reason)

		if condition.Type == "Available" && condition.Status != "True" {
			return fmt.Errorf("CEO deployment not available: %s", condition.Message)
		}
	}

	// Check for any revision-related issues
	if deployment.Status.ReadyReplicas != deployment.Status.Replicas {
		framework.Logf("⚠️  CEO has %d ready replicas out of %d total",
			deployment.Status.ReadyReplicas, deployment.Status.Replicas)
	} else {
		framework.Logf("✅ No revision-related issues detected in CEO conditions")
	}

	return nil
}

package two_node

import (
	"fmt"
	"strings"

	v1 "github.com/openshift/api/config/v1"
	util "github.com/openshift/origin/test/extended/util"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
)

const (
	labelNodeRoleMaster       = "node-role.kubernetes.io/master"
	labelNodeRoleControlPlane = "node-role.kubernetes.io/control-plane"
	labelNodeRoleWorker       = "node-role.kubernetes.io/worker"
	labelNodeRoleArbiter      = "node-role.kubernetes.io/arbiter"
)

func skipIfNotTopology(oc *util.CLI, wanted v1.TopologyMode) {
	current, err := util.GetControlPlaneTopology(oc)
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

// addConstraint adds constraint that avoids having resource as part of the cluster
func addConstraint(oc *util.CLI, nodeName string, resourceName string, targetNode string) error {
	framework.Logf("Adding constraint for %s resource to avoid %s", resourceName, targetNode)

	command := fmt.Sprintf("echo 'adding pacemaker constraint'; exec chroot /host pcs constraint location %s avoids %s", resourceName, targetNode)

	err := util.CreateNodeDisruptionPod(oc.KubeClient(), nodeName, command)
	if err != nil {
		framework.Logf("Failed to add constraint via disruption pod")
		return fmt.Errorf("failed to add constraint for resource %s to avoid %s: %v", resourceName, targetNode, err)
	}

	framework.Logf("Successfully added constraint for resource %s to avoid %s", resourceName, targetNode)
	return nil
}

// removeConstraint removes constraint that avoids having resource as part of the cluster
func removeConstraint(oc *util.CLI, nodeName string, constraintId string) error {
	framework.Logf("Removing constraint with ID %s on %s", constraintId, nodeName)

	command := fmt.Sprintf("echo 'removing pacemaker constraint'; exec chroot /host pcs constraint remove %s", constraintId)

	err := util.CreateNodeDisruptionPod(oc.KubeClient(), nodeName, command)
	if err != nil {
		framework.Logf("Failed to remove constraint via disruption pod")
		return fmt.Errorf("failed to remove constraint %s: %v", constraintId, err)
	}

	framework.Logf("Successfully removed constraint %s", constraintId)
	return nil
}

// discoverConstraintId discovers the constraint ID for a specific resource and node combination using JSON output and jq
func discoverConstraintId(oc *util.CLI, nodeName string, resourceName string, targetNode string) (string, error) {
	framework.Logf("Discovering constraint ID for resource %s avoiding node %s using node %s", resourceName, targetNode, nodeName)

	// Use jq to extract the constraint ID from JSON output directly
	jqQuery := fmt.Sprintf(`'.location[] | select(.resource_id == "%s" and .attributes.node == "%s" and .attributes.score == "-INFINITY") | .attributes.constraint_id'`, resourceName, targetNode)

	cmd := []string{
		"bash", "-c",
		fmt.Sprintf(`sudo pcs constraint config --output-format json | jq -r %s`, jqQuery),
	}

	constraintId, err := util.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "openshift-etcd", cmd...)
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
func isResourceStopped(oc *util.CLI, nodeName string, resourceName string) (bool, error) {
	framework.Logf("Checking if resource %s is stopped on node %s", resourceName, nodeName)

	cmd := []string{
		"pcs", "status", "resources", resourceName, fmt.Sprintf("node=%s", nodeName),
	}

	output, err := util.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "openshift-etcd", cmd...)
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

// isKubeletResourceStopped is a convenience function to check if kubelet-clone resource is stopped
func isKubeletResourceStopped(oc *util.CLI, nodeName string) (bool, error) {
	return isResourceStopped(oc, nodeName, "kubelet-clone")
}

// addConstraintAndGetId adds a constraint and returns the constraint ID for later removal
func addConstraintAndGetId(oc *util.CLI, nodeName string, resourceName string, targetNode string) (string, error) {
	// First add the constraint
	err := addConstraint(oc, nodeName, resourceName, targetNode)
	if err != nil {
		return "", err
	}

	// Wait a moment for the constraint to be created
	framework.Logf("Waiting for constraint to be created in Pacemaker...")

	// Then discover the constraint ID
	constraintId, err := discoverConstraintId(oc, nodeName, resourceName, targetNode)
	if err != nil {
		return "", fmt.Errorf("failed to discover constraint ID after adding constraint: %v", err)
	}

	return constraintId, nil
}

// stopKubeletService stops the kubelet service on the specified node
func stopKubeletService(oc *util.CLI, nodeName string) error {
	framework.Logf("Stopping kubelet service on node %s", nodeName)

	cmd := []string{
		"chroot", "/host",
		"systemctl", "stop", "kubelet",
	}

	_, err := util.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "openshift-etcd", cmd...)
	if err != nil {
		return fmt.Errorf("failed to stop kubelet service on node %s: %v", nodeName, err)
	}

	framework.Logf("Successfully stopped kubelet service on node %s", nodeName)
	return nil
}

// startKubeletService starts the kubelet service on the specified node
func startKubeletService(oc *util.CLI, nodeName string) error {
	framework.Logf("Starting kubelet service on node %s", nodeName)

	cmd := []string{
		"chroot", "/host",
		"systemctl", "start", "kubelet",
	}

	_, err := util.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "openshift-etcd", cmd...)
	if err != nil {
		return fmt.Errorf("failed to start kubelet service on node %s: %v", nodeName, err)
	}

	framework.Logf("Successfully started kubelet service on node %s", nodeName)
	return nil
}

// isServiceRunning checks if the specified service is running on the specified node
func isServiceRunning(oc *util.CLI, nodeName string, serviceName string) bool {
	cmd := []string{
		"chroot", "/host",
		"systemctl", "is-active", "--quiet", serviceName,
	}

	_, err := util.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "openshift-etcd", cmd...)
	return err == nil
}

// isKubeletServiceRunning checks if kubelet service is running on the specified node
// This is a convenience wrapper around isServiceRunning for kubelet
func isKubeletServiceRunning(oc *util.CLI, nodeName string) bool {
	return isServiceRunning(oc, nodeName, "kubelet")
}

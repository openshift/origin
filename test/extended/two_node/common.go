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
func addConstraint(oc *util.CLI, nodeName string, resourceName string) error {
	framework.Logf("Avoiding %s resource on %s", resourceName, nodeName)

	cmd := []string{
		"chroot", "/host",
		"pcs", "constraint", "location", resourceName, "avoids", "master-1",
	}

	_, err := util.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "openshift-etcd", cmd...)
	if err != nil {
		return fmt.Errorf("failed to avoid resource %s on node %s: %v", resourceName, nodeName, err)
	}

	framework.Logf("Successfully added constraint to avoid resource %s on node %s", resourceName, nodeName)
	return nil
}

// removeConstraint removes constraint that avoids having resource as part of the cluster
func removeConstraint(oc *util.CLI, nodeName string, constraintId string) error {
	framework.Logf("Removing constraint to avoid %s resource on %s", constraintId, nodeName)

	cmd := []string{
		"chroot", "/host",
		"pcs", "constraint", "remove", constraintId,
	}

	_, err := util.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "openshift-etcd", cmd...)
	if err != nil {
		return fmt.Errorf("failed to remove constraint to avoid resource %s on node %s: %v", constraintId, nodeName, err)
	}

	framework.Logf("Successfully removed constraint to avoid resource %s on node %s", constraintId, nodeName)
	return nil
}

// discoverConstraintId discovers the constraint ID for a specific resource and node combination
func discoverConstraintId(oc *util.CLI, nodeName string, resourceName string, targetNode string) (string, error) {
	framework.Logf("Discovering constraint ID for resource %s avoiding node %s", resourceName, targetNode)

	cmd := []string{
		"chroot", "/host",
		"pcs", "constraint", "list", "--full",
	}

	output, err := util.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "openshift-etcd", cmd...)
	if err != nil {
		return "", fmt.Errorf("failed to list constraints on node %s: %v", nodeName, err)
	}

	// Parse the output to find the constraint ID
	// Example output line: "Location Constraints:\n  Resource: kubelet-clone\n    Constraint: location-kubelet-clone-master-1-INFINITY\n      Avoid: master-1 (score:-INFINITY)"
	lines := strings.Split(output, "\n")

	var constraintId string
	resourceFound := false

	for i, line := range lines {
		line = strings.TrimSpace(line)

		// Look for the resource line
		if strings.HasPrefix(line, "Resource: "+resourceName) {
			resourceFound = true
			continue
		}

		// If we found the resource, look for the constraint line
		if resourceFound && strings.HasPrefix(line, "Constraint: ") {
			constraintId = strings.TrimPrefix(line, "Constraint: ")

			// Check if this constraint affects the target node
			if i+1 < len(lines) {
				nextLine := strings.TrimSpace(lines[i+1])
				if strings.Contains(nextLine, "Avoid: "+targetNode) {
					framework.Logf("Found constraint ID: %s for resource %s avoiding node %s", constraintId, resourceName, targetNode)
					return constraintId, nil
				}
			}
		}

		// Reset if we hit a new resource section
		if strings.HasPrefix(line, "Resource: ") && !strings.HasPrefix(line, "Resource: "+resourceName) {
			resourceFound = false
		}
	}

	return "", fmt.Errorf("constraint ID not found for resource %s avoiding node %s", resourceName, targetNode)
}

// addConstraintAndGetId adds a constraint and returns the constraint ID for later removal
func addConstraintAndGetId(oc *util.CLI, nodeName string, resourceName string, targetNode string) (string, error) {
	// First add the constraint
	err := addConstraint(oc, nodeName, resourceName)
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

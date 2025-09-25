package two_node

import (
	"fmt"

	v1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/test/extended/util"
	exutil "github.com/openshift/origin/test/extended/util"
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
func removeConstraint(oc *util.CLI, nodeName string, resourceName string) error {
	framework.Logf("Removing constraint to avoid %s resource on %s", resourceName, nodeName)

	cmd := []string{
		"chroot", "/host",
		"pcs", "constraint", "location", resourceName, "avoids", "master-1",
	}

	_, err := util.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "openshift-etcd", cmd...)
	if err != nil {
		return fmt.Errorf("failed to remove constraint to avoid resource %s on node %s: %v", resourceName, nodeName, err)
	}

	framework.Logf("Successfully removed constraint to avoid resource %s on node %s", resourceName, nodeName)
	return nil
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

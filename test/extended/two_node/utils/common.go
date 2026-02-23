// Package utils provides common cluster utilities: topology validation, CLI management, node filtering, and operator health checks.
package utils

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net"
	"slices"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	v1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/pkg/test/preconditions"
	"github.com/openshift/origin/test/extended/etcd/helpers"
	"github.com/openshift/origin/test/extended/two_node/utils/services"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/pkg/errors"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/kubectl/pkg/util/podutils"
	nodeutil "k8s.io/kubernetes/pkg/util/node"
	"k8s.io/kubernetes/test/e2e/framework"
	nodehelper "k8s.io/kubernetes/test/e2e/framework/node"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
)

const (
	AllNodes                  = ""                                      // No label filter for GetNodes
	LabelNodeRoleControlPlane = "node-role.kubernetes.io/control-plane" // Control plane node label
	LabelNodeRoleWorker       = "node-role.kubernetes.io/worker"        // Worker node label
	LabelNodeRoleArbiter      = "node-role.kubernetes.io/arbiter"       // Arbiter node label
	CLIPrivilegeNonAdmin      = false                                   // Standard user CLI
	CLIPrivilegeAdmin         = true                                    // Admin CLI with cluster-admin permissions
	KubeletPort               = "10250"                                 // Kubelet API port

	// Common timeouts used across TNF tests
	clusterIsHealthyTimeout = 15 * time.Minute
	debugContainerTimeout   = 60 * time.Second

	// Common poll intervals used across TNF tests
	FiveSecondPollInterval   = 5 * time.Second  // Default poll interval for most operations
	ThirtySecondPollInterval = 30 * time.Second // Poll interval for longer operations (VM state, provisioning)

	// SecretRecreationTimeout is the time to wait for operator to recreate secrets
	SecretRecreationTimeout = 5 * time.Minute
	// SecretRecreationInterval is the poll interval for secret recreation checks
	SecretRecreationInterval = 5 * time.Second
	// Pacemaker timestamp format for parsing operation history
	pacemakerTimeFormat = "Mon Jan 2 15:04:05 2006"
)

// Minimal XML types for parsing "pcs status xml" node history.
// Used to detect recent resource failures via operation history.
type pcsStatusResult struct {
	XMLName     xml.Name       `xml:"pacemaker-result"`
	NodeHistory pcsNodeHistory `xml:"node_history"`
}

type pcsNodeHistory struct {
	Node []pcsNodeHistoryNode `xml:"node"`
}

type pcsNodeHistoryNode struct {
	Name            string               `xml:"name,attr"`
	ResourceHistory []pcsResourceHistory `xml:"resource_history"`
}

type pcsResourceHistory struct {
	ID               string                `xml:"id,attr"`
	OperationHistory []pcsOperationHistory `xml:"operation_history"`
}

// pcsOperationHistory tracks resource operation results for failure detection.
type pcsOperationHistory struct {
	Task         string `xml:"task,attr"`
	RC           string `xml:"rc,attr"`
	RCText       string `xml:"rc_text,attr"`
	LastRCChange string `xml:"last-rc-change,attr"`
}

// DecodeObject decodes YAML or JSON data into a Kubernetes runtime object using generics.
//
//	var bmh metal3v1alpha1.BareMetalHost
//	if err := DecodeObject(yamlData, &bmh); err != nil { return err }
func DecodeObject[T runtime.Object](data string, target T) error {
	decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(data), 4096)
	return decoder.Decode(target)
}

// SkipIfNotTopology skips the test if cluster topology doesn't match the wanted mode.
// API errors or empty topology trigger precondition failure; mismatches are valid skips.
//
//	SkipIfNotTopology(oc, v1.DualReplicaTopologyMode)
func SkipIfNotTopology(oc *exutil.CLI, wanted v1.TopologyMode) {
	framework.Logf("%s", preconditions.RecordCheck("validating cluster topology is %s", wanted))

	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		// API errors are precondition failures - the cluster should be accessible
		e2eskipper.Skip(preconditions.FormatSkipMessage(fmt.Sprintf("failed to get Infrastructure resource: %v", err)))
	}

	current := infra.Status.ControlPlaneTopology

	if current == "" {
		// Empty topology is a precondition violation - the cluster should have a valid topology set
		e2eskipper.Skip(preconditions.FormatSkipMessage("Infrastructure.Status.ControlPlaneTopology is empty - cluster may not be properly configured"))
	}

	// Topology mismatch is a valid skip - test doesn't apply to this cluster type
	// This is NOT a precondition failure, just a test that doesn't apply to this topology
	if current != wanted {
		e2eskipper.Skipf("Test requires %v topology, but cluster has %v topology", wanted, current)
	}
}

// SkipIfClusterIsNotHealthy skips the test if cluster health checks fail.
// Performs comprehensive validation: all nodes ready, all cluster operators healthy,
// etcd pods running, two voting etcd members, cluster-etcd-operator healthy, and
// API services available (no stale GroupVersions).
// Skips are detected by the test runner via preconditions.SkipMarker for JUnit generation.
//
//	SkipIfClusterIsNotHealthy(oc, etcdClientFactory)
func SkipIfClusterIsNotHealthy(oc *exutil.CLI, ecf *helpers.EtcdClientFactoryImpl) {
	framework.Logf("%s", preconditions.RecordCheck("validating cluster health"))

	var skipReasons []string

	// Check API discovery health first - StaleGroupVersionError indicates API server issues
	if err := ensureAPIDiscoveryHealthy(oc); err != nil {
		skipReasons = append(skipReasons, fmt.Sprintf("API discovery unhealthy: %v", err))
	}

	// Fetch nodes - failure to query the API is a precondition failure
	nodes, err := GetNodes(oc, AllNodes)
	if err != nil {
		skipReasons = append(skipReasons, fmt.Sprintf("failed to get nodes: %v", err))
	} else if len(nodes.Items) != 2 {
		skipReasons = append(skipReasons, fmt.Sprintf("expected 2 nodes for two-node cluster, found %d", len(nodes.Items)))
	}

	if err := IsClusterHealthyWithTimeout(oc, clusterIsHealthyTimeout); err != nil {
		skipReasons = append(skipReasons, fmt.Sprintf("cluster-wide health failed: %v", err))
	}
	if err := ensureEtcdPodsAreRunning(oc); err != nil {
		skipReasons = append(skipReasons, fmt.Sprintf("etcd pods not running: %v", err))
	}
	// Only check etcd members if we successfully retrieved nodes
	if nodes != nil && len(nodes.Items) == 2 {
		if err := ensureEtcdHasTwoVotingMembers(nodes, ecf); err != nil {
			skipReasons = append(skipReasons, fmt.Sprintf("etcd doesn't have two voting members: %v", err))
		}
	}
	if err := ensureClusterOperatorHealthy(oc); err != nil {
		skipReasons = append(skipReasons, fmt.Sprintf("cluster-etcd-operator not healthy: %v", err))
	}

	if len(skipReasons) > 0 {
		e2eskipper.Skip(preconditions.FormatSkipMessage(strings.Join(skipReasons, "; ")))
	}
}

// ensureAPIDiscoveryHealthy verifies that API server discovery is working correctly.
// This catches StaleGroupVersionError issues where aggregated API servers are unavailable.
func ensureAPIDiscoveryHealthy(oc *exutil.CLI) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Use ServerGroups which exercises the discovery layer and will return
	// StaleGroupVersionError if any API group version is stale
	_, err := oc.AdminKubeClient().Discovery().ServerGroups()
	if err != nil {
		return fmt.Errorf("API server discovery failed: %w", err)
	}

	// Additionally verify key OpenShift APIs are responsive by making simple list calls
	// These exercise the aggregated API layer for OpenShift-specific APIs
	_, err = oc.AdminConfigClient().ConfigV1().ClusterOperators().List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil {
		return fmt.Errorf("OpenShift config API unavailable: %w", err)
	}

	return nil
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

// GetNodes returns nodes filtered by role label (LabelNodeRoleControlPlane, LabelNodeRoleWorker, etc), or all nodes if roleLabel is AllNodes.
// This is the preferred method for retrieving nodes in tests instead of calling the Kubernetes API directly.
// It provides a consistent abstraction and single point of change for node retrieval logic.
//
//	allNodes, err := GetNodes(oc, AllNodes)
//	controlPlaneNodes, err := GetNodes(oc, LabelNodeRoleControlPlane)
//	workerNodes, err := GetNodes(oc, LabelNodeRoleWorker)
func GetNodes(oc *exutil.CLI, roleLabel string) (*corev1.NodeList, error) {
	return oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
		LabelSelector: roleLabel,
	})
}

// GetNodeInternalIP returns the internal IP address of a node, or empty string if not found.
//
//	nodeIP := GetNodeInternalIP(&node)
//	if nodeIP == "" { return fmt.Errorf("no internal IP for node %s", node.Name) }
func GetNodeInternalIP(node *corev1.Node) string {
	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			return addr.Address
		}
	}
	return ""
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

// TryPacemakerCleanup clears failed pacemaker resource and STONITH states before health checks.
// Finds a Ready control-plane node and runs 'pcs resource cleanup' and 'pcs stonith cleanup'.
// This is a best-effort operation - failures are logged but don't cause errors.
func TryPacemakerCleanup(oc *exutil.CLI) {
	framework.Logf("Attempting pacemaker cleanup...")

	nodes, err := GetNodes(oc, LabelNodeRoleControlPlane)
	if err != nil {
		framework.Logf("WARNING: Failed to get control-plane nodes: %v", err)
		return
	}
	if len(nodes.Items) == 0 {
		framework.Logf("WARNING: No control-plane nodes found")
		return
	}

	var readyNode *corev1.Node
	for i := range nodes.Items {
		node := &nodes.Items[i]
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
				readyNode = node
				break
			}
		}
		if readyNode != nil {
			break
		}
	}
	if readyNode == nil {
		framework.Logf("WARNING: No Ready control-plane node found")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), debugContainerTimeout)
	defer cancel()

	if status, err := services.PcsStatusViaDebug(ctx, oc, readyNode.Name); err == nil {
		framework.Logf("Pacemaker status on %s:\n%s", readyNode.Name, status)
	}

	if output, err := services.PcsResourceCleanupViaDebug(ctx, oc, readyNode.Name); err != nil {
		framework.Logf("WARNING: pcs resource cleanup failed: %v", err)
		return
	} else {
		framework.Logf("Resource cleanup on %s: %s", readyNode.Name, output)
	}

	if output, err := services.PcsStonithCleanupViaDebug(ctx, oc, readyNode.Name); err != nil {
		framework.Logf("WARNING: pcs stonith cleanup failed: %v", err)
	} else {
		framework.Logf("Stonith cleanup on %s: %s", readyNode.Name, output)
	}
}

// IsClusterHealthyWithTimeout checks if the cluster is in a healthy state with a configurable timeout.
// It verifies that all nodes are ready and all cluster operators are available (not degraded or progressing).
// Returns an error with details if the cluster is not healthy, nil if healthy.
//
// As a first step, this function attempts to run 'pcs resource cleanup' on a Ready node to clear
// any failed pacemaker resource states, maximizing the odds of etcd recovery.
//
//	if err := IsClusterHealthyWithTimeout(oc, 5*time.Minute); err != nil {
//		return err
//	}
func IsClusterHealthyWithTimeout(oc *exutil.CLI, timeout time.Duration) error {
	ctx := context.Background()

	// First, try to clean up any pacemaker resource failures to maximize recovery chances
	TryPacemakerCleanup(oc)

	// Check all nodes are ready first using upstream framework function
	framework.Logf("Checking if all nodes are ready (timeout: %v)...", timeout)
	if err := nodehelper.AllNodesReady(ctx, oc.AdminKubeClient(), timeout); err != nil {
		return fmt.Errorf("not all nodes are ready: %w", err)
	}
	framework.Logf("All nodes are ready")

	// Check all cluster operators using MonitorClusterOperators
	framework.Logf("Checking if all cluster operators are healthy (timeout: %v)...", timeout)
	_, err := MonitorClusterOperators(oc, timeout, 5*time.Second)
	if err != nil {
		return fmt.Errorf("cluster operators not healthy: %w", err)
	}
	framework.Logf("All cluster operators are healthy")

	return nil
}

// MonitorClusterOperators monitors cluster operators and ensures they are all available.
// Returns the cluster operators status output and an error if operators are not healthy within timeout.
//
//	output, err := MonitorClusterOperators(oc, 5*time.Minute, 15*time.Second)
func MonitorClusterOperators(oc *exutil.CLI, timeout time.Duration, FiveSecondPollInterval time.Duration) (string, error) {
	ctx := context.Background()
	startTime := time.Now()

	for {
		// Get cluster operators status
		clusterOperators, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().List(ctx, metav1.ListOptions{})
		if err != nil {
			framework.Logf("Error getting cluster operators: %v", err)
			if time.Since(startTime) >= timeout {
				return "", fmt.Errorf("timeout waiting for cluster operators: %w", err)
			}
			time.Sleep(FiveSecondPollInterval)
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

		// Log summary status on each iteration (but not full oc get co output)
		healthyCount := len(clusterOperators.Items) - len(degradedOperators) - len(progressingOperators)
		framework.Logf("Cluster operators: %d healthy, %d degraded, %d progressing (elapsed: %v)",
			healthyCount, len(degradedOperators), len(progressingOperators), time.Since(startTime).Round(time.Second))

		// If all operators are healthy, we're done
		if allHealthy {
			wideOutput, _ := oc.AsAdmin().Run("get").Args("co", "-o", "wide").Output()
			framework.Logf("All cluster operators healthy:\n%s", wideOutput)
			return wideOutput, nil
		}

		// Check timeout
		if time.Since(startTime) >= timeout {
			wideOutput, _ := oc.AsAdmin().Run("get").Args("co", "-o", "wide").Output()
			framework.Logf("Cluster operators not healthy after %v timeout:\n%s", timeout, wideOutput)
			if len(degradedOperators) > 0 {
				framework.Logf("Degraded operators: %v", degradedOperators)
			}
			if len(progressingOperators) > 0 {
				framework.Logf("Progressing operators: %v", progressingOperators)
			}
			return wideOutput, fmt.Errorf("cluster operators did not become healthy within %v", timeout)
		}

		time.Sleep(FiveSecondPollInterval)
	}
}

// AddConstraint bans a pacemaker resource from running on a specific node (temporary, doesn't survive reboots).
//
//	err := AddConstraint(oc, "master-0", "kubelet-clone", "master-1")
func AddConstraint(oc *exutil.CLI, nodeName string, resourceName string, targetNode string) error {

	cmd := fmt.Sprintf("sudo pcs resource ban %s %s", resourceName, targetNode)

	output, err := exutil.DebugNodeRetryWithOptionsAndChroot(
		oc, nodeName, "default", "bash", "-c", cmd)

	if err != nil {
		return fmt.Errorf("failed to ban resource: %v, output: %s", err, output)
	}

	return nil
}

// RemoveConstraint clears all pacemaker resource bans and failures for a resource (comprehensive cleanup).
//
//	err := RemoveConstraint(oc, "master-0", "kubelet-clone")
func RemoveConstraint(oc *exutil.CLI, nodeName string, resourceName string) error {

	cmd := fmt.Sprintf("sudo pcs resource clear %s", resourceName)

	output, err := exutil.DebugNodeRetryWithOptionsAndChroot(
		oc, nodeName, "default", "bash", "-c", cmd)

	if err != nil {
		return fmt.Errorf("failed to clear resource: %v, output: %s", err, output)
	}

	return nil
}

// PcsNodeStandby puts a node in standby mode (resources moved away, node excluded from cluster).
//
//	err := PcsNodeStandby(oc, "master-0", "master-1")
func PcsNodeStandby(oc *exutil.CLI, execNodeName string, targetNodeName string) error {
	cmd := fmt.Sprintf("sudo pcs node standby %s", targetNodeName)
	output, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, execNodeName, "default", "bash", "-c", cmd)
	if err != nil {
		return fmt.Errorf("failed to standby node %s: %v, output: %s", targetNodeName, err, output)
	}
	return nil
}

// PcsNodeUnstandby brings a node out of standby mode.
//
//	err := PcsNodeUnstandby(oc, "master-0", "master-1")
func PcsNodeUnstandby(oc *exutil.CLI, execNodeName string, targetNodeName string) error {
	cmd := fmt.Sprintf("sudo pcs node unstandby %s", targetNodeName)
	output, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, execNodeName, "default", "bash", "-c", cmd)
	if err != nil {
		return fmt.Errorf("failed to unstandby node %s: %v, output: %s", targetNodeName, err, output)
	}
	return nil
}

// PcsPropertySetMaintenanceMode sets cluster-wide maintenance mode (true or false).
//
//	err := PcsPropertySetMaintenanceMode(oc, "master-0", true)
func PcsPropertySetMaintenanceMode(oc *exutil.CLI, execNodeName string, enabled bool) error {
	val := "false"
	if enabled {
		val = "true"
	}
	cmd := fmt.Sprintf("sudo pcs property set maintenance-mode=%s", val)
	output, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, execNodeName, "default", "bash", "-c", cmd)
	if err != nil {
		return fmt.Errorf("failed to set maintenance-mode=%s: %v, output: %s", val, err, output)
	}
	return nil
}

// PcsNodeMaintenance puts a single node in maintenance mode (its resources become unmanaged).
//
//	err := PcsNodeMaintenance(oc, "master-0", "master-1")
func PcsNodeMaintenance(oc *exutil.CLI, execNodeName string, targetNodeName string) error {
	cmd := fmt.Sprintf("sudo pcs node maintenance %s", targetNodeName)
	output, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, execNodeName, "default", "bash", "-c", cmd)
	if err != nil {
		return fmt.Errorf("failed to put node %s in maintenance: %v, output: %s", targetNodeName, err, output)
	}
	return nil
}

// PcsNodeUnmaintenance brings a node out of maintenance mode.
//
//	err := PcsNodeUnmaintenance(oc, "master-0", "master-1")
func PcsNodeUnmaintenance(oc *exutil.CLI, execNodeName string, targetNodeName string) error {
	cmd := fmt.Sprintf("sudo pcs node unmaintenance %s", targetNodeName)
	output, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, execNodeName, "default", "bash", "-c", cmd)
	if err != nil {
		return fmt.Errorf("failed to unmaintenance node %s: %v, output: %s", targetNodeName, err, output)
	}
	return nil
}

// PcsStonithUpdatePassword updates the password for a STONITH device (e.g. master-1_redfish).
// Used to simulate fencing agent degraded by setting a wrong password; restore via secret recreation.
//
//	err := PcsStonithUpdatePassword(oc, "master-0", "master-1_redfish", "wrongpassword")
func PcsStonithUpdatePassword(oc *exutil.CLI, execNodeName string, stonithID string, password string) error {
	// Shell-escape the password for use in pcs stonith update
	cmd := fmt.Sprintf("sudo pcs stonith update %s password=%q", stonithID, password)
	output, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, execNodeName, "default", "bash", "-c", cmd)
	if err != nil {
		return fmt.Errorf("failed to update stonith %s password: %v, output: %s", stonithID, err, output)
	}
	return nil
}

// IsResourceStopped checks if a pacemaker resource is in stopped state.
//
//	stopped, err := IsResourceStopped(oc, "master-0", "kubelet-clone")
func IsResourceStopped(oc *exutil.CLI, nodeName string, resourceName string) (bool, error) {
	framework.Logf("Checking if resource %s is stopped on node %s", resourceName, nodeName)

	cmd := fmt.Sprintf("sudo pcs status resources %s", resourceName)

	output, err := exutil.DebugNodeRetryWithOptionsAndChroot(
		oc, nodeName, "default", "bash", "-c", cmd)

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
	cmd := "sudo systemctl stop kubelet"

	output, err := exutil.DebugNodeRetryWithOptionsAndChroot(
		oc, nodeName, "default", "bash", "-c", cmd)

	if err != nil {
		// When kubelet stops, the connection to port 10250 is lost, causing the debug pod cleanup to fail
		// This is expected behavior - check if the error is due to connection refusal
		errStr := err.Error()
		if strings.Contains(errStr, "connection refused") || strings.Contains(errStr, KubeletPort) {
			framework.Logf("Kubelet stopped verified: connection to port %s lost (error: %v)", KubeletPort, err)
			return nil
		}

		framework.Logf("Failed to stop kubelet service: %v, output: %s", err, output)
		return fmt.Errorf("failed to stop kubelet service: %v", err)
	}

	framework.Logf("Kubelet stop command completed on %s (output: %s)", nodeName, output)
	return nil
}

// IsServiceRunning checks if a service is running on a specific target node.
// For kubelet in pacemaker clusters, checks the kubelet-clone resource status.
// execNode: the node to execute the check command from (important when target node is down)
// targetNode: the node to check service status for
//
//	running := IsServiceRunning(oc, "master-1", "master-0", "kubelet")
func IsServiceRunning(oc *exutil.CLI, execNode string, targetNode string, serviceName string) bool {
	// For kubelet in pacemaker environment, check the pacemaker resource directly
	// Always run from execNode since target node may be unavailable
	if serviceName == "kubelet" {
		cmd := "sudo pcs status resources kubelet-clone"

		output, err := exutil.DebugNodeRetryWithOptionsAndChroot(
			oc, execNode, "default", "bash", "-c", cmd)

		if err != nil {
			framework.Logf("ERROR: Failed to check pacemaker resource kubelet-clone: %v", err)
			return false
		}

		// Check if kubelet-clone is started on the target node
		isRunning := strings.Contains(output, "Started "+targetNode) ||
			strings.Contains(output, targetNode+" (Started)") ||
			(strings.Contains(output, "Started") && strings.Contains(output, targetNode))

		return isRunning
	}

	// For other services, use systemctl on the target node directly
	cmd := fmt.Sprintf("sudo systemctl is-active %s", serviceName)

	output, err := exutil.DebugNodeRetryWithOptionsAndChroot(
		oc, targetNode, "default", "bash", "-c", cmd)

	if err != nil {
		framework.Logf("ERROR: Failed to check service %s on node %s: %v", serviceName, targetNode, err)
		return false
	}

	trimmedOutput := strings.TrimSpace(output)
	isActive := trimmedOutput == "active"
	framework.Logf("Trimmed output: '%s', IsActive: %t", trimmedOutput, isActive)

	return isActive
}

// RecentResourceFailure captures details about a resource failure from Pacemaker operation history.
type RecentResourceFailure struct {
	ResourceID   string
	Task         string
	NodeName     string
	RC           string
	RCText       string
	LastRCChange time.Time
}

// HasRecentResourceFailure checks if a resource had any failed operations within the given time window.
// Uses "pcs status xml" to parse the node_history section for operations with non-zero return codes.
// This is useful for detecting that pacemaker noticed and responded to a resource failure,
// even if auto-recovery has already restored the resource.
//
//	hasFailure, failures, err := HasRecentResourceFailure(oc, "master-0", "kubelet-clone", 5*time.Minute)
func HasRecentResourceFailure(oc *exutil.CLI, execNodeName string, resourceID string, timeWindow time.Duration) (bool, []RecentResourceFailure, error) {
	framework.Logf("Checking for recent failures of resource %s within %v", resourceID, timeWindow)

	output, err := exutil.DebugNodeRetryWithOptionsAndChroot(
		oc, execNodeName, "default", "bash", "-c", "sudo pcs status xml")

	if err != nil {
		return false, nil, fmt.Errorf("failed to get pcs status xml: %v", err)
	}

	var result pcsStatusResult
	if parseErr := xml.Unmarshal([]byte(output), &result); parseErr != nil {
		return false, nil, fmt.Errorf("failed to parse pcs status xml: %v", parseErr)
	}

	cutoffTime := time.Now().Add(-timeWindow)
	var failures []RecentResourceFailure

	for _, node := range result.NodeHistory.Node {
		// Match resource ID (handles clone resources like "kubelet-clone" matching "kubelet:0", "kubelet:1")
		for _, resourceHistory := range node.ResourceHistory {
			if !strings.HasPrefix(resourceHistory.ID, strings.TrimSuffix(resourceID, "-clone")) {
				continue
			}

			for _, operation := range resourceHistory.OperationHistory {
				// RC "0" means success, anything else is a failure
				if operation.RC == "0" {
					continue
				}

				// Parse the timestamp
				opTime, parseErr := time.Parse(pacemakerTimeFormat, operation.LastRCChange)
				if parseErr != nil {
					framework.Logf("Warning: failed to parse timestamp %q: %v", operation.LastRCChange, parseErr)
					continue
				}

				// Check if within time window
				if !opTime.After(cutoffTime) {
					continue
				}

				failure := RecentResourceFailure{
					ResourceID:   resourceHistory.ID,
					Task:         operation.Task,
					NodeName:     node.Name,
					RC:           operation.RC,
					RCText:       operation.RCText,
					LastRCChange: opTime,
				}
				failures = append(failures, failure)
				framework.Logf("Found recent failure: resource=%s task=%s node=%s rc=%s (%s) at %s",
					failure.ResourceID, failure.Task, failure.NodeName, failure.RC, failure.RCText, failure.LastRCChange)
			}
		}
	}

	hasFailure := len(failures) > 0
	framework.Logf("Resource %s has %d recent failures within %v window", resourceID, len(failures), timeWindow)
	return hasFailure, failures, nil
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
// If etcdClientFactory is nil, member promotion status checks will be skipped.
//
//	if err := LogEtcdClusterStatus(oc, "BeforeEach validation", nil); err != nil { return err }
//	if err := LogEtcdClusterStatus(oc, "AfterEach cleanup", etcdClientFactory); err != nil { return err }
func LogEtcdClusterStatus(oc *exutil.CLI, testContext string, etcdClientFactory *helpers.EtcdClientFactoryImpl) error {
	// Check etcd ClusterOperator status
	framework.Logf("Checking etcd ClusterOperator status...")
	etcdOperator, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().Get(context.Background(), "etcd", metav1.GetOptions{})
	if err != nil {
		framework.Logf("ERROR: Failed to retrieve etcd ClusterOperator: %v", err)
		return fmt.Errorf("failed to retrieve etcd ClusterOperator: %v", err)
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
			pod.Name, pod.Status.Phase, podutils.IsPodReady(&pod), pod.Spec.NodeName)

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

		// Check if both nodes are healthy using vendored nodeutil.IsNodeReady
		framework.Logf("Checking node health status...")
		readyNodes := 0
		for i := range nodeList.Items {
			node := &nodeList.Items[i]
			isReady := nodeutil.IsNodeReady(node)
			if isReady {
				readyNodes++
			}
			framework.Logf("  - Node %s: Ready=%t, Roles=%s",
				node.Name, isReady, getNodeRoles(node))
		}
		framework.Logf("Node health summary: %d total nodes, %d ready nodes", len(nodeList.Items), readyNodes)

		// Enhanced etcd member analysis
		framework.Logf("Checking detailed etcd member status...")
		votingMembers := 0
		learnerMembers := 0
		healthyMembers := 0

		// Fetch etcd members once for all nodes
		var members []*etcdserverpb.Member
		if etcdClientFactory != nil {
			var err error
			members, err = GetMembers(etcdClientFactory)
			if err != nil {
				framework.Logf("WARNING: Failed to get etcd members: %v", err)
			}
		}

		for i := range nodeList.Items {
			node := &nodeList.Items[i]
			// Check if this node has an etcd pod
			var etcdPod *corev1.Pod
			for j := range etcdPods.Items {
				pod := &etcdPods.Items[j]
				if pod.Spec.NodeName == node.Name && pod.Status.Phase == corev1.PodRunning {
					etcdPod = pod
					break
				}
			}

			if etcdPod != nil {
				framework.Logf("  - Node %s: has running etcd pod %s", node.Name, etcdPod.Name)
				healthyMembers++

				// Determine if member is promoted (voting) or learner using pre-fetched members
				if members != nil {
					started, isLearner, err := GetMemberState(node, members)
					if err != nil {
						framework.Logf("    └─ Member status: UNKNOWN (%v)", err)
					} else if !started {
						framework.Logf("    └─ Member status: NOT STARTED (added but not joined)")
					} else if isLearner {
						learnerMembers++
						framework.Logf("    └─ Member status: LEARNER (not yet promoted)")
					} else {
						votingMembers++
						framework.Logf("    └─ Member status: VOTING (promoted)")
					}
				} else {
					framework.Logf("    └─ Member status: UNKNOWN (etcdClientFactory not provided)")
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
				framework.Logf("Both etcd members are promoted (voting members)")
			} else if learnerMembers > 0 {
				framework.Logf("Found %d learner members - waiting for promotion to voting members", learnerMembers)
			} else {
				framework.Logf("Unable to determine promotion status for all members")
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
		framework.Logf("CEO has %d ready replicas out of %d total",
			deployment.Status.ReadyReplicas, deployment.Status.Replicas)
	} else {
		framework.Logf("No revision-related issues detected in CEO conditions")
	}

	return nil
}

// GetMembers returns the etcd member list
func GetMembers(etcdClientFactory helpers.EtcdClientCreator) ([]*etcdserverpb.Member, error) {
	etcdClient, closeFn, err := etcdClientFactory.NewEtcdClient()
	if err != nil {
		return []*etcdserverpb.Member{}, errors.Wrap(err, "could not get a etcd client")
	}
	defer closeFn()

	ctx, cancel := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancel()
	m, err := etcdClient.MemberList(ctx)
	if err != nil {
		return []*etcdserverpb.Member{}, errors.Wrap(err, "could not get the member list")
	}
	return m.Members, nil
}

// GetMemberState returns whether a node's etcd member is started and whether it's a learner
func GetMemberState(node *corev1.Node, members []*etcdserverpb.Member) (started, learner bool, err error) {
	// Etcd members that have been added to the member list but haven't
	// joined yet will have an empty Name field. We can match them via Peer URL.
	hostPort := net.JoinHostPort(node.Status.Addresses[0].Address, "2380")
	peerURL := fmt.Sprintf("https://%s", hostPort)
	var found bool
	for _, m := range members {
		if m.Name == node.Name {
			found = true
			started = true
			learner = m.IsLearner
			break
		}
		if slices.Contains(m.PeerURLs, peerURL) {
			found = true
			learner = m.IsLearner
			break
		}
	}
	if !found {
		return false, false, fmt.Errorf("could not find node %v via peer URL %s", node.Name, peerURL)
	}
	return started, learner, nil
}

// ensureClusterOperatorHealthy checks if the cluster-etcd-operator is healthy before running etcd tests
func ensureClusterOperatorHealthy(oc *exutil.CLI) error {
	framework.Logf("Ensure cluster-etcd-operator is healthy (timeout: %v)", clusterIsHealthyTimeout)
	ctx, cancel := context.WithTimeout(context.Background(), clusterIsHealthyTimeout)
	defer cancel()

	for {
		co, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().Get(ctx, "etcd", metav1.GetOptions{})
		if err != nil {
			err = fmt.Errorf("failed to retrieve ClusterOperator: %v", err)
		} else {
			// Check if etcd operator is Available
			available := findClusterOperatorCondition(co.Status.Conditions, v1.OperatorAvailable)
			if available == nil {
				err = fmt.Errorf("ClusterOperator Available condition not found")
			} else if available.Status != v1.ConditionTrue {
				err = fmt.Errorf("ClusterOperator is not Available: %s", available.Message)
			} else {
				// Check if etcd operator is not Degraded
				degraded := findClusterOperatorCondition(co.Status.Conditions, v1.OperatorDegraded)
				if degraded != nil && degraded.Status == v1.ConditionTrue {
					err = fmt.Errorf("ClusterOperator is Degraded: %s", degraded.Message)
				} else {
					framework.Logf("SUCCESS: cluster-etcd-operator is healthy")
					return nil
				}
			}
		}

		select {
		case <-ctx.Done():
			return err
		default:
		}
		time.Sleep(FiveSecondPollInterval)
	}
}

func ensureEtcdPodsAreRunning(oc *exutil.CLI) error {
	framework.Logf("Ensure Etcd pods are running (timeout: %v)", clusterIsHealthyTimeout)
	ctx, cancel := context.WithTimeout(context.Background(), clusterIsHealthyTimeout)
	defer cancel()
	for {
		etcdPods, err := oc.AdminKubeClient().CoreV1().Pods("openshift-etcd").List(context.Background(), metav1.ListOptions{
			LabelSelector: "app=etcd",
		})
		if err != nil {
			err = fmt.Errorf("failed to retrieve etcd pods: %v", err)
		} else {
			runningPods := 0
			for _, pod := range etcdPods.Items {
				if pod.Status.Phase == corev1.PodRunning {
					runningPods++
				}
			}
			if runningPods < 2 {
				return fmt.Errorf("expected at least 2 etcd pods running, found %d", runningPods)
			}

			framework.Logf("SUCCESS: found the 2 expected Etcd pods")
			return nil
		}

		select {
		case <-ctx.Done():
			return err
		default:
		}
		time.Sleep(FiveSecondPollInterval)
	}
}

// findClusterOperatorCondition finds a condition in ClusterOperator status
func findClusterOperatorCondition(conditions []v1.ClusterOperatorStatusCondition, conditionType v1.ClusterStatusConditionType) *v1.ClusterOperatorStatusCondition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}

func ensureEtcdHasTwoVotingMembers(nodes *corev1.NodeList, ecf *helpers.EtcdClientFactoryImpl) error {
	framework.Logf("Ensure Etcd member list has two voting members (timeout: %v)", clusterIsHealthyTimeout)
	ctx, cancel := context.WithTimeout(context.Background(), clusterIsHealthyTimeout)
	defer cancel()

	for {
		// Check all conditions sequentially
		members, err := GetMembers(ecf)
		if err == nil && len(members) != 2 {
			err = fmt.Errorf("expected 2 members, found %d", len(members))
		}

		if err == nil {
			for _, node := range nodes.Items {
				isStarted, isLearner, checkErr := GetMemberState(&node, members)
				if checkErr != nil {
					err = checkErr
				} else if !isStarted || isLearner {
					err = fmt.Errorf("member %s is not a voting member (started=%v, learner=%v)",
						node.Name, isStarted, isLearner)
					break
				}
			}

		}

		// All checks passed - success!
		if err == nil {
			framework.Logf("SUCCESS: got membership with two voting members: %+v", members)
			return nil
		}

		// Checks failed - evaluate timeout
		select {
		case <-ctx.Done():
			return fmt.Errorf("etcd membership does not have two voters: %v, membership: %+v", err, members)
		default:
		}
		time.Sleep(FiveSecondPollInterval)
	}
}

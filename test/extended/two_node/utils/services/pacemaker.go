// Package services provides Pacemaker utilities: cluster status, etcd resource management, STONITH control, and job handling via SSH.
package services

import (
	"context"
	"encoding/xml"
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/test/extended/two_node/utils/core"
	exutil "github.com/openshift/origin/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// Pacemaker-related constants
const (
	superuserPrefix = "sudo"
	pcsExecutable   = "pcs"
	noEnvVars       = ""

	// PCS commands
	pcsClusterNodeAdd            = "cluster node add %s addr=%s --start --enable"
	pcsResourceDebugStop         = "resource debug-stop etcd"
	pcsResourceDebugStartEnvVars = "OCF_RESKEY_CRM_meta_notify_start_resource='etcd'"
	pcsResourceDebugStart        = "resource debug-start etcd"
	pcsClusterNodeRemove         = "cluster node remove %s"
	pcsResourceStatus            = "resource status etcd node=%s"
	pcsStatus                    = "status"
	pcsStatusXML                 = "status xml"
	pcsResourceCleanup           = "resource cleanup"
	pcsStonithCleanup            = "stonith cleanup"
	pcsStonithDisable            = "property set stonith-enabled=false"
	pcsStonithEnable             = "property set stonith-enabled=true"
	pcsStonithConfirm            = "stonith confirm %s --force"
	pcsProperty                  = "property"

	pcsNodeStandby   = "node standby %s"
	pcsNodeUnstandby = "node unstandby %s"
)

func formatPcsCommandString(command string, envVars string) string {
	if envVars != "" {
		return fmt.Sprintf("%s %s %s %s", superuserPrefix, envVars, pcsExecutable, command)
	}

	return fmt.Sprintf("%s %s %s", superuserPrefix, pcsExecutable, command)
}

// PcsDebugStart restores etcd quorum by performing debug-start (bypasses cluster checks for two-node scenarios).
//
//	stdout, stderr, err := PcsDebugStart(nodeIP, false, sshConfig, localKH, remoteKH)
func PcsDebugStart(remoteNodeIP string, fullOutput bool, sshConfig *core.SSHConfig, localKnownHostsPath, remoteKnownHostsPath string) (string, string, error) {
	resourceStartCmd := pcsResourceDebugStart
	if fullOutput {
		resourceStartCmd = fmt.Sprintf("%s %s", resourceStartCmd, "--full")
	}

	output, stderr, err := core.ExecuteRemoteSSHCommand(remoteNodeIP, formatPcsCommandString(resourceStartCmd, pcsResourceDebugStartEnvVars), sshConfig, localKnownHostsPath, remoteKnownHostsPath)
	if err != nil {
		e2e.Logf("PcsDebugStart failed on node %s: %v, stderr: %s", remoteNodeIP, err, stderr)
		return output, stderr, err
	}

	return output, stderr, nil
}

// PcsDebugStop stops the etcd resource using debug-stop (controlled shutdown without triggering recovery).
//
//	stdout, stderr, err := PcsDebugStop(nodeIP, false, sshConfig, localKH, remoteKH)
func PcsDebugStop(remoteNodeIP string, fullOutput bool, sshConfig *core.SSHConfig, localKnownHostsPath, remoteKnownHostsPath string) (string, string, error) {
	resourceStopCmd := pcsResourceDebugStop
	if fullOutput {
		resourceStopCmd = fmt.Sprintf("%s %s", resourceStopCmd, "--full")
	}

	output, stderr, err := core.ExecuteRemoteSSHCommand(remoteNodeIP, formatPcsCommandString(resourceStopCmd, noEnvVars), sshConfig, localKnownHostsPath, remoteKnownHostsPath)
	if err != nil {
		e2e.Logf("PcsDebugStop failed on node %s: %v, stderr: %s", remoteNodeIP, err, stderr)
		return output, stderr, err
	}

	return output, stderr, nil
}

// PcsDebugRestart restores etcd quorum by performing debug-stop then debug-start.
//
//	stdout, stderr, err := PcsDebugRestart(nodeIP, false, sshConfig, localKH, remoteKH)
func PcsDebugRestart(remoteNodeIP string, fullOutput bool, sshConfig *core.SSHConfig, localKnownHostsPath, remoteKnownHostsPath string) (string, string, error) {
	output, stderr, err := PcsDebugStop(remoteNodeIP, fullOutput, sshConfig, localKnownHostsPath, remoteKnownHostsPath)
	if err != nil {
		return output, stderr, err
	}

	return PcsDebugStart(remoteNodeIP, fullOutput, sshConfig, localKnownHostsPath, remoteKnownHostsPath)
}

// PcsStatus retrieves the overall pacemaker cluster status.
// This shows the state of all cluster resources, nodes, and any failures.
func PcsStatus(remoteNodeIP string, sshConfig *core.SSHConfig, localKnownHostsPath, remoteKnownHostsPath string) (string, string, error) {
	return core.ExecuteRemoteSSHCommand(remoteNodeIP, formatPcsCommandString(pcsStatus, noEnvVars), sshConfig, localKnownHostsPath, remoteKnownHostsPath)
}

// PcsStatusFull retrieves the full pacemaker cluster status with additional details.
// This includes all nodes, resources, fence status, and any pending operations.
func PcsStatusFull(remoteNodeIP string, sshConfig *core.SSHConfig, localKnownHostsPath, remoteKnownHostsPath string) (string, string, error) {
	return core.ExecuteRemoteSSHCommand(remoteNodeIP, formatPcsCommandString("status --full", noEnvVars), sshConfig, localKnownHostsPath, remoteKnownHostsPath)
}

// PcsResourceStatus retrieves the status of a specific pacemaker resource (etcd) on a node.
// This is more targeted than PcsStatus and shows whether the etcd resource is started/stopped.
func PcsResourceStatus(nodeName, remoteNodeIP string, sshConfig *core.SSHConfig, localKnownHostsPath, remoteKnownHostsPath string) (string, string, error) {
	output, stderr, err := core.ExecuteRemoteSSHCommand(remoteNodeIP, formatPcsCommandString(fmt.Sprintf(pcsResourceStatus, nodeName), noEnvVars), sshConfig, localKnownHostsPath, remoteKnownHostsPath)
	if err != nil {
		e2e.Logf("PcsResourceStatus failed for node %s: %v, stderr: %s", nodeName, err, stderr)
	}
	return output, stderr, err
}

// PcsJournal retrieves the last N lines of pacemaker journal logs filtered for podman-etcd.
//
//	stdout, stderr, err := PcsJournal(100, nodeIP, sshConfig, localKH, remoteKH)
func PcsJournal(pcsJournalTailLines int, remoteNodeIP string, sshConfig *core.SSHConfig, localKnownHostsPath, remoteKnownHostsPath string) (string, string, error) {
	// Validate line count to prevent abuse and ensure reasonable bounds
	if err := core.ValidateIntegerBounds(pcsJournalTailLines, 1, 10000, "journal line count"); err != nil {
		return "", "", err
	}

	return core.ExecuteRemoteSSHCommand(remoteNodeIP, fmt.Sprintf("sudo journalctl -u pacemaker --no-pager | grep podman-etcd | tail -n %d", pcsJournalTailLines), sshConfig, localKnownHostsPath, remoteKnownHostsPath)
}

// pcsStatusXMLResponse represents the XML structure returned by "pcs status xml"
// which internally calls /usr/sbin/crm_mon --one-shot --inactive --output-as xml
type pcsStatusXMLResponse struct {
	XMLName xml.Name `xml:"pacemaker-result"`
	Nodes   struct {
		Node []struct {
			Name   string `xml:"name,attr"`
			Online string `xml:"online,attr"` // Changed from bool to string as XML uses "true"/"false" strings
		} `xml:"node"`
	} `xml:"nodes"`
}

// WaitForNodesOnline waits for all specified nodes to be online in the pacemaker cluster by polling XML status.
//
//	err := WaitForNodesOnline([]string{"master-0", "master-1"}, nodeIP, 5*time.Minute, 10*time.Second, sshConfig, localKH, remoteKH)
func WaitForNodesOnline(nodeNames []string, remoteNodeIP string, timeout, pollInterval time.Duration, sshConfig *core.SSHConfig, localKnownHostsPath, remoteKnownHostsPath string) error {
	e2e.Logf("Waiting for nodes %v to be online in pacemaker cluster (timeout: %v)", nodeNames, timeout)

	return core.PollUntil(func() (bool, error) {
		// Get pacemaker cluster status in XML format
		statusOutput, stderr, err := core.ExecuteRemoteSSHCommand(remoteNodeIP, formatPcsCommandString(pcsStatusXML, noEnvVars), sshConfig, localKnownHostsPath, remoteKnownHostsPath)
		if err != nil {
			e2e.Logf("Failed to get pacemaker status, retrying: %v, stderr: %s", err, stderr)
			return false, nil // Temporary error, continue polling
		}

		// Parse XML response
		var status pcsStatusXMLResponse
		if err := xml.Unmarshal([]byte(statusOutput), &status); err != nil {
			e2e.Logf("Failed to parse pacemaker status XML, retrying: %v", err)
			return false, nil // Parse error, continue polling
		}

		// Check if all requested nodes are online
		onlineNodes := make(map[string]bool)
		for _, node := range status.Nodes.Node {
			if node.Online == "true" {
				onlineNodes[node.Name] = true
			}
		}

		// Verify all nodes are online
		allOnline := true
		var offlineNodes []string
		for _, nodeName := range nodeNames {
			if !onlineNodes[nodeName] {
				allOnline = false
				offlineNodes = append(offlineNodes, nodeName)
			}
		}

		if allOnline {
			e2e.Logf("All nodes %v are online in pacemaker cluster", nodeNames)
			return true, nil // All nodes online, stop polling
		}

		e2e.Logf("Waiting for nodes to be online... Online: %v, Offline: %v",
			getOnlineNodesList(onlineNodes, nodeNames), offlineNodes)
		return false, nil // Not all online yet, continue polling
	}, timeout, pollInterval, fmt.Sprintf("pacemaker nodes %v to be online", nodeNames))
}

// getOnlineNodesList returns a list of nodes from nodeNames that are marked as online
func getOnlineNodesList(onlineNodes map[string]bool, nodeNames []string) []string {
	var online []string
	for _, name := range nodeNames {
		if onlineNodes[name] {
			online = append(online, name)
		}
	}
	return online
}

// CycleRemovedNode removes and re-adds a node in the pacemaker cluster configuration.
//
//	err := CycleRemovedNode("master-0", "192.168.111.20", runningNodeIP, sshConfig, localKH, remoteKH)
func CycleRemovedNode(failedNodeName, failedNodeIP, runningNodeIP string, sshConfig *core.SSHConfig, localKnownHostsPath, remoteKnownHostsPath string) error {
	// Remove and re-add the node in pacemaker using constants
	pcsScript := fmt.Sprintf(`
		%s
		%s
	`,
		formatPcsCommandString(fmt.Sprintf(pcsClusterNodeRemove, failedNodeName), noEnvVars),
		formatPcsCommandString(fmt.Sprintf(pcsClusterNodeAdd, failedNodeName, failedNodeIP), noEnvVars),
	)

	_, _, err := core.ExecuteRemoteSSHCommand(runningNodeIP, pcsScript, sshConfig, localKnownHostsPath, remoteKnownHostsPath)
	if err != nil {
		return core.WrapError("cycle node in pacemaker cluster", failedNodeName, err)
	}

	e2e.Logf("Successfully cycled node %s in pacemaker cluster", failedNodeName)
	return nil
}

// PcsResourceCleanup cleans up resource failures in the pacemaker cluster.
//
//	stdout, stderr, err := PcsResourceCleanup(nodeIP, sshConfig, localKH, remoteKH)
func PcsResourceCleanup(remoteNodeIP string, sshConfig *core.SSHConfig, localKnownHostsPath, remoteKnownHostsPath string) (string, string, error) {
	output, stderr, err := core.ExecuteRemoteSSHCommand(remoteNodeIP, formatPcsCommandString(pcsResourceCleanup, noEnvVars), sshConfig, localKnownHostsPath, remoteKnownHostsPath)
	if err != nil {
		e2e.Logf("PcsResourceCleanup failed on node %s: %v, stderr: %s", remoteNodeIP, err, stderr)
	}
	return output, stderr, err
}

// PcsStonithCleanup cleans up STONITH failures in the pacemaker cluster.
//
//	stdout, stderr, err := PcsStonithCleanup(nodeIP, sshConfig, localKH, remoteKH)
func PcsStonithCleanup(remoteNodeIP string, sshConfig *core.SSHConfig, localKnownHostsPath, remoteKnownHostsPath string) (string, string, error) {
	output, stderr, err := core.ExecuteRemoteSSHCommand(remoteNodeIP, formatPcsCommandString(pcsStonithCleanup, noEnvVars), sshConfig, localKnownHostsPath, remoteKnownHostsPath)
	if err != nil {
		e2e.Logf("PcsStonithCleanup failed on node %s: %v, stderr: %s", remoteNodeIP, err, stderr)
	}
	return output, stderr, err
}

// PcsStonithDisable disables STONITH in the pacemaker cluster.
//
//	stdout, stderr, err := PcsStonithDisable(nodeIP, sshConfig, localKH, remoteKH)
func PcsStonithDisable(remoteNodeIP string, sshConfig *core.SSHConfig, localKnownHostsPath, remoteKnownHostsPath string) (string, string, error) {
	output, stderr, err := core.ExecuteRemoteSSHCommand(remoteNodeIP, formatPcsCommandString(pcsStonithDisable, noEnvVars), sshConfig, localKnownHostsPath, remoteKnownHostsPath)
	if err != nil {
		e2e.Logf("PcsStonithDisable failed on node %s: %v, stderr: %s", remoteNodeIP, err, stderr)
	}
	return output, stderr, err
}

// PcsStonithEnable enables STONITH in the pacemaker cluster.
//
//	stdout, stderr, err := PcsStonithEnable(nodeIP, sshConfig, localKH, remoteKH)
func PcsStonithEnable(remoteNodeIP string, sshConfig *core.SSHConfig, localKnownHostsPath, remoteKnownHostsPath string) (string, string, error) {
	output, stderr, err := core.ExecuteRemoteSSHCommand(remoteNodeIP, formatPcsCommandString(pcsStonithEnable, noEnvVars), sshConfig, localKnownHostsPath, remoteKnownHostsPath)
	if err != nil {
		e2e.Logf("PcsStonithEnable failed on node %s: %v, stderr: %s", remoteNodeIP, err, stderr)
	}
	return output, stderr, err
}

// PcsStonithConfirm manually confirms to the pacemaker cluster that a node has been fenced.
// This is used when the fencing device cannot fence the node (e.g., when the node has been
// destroyed and the BMC is unreachable). The --force flag bypasses interactive confirmation.
//
// WARNING: Only use this when you have manually verified the node is powered off and has
// no access to shared resources. Using this incorrectly can cause data corruption.
//
//	stdout, stderr, err := PcsStonithConfirm("master-1", nodeIP, sshConfig, localKH, remoteKH)
func PcsStonithConfirm(targetNodeName, remoteNodeIP string, sshConfig *core.SSHConfig, localKnownHostsPath, remoteKnownHostsPath string) (string, string, error) {
	cmd := fmt.Sprintf(pcsStonithConfirm, targetNodeName)
	output, stderr, err := core.ExecuteRemoteSSHCommand(remoteNodeIP, formatPcsCommandString(cmd, noEnvVars), sshConfig, localKnownHostsPath, remoteKnownHostsPath)
	if err != nil {
		e2e.Logf("PcsStonithConfirm failed for node %s on %s: %v, stderr: %s", targetNodeName, remoteNodeIP, err, stderr)
	}
	return output, stderr, err
}

// PcsProperty gets cluster properties from pacemaker.
//
//	stdout, stderr, err := PcsProperty(nodeIP, sshConfig, localKH, remoteKH)
func PcsProperty(remoteNodeIP string, sshConfig *core.SSHConfig, localKnownHostsPath, remoteKnownHostsPath string) (string, string, error) {
	output, stderr, err := core.ExecuteRemoteSSHCommand(remoteNodeIP, formatPcsCommandString(pcsProperty, noEnvVars), sshConfig, localKnownHostsPath, remoteKnownHostsPath)
	if err != nil {
		e2e.Logf("PcsProperty failed on node %s: %v, stderr: %s", remoteNodeIP, err, stderr)
	}
	return output, stderr, err
}

// PcsNodeStandby puts a node in standby mode to prevent pacemaker resources from starting on it.
// This is run from a remote node (typically the surviving node) to put the target node in standby.
//
//	stdout, stderr, err := PcsNodeStandby("master-1", survivorIP, sshConfig, localKH, remoteKH)
func PcsNodeStandby(targetNodeName, remoteNodeIP string, sshConfig *core.SSHConfig, localKnownHostsPath, remoteKnownHostsPath string) (string, string, error) {
	cmd := fmt.Sprintf(pcsNodeStandby, targetNodeName)
	output, stderr, err := core.ExecuteRemoteSSHCommand(remoteNodeIP, formatPcsCommandString(cmd, noEnvVars), sshConfig, localKnownHostsPath, remoteKnownHostsPath)
	if err != nil {
		e2e.Logf("PcsNodeStandby failed for node %s on %s: %v, stderr: %s", targetNodeName, remoteNodeIP, err, stderr)
	}
	return output, stderr, err
}

// PcsNodeUnstandby removes a node from standby mode, allowing pacemaker resources to start on it.
//
//	stdout, stderr, err := PcsNodeUnstandby("master-1", survivorIP, sshConfig, localKH, remoteKH)
func PcsNodeUnstandby(targetNodeName, remoteNodeIP string, sshConfig *core.SSHConfig, localKnownHostsPath, remoteKnownHostsPath string) (string, string, error) {
	cmd := fmt.Sprintf(pcsNodeUnstandby, targetNodeName)
	output, stderr, err := core.ExecuteRemoteSSHCommand(remoteNodeIP, formatPcsCommandString(cmd, noEnvVars), sshConfig, localKnownHostsPath, remoteKnownHostsPath)
	if err != nil {
		e2e.Logf("PcsNodeUnstandby failed for node %s on %s: %v, stderr: %s", targetNodeName, remoteNodeIP, err, stderr)
	}
	return output, stderr, err
}

// WaitForEtcdCertificateRestart waits for podman-etcd on a node to complete a restart cycle
// triggered by certificate rollout. Monitors pacemaker journal logs for:
// 1. "certificate files have changed" message
// 2. podman-etcd stop event
// 3. podman-etcd start event (restart complete)
//
// Uses journalctl to query pacemaker logs filtered for podman-etcd events.
//
//	err := WaitForEtcdCertificateRestart("master-0", nodeIP, 10*time.Minute, 15*time.Second, sshConfig, localKH, remoteKH)
func WaitForEtcdCertificateRestart(nodeName, nodeIP string, timeout, pollInterval time.Duration,
	sshConfig *core.SSHConfig, localKnownHostsPath, remoteKnownHostsPath string) error {

	e2e.Logf("Waiting for etcd certificate restart on node %s (timeout: %v)", nodeName, timeout)

	// Track state: waiting_for_cert_change -> waiting_for_stop -> waiting_for_start -> done
	certChangeDetected := false
	stopDetected := false

	return core.PollUntil(func() (bool, error) {
		// Query recent pacemaker journal logs for podman-etcd events
		cmd := `sudo journalctl -u pacemaker --since "10 minutes ago" --no-pager | grep podman-etcd`
		output, _, err := core.ExecuteRemoteSSHCommand(nodeIP, cmd, sshConfig,
			localKnownHostsPath, remoteKnownHostsPath)
		if err != nil {
			e2e.Logf("Failed to query journal on %s: %v", nodeName, err)
			return false, nil // Continue polling
		}

		// Check for certificate change message
		if !certChangeDetected && strings.Contains(output, "certificate files have changed") {
			e2e.Logf("Detected certificate change on %s", nodeName)
			certChangeDetected = true
		}

		// Check for stop event (after cert change)
		if certChangeDetected && !stopDetected && strings.Contains(output, "podman-etcd stop") {
			e2e.Logf("Detected podman-etcd stop on %s", nodeName)
			stopDetected = true
		}

		// Check for start event (after stop)
		if stopDetected && strings.Contains(output, "podman-etcd start") {
			e2e.Logf("Detected podman-etcd start on %s - restart cycle complete", nodeName)
			return true, nil
		}

		e2e.Logf("Waiting for etcd certificate restart on %s (cert_change=%v, stop=%v)",
			nodeName, certChangeDetected, stopDetected)
		return false, nil
	}, timeout, pollInterval, fmt.Sprintf("etcd certificate restart on %s", nodeName))
}

type debugContainerResult struct {
	output string
	err    error
}

// runDebugContainerWithContext executes a command via debug container with context-based cancellation.
// The context allows callers to set timeouts; if cancelled, the operation returns immediately
// but the underlying debug container may continue until its internal timeout.
func runDebugContainerWithContext(ctx context.Context, oc *exutil.CLI, nodeName string, cmd ...string) (string, error) {
	resultCh := make(chan debugContainerResult, 1)

	go func() {
		output, err := exutil.DebugNodeRetryWithOptionsAndChroot(oc, nodeName, "openshift-etcd", cmd...)
		resultCh <- debugContainerResult{output: output, err: err}
	}()

	select {
	case <-ctx.Done():
		return "", fmt.Errorf("debug container command cancelled: %w", ctx.Err())
	case result := <-resultCh:
		return result.output, result.err
	}
}

// PcsResourceCleanupViaDebug clears any failed actions in the cluster's resource manager.
// This is the debug container equivalent of PcsResourceCleanup for use when SSH is unavailable.
//
//	output, err := PcsResourceCleanupViaDebug(ctx, oc, "master-0")
func PcsResourceCleanupViaDebug(ctx context.Context, oc *exutil.CLI, nodeName string) (string, error) {
	output, err := runDebugContainerWithContext(ctx, oc, nodeName, "pcs", "resource", "cleanup")
	if err != nil {
		e2e.Logf("PcsResourceCleanupViaDebug failed on node %s: %v", nodeName, err)
	}
	return output, err
}

// PcsStonithCleanupViaDebug clears any failed STONITH (fencing) actions in the cluster.
// This is the debug container equivalent of PcsStonithCleanup for use when SSH is unavailable.
//
//	output, err := PcsStonithCleanupViaDebug(ctx, oc, "master-0")
func PcsStonithCleanupViaDebug(ctx context.Context, oc *exutil.CLI, nodeName string) (string, error) {
	output, err := runDebugContainerWithContext(ctx, oc, nodeName, "pcs", "stonith", "cleanup")
	if err != nil {
		e2e.Logf("PcsStonithCleanupViaDebug failed on node %s: %v", nodeName, err)
	}
	return output, err
}

// PcsStatusViaDebug retrieves the overall pacemaker cluster status via debug container.
// This shows the state of all cluster resources, nodes, and any failures.
// Use instead of PcsStatus when SSH to the hypervisor is unavailable.
//
//	output, err := PcsStatusViaDebug(ctx, oc, "master-0")
func PcsStatusViaDebug(ctx context.Context, oc *exutil.CLI, nodeName string) (string, error) {
	output, err := runDebugContainerWithContext(ctx, oc, nodeName, "pcs", "status")
	if err != nil {
		e2e.Logf("PcsStatusViaDebug failed on node %s: %v", nodeName, err)
	}
	return output, err
}

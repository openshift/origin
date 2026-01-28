// Package services provides Pacemaker utilities: cluster status, etcd resource management, STONITH control, and job handling via SSH.
package services

import (
	"encoding/xml"
	"fmt"
	"time"

	"github.com/openshift/origin/test/extended/two_node/utils/core"
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
	pcsProperty                  = "property"
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
	e2e.Logf("PcsDebugStart: Restoring etcd quorum on remote node: %s", remoteNodeIP)

	resourceStartCmd := pcsResourceDebugStart
	if fullOutput {
		resourceStartCmd = fmt.Sprintf("%s %s", resourceStartCmd, "--full")
	}

	// SSH to hypervisor, then to remote node to run pcs debug-start
	// We need to chain the SSH commands: host -> hypervisor -> remote node
	e2e.Logf("PcsDebugStart: Executing command on node %s: %s", remoteNodeIP, formatPcsCommandString(resourceStartCmd, pcsResourceDebugStartEnvVars))
	output, stderr, err := core.ExecuteRemoteSSHCommand(remoteNodeIP, formatPcsCommandString(resourceStartCmd, pcsResourceDebugStartEnvVars), sshConfig, localKnownHostsPath, remoteKnownHostsPath)
	if err != nil {
		e2e.Logf("ERROR: PcsDebugStart failed to restart etcd on node %s: %v, stderr: %s", remoteNodeIP, err, stderr)
		return output, stderr, err
	}
	e2e.Logf("PcsDebugStart: Command output: %s", output)

	// Log pacemaker status to check if etcd has been started on the remote node
	e2e.Logf("PcsDebugStart: Getting pacemaker status on node %s", remoteNodeIP)
	pcsStatusOutput, stderr, err := PcsStatus(remoteNodeIP, sshConfig, localKnownHostsPath, remoteKnownHostsPath)
	if err != nil {
		e2e.Logf("WARNING: PcsDebugStart failed to get pacemaker status on node %s: %v", remoteNodeIP, err)
	} else {
		e2e.Logf("PcsDebugStart: Pacemaker status on node %s:\n%s", remoteNodeIP, pcsStatusOutput)
	}

	e2e.Logf("PcsDebugStart: Successfully restored etcd quorum on remote node: %s", remoteNodeIP)
	return output, stderr, nil
}

// PcsDebugStop stops the etcd resource using debug-stop (controlled shutdown without triggering recovery).
//
//	stdout, stderr, err := PcsDebugStop(nodeIP, false, sshConfig, localKH, remoteKH)
func PcsDebugStop(remoteNodeIP string, fullOutput bool, sshConfig *core.SSHConfig, localKnownHostsPath, remoteKnownHostsPath string) (string, string, error) {
	e2e.Logf("PcsDebugStop: Stopping podman-etcd on remote node: %s", remoteNodeIP)

	resourceStopCmd := pcsResourceDebugStop
	if fullOutput {
		resourceStopCmd = fmt.Sprintf("%s %s", resourceStopCmd, "--full")
	}

	// SSH to hypervisor, then to remote node to run pcs debug-stop
	// We need to chain the SSH commands: host -> hypervisor -> remote node
	e2e.Logf("PcsDebugStop: Executing command on node %s: %s", remoteNodeIP, formatPcsCommandString(resourceStopCmd, noEnvVars))
	output, stderr, err := core.ExecuteRemoteSSHCommand(remoteNodeIP, formatPcsCommandString(resourceStopCmd, noEnvVars), sshConfig, localKnownHostsPath, remoteKnownHostsPath)
	if err != nil {
		e2e.Logf("ERROR: PcsDebugStop failed to stop etcd on node %s: %v, stderr: %s", remoteNodeIP, err, stderr)
		return output, stderr, err
	}
	e2e.Logf("PcsDebugStop: Command output: %s", output)

	// Log pacemaker status to check if etcd has been stopped on the remote node
	e2e.Logf("PcsDebugStop: Getting pacemaker status on node %s", remoteNodeIP)
	pcsStatusOutput, stderr, err := PcsStatus(remoteNodeIP, sshConfig, localKnownHostsPath, remoteKnownHostsPath)
	if err != nil {
		e2e.Logf("WARNING: PcsDebugStop failed to get pacemaker status on node %s: %v", remoteNodeIP, err)
	} else {
		e2e.Logf("PcsDebugStop: Pacemaker status on node %s:\n%s", remoteNodeIP, pcsStatusOutput)
	}

	e2e.Logf("PcsDebugStop: Successfully stopped podman-etcd on remote node: %s", remoteNodeIP)
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

// PcsResourceStatus retrieves the status of a specific pacemaker resource (etcd) on a node.
// This is more targeted than PcsStatus and shows whether the etcd resource is started/stopped.
func PcsResourceStatus(nodeName, remoteNodeIP string, sshConfig *core.SSHConfig, localKnownHostsPath, remoteKnownHostsPath string) (string, string, error) {
	e2e.Logf("PcsResourceStatus: Getting etcd resource status for node %s (remote IP: %s)", nodeName, remoteNodeIP)
	output, stderr, err := core.ExecuteRemoteSSHCommand(remoteNodeIP, formatPcsCommandString(fmt.Sprintf(pcsResourceStatus, nodeName), noEnvVars), sshConfig, localKnownHostsPath, remoteKnownHostsPath)
	if err != nil {
		e2e.Logf("ERROR: PcsResourceStatus failed for node %s: %v, stderr: %s", nodeName, err, stderr)
	} else {
		e2e.Logf("PcsResourceStatus: Got status for node %s: %s", nodeName, output)
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
	e2e.Logf("Running pcs resource cleanup on node: %s", remoteNodeIP)

	output, stderr, err := core.ExecuteRemoteSSHCommand(remoteNodeIP, formatPcsCommandString(pcsResourceCleanup, noEnvVars), sshConfig, localKnownHostsPath, remoteKnownHostsPath)
	if err != nil {
		e2e.Logf("ERROR: Failed to run pcs resource cleanup on node %s (stderr: %s): %v", remoteNodeIP, stderr, err)
		return output, stderr, err
	}

	e2e.Logf("Successfully ran pcs resource cleanup on node: %s", remoteNodeIP)
	return output, stderr, nil
}

// PcsStonithCleanup cleans up STONITH failures in the pacemaker cluster.
//
//	stdout, stderr, err := PcsStonithCleanup(nodeIP, sshConfig, localKH, remoteKH)
func PcsStonithCleanup(remoteNodeIP string, sshConfig *core.SSHConfig, localKnownHostsPath, remoteKnownHostsPath string) (string, string, error) {
	e2e.Logf("Running pcs stonith cleanup on node: %s", remoteNodeIP)

	output, stderr, err := core.ExecuteRemoteSSHCommand(remoteNodeIP, formatPcsCommandString(pcsStonithCleanup, noEnvVars), sshConfig, localKnownHostsPath, remoteKnownHostsPath)
	if err != nil {
		e2e.Logf("ERROR: Failed to run pcs stonith cleanup on node %s (stderr: %s): %v", remoteNodeIP, stderr, err)
		return output, stderr, err
	}

	e2e.Logf("Successfully ran pcs stonith cleanup on node: %s", remoteNodeIP)
	return output, stderr, nil
}

// PcsStonithDisable disables STONITH in the pacemaker cluster.
//
//	stdout, stderr, err := PcsStonithDisable(nodeIP, sshConfig, localKH, remoteKH)
func PcsStonithDisable(remoteNodeIP string, sshConfig *core.SSHConfig, localKnownHostsPath, remoteKnownHostsPath string) (string, string, error) {
	e2e.Logf("Disabling STONITH on node: %s", remoteNodeIP)

	output, stderr, err := core.ExecuteRemoteSSHCommand(remoteNodeIP, formatPcsCommandString(pcsStonithDisable, noEnvVars), sshConfig, localKnownHostsPath, remoteKnownHostsPath)
	if err != nil {
		e2e.Logf("ERROR: Failed to disable STONITH on node %s (stderr: %s): %v", remoteNodeIP, stderr, err)
		return output, stderr, err
	}

	e2e.Logf("Successfully disabled STONITH on node: %s", remoteNodeIP)
	return output, stderr, nil
}

// PcsStonithEnable enables STONITH in the pacemaker cluster.
//
//	stdout, stderr, err := PcsStonithEnable(nodeIP, sshConfig, localKH, remoteKH)
func PcsStonithEnable(remoteNodeIP string, sshConfig *core.SSHConfig, localKnownHostsPath, remoteKnownHostsPath string) (string, string, error) {
	e2e.Logf("Enabling STONITH on node: %s", remoteNodeIP)

	output, stderr, err := core.ExecuteRemoteSSHCommand(remoteNodeIP, formatPcsCommandString(pcsStonithEnable, noEnvVars), sshConfig, localKnownHostsPath, remoteKnownHostsPath)
	if err != nil {
		e2e.Logf("ERROR: Failed to enable STONITH on node %s (stderr: %s): %v", remoteNodeIP, stderr, err)
		return output, stderr, err
	}

	e2e.Logf("Successfully enabled STONITH on node: %s", remoteNodeIP)
	return output, stderr, nil
}

// PcsProperty gets cluster properties from pacemaker.
//
//	stdout, stderr, err := PcsProperty(nodeIP, sshConfig, localKH, remoteKH)
func PcsProperty(remoteNodeIP string, sshConfig *core.SSHConfig, localKnownHostsPath, remoteKnownHostsPath string) (string, string, error) {
	e2e.Logf("Getting pcs property on node: %s", remoteNodeIP)

	output, stderr, err := core.ExecuteRemoteSSHCommand(remoteNodeIP, formatPcsCommandString(pcsProperty, noEnvVars), sshConfig, localKnownHostsPath, remoteKnownHostsPath)
	if err != nil {
		e2e.Logf("ERROR: Failed to get pcs property on node %s (stderr: %s): %v", remoteNodeIP, stderr, err)
		return output, stderr, err
	}

	e2e.Logf("Successfully got pcs property on node: %s", remoteNodeIP)
	return output, stderr, nil
}

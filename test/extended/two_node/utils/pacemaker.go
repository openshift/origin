// Package utils provides Pacemaker cluster management utilities for two-node OpenShift cluster testing.
//
// This package enables management and recovery operations for Pacemaker-managed etcd clusters in
// two-node OpenShift deployments. It provides high-level functions for cluster operations, resource
// management, and disaster recovery scenarios.
//
// Background:
//
// Two-node OpenShift clusters use Pacemaker to manage etcd quorum and provide high availability.
// Pacemaker uses the PCS (Pacemaker Configuration System) command-line tool for cluster management.
// This package wraps PCS commands and provides utilities specific to two-node cluster recovery.
//
// Key Features:
//   - Pacemaker cluster status monitoring
//   - etcd resource management (start, stop, debug operations)
//   - STONITH (node fencing) control
//   - Cluster membership management (add/remove nodes)
//   - etcd revision file restoration
//   - Node job cleanup for test scenarios
//   - Retry utilities for handling transient failures
//
// Error Handling:
//
// All functions in this package return errors instead of using assertions (o.Expect).
// This makes them suitable for use as library functions. Calling code should check
// and handle errors appropriately, typically using o.Expect() in test code.
//
// Common Usage Patterns:
//
// 1. Monitoring Cluster Status:
//
//	status, stderr, err := PcsStatus(remoteNodeIP, sshConfig, localKnownHostsPath, remoteKnownHostsPath)
//	resourceStatus, stderr, err := PcsResourceStatus("master-0", remoteNodeIP, sshConfig, localKnownHostsPath, remoteKnownHostsPath)
//	journal, stderr, err := PcsJournal(remoteNodeIP, sshConfig, localKnownHostsPath, remoteKnownHostsPath)
//
// 2. Quorum Recovery Operations:
//
//	// Disable STONITH before recovery
//	_, _, err := PcsDisableStonith(remoteNodeIP, sshConfig, localKnownHostsPath, remoteKnownHostsPath)
//
//	// Restore etcd quorum on remote node
//	_, _, err := PcsDebugRestart(remoteNodeIP, sshConfig, localKnownHostsPath, remoteKnownHostsPath)
//
//	// Re-enable STONITH after recovery
//	_, _, err := PcsEnableStonith(remoteNodeIP, sshConfig, localKnownHostsPath, remoteKnownHostsPath)
//
// 3. Node Replacement Operations:
//
//	// Remove old node and add replacement
//	err := CycleRemovedNode(failedNodeName, failedNodeIP, runningNodeName, runningNodeIP, sshConfig, localKnownHostsPath, remoteKnownHostsPath)
//	if err != nil {
//	    return fmt.Errorf("failed to cycle node: %w", err)
//	}
//
//	// Restore etcd revision on new node
//	err = RestoreEtcdRevision(nodeName, remoteNodeIP, sshConfig, localKnownHostsPath, remoteKnownHostsPath, oc)
//	if err != nil {
//	    return fmt.Errorf("failed to restore etcd revision: %w", err)
//	}
//
//	// Clean up old jobs
//	err = DeleteNodeJobs(authJobName, afterSetupJobName, oc)
//	if err != nil {
//	    return fmt.Errorf("failed to delete jobs: %w", err)
//	}
//
// STONITH (Shoot The Other Node In The Head):
//
// STONITH is Pacemaker's node-level fencing mechanism that ensures cluster integrity by forcefully
// powering off or isolating unresponsive nodes. During recovery operations, STONITH is typically
// disabled to prevent automatic fencing, then re-enabled after the cluster is stable.
//
// Two-Node Quorum Challenge:
//
// In a two-node cluster, losing one node means losing quorum (majority). If fencing is properly enabled,
// pacemaker will restore quorum automatically by fencing the failed node and restarting the running node
// as a cluster of one. However, in the case that fencing fails, the PcsDebugRestart function can be used to
// bypass normal cluster checks and force etcd to start on the running node, restoring cluster operations
// until the failed node can be recovered or replaced.
//
// All PCS commands are executed on cluster nodes via two-hop SSH connections through a hypervisor,
// using the SSH utilities from this package.
package utils

import (
	"fmt"
	"time"

	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/klog/v2"
)

// Pacemaker-related constants
const (
	superuserPrefix = "sudo"
	pcsExecutable   = "pcs"
	noEnvVars       = ""

	// PCS commands
	pcsClusterNodeAdd            = "cluster node add %s addr=%s --start --enable"
	pcsResourceDebugStop         = "resource debug-stop etcd --full"
	pcsResourceDebugStartEnvVars = "OCF_RESKEY_CRM_meta_notify_start_resource='etcd'"
	pcsResourceDebugStart        = "resource debug-start etcd --full"
	pcsDisableStonith            = "property set stonith-enabled=false"
	pcsEnableStonith             = "property set stonith-enabled=true"
	pcsClusterNodeRemove         = "cluster node remove %s"
	pcsResourceStatus            = "resource status etcd node=%s"
	pcsStatus                    = "status"

	etcdNamespace        = "openshift-etcd"
	mkdirEtcdDir         = "sudo mkdir /var/lib/etcd"
	chmodEtcdDir         = "sudo chmod %o /var/lib/etcd"
	revisionJSONTemplate = `{"clusterId":"0","raftIndex":{"https://%s:%s":0},"maxRaftIndex":0,"created":""}`
	etcdDirPermissions   = 0766
	etcdFilePermissions  = 0644
	etcdPort             = "2379"
	chmodRevisionJSON    = "sudo chmod %o /var/lib/etcd/revision.json"
)

func formatPcsCommandString(command string, envVars string) string {
	if envVars != "" {
		return fmt.Sprintf("%s %s %s %s", superuserPrefix, envVars, pcsExecutable, command)
	}

	return fmt.Sprintf("%s %s %s", superuserPrefix, pcsExecutable, command)
}

// PcsDebugRestart restores etcd quorum on a node by performing a debug stop and start.
// This is used in single-node quorum recovery scenarios after a node failure.
//
// The function performs the following operations:
//  1. Stops the etcd resource using "pcs resource debug-stop etcd --full"
//  2. Starts the etcd resource using "pcs resource debug-start etcd --full" with notify metadata
//  3. Verifies the operation by checking pacemaker status
//
// This is critical for two-node clusters where losing one node would normally prevent etcd from
// achieving quorum. The debug-start bypasses normal cluster checks to force etcd to start.
//
// Parameters:
//   - remoteNodeIP: IP address of the remote node to restore etcd on
//   - sshConfig: SSH configuration for connecting to the hypervisor
//   - localKnownHostsPath: Path to the known_hosts file for the hypervisor connection
//   - remoteKnownHostsPath: Path to the known_hosts file on the hypervisor for the node connection
//
// Returns:
//   - string: Command stdout
//   - string: Command stderr
//   - error: Any error that occurred during the restart operation
func PcsDebugRestart(remoteNodeIP string, sshConfig *SSHConfig, localKnownHostsPath, remoteKnownHostsPath string) (string, string, error) {
	klog.V(2).Infof("Restoring etcd quorum on remote node: %s", remoteNodeIP)

	// SSH to hypervisor, then to remote node to run pcs debug-start
	// We need to chain the SSH commands: host -> hypervisor -> remote node
	output, stderr, err := ExecuteRemoteSSHCommand(remoteNodeIP, fmt.Sprintf("%s && %s", formatPcsCommandString(pcsResourceDebugStop, noEnvVars), formatPcsCommandString(pcsResourceDebugStart, pcsResourceDebugStartEnvVars)), sshConfig, localKnownHostsPath, remoteKnownHostsPath)
	if err != nil {
		klog.ErrorS(err, "Failed to restart etcd", "node", remoteNodeIP, "stderr", stderr)
		return output, stderr, err
	}

	// Log pacemaker status to check if etcd has been started on the remote node
	pcsStatusOutput, stderr, err := PcsStatus(remoteNodeIP, sshConfig, localKnownHostsPath, remoteKnownHostsPath)
	if err != nil {
		klog.Warning("Failed to get pacemaker status on remote node", "node", remoteNodeIP, "error", err)
	} else {
		klog.V(4).Infof("Pacemaker status on remote node %s:\n%s", remoteNodeIP, pcsStatusOutput)
	}

	klog.V(2).Infof("Successfully restored etcd quorum on remote node: %s", remoteNodeIP)
	return output, stderr, nil
}

// PcsDebugStart restores etcd quorum on a node by performing a debug start.
// This is used in single-node quorum recovery scenarios after a node failure.
//
// The function performs the following operations:
//  1. Starts the etcd resource using "pcs resource debug-start etcd --full" with notify metadata
//  2. Verifies the operation by checking pacemaker status
//
// This is critical for two-node clusters where losing one node would normally prevent etcd from
// achieving quorum. The debug-start bypasses normal cluster checks to force etcd to start.
//
// Parameters:
//   - remoteNodeIP: IP address of the remote node to restore etcd on
//   - sshConfig: SSH configuration for connecting to the hypervisor
//   - localKnownHostsPath: Path to the known_hosts file for the hypervisor connection
//   - remoteKnownHostsPath: Path to the known_hosts file on the hypervisor for the node connection
//
// Returns:
//   - string: Command stdout
//   - string: Command stderr
//   - error: Any error that occurred during the restart operation
func PcsDebugStart(remoteNodeIP string, sshConfig *SSHConfig, localKnownHostsPath, remoteKnownHostsPath string) (string, string, error) {
	klog.V(2).Infof("Restoring etcd quorum on remote node: %s", remoteNodeIP)

	// SSH to hypervisor, then to remote node to run pcs debug-start
	// We need to chain the SSH commands: host -> hypervisor -> remote node
	output, stderr, err := ExecuteRemoteSSHCommand(remoteNodeIP, formatPcsCommandString(pcsResourceDebugStart, pcsResourceDebugStartEnvVars), sshConfig, localKnownHostsPath, remoteKnownHostsPath)
	if err != nil {
		klog.ErrorS(err, "Failed to restart etcd", "node", remoteNodeIP, "stderr", stderr)
		return output, stderr, err
	}

	// Log pacemaker status to check if etcd has been started on the remote node
	pcsStatusOutput, stderr, err := PcsStatus(remoteNodeIP, sshConfig, localKnownHostsPath, remoteKnownHostsPath)
	if err != nil {
		klog.Warning("Failed to get pacemaker status on remote node", "node", remoteNodeIP, "error", err)
	} else {
		klog.V(4).Infof("Pacemaker status on remote node %s:\n%s", remoteNodeIP, pcsStatusOutput)
	}

	klog.V(2).Infof("Successfully restored etcd quorum on remote node: %s", remoteNodeIP)
	return output, stderr, nil
}

// PcsStatus retrieves the overall pacemaker cluster status.
// This shows the state of all cluster resources, nodes, and any failures.
func PcsStatus(remoteNodeIP string, sshConfig *SSHConfig, localKnownHostsPath, remoteKnownHostsPath string) (string, string, error) {
	return ExecuteRemoteSSHCommand(remoteNodeIP, formatPcsCommandString(pcsStatus, noEnvVars), sshConfig, localKnownHostsPath, remoteKnownHostsPath)
}

// PcsResourceStatus retrieves the status of a specific pacemaker resource (etcd) on a node.
// This is more targeted than PcsStatus and shows whether the etcd resource is started/stopped.
func PcsResourceStatus(nodeName, remoteNodeIP string, sshConfig *SSHConfig, localKnownHostsPath, remoteKnownHostsPath string) (string, string, error) {
	return ExecuteRemoteSSHCommand(remoteNodeIP, formatPcsCommandString(fmt.Sprintf(pcsResourceStatus, nodeName), noEnvVars), sshConfig, localKnownHostsPath, remoteKnownHostsPath)
}

// PcsDisableStonith disables STONITH (Shoot The Other Node In The Head) in the pacemaker cluster.
// This is typically done during maintenance or recovery operations to prevent automatic fencing.
func PcsDisableStonith(remoteNodeIP string, sshConfig *SSHConfig, localKnownHostsPath, remoteKnownHostsPath string) (string, string, error) {
	return ExecuteRemoteSSHCommand(remoteNodeIP, formatPcsCommandString(pcsDisableStonith, noEnvVars), sshConfig, localKnownHostsPath, remoteKnownHostsPath)
}

// PcsEnableStonith re-enables STONITH in the pacemaker cluster after maintenance is complete.
// STONITH provides node-level fencing to ensure cluster integrity.
func PcsEnableStonith(remoteNodeIP string, sshConfig *SSHConfig, localKnownHostsPath, remoteKnownHostsPath string) (string, string, error) {
	return ExecuteRemoteSSHCommand(remoteNodeIP, formatPcsCommandString(pcsEnableStonith, noEnvVars), sshConfig, localKnownHostsPath, remoteKnownHostsPath)
}

// PcsJournal retrieves the last pcsJournalTailLines lines of the pacemaker systemd journal logs.
// This is useful for debugging pacemaker behavior and troubleshooting cluster issues.
func PcsJournal(pcsJournalTailLines int, remoteNodeIP string, sshConfig *SSHConfig, localKnownHostsPath, remoteKnownHostsPath string) (string, string, error) {
	return ExecuteRemoteSSHCommand(remoteNodeIP, fmt.Sprintf("sudo journalctl -u pacemaker --no-pager | grep podman-etcd | tail -n %d", pcsJournalTailLines), sshConfig, localKnownHostsPath, remoteKnownHostsPath)
}

// RestoreEtcdRevision restores the etcd revision.json file on a replacement node and triggers etcd redeployment.
// This is a critical step in node replacement to ensure the new node can join the etcd cluster correctly.
//
// The function performs the following steps:
//  1. Creates /var/lib/etcd directory on the new node
//  2. Sets appropriate permissions on the directory (0766)
//  3. Creates revision.json with cluster metadata pointing to the new node's IP
//  4. Sets file permissions on revision.json (0644)
//  5. Triggers an etcd redeployment via the etcd operator using forceRedeploymentReason
//
// Parameters:
//   - nodeName: Name of the replacement OpenShift node (unused but kept for clarity)
//   - remoteNodeIP: IP address of the replacement node
//   - sshConfig: SSH configuration for connecting to the hypervisor
//   - localKnownHostsPath: Path to the known_hosts file for the hypervisor connection
//   - remoteKnownHostsPath: Path to the known_hosts file on the hypervisor for the node connection
//   - oc: OpenShift CLI client for patching the etcd operator
//
// Returns:
//   - error: Any error encountered during revision file creation or etcd redeployment
func RestoreEtcdRevision(nodeName, remoteNodeIP string, sshConfig *SSHConfig, localKnownHostsPath, remoteKnownHostsPath string, oc *exutil.CLI) error {
	// Create the revision.json file on the new node using constants
	revisionScript := fmt.Sprintf(`
		%s
		%s
		echo '%s' | sudo tee -a /var/lib/etcd/revision.json
		%s
	`, mkdirEtcdDir, fmt.Sprintf(chmodEtcdDir, etcdDirPermissions), fmt.Sprintf(revisionJSONTemplate, remoteNodeIP, etcdPort), fmt.Sprintf(chmodRevisionJSON, etcdFilePermissions))

	_, _, err := ExecuteRemoteSSHCommand(remoteNodeIP, revisionScript, sshConfig, localKnownHostsPath, remoteKnownHostsPath)
	if err != nil {
		return fmt.Errorf("failed to create etcd revision.json on node %s: %w", remoteNodeIP, err)
	}

	// Redeploy etcd with a force redeployment reason
	forceRedeploymentReason := fmt.Sprintf("recovery-%s", time.Now().Format(time.RFC3339Nano))
	_, err = oc.AsAdmin().Run("patch").Args("etcd", "cluster", "-p", fmt.Sprintf(`{"spec": {"forceRedeploymentReason": "%s"}}`, forceRedeploymentReason), "--type=merge").Output()
	if err != nil {
		return fmt.Errorf("failed to trigger etcd redeployment: %w", err)
	}

	klog.V(2).Infof("Successfully restored etcd revision on node %s and triggered redeployment", remoteNodeIP)
	return nil
}

// CycleRemovedNode removes and re-adds a node in the pacemaker cluster configuration.
// This is necessary when replacing a failed node to update the cluster membership.
//
// The function executes two pcs commands on the remote node:
//  1. "pcs cluster node remove <failedNodeName>" - removes the old/failed node
//  2. "pcs cluster node add <failedNodeName> addr=<targetNodeIP> --start --enable" - adds the replacement node
//
// Parameters:
//   - failedNodeName: Name of the replacement node to cycle
//   - failedNodeIP: IP address of the replacement node
//   - runningNodeIP: IP address of the remote node where commands are executed
//   - sshConfig: SSH configuration for connecting to the hypervisor
//   - localKnownHostsPath: Path to the known_hosts file for the hypervisor connection
//   - remoteKnownHostsPath: Path to the known_hosts file on the hypervisor for the node connection
//
// Returns:
//   - error: Any error encountered during node removal or addition
func CycleRemovedNode(failedNodeName, failedNodeIP, runningNodeIP string, sshConfig *SSHConfig, localKnownHostsPath, remoteKnownHostsPath string) error {
	// Remove and re-add the node in pacemaker using constants
	pcsScript := fmt.Sprintf(`
		%s
		%s
	`,
		formatPcsCommandString(fmt.Sprintf(pcsClusterNodeRemove, failedNodeName), noEnvVars),
		formatPcsCommandString(fmt.Sprintf(pcsClusterNodeAdd, failedNodeName, failedNodeIP), noEnvVars),
	)

	_, _, err := ExecuteRemoteSSHCommand(runningNodeIP, pcsScript, sshConfig, localKnownHostsPath, remoteKnownHostsPath)
	if err != nil {
		return fmt.Errorf("failed to cycle node %s in pacemaker cluster: %w", failedNodeName, err)
	}

	klog.V(2).Infof("Successfully cycled node %s in pacemaker cluster", failedNodeName)
	return nil
}

// DeleteNodeJobs deletes TNF (Two Node Federation) related jobs for node authentication and setup.
// These jobs need to be cleaned up during node replacement to allow new jobs to be created.
//
// Parameters:
//   - authJobName: Name of the TNF authentication job to delete (e.g., "tnf-auth-job-master-0")
//   - afterSetupJobName: Name of the TNF after-setup job to delete (e.g., "tnf-after-setup-job-master-0")
//   - oc: OpenShift CLI client for deleting the jobs
//
// Returns:
//   - error: Any error encountered during job deletion
func DeleteNodeJobs(authJobName, afterSetupJobName string, oc *exutil.CLI) error {
	// Delete the old tnf-auth-job using dynamic name
	_, err := oc.AsAdmin().Run("delete").Args("job", authJobName, "-n", etcdNamespace).Output()
	if err != nil {
		return fmt.Errorf("failed to delete job %s: %w", authJobName, err)
	}
	klog.V(2).Infof("Deleted job %s", authJobName)

	// Delete the old tnf-after-setup-job using dynamic name
	_, err = oc.AsAdmin().Run("delete").Args("job", afterSetupJobName, "-n", etcdNamespace).Output()
	if err != nil {
		return fmt.Errorf("failed to delete job %s: %w", afterSetupJobName, err)
	}
	klog.V(2).Infof("Deleted job %s", afterSetupJobName)

	return nil
}

// RetryOperationWithTimeout retries an operation until it succeeds or times out.
// This is a general-purpose retry utility used throughout the two-node test utilities.
//
// The function polls the operation at regular intervals until either:
//   - The operation succeeds (returns nil error)
//   - The timeout is exceeded
//
// This is useful for operations that may fail temporarily due to cluster state transitions,
// API server unavailability, or resource propagation delays.
//
// Parameters:
//   - operation: Function to execute that returns an error (nil on success)
//   - timeout: Maximum time to wait for the operation to succeed
//   - pollInterval: Time to wait between retry attempts
//   - operationName: Descriptive name for logging purposes
//
// Returns:
//   - error: nil if operation succeeded, timeout error if it failed within the timeout period
//
// Example:
//
//	err := RetryOperationWithTimeout(func() error {
//	    _, err := oc.AsAdmin().Run("get").Args("node", "master-0").Output()
//	    return err
//	}, 5*time.Minute, 10*time.Second, "get node master-0")
func RetryOperationWithTimeout(operation func() error, timeout, pollInterval time.Duration, operationName string) error {
	startTime := time.Now()

	for time.Since(startTime) < timeout {
		err := operation()
		if err == nil {
			klog.V(2).Infof("Operation %s succeeded after %v", operationName, time.Since(startTime))
			return nil
		}

		klog.V(4).Infof("Operation %s failed, retrying in %v: %v", operationName, pollInterval, err)
		time.Sleep(pollInterval)
	}

	return fmt.Errorf("operation %s failed after %v timeout", operationName, timeout)
}

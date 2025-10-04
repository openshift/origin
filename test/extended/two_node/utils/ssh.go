// Package utils provides SSH utilities for remote command execution in two-node cluster tests.
//
// Supports direct SSH connections (local → hypervisor) and two-hop connections (local → hypervisor → node).
//
// Usage example:
//
//	// Prepare known_hosts files
//	localKnownHostsPath, err := PrepareLocalKnownHostsFile(hypervisorConfig)
//	remoteKnownHostsPath, err := PrepareRemoteKnownHostsFile(remoteNodeIP, hypervisorConfig, localKnownHostsPath)
//
//	// Execute commands
//	output, stderr, err := ExecuteSSHCommand("virsh list --all", hypervisorConfig, localKnownHostsPath)
//	output, stderr, err := ExecuteRemoteSSHCommand(remoteNodeIP, "oc get nodes", hypervisorConfig, localKnownHostsPath, remoteKnownHostsPath)
//
//	// Cleanup
//	CleanupRemoteKnownHostsFile(hypervisorConfig, localKnownHostsPath, remoteKnownHostsPath)
//	CleanupLocalKnownHostsFile(hypervisorConfig, localKnownHostsPath)
package utils

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"k8s.io/klog/v2"
)

// SSHConfig contains the configuration needed to establish SSH connections to remote hosts
type SSHConfig struct {
	IP             string // IP address of the remote host
	User           string // SSH username for authentication
	PrivateKeyPath string // Path to the SSH private key file
}

// SSH-related constants
const (
	// SSH command patterns
	sshStrictHostKeyChecking = "StrictHostKeyChecking=no"
	userKnownHostsFile       = "UserKnownHostsFile"
	sshKeyscanCommand        = "ssh-keyscan"
	sshConnectivityTest      = "echo 'SSH connectivity test successful'"

	// Startup operation timeouts and intervals
	vmStartTimeout      = 2 * time.Minute  // Maximum time to wait for VM startup
	vmStartPollInterval = 15 * time.Second // Interval between VM state checks

	// File paths
	knownHostsTempPrefix = "known_hosts_" // Prefix for temporary known_hosts files
	remoteInfix          = "remote_"      // Infix for remote known_hosts files
)

// PrepareLocalKnownHostsFile creates a temporary known_hosts file and scans the SSH host key.
// This prevents "permanently added" warnings that cause SSH commands to fail.
//
// Parameters:
//   - sshConfig: SSH configuration for the host to scan
//
// Returns:
//   - string: Path to the created temporary known_hosts file
//   - error: Any error that occurred during file creation or host key scanning
func PrepareLocalKnownHostsFile(sshConfig *SSHConfig) (string, error) {
	klog.V(2).Infof("Preparing local known_hosts file for %q", sshConfig.IP)

	// Create a temporary known hosts file
	tempFile, err := os.CreateTemp("", knownHostsTempPrefix+"*")
	if err != nil {
		klog.ErrorS(err, "Failed to create temporary known_hosts file")
		return "", err
	}

	knownHostsPath := tempFile.Name()
	tempFile.Close()

	// Use ssh-keyscan to get the host key and add it to our known hosts file
	keyscanCmd := exec.Command(sshKeyscanCommand, "-H", sshConfig.IP)
	keyscanOutput, err := keyscanCmd.Output()
	if err != nil {
		klog.ErrorS(err, "Failed to scan host key", "host", sshConfig.IP)
		return "", err
	}

	// Write the host key to our known hosts file with secure permissions (0600)
	err = os.WriteFile(knownHostsPath, []byte(keyscanOutput), 0600)
	if err != nil {
		klog.ErrorS(err, "Failed to write known_hosts file")
		return "", err
	}

	klog.V(2).Infof("Successfully created local known_hosts file: %q", knownHostsPath)
	return knownHostsPath, nil
}

// PrepareRemoteKnownHostsFile creates a known_hosts file on the proxy node for accessing the remote node.
// Used for two-hop SSH connections (local → proxy → remote).
//
// Parameters:
//   - remoteNodeIP: IP address of the remote node to scan
//   - proxyNodeSSHConfig: SSH configuration for the proxy node (hypervisor)
//   - localKnownHostsPath: Path to the local known_hosts file for connecting to the proxy node
//
// Returns:
//   - string: Path to the created remote known_hosts file on the proxy node
//   - error: Any error that occurred during file creation or host key scanning
func PrepareRemoteKnownHostsFile(remoteNodeIP string, proxyNodeSSHConfig *SSHConfig, localKnownHostsPath string) (string, error) {
	klog.V(2).Infof("Preparing remote known_hosts file on proxy node %q for remote node %q", proxyNodeSSHConfig.IP, remoteNodeIP)

	// Create a temporary known hosts file on the proxy node for the remote node
	knownHostsPath := fmt.Sprintf("/tmp/%s%s%s", knownHostsTempPrefix, remoteInfix, remoteNodeIP)

	// Use ssh-keyscan on the proxy node to get the remote node's host key and create the file
	// Capture stderr for logging instead of suppressing it
	keyscanCmd := fmt.Sprintf(`ssh-keyscan -H %s`, remoteNodeIP)
	keyscanOutput, stderr, err := ExecuteSSHCommand(keyscanCmd, proxyNodeSSHConfig, localKnownHostsPath)
	if err != nil {
		klog.ErrorS(err, "Failed to scan host key for remote node", "remoteNode", remoteNodeIP, "stderr", stderr)
		return "", err
	}

	// Log any warnings from ssh-keyscan
	if stderr != "" {
		klog.V(4).Infof("ssh-keyscan warnings for %s: %s", remoteNodeIP, stderr)
	}

	// Create the known hosts file on the proxy node with secure permissions
	createKnownHostsCmd := fmt.Sprintf(`echo '%s' > %s && chmod 600 %s`, strings.TrimSpace(keyscanOutput), knownHostsPath, knownHostsPath)
	_, _, err = ExecuteSSHCommand(createKnownHostsCmd, proxyNodeSSHConfig, localKnownHostsPath)
	if err != nil {
		klog.ErrorS(err, "Failed to create known_hosts file on proxy node")
		return "", err
	}

	klog.V(2).Infof("Successfully created remote known_hosts file: %q", knownHostsPath)
	return knownHostsPath, nil
}

// ExecuteSSHCommand executes a command on a remote host via SSH.
//
// Parameters:
//   - command: The command to execute on the remote host
//   - sshConfig: SSH configuration for the remote host
//   - knownHostsPath: Path to the known_hosts file to use for the connection
//
// Returns:
//   - string: Standard output from the command
//   - string: Standard error from the command
//   - error: Any error that occurred (only non-zero exit codes are treated as errors)
func ExecuteSSHCommand(command string, sshConfig *SSHConfig, knownHostsPath string) (string, string, error) {
	// Build the SSH command to run directly on the host
	sshArgs := []string{
		"-i", sshConfig.PrivateKeyPath,
		"-o", sshStrictHostKeyChecking,
		"-o", fmt.Sprintf("%s=%s", userKnownHostsFile, knownHostsPath),
		fmt.Sprintf("%s@%s", sshConfig.User, sshConfig.IP),
		command,
	}

	// Log the SSH command being executed
	klog.V(4).Infof("Executing SSH command on %q: ssh %s", sshConfig.IP, strings.Join(sshArgs, " "))

	// Execute SSH command directly on the host
	cmd := exec.Command("ssh", sshArgs...)

	// Capture stdout and stderr separately
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Log the output for debugging (debug level)
	if stdout.Len() > 0 {
		klog.V(5).Infof("SSH stdout: %q", stdout.String())
	}
	if stderr.Len() > 0 {
		klog.V(5).Infof("SSH stderr: %q", stderr.String())
	}

	// Only treat non-zero exit codes as errors
	// stderr may contain warnings or informational messages that don't indicate failure
	if err != nil {
		klog.ErrorS(err, "SSH command failed", "host", sshConfig.IP, "stderr", stderr.String())
		return stdout.String(), stderr.String(), fmt.Errorf("SSH command failed: %v, stderr: %q, stdout: %q", err, stderr.String(), stdout.String())
	}

	klog.V(4).Infof("SSH command completed successfully on %q", sshConfig.IP)
	return stdout.String(), stderr.String(), nil
}

// ExecuteRemoteSSHCommand executes a command on an OpenShift node via two-hop SSH (local → hypervisor → node).
// Uses 'core' user for the node connection.
//
// Parameters:
//   - remoteNodeIP: IP address of the remote node to execute the command on
//   - command: The command to execute on the remote node
//   - sshConfig: SSH configuration for the proxy node (hypervisor)
//   - localKnownHostsPath: Path to the local known_hosts file for connecting to the proxy node
//   - remoteKnownHostsPath: Path to the remote known_hosts file on the proxy node for connecting to the remote node
//
// Returns:
//   - string: Standard output from the command
//   - string: Standard error from the command
//   - error: Any error that occurred during command execution
func ExecuteRemoteSSHCommand(remoteNodeIP, command string, sshConfig *SSHConfig, localKnownHostsPath, remoteKnownHostsPath string) (string, string, error) {
	// Build the nested SSH command that will be executed on the hypervisor to reach the node
	// This creates: ssh -i key -o options -o UserKnownHostsFile=<remote> core@remoteNodeIP 'command'
	nestedSSHCommand := fmt.Sprintf("ssh -o %s -o %s=%s core@%s '%s'",
		sshStrictHostKeyChecking,
		userKnownHostsFile,
		remoteKnownHostsPath,
		remoteNodeIP,
		strings.ReplaceAll(command, "'", "'\\''"), // Escape single quotes in the command
	)

	// Log the full two-hop SSH command being executed
	klog.V(4).Infof("Executing two-hop SSH command to node %q via hypervisor %q", remoteNodeIP, sshConfig.IP)

	// Execute the nested SSH command on the hypervisor (which will SSH to the node)
	stdout, stderr, err := ExecuteSSHCommand(nestedSSHCommand, sshConfig, localKnownHostsPath)
	if err != nil {
		klog.ErrorS(err, "Remote SSH command to node failed", "node", remoteNodeIP, "stderr", stderr, "stdout", stdout)
	} else {
		klog.V(4).Infof("Successfully executed command on remote node %q", remoteNodeIP)
	}

	return stdout, stderr, err
}

// CleanupRemoteKnownHostsFile removes the temporary known_hosts file from the proxy node.
// Errors are logged but not critical.
//
// Parameters:
//   - sshConfig: SSH configuration for the proxy node
//   - localKnownHostsPath: Path to the local known_hosts file for connecting to the proxy node
//   - remoteKnownHostsPath: Path to the remote known_hosts file on the proxy node to remove
//
// Returns:
//   - error: Any error that occurred during cleanup (logged as warning, not critical)
func CleanupRemoteKnownHostsFile(sshConfig *SSHConfig, localKnownHostsPath string, remoteKnownHostsPath string) error {
	// Clean up the known hosts file on the proxy node (while we still have connectivity)
	if remoteKnownHostsPath == "" {
		klog.V(2).Info("No remote known_hosts file to clean up")
		return nil
	}

	klog.V(2).Infof("Cleaning up remote known_hosts file: %q", remoteKnownHostsPath)

	// Clean up the known hosts file on the proxy node
	_, _, err := ExecuteSSHCommand(fmt.Sprintf("rm -f %s", remoteKnownHostsPath), sshConfig, localKnownHostsPath)
	if err != nil {
		klog.Warning("Failed to clean up remote known_hosts file", "error", err)
		return err
	}

	klog.V(2).Info("Successfully cleaned up remote known_hosts file")
	return nil
}

// CleanupLocalKnownHostsFile removes the temporary local known hosts file.
// This should be called after completing operations that required the local known_hosts file.
//
// The function performs a non-critical cleanup operation. If the cleanup fails, it logs a warning
// but does not fail the test, as the temporary file will eventually be cleaned up by the system.
//
// Parameters:
//   - sshConfig: SSH configuration (used for logging context)
//   - knownHostsPath: Path to the local known_hosts file to remove
//
// Returns:
//   - error: Any error that occurred during cleanup (logged as warning, not critical)
func CleanupLocalKnownHostsFile(sshConfig *SSHConfig, knownHostsPath string) error {
	// Clean up the local known hosts file
	if knownHostsPath == "" {
		klog.V(2).Info("No local known_hosts file to clean up")
		return nil
	}

	klog.V(2).Infof("Cleaning up local known_hosts file: %q", knownHostsPath)

	err := os.Remove(knownHostsPath)
	if err != nil {
		klog.Warning("Failed to clean up local known_hosts file", "error", err)
		return err
	}

	klog.V(2).Info("Successfully cleaned up local known_hosts file")
	return nil
}

// VerifyConnectivity tests SSH connectivity to a remote host by executing a simple echo command.
// This is useful for verifying that SSH is properly configured before attempting more complex operations.
//
// Parameters:
//   - sshConfig: SSH configuration for the host to test connectivity to
//   - knownHostsPath: Path to the known_hosts file to use for the connection
//
// Returns:
//   - string: Standard output from the connectivity test command
//   - string: Standard error from the connectivity test command
//   - error: Any error that occurred during the connectivity test
func VerifyConnectivity(sshConfig *SSHConfig, knownHostsPath string) (string, string, error) {
	return ExecuteSSHCommand(sshConnectivityTest, sshConfig, knownHostsPath)
}

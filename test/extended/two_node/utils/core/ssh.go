// Package ssh provides SSH utilities: direct/two-hop connections, known hosts management, and remote file operations.
package core

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"

	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// SSHConfig contains configuration for SSH connections.
type SSHConfig struct {
	IP             string
	User           string
	PrivateKeyPath string
}

// SSH-related constants
const (
	// SSH command patterns
	sshStrictHostKeyChecking = "StrictHostKeyChecking=no"
	userKnownHostsFile       = "UserKnownHostsFile"
	sshKeyscanCommand        = "ssh-keyscan"
	sshConnectivityTest      = "echo 'SSH connectivity test successful'"

	// File paths
	knownHostsTempPrefix = "known_hosts_" // Prefix for temporary known_hosts files
	remoteInfix          = "remote_"      // Infix for remote known_hosts files
)

// PrepareLocalKnownHostsFile creates a temporary known_hosts file and scans the SSH host key.
//
//	knownHostsPath, err := PrepareLocalKnownHostsFile(sshConfig)
func PrepareLocalKnownHostsFile(sshConfig *SSHConfig) (string, error) {
	e2e.Logf("Preparing local known_hosts file for %q", sshConfig.IP)

	// Create a temporary known hosts file
	tempFile, err := os.CreateTemp("", knownHostsTempPrefix+"*")
	if err != nil {
		e2e.Logf("ERROR: Failed to create temporary known_hosts file: %v", err)
		return "", err
	}

	knownHostsPath := tempFile.Name()
	tempFile.Close()

	// Use ssh-keyscan to get the host key and add it to our known hosts file
	keyscanCmd := exec.Command(sshKeyscanCommand, "-H", sshConfig.IP)
	keyscanOutput, err := keyscanCmd.Output()
	if err != nil {
		e2e.Logf("ERROR: Failed to scan host key for host %s: %v", sshConfig.IP, err)
		return "", err
	}

	// Write the host key to our known hosts file with secure permissions (0600)
	err = os.WriteFile(knownHostsPath, []byte(keyscanOutput), 0600)
	if err != nil {
		e2e.Logf("ERROR: Failed to write known_hosts file: %v", err)
		return "", err
	}

	e2e.Logf("Successfully created local known_hosts file: %q", knownHostsPath)
	return knownHostsPath, nil
}

// PrepareRemoteKnownHostsFile creates a known_hosts file on the proxy node for two-hop SSH.
//
//	remoteKHPath, err := PrepareRemoteKnownHostsFile(nodeIP, proxySshConfig, localKHPath)
func PrepareRemoteKnownHostsFile(remoteNodeIP string, proxyNodeSSHConfig *SSHConfig, localKnownHostsPath string) (string, error) {
	e2e.Logf("Preparing remote known_hosts file on proxy node %q for remote node %q", proxyNodeSSHConfig.IP, remoteNodeIP)

	// Create a temporary known hosts file on the proxy node for the remote node
	knownHostsPath := fmt.Sprintf("/tmp/%s%s%s", knownHostsTempPrefix, remoteInfix, remoteNodeIP)

	// Use ssh-keyscan on the proxy node to get the remote node's host key and create the file
	// Capture stderr for logging instead of suppressing it
	keyscanCmd := fmt.Sprintf(`ssh-keyscan -H %s`, remoteNodeIP)
	keyscanOutput, stderr, err := ExecuteSSHCommand(keyscanCmd, proxyNodeSSHConfig, localKnownHostsPath)
	if err != nil {
		e2e.Logf("ERROR: Failed to scan host key for remote node %s (stderr: %s): %v", remoteNodeIP, stderr, err)
		return "", err
	}

	// Log any warnings from ssh-keyscan
	if stderr != "" {
		e2e.Logf("ssh-keyscan warnings for %s: %s", remoteNodeIP, stderr)
	}

	// Create the known hosts file on the proxy node with secure permissions
	createKnownHostsCmd := fmt.Sprintf(`echo '%s' > %s && chmod 600 %s`, strings.TrimSpace(keyscanOutput), knownHostsPath, knownHostsPath)
	_, _, err = ExecuteSSHCommand(createKnownHostsCmd, proxyNodeSSHConfig, localKnownHostsPath)
	if err != nil {
		e2e.Logf("ERROR: Failed to create known_hosts file on proxy node: %v", err)
		return "", err
	}

	e2e.Logf("Successfully created remote known_hosts file: %q", knownHostsPath)
	return knownHostsPath, nil
}

// ExecuteSSHCommand executes a command on a remote host via SSH.
//
//	stdout, stderr, err := ExecuteSSHCommand("ls /var/lib", sshConfig, knownHostsPath)
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
	e2e.Logf("Executing SSH command on %q: ssh %s", sshConfig.IP, strings.Join(sshArgs, " "))

	// Execute SSH command directly on the host
	cmd := exec.Command("ssh", sshArgs...)

	// Capture stdout and stderr separately
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Log the output for debugging (debug level)
	if stdout.Len() > 0 {
		e2e.Logf("SSH stdout: %q", stdout.String())
	}
	if stderr.Len() > 0 {
		e2e.Logf("SSH stderr: %q", stderr.String())
	}

	// Only treat non-zero exit codes as errors
	// stderr may contain warnings or informational messages that don't indicate failure
	if err != nil {
		e2e.Logf("ERROR: SSH command failed on host %s (stderr: %s): %v", sshConfig.IP, stderr.String(), err)
		return stdout.String(), stderr.String(), fmt.Errorf("SSH command failed: %v, stderr: %q, stdout: %q", err, stderr.String(), stdout.String())
	}

	e2e.Logf("SSH command completed successfully on %q", sshConfig.IP)
	return stdout.String(), stderr.String(), nil
}

// ExecuteRemoteSSHCommand executes a command via two-hop SSH (local → hypervisor → node).
//
//	stdout, stderr, err := ExecuteRemoteSSHCommand(nodeIP, "systemctl status etcd", sshConfig, localKH, remoteKH)
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
	e2e.Logf("Executing two-hop SSH command to node %q via hypervisor %q", remoteNodeIP, sshConfig.IP)

	// Execute the nested SSH command on the hypervisor (which will SSH to the node)
	stdout, stderr, err := ExecuteSSHCommand(nestedSSHCommand, sshConfig, localKnownHostsPath)
	if err != nil {
		e2e.Logf("ERROR: Remote SSH command to node %s failed (stderr: %s, stdout: %s): %v", remoteNodeIP, stderr, stdout, err)
	} else {
		e2e.Logf("Successfully executed command on remote node %q", remoteNodeIP)
	}

	return stdout, stderr, err
}

// CleanupRemoteKnownHostsFile removes the temporary known_hosts file from the proxy node.
//
//	err := CleanupRemoteKnownHostsFile(sshConfig, localKH, remoteKH)
func CleanupRemoteKnownHostsFile(sshConfig *SSHConfig, localKnownHostsPath string, remoteKnownHostsPath string) error {
	// Clean up the known hosts file on the proxy node (while we still have connectivity)
	if remoteKnownHostsPath == "" {
		e2e.Logf("No remote known_hosts file to clean up")
		return nil
	}

	e2e.Logf("Cleaning up remote known_hosts file: %q", remoteKnownHostsPath)

	// Clean up the known hosts file on the proxy node
	_, _, err := ExecuteSSHCommand(fmt.Sprintf("rm -f %s", remoteKnownHostsPath), sshConfig, localKnownHostsPath)
	if err != nil {
		e2e.Logf("WARNING: "+"Failed to clean up remote known_hosts file", "error", err)
		return err
	}

	e2e.Logf("Successfully cleaned up remote known_hosts file")
	return nil
}

// CleanupLocalKnownHostsFile removes the temporary local known hosts file.
//
//	err := CleanupLocalKnownHostsFile(sshConfig, knownHostsPath)
func CleanupLocalKnownHostsFile(sshConfig *SSHConfig, knownHostsPath string) error {
	// Clean up the local known hosts file
	if knownHostsPath == "" {
		e2e.Logf("No local known_hosts file to clean up")
		return nil
	}

	e2e.Logf("Cleaning up local known_hosts file: %q", knownHostsPath)

	err := os.Remove(knownHostsPath)
	if err != nil {
		e2e.Logf("WARNING: "+"Failed to clean up local known_hosts file", "error", err)
		return err
	}

	e2e.Logf("Successfully cleaned up local known_hosts file")
	return nil
}

// VerifyConnectivity tests SSH connectivity by executing a simple echo command.
//
//	stdout, stderr, err := VerifyConnectivity(sshConfig, knownHostsPath)
func VerifyConnectivity(sshConfig *SSHConfig, knownHostsPath string) (string, string, error) {
	return ExecuteSSHCommand(sshConnectivityTest, sshConfig, knownHostsPath)
}

// CreateRemoteFile creates a file on a remote host via SSH using base64 encoding for security.
//
//	err := CreateRemoteFile("/tmp/config.xml", xmlContent, SecureFileMode, sshConfig, knownHostsPath)
func CreateRemoteFile(filepath, content string, mode os.FileMode, sshConfig *SSHConfig, knownHostsPath string) error {
	// Base64-encode content to avoid any shell interpretation
	encodedContent := base64.StdEncoding.EncodeToString([]byte(content))

	// Validate filepath doesn't contain directory traversal
	if strings.Contains(filepath, "..") {
		return fmt.Errorf("filepath contains directory traversal: %s", filepath)
	}

	// Use printf with base64 decoding to safely write the file
	// This avoids any issues with special characters in the content
	createCommand := fmt.Sprintf(`printf '%%s' '%s' | base64 -d > %s && chmod %o %s`,
		encodedContent, filepath, mode, filepath)

	_, _, err := ExecuteSSHCommand(createCommand, sshConfig, knownHostsPath)
	if err != nil {
		return fmt.Errorf("failed to create remote file %s: %w", filepath, err)
	}

	e2e.Logf("Successfully created remote file: %s with mode %o", filepath, mode)
	return nil
}

// CreateRemoteTempFile creates a temporary file in /tmp on a remote host via SSH.
//
//	tmpPath, err := CreateRemoteTempFile("master-0.xml", xmlContent, SecureFileMode, sshConfig, knownHostsPath)
func CreateRemoteTempFile(filename, content string, mode os.FileMode, sshConfig *SSHConfig, knownHostsPath string) (string, error) {
	remotePath := fmt.Sprintf("/tmp/%s", filename)

	if err := CreateRemoteFile(remotePath, content, mode, sshConfig, knownHostsPath); err != nil {
		return "", fmt.Errorf("failed to create remote temp file %s: %w", remotePath, err)
	}

	e2e.Logf("Created remote temporary file: %s", remotePath)
	return remotePath, nil
}

// DeleteRemoteTempFile removes a temporary file from a remote host via SSH.
//
//	defer DeleteRemoteTempFile(tmpPath, sshConfig, knownHostsPath)
func DeleteRemoteTempFile(remotePath string, sshConfig *SSHConfig, knownHostsPath string) error {
	deleteCommand := fmt.Sprintf("rm -f %s", remotePath)

	_, stderr, err := ExecuteSSHCommand(deleteCommand, sshConfig, knownHostsPath)
	if err != nil {
		// Log warning but don't fail - cleanup is best-effort
		e2e.Logf("Warning: failed to delete remote temp file %s: %v (stderr: %s)", remotePath, err, stderr)
		return fmt.Errorf("failed to delete remote temp file %s: %w", remotePath, err)
	}

	e2e.Logf("Deleted remote temporary file: %s", remotePath)
	return nil
}

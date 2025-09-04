package utils

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
)

type SSHConfig struct {
	IP string
	User string
	PrivateKeyPath string
}

// SSH-related constants
const (
	// SSH command patterns
	sshStrictHostKeyChecking = "StrictHostKeyChecking=no"
	userKnownHostsFile    = "UserKnownHostsFile"
	sshKeyscanCommand        = "ssh-keyscan"
	sshConnectivityTest      = "echo 'SSH connectivity test successful'"

	// File paths
	knownHostsTempPrefix = "known_hosts_"
)

// SSH-related variables
var (
	knownHostsPath            string // Path to prepared known hosts file
)

// prepareKnownHostsFile creates a known hosts file and adds the hypervisor's host key
func PrepareKnownHostsFile(sshConfig *SSHConfig) {
	// Create a temporary known hosts file
	tempFile, err := os.CreateTemp("", knownHostsTempPrefix+"*")
	o.Expect(err).To(o.BeNil(), "Expected to create temporary known hosts file without error")
	knownHostsPath = tempFile.Name()
	tempFile.Close()

	// Use ssh-keyscan to get the hypervisor's host key and add it to our known hosts file
	keyscanCmd := exec.Command(sshKeyscanCommand, "-H", sshConfig.IP)
	keyscanOutput, err := keyscanCmd.Output()
	if err != nil {
		// If ssh-keyscan fails, we'll still proceed - the first SSH connection will add the key
		g.GinkgoT().Logf("Warning: ssh-keyscan failed for %s: %v", sshConfig.IP, err)
	} else {
		// Write the host key to our known hosts file
		err = writeFileWithPermissions(knownHostsPath, string(keyscanOutput), 0644)
		o.Expect(err).To(o.BeNil(), "Expected to write host key to known hosts file without error")
		g.GinkgoT().Logf("Added hypervisor host key to known hosts file: %s", knownHostsPath)
	}
}

// prepareSurvivingNodeKnownHostsFile creates a known hosts file on the hypervisor for the surviving node
func PrepareSurvivingNodeKnownHostsFile(nodeName, nodeIP string, sshConfig *SSHConfig) {
	// Create a temporary known hosts file on the hypervisor for the surviving node
	hypervisorKnownHostsPath := fmt.Sprintf("/tmp/known_hosts_survivor_%s", nodeIP)

	// Use ssh-keyscan on the hypervisor to get the surviving node's host key and create the file
	// ssh-keyscan only takes the hostname/IP, not the username
	// Redirect stderr to /dev/null to ignore warnings
	keyscanCmd := fmt.Sprintf(`ssh-keyscan -H %s 2>/dev/null`, nodeIP)
	keyscanOutput, err := ExecuteSSHCommand(keyscanCmd, sshConfig)
	o.Expect(err).To(o.BeNil(), "Expected ssh-keyscan to succeed for surviving node %s: %v", nodeName, err)

	// Create the known hosts file on the hypervisor
	createKnownHostsCmd := fmt.Sprintf(`echo '%s' > %s && chmod 644 %s`, strings.TrimSpace(keyscanOutput), hypervisorKnownHostsPath, hypervisorKnownHostsPath)
	_, err = ExecuteSSHCommand(createKnownHostsCmd, sshConfig)
	o.Expect(err).To(o.BeNil(), "Expected to create known hosts file on hypervisor for surviving node %s: %v", nodeName, err)

	g.GinkgoT().Logf("Created known hosts file on hypervisor for surviving node: %s", hypervisorKnownHostsPath)

	// Store the hypervisor path for use in chained SSH commands
	survivingNodeKnownHostsPath = hypervisorKnownHostsPath
}

// executeSSHCommand executes a command on the hypervisor via SSH directly from the host
func ExecuteSSHCommand(command string, sshConfig *SSHConfig) (string, error) {
	// Build the SSH command to run directly on the host
	sshArgs := []string{
		"-i", sshConfig.PrivateKeyPath,
		"-o", sshStrictHostKeyChecking,
		"-o", fmt.Sprintf("%s=%s", userKnownHostsFile, knownHostsPath),
		fmt.Sprintf("%s@%s", sshConfig.User, sshConfig.IP),
		command,
	}

	// Log the SSH command being executed
	g.GinkgoT().Logf("Executing SSH command: ssh %s", strings.Join(sshArgs, " "))

	// Execute SSH command directly on the host
	cmd := exec.Command("ssh", sshArgs...)

	// Capture stdout and stderr separately
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Log the output for debugging (debug level)
	if stdout.Len() > 0 {
		g.GinkgoT().Logf("[DEBUG] SSH stdout: %s", stdout.String())
	}
	if stderr.Len() > 0 {
		g.GinkgoT().Logf("[DEBUG] SSH stderr: %s", stderr.String())
	}

	// If there's anything in stderr, treat it as an error
	if stderr.Len() > 0 {
		return "", fmt.Errorf("SSH command failed with stderr: %s, stdout: %s", stderr.String(), stdout.String())
	}

	if err != nil {
		return "", fmt.Errorf("SSH command failed: %v, stdout: %s", err, stdout.String())
	}

	return stdout.String(), nil
}

// executeSSHToNode executes a command on an OpenShift node via SSH directly from the host
func ExecuteSSHToNode(nodeName, nodeIP, command string, sshConfig *SSHConfig) (string, error) {
	// Build the SSH command to run directly on the host
	sshArgs := []string{
		"-i", sshConfig.PrivateKeyPath,
		"-o", sshStrictHostKeyChecking,
		"-o", fmt.Sprintf("%s=%s", userKnownHostsFile, knownHostsPath),
		fmt.Sprintf("core@%s", nodeIP),
		command,
	}

	// Log the SSH command being executed
	g.GinkgoT().Logf("Executing SSH to node %s: ssh %s", nodeName, strings.Join(sshArgs, " "))

	// Execute SSH command directly on the host
	cmd := exec.Command("ssh", sshArgs...)

	// Capture stdout and stderr separately
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Log the output for debugging
	if stdout.Len() > 0 {
		g.GinkgoT().Logf("SSH to node %s stdout: %s", nodeName, stdout.String())
	}
	if stderr.Len() > 0 {
		g.GinkgoT().Logf("SSH to node %s stderr: %s", nodeName, stderr.String())
	}

	// If there's anything in stderr, treat it as an error
	if stderr.Len() > 0 {
		return "", fmt.Errorf("SSH to node %s failed with stderr: %s, stdout: %s", nodeName, stderr.String(), stdout.String())
	}

	if err != nil {
		return "", fmt.Errorf("SSH to node %s failed: %v, stdout: %s", nodeName, err, stdout.String())
	}

	return stdout.String(), nil
}

// executeSSHScript executes a script using SSH directly from the host
// This function is kept for backward compatibility but now uses direct SSH
func ExecuteSSHScript(nodeName, nodeIP, scriptName, script string, sshConfig *SSHConfig) {
	if strings.Contains(script, "sudo") {
		// This looks like a node command, execute it on the target node
		output, err := ExecuteSSHToNode(nodeName, nodeIP, script, sshConfig)
		if err != nil {
			o.Expect(err).To(o.BeNil(), fmt.Sprintf("SSH script %s failed: %v, output: %s", scriptName, err, output))
		}
	} else {
		// This looks like a hypervisor command
		output, err := ExecuteSSHCommand(script, sshConfig)
		if err != nil {
			o.Expect(err).To(o.BeNil(), fmt.Sprintf("SSH script %s failed: %v, output: %s", scriptName, err, output))
		}
	}
}

// verifyConnectivity verifies that we can connect to the target host
func VerifyConnectivity(sshConfig *SSHConfig) (string, error) {
	g.GinkgoT().Logf("Verifying hypervisor connectivity and virsh availability...")

	// Prepare known hosts file to avoid "permanently added" warnings
	PrepareKnownHostsFile(sshConfig)

	// Test basic SSH connectivity to the hypervisor
	return ExecuteSSHCommand(sshConnectivityTest, sshConfig)
}

// cleanupKnownHostsFile cleans up the temporary known hosts files
func CleanupKnownHostsFile(sshConfig *SSHConfig) {
	// Clean up the known hosts file on the hypervisor first (while we still have connectivity)
	if survivingNodeKnownHostsPath != "" {
		// Clean up the known hosts file on the hypervisor
		_, err := ExecuteSSHCommand(fmt.Sprintf("rm -f %s", survivingNodeKnownHostsPath), sshConfig)
		if err != nil {
			g.GinkgoT().Logf("Warning: Failed to clean up known hosts file on hypervisor: %v", err)
		}
		survivingNodeKnownHostsPath = ""
	}

	// Then clean up the local known hosts file
	if knownHostsPath != "" {
		os.Remove(knownHostsPath)
		knownHostsPath = ""
	}
}

// writeFileWithPermissions writes content to a file with specific permissions
func writeFileWithPermissions(filePath, content string, permissions os.FileMode) error {
	return os.WriteFile(filePath, []byte(content), permissions)
}

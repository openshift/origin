// Package utils provides hypervisor configuration and validation utilities for two-node cluster testing.
//
// Tests requiring hypervisor access should include the [Requires:HypervisorSSHConfig] annotation.
//
// Configuration can be provided via command-line flag or environment variable:
//
//	openshift-tests run openshift/two-node --with-hypervisor-json='{
//	  "IP": "192.168.111.1",
//	  "User": "root",
//	  "privateKeyPath": "/path/to/private/key"
//	}'
//
// Or:
//
//	export HYPERVISOR_CONFIG='{"IP":"192.168.111.1","User":"root","privateKeyPath":"/path/to/key"}'
//	openshift-tests run openshift/two-node
//
// Usage example:
//
//	if !exutil.HasHypervisorConfig() {
//	    utils.PrintHypervisorConfigUsage()
//	    return
//	}
//	config := exutil.GetHypervisorConfig()
//	utils.VerifyHypervisorConnectivity(&config, knownHostsPath)
package utils

import (
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	"k8s.io/klog/v2"
)

// PrintHypervisorConfigUsage prints usage instructions for configuring hypervisor SSH access.
// Call this when HasHypervisorConfig() returns false to provide configuration guidance.
func PrintHypervisorConfigUsage() {
	usageMessage := `
================================================================================
Two-Node Test Suite - Hypervisor Configuration Required
================================================================================

This test requires hypervisor SSH configuration to manage virtual machines
and perform node operations. The [Requires:HypervisorSSHConfig] annotation
indicates this requirement.

CONFIGURATION METHODS:

1. Command-Line Flag (recommended for interactive testing):

   openshift-tests run openshift/two-node --with-hypervisor-json='{
     "IP": "192.168.111.1",
     "User": "root",
     "privateKeyPath": "/path/to/private/key"
   }'

2. Environment Variable (recommended for CI/CD):

   export HYPERVISOR_CONFIG='{"IP":"192.168.111.1","User":"root","privateKeyPath":"/path/to/key"}'
   openshift-tests run openshift/two-node

CONFIGURATION FIELDS:

- IP: IP address or hostname of the hypervisor
- User: SSH username (typically "root")
- privateKeyPath: Absolute path to SSH private key file

TROUBLESHOOTING:

If configuration fails:
1. Verify JSON syntax is valid
2. Check that the private key file exists
3. Test SSH connectivity: ssh -i <privateKeyPath> <User>@<IP>
4. Verify virsh is available: ssh <User>@<IP> 'virsh version'

================================================================================
`
	g.GinkgoT().Logf(usageMessage)
}

// VerifyHypervisorAvailability verifies SSH connectivity to the hypervisor and checks
// that virsh and libvirt are available.
func VerifyHypervisorAvailability(sshConfig *SSHConfig, knownHostsPath string) error {
	klog.V(2).Infof("Verifying hypervisor connectivity to %s@%s", sshConfig.User, sshConfig.IP)

	// Test basic SSH connectivity
	output, _, err := VerifyConnectivity(sshConfig, knownHostsPath)
	if err != nil {
		klog.ErrorS(err, "Failed to establish SSH connection to hypervisor",
			"user", sshConfig.User,
			"host", sshConfig.IP,
			"output", output)
		klog.ErrorS(nil, "Ensure the hypervisor is accessible and SSH key is correct")
		return fmt.Errorf("failed to establish SSH connection to hypervisor %s@%s: %w", sshConfig.User, sshConfig.IP, err)
	}
	klog.V(2).Infof("SSH connectivity verified: %s", strings.TrimSpace(output))

	// Test virsh availability and basic functionality
	output, err = VerifyVirsh(sshConfig, knownHostsPath)
	if err != nil {
		klog.ErrorS(err, "virsh is not available or not working on hypervisor",
			"user", sshConfig.User,
			"host", sshConfig.IP,
			"output", output)
		klog.ErrorS(nil, "Ensure libvirt and virsh are installed on the hypervisor")
		return fmt.Errorf("virsh is not available or not working on hypervisor %s@%s: %w", sshConfig.User, sshConfig.IP, err)
	}
	klog.V(2).Infof("virsh availability verified: %s", strings.TrimSpace(output))

	// Test libvirt connection by listing VMs
	output, err = VirshListAllVMs(sshConfig, knownHostsPath)
	if err != nil {
		klog.ErrorS(err, "Failed to connect to libvirt on hypervisor",
			"user", sshConfig.User,
			"host", sshConfig.IP,
			"output", output)
		klog.ErrorS(nil, "Ensure libvirtd service is running and user has access")
		return fmt.Errorf("failed to connect to libvirt on hypervisor %s@%s: %w", sshConfig.User, sshConfig.IP, err)
	}
	klog.V(2).Infof("libvirt connection verified, found VMs: %s", strings.TrimSpace(output))

	klog.V(2).Infof("Hypervisor connectivity verification completed successfully")
	return nil
}

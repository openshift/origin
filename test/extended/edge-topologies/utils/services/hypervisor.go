// Package services provides hypervisor utilities: configuration validation, SSH connectivity checks, and virsh availability verification.
package services

import (
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	"github.com/openshift/origin/test/extended/edge-topologies/utils/core"
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
     "hypervisorIP": "192.168.111.1",
     "sshUser": "root",
     "privateKeyPath": "/path/to/private/key"
   }'

2. Environment Variable (recommended for CI/CD):

   export HYPERVISOR_CONFIG='{"hypervisorIP":"192.168.111.1","sshUser":"root","privateKeyPath":"/path/to/key"}'
   openshift-tests run openshift/two-node

CONFIGURATION FIELDS:

- hypervisorIP: IP address or hostname of the hypervisor
- sshUser: SSH username (typically "root")
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

// VerifyHypervisorAvailability verifies SSH connectivity and checks virsh/libvirt availability.
//
//	err := VerifyHypervisorAvailability(sshConfig, knownHostsPath)
func VerifyHypervisorAvailability(sshConfig *core.SSHConfig, knownHostsPath string) error {
	klog.V(2).Infof("Verifying hypervisor connectivity to %s@%s", sshConfig.User, sshConfig.IP)

	// Test basic SSH connectivity
	output, _, err := core.VerifyConnectivity(sshConfig, knownHostsPath)
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

// Package utils provides libvirt/virsh utilities for managing virtual machines in two-node cluster testing.
//
// This package enables VM lifecycle management, inspection, and configuration through the virsh
// command-line tool. It supports operations on remote hypervisors via SSH, making it suitable
// for test environments where VMs are managed on a separate hypervisor host.
//
// Key Features:
//   - VM lifecycle operations (define, start, stop, destroy, autostart)
//   - VM inspection (list VMs, get UUID, dump XML configuration)
//   - XML parsing for extracting network configuration (MAC addresses, bridges)
//   - VM discovery by MAC address correlation
//   - VM recreation from saved XML configurations
//   - Wait utilities for VM state transitions
//
// Error Handling:
//
// All functions return errors instead of using assertions. Virsh command failures,
// XML parsing errors, and timeout conditions are returned as errors for the calling
// code to handle appropriately.
//
// Common Usage Patterns:
//
// 1. Listing and Inspecting VMs:
//
//	vms, err := VirshListAllVMs(sshConfig, knownHostsPath)
//	uuid, err := VirshGetVMUUID("master-0", sshConfig, knownHostsPath)
//	xml, err := VirshDumpXML("master-0", sshConfig, knownHostsPath)
//
// 2. VM Lifecycle Management:
//
//	err := VirshStartVM("master-0", sshConfig, knownHostsPath)
//	err := WaitForVMToStart("master-0", sshConfig, knownHostsPath)
//	err := VirshDestroyVM("master-0", sshConfig, knownHostsPath)
//	err := VirshUndefineVM("master-0", sshConfig, knownHostsPath)
//
// 3. VM Network Configuration:
//
//	mac, err := ExtractMACAddressFromXML(xmlContent, "ostestbm")
//	vmName, err := GetVMNameByMACMatch("master-0", "52:54:00:12:34:56", "ostestpr", sshConfig, knownHostsPath)
//	uuid, mac, err := GetVMNetworkInfo("master-0", "ostestpr", sshConfig, knownHostsPath)
//
// 4. VM Recovery Operations:
//
//	err := RecreateVMFromXML("master-0", xmlContent, sshConfig, knownHostsPath)
//
// All virsh commands are executed on a remote hypervisor via SSH. The functions in this package
// wrap the low-level SSH utilities from this package to provide a higher-level API for VM management.
//
// Retry Utilities:
//
// Some operations like WaitForVMToStart use the RetryOperationWithTimeout utility from the
// pacemaker utilities package to handle transient failures and wait for state transitions.
//
// XML Parsing:
//
// The package includes structures for parsing libvirt domain XML, focusing on network configuration.
// The Domain, Devices, Interface, MAC, and Source types map to libvirt XML elements and enable
// programmatic extraction of VM configuration details.
package utils

import (
	"encoding/xml"
	"fmt"
	"strings"

	"k8s.io/klog/v2"
)

// Domain represents a libvirt domain (virtual machine) configuration
// It maps to the root <domain> element in libvirt XML
type Domain struct {
	XMLName xml.Name `xml:"domain"`
	Name    string   `xml:"name"`
	UUID    string   `xml:"uuid"`
	Devices Devices  `xml:"devices"`
}

// Devices contains the hardware devices attached to a VM
type Devices struct {
	Interfaces []Interface `xml:"interface"`
}

// Interface represents a network interface configuration in libvirt XML
type Interface struct {
	Type   string `xml:"type,attr"`
	MAC    MAC    `xml:"mac"`
	Source Source `xml:"source"`
}

// MAC contains the MAC address of a network interface
type MAC struct {
	Address string `xml:"address,attr"`
}

// Source specifies the network source (bridge, network, etc) for an interface
type Source struct {
	Bridge string `xml:"bridge,attr"`
}

// Constants for virsh commands
const (
	virshCommand          = "virsh"
	virshListAllName      = "list --all --name"
	virshConnectionOption = "-c qemu:///system"
)

// VerifyVirsh checks if virsh is available and working on the target host
// by executing 'virsh version' command
//
// Parameters:
//   - sshConfig: SSH configuration for connecting to the hypervisor
//   - knownHostsPath: Path to the known_hosts file for SSH
//
// Returns:
//   - string: The virsh version output
//   - error: Any error that occurred during the check
func VerifyVirsh(sshConfig *SSHConfig, knownHostsPath string) (string, error) {
	klog.V(4).Infof("VerifyVirsh: Checking virsh availability on %s", sshConfig.IP)
	output, err := VirshCommand("version", sshConfig, knownHostsPath)
	if err != nil {
		klog.ErrorS(err, "VerifyVirsh failed", "host", sshConfig.IP)
	} else {
		klog.V(2).Infof("VerifyVirsh: Success - %s", output)
	}
	return output, err
}

// VirshCommand executes a virsh command on the remote hypervisor via SSH
//
// Parameters:
//   - command: The virsh command to execute (without 'virsh' prefix)
//   - sshConfig: SSH configuration for connecting to the hypervisor
//   - knownHostsPath: Path to the known_hosts file for SSH
//
// Returns:
//   - string: The command output
//   - error: Any error that occurred during execution
func VirshCommand(command string, sshConfig *SSHConfig, knownHostsPath string) (string, error) {
	fullCommand := fmt.Sprintf("%s %s %s", virshCommand, virshConnectionOption, command)
	klog.V(4).Infof("VirshCommand: Executing '%s' on %s", fullCommand, sshConfig.IP)
	output, _, err := ExecuteSSHCommand(fullCommand, sshConfig, knownHostsPath)
	if err != nil {
		klog.ErrorS(err, "VirshCommand failed", "command", fullCommand, "host", sshConfig.IP)
	} else {
		klog.V(4).Infof("VirshCommand: Success - output length: %d bytes", len(output))
	}
	return output, err
}

// VirshDumpXML retrieves the XML configuration of a VM
//
// Parameters:
//   - vmName: Name of the VM to dump XML for
//   - sshConfig: SSH configuration for connecting to the hypervisor
//   - knownHostsPath: Path to the known_hosts file for SSH
//
// Returns:
//   - string: The VM's XML configuration
//   - error: Any error that occurred during retrieval
func VirshDumpXML(vmName string, sshConfig *SSHConfig, knownHostsPath string) (string, error) {
	klog.V(4).Infof("VirshDumpXML: Getting XML for VM '%s'", vmName)
	output, err := VirshCommand(fmt.Sprintf("dumpxml %s", vmName), sshConfig, knownHostsPath)
	if err != nil {
		klog.ErrorS(err, "VirshDumpXML failed", "vm", vmName)
	} else {
		klog.V(4).Infof("VirshDumpXML: Success for VM '%s' - XML length: %d bytes", vmName, len(output))
	}
	return output, err
}

// VirshListAllVMs lists all VMs (running and stopped) on the hypervisor
//
// Parameters:
//   - sshConfig: SSH configuration for connecting to the hypervisor
//   - knownHostsPath: Path to the known_hosts file for SSH
//
// Returns:
//   - string: Newline-separated list of VM names
//   - error: Any error that occurred during listing
func VirshListAllVMs(sshConfig *SSHConfig, knownHostsPath string) (string, error) {
	klog.V(4).Infof("VirshListAllVMs: Listing all VMs on %s", sshConfig.IP)
	output, err := VirshCommand(virshListAllName, sshConfig, knownHostsPath)
	if err != nil {
		klog.ErrorS(err, "VirshListAllVMs failed", "host", sshConfig.IP)
	} else {
		vmCount := len(strings.Fields(output))
		klog.V(2).Infof("VirshListAllVMs: Found %d VMs", vmCount)
	}
	return output, err
}

// VirshVMExists checks if a VM with the given name exists on the hypervisor
//
// Parameters:
//   - vmName: Name of the VM to check
//   - sshConfig: SSH configuration for connecting to the hypervisor
//   - knownHostsPath: Path to the known_hosts file for SSH
//
// Returns:
//   - string: Command output (empty if VM doesn't exist)
//   - error: Error if VM doesn't exist or command fails
func VirshVMExists(vmName string, sshConfig *SSHConfig, knownHostsPath string) (string, error) {
	klog.V(4).Infof("VirshVMExists: Checking if VM '%s' exists", vmName)
	output, err := VirshCommand(fmt.Sprintf("%s | grep -q %s", virshListAllName, vmName), sshConfig, knownHostsPath)
	if err != nil {
		klog.V(4).Infof("VirshVMExists: VM '%s' does not exist or grep failed - %v", vmName, err)
	} else {
		klog.V(2).Infof("VirshVMExists: VM '%s' exists", vmName)
	}
	return output, err
}

// VirshGetVMUUID retrieves the UUID of a VM
//
// Parameters:
//   - vmName: Name of the VM to get UUID for
//   - sshConfig: SSH configuration for connecting to the hypervisor
//   - knownHostsPath: Path to the known_hosts file for SSH
//
// Returns:
//   - string: The VM's UUID (trimmed of whitespace)
//   - error: Any error that occurred during retrieval
func VirshGetVMUUID(vmName string, sshConfig *SSHConfig, knownHostsPath string) (string, error) {
	klog.V(4).Infof("VirshGetVMUUID: Getting UUID for VM '%s'", vmName)
	output, err := VirshCommand(fmt.Sprintf("domuuid %s", vmName), sshConfig, knownHostsPath)
	uuid := strings.TrimSpace(output)
	if err != nil {
		klog.ErrorS(err, "VirshGetVMUUID failed", "vm", vmName)
	} else {
		klog.V(2).Infof("VirshGetVMUUID: VM '%s' has UUID: %s", vmName, uuid)
	}
	return uuid, err
}

// VirshUndefineVM undefines (removes the configuration of) a VM
// Note: This does not delete the VM's disk images, only the libvirt configuration
//
// Parameters:
//   - vmName: Name of the VM to undefine
//   - sshConfig: SSH configuration for connecting to the hypervisor
//   - knownHostsPath: Path to the known_hosts file for SSH
//
// Returns:
//   - error: Any error that occurred during undefine operation
func VirshUndefineVM(vmName string, sshConfig *SSHConfig, knownHostsPath string) error {
	klog.V(2).Infof("VirshUndefineVM: Undefining VM '%s' (including NVRAM)", vmName)
	_, err := VirshCommand(fmt.Sprintf("undefine %s --nvram", vmName), sshConfig, knownHostsPath)
	if err != nil {
		klog.ErrorS(err, "VirshUndefineVM failed", "vm", vmName)
	} else {
		klog.V(2).Infof("VirshUndefineVM: Successfully undefined VM '%s'", vmName)
	}
	return err
}

// VirshDestroyVM forcefully stops (destroys) a running VM
// This is equivalent to pulling the power plug on a physical machine
//
// Parameters:
//   - vmName: Name of the VM to destroy
//   - sshConfig: SSH configuration for connecting to the hypervisor
//   - knownHostsPath: Path to the known_hosts file for SSH
//
// Returns:
//   - error: Any error that occurred during destroy operation
func VirshDestroyVM(vmName string, sshConfig *SSHConfig, knownHostsPath string) error {
	klog.V(2).Infof("VirshDestroyVM: Forcefully stopping VM '%s'", vmName)
	_, err := VirshCommand(fmt.Sprintf("destroy %s", vmName), sshConfig, knownHostsPath)
	if err != nil {
		klog.ErrorS(err, "VirshDestroyVM failed", "vm", vmName)
	} else {
		klog.V(2).Infof("VirshDestroyVM: Successfully destroyed VM '%s'", vmName)
	}
	return err
}

// VirshDefineVM defines (registers) a new VM from an XML configuration file
//
// Parameters:
//   - xmlFilePath: Path to the XML file on the hypervisor containing VM configuration
//   - sshConfig: SSH configuration for connecting to the hypervisor
//   - knownHostsPath: Path to the known_hosts file for SSH
//
// Returns:
//   - error: Any error that occurred during define operation
func VirshDefineVM(xmlFilePath string, sshConfig *SSHConfig, knownHostsPath string) error {
	klog.V(2).Infof("VirshDefineVM: Defining VM from XML file '%s'", xmlFilePath)
	_, err := VirshCommand(fmt.Sprintf("define %s", xmlFilePath), sshConfig, knownHostsPath)
	if err != nil {
		klog.ErrorS(err, "VirshDefineVM failed", "xmlFile", xmlFilePath)
	} else {
		klog.V(2).Infof("VirshDefineVM: Successfully defined VM from '%s'", xmlFilePath)
	}
	return err
}

// VirshStartVM starts a defined VM
//
// Parameters:
//   - vmName: Name of the VM to start
//   - sshConfig: SSH configuration for connecting to the hypervisor
//   - knownHostsPath: Path to the known_hosts file for SSH
//
// Returns:
//   - error: Any error that occurred during start operation
func VirshStartVM(vmName string, sshConfig *SSHConfig, knownHostsPath string) error {
	klog.V(2).Infof("VirshStartVM: Starting VM '%s'", vmName)
	_, err := VirshCommand(fmt.Sprintf("start %s", vmName), sshConfig, knownHostsPath)
	if err != nil {
		klog.ErrorS(err, "VirshStartVM failed", "vm", vmName)
	} else {
		klog.V(2).Infof("VirshStartVM: Successfully started VM '%s'", vmName)
	}
	return err
}

// VirshAutostartVM enables autostart for a VM (starts automatically on hypervisor boot)
//
// Parameters:
//   - vmName: Name of the VM to enable autostart for
//   - sshConfig: SSH configuration for connecting to the hypervisor
//   - knownHostsPath: Path to the known_hosts file for SSH
//
// Returns:
//   - error: Any error that occurred during autostart enable operation
func VirshAutostartVM(vmName string, sshConfig *SSHConfig, knownHostsPath string) error {
	klog.V(2).Infof("VirshAutostartVM: Enabling autostart for VM '%s'", vmName)
	_, err := VirshCommand(fmt.Sprintf("autostart %s", vmName), sshConfig, knownHostsPath)
	if err != nil {
		klog.ErrorS(err, "VirshAutostartVM failed", "vm", vmName)
	} else {
		klog.V(2).Infof("VirshAutostartVM: Successfully enabled autostart for VM '%s'", vmName)
	}
	return err
}

// ExtractIPFromVMXML attempts to extract the IP address for a VM from its XML configuration
// Note: This typically does not work as IP addresses are usually assigned dynamically by DHCP
// and are not stored in the domain XML. This function is kept for reference but may need
// to be replaced with a different IP discovery mechanism (e.g., checking DHCP leases).
//
// Parameters:
//   - xmlContent: The VM's XML configuration as a string
//   - networkName: The name of the network bridge to find the interface for
//
// Returns:
//   - string: The IP address (typically empty as IPs aren't stored in XML)
//   - error: Error indicating IP addresses are not in domain XML or parsing failed
func ExtractIPFromVMXML(xmlContent, networkName string) (string, error) {
	klog.V(4).Infof("ExtractIPFromVMXML: Attempting to extract IP for network '%s'", networkName)

	var domain Domain
	err := xml.Unmarshal([]byte(xmlContent), &domain)
	if err != nil {
		klog.ErrorS(err, "ExtractIPFromVMXML failed to parse domain XML")
		return "", fmt.Errorf("failed to parse domain XML: %v", err)
	}

	klog.V(4).Infof("ExtractIPFromVMXML: Parsed domain '%s', checking %d interfaces", domain.Name, len(domain.Devices.Interfaces))

	// Look for the interface with the specified network
	for _, iface := range domain.Devices.Interfaces {
		klog.V(4).Infof("ExtractIPFromVMXML: Checking interface with bridge '%s'", iface.Source.Bridge)
		if iface.Source.Bridge == networkName {
			// Note: IP addresses are typically not stored in the domain XML
			// They are assigned dynamically by DHCP. This function might need
			// to be updated to get IP from a different source.
			klog.Warningf("Found interface for network '%s', but IPs are not in domain XML", networkName)
			klog.V(2).Infof("Found interface for network %s, but IP addresses are not stored in domain XML", networkName)
			return "", fmt.Errorf("interface found for network %s, but IP addresses are not stored in domain XML", networkName)
		}
	}

	klog.Warningf("No interface found for network '%s'", networkName)
	return "", fmt.Errorf("no interface found for network %s", networkName)
}

// ExtractMACAddressFromXML extracts the MAC address for a specific network bridge from VM XML.
// This parses the libvirt domain XML to find the network interface attached to the specified
// bridge and returns its MAC address.
//
// The function is commonly used to:
//   - Correlate VMs with OpenShift nodes by matching MAC addresses
//   - Retrieve network configuration for node replacement operations
//   - Discover VM network topology
//
// Parameters:
//   - xmlContent: The VM's XML configuration as a string (from virsh dumpxml)
//   - networkBridge: The name of the network bridge to find the MAC address for (e.g., "ostestbm", "ostestpr")
//
// Returns:
//   - string: The MAC address in standard format (e.g., "52:54:00:12:34:56")
//   - error: Error if parsing fails or no interface is found on the specified bridge
func ExtractMACAddressFromXML(xmlContent string, networkBridge string) (string, error) {
	klog.V(4).Infof("ExtractMACAddressFromXML: Extracting MAC for bridge '%s'", networkBridge)

	var domain Domain
	err := xml.Unmarshal([]byte(xmlContent), &domain)
	if err != nil {
		klog.ErrorS(err, "ExtractMACAddressFromXML failed to parse domain XML")
		return "", fmt.Errorf("failed to parse domain XML: %v", err)
	}

	klog.V(4).Infof("ExtractMACAddressFromXML: Parsed domain '%s', checking %d interfaces", domain.Name, len(domain.Devices.Interfaces))

	// Look for the interface with ostestpr bridge
	for _, iface := range domain.Devices.Interfaces {
		klog.V(4).Infof("ExtractMACAddressFromXML: Checking interface with bridge '%s', MAC '%s'", iface.Source.Bridge, iface.MAC.Address)
		if iface.Source.Bridge == networkBridge {
			klog.V(2).Infof("ExtractMACAddressFromXML: Found MAC address '%s' for bridge '%s'", iface.MAC.Address, networkBridge)
			klog.V(2).Infof("Found %s interface with MAC: %s", networkBridge, iface.MAC.Address)
			return iface.MAC.Address, nil
		}
	}

	klog.ErrorS(nil, "ExtractMACAddressFromXML: No interface found for bridge", "bridge", networkBridge)
	return "", fmt.Errorf("no %s interface found in domain XML", networkBridge)
}

// GetVMNameByMACMatch finds the VM name that has a specific MAC address on a given network bridge.
// This is used to correlate OpenShift nodes (identified by MAC address) with their underlying VMs.
//
// The function performs an exhaustive search by:
//  1. Listing all VMs on the hypervisor (both running and stopped)
//  2. For each VM, retrieving its XML configuration via virsh dumpxml
//  3. Parsing the XML to extract MAC addresses for interfaces on the specified bridge
//  4. Comparing the extracted MAC with the target MAC address
//  5. Returning the VM name when a match is found
//
// This is useful in node replacement scenarios where you need to find which VM corresponds
// to a specific OpenShift node based on its BareMetalHost MAC address.
//
// Parameters:
//   - nodeName: Name of the OpenShift node (used for logging and error messages)
//   - nodeMAC: The MAC address to search for (in format "52:54:00:xx:xx:xx")
//   - networkBridge: The network bridge name to check (e.g., "ostestpr" for provisioning network)
//   - sshConfig: SSH configuration for connecting to the hypervisor
//   - knownHostsPath: Path to the known_hosts file for SSH
//
// Returns:
//   - string: The name of the matching VM
//   - error: Error if no VM found with the specified MAC or if any operation fails
func GetVMNameByMACMatch(nodeName, nodeMAC string, networkBridge string, sshConfig *SSHConfig, knownHostsPath string) (string, error) {
	klog.V(4).Infof("GetVMNameByMACMatch: Searching for VM with MAC '%s' on bridge '%s' (node: %s)", nodeMAC, networkBridge, nodeName)

	// Get list of all VMs using SSH to hypervisor
	vmListOutput, err := VirshListAllVMs(sshConfig, knownHostsPath)
	klog.V(4).Infof("VirshListAllVMs output: %s", vmListOutput)
	if err != nil {
		klog.ErrorS(err, "GetVMNameByMACMatch failed to get VM list")
		return "", fmt.Errorf("failed to get VM list: %v", err)
	}

	vmNames := strings.Fields(vmListOutput)
	klog.V(4).Infof("GetVMNameByMACMatch: Found %d VMs to check: %v", len(vmNames), vmNames)
	klog.V(2).Infof("Found VMs: %v", vmNames)

	// Check each VM to find the one with matching MAC address
	for i, vmName := range vmNames {
		if vmName == "" {
			klog.V(4).Infof("GetVMNameByMACMatch: Skipping empty VM name at index %d", i)
			continue
		}

		klog.V(4).Infof("GetVMNameByMACMatch: Checking VM %d/%d: '%s'", i+1, len(vmNames), vmName)

		// Get VM XML configuration using SSH to hypervisor
		vmXML, err := VirshDumpXML(vmName, sshConfig, knownHostsPath)
		klog.V(4).Infof("Getting XML for VM: %s", vmName)
		if err != nil {
			klog.Warningf("Could not get XML for VM '%s', skipping - %v", vmName, err)
			continue
		}

		// Extract MAC address from VM XML for the ostestpr bridge
		vmMAC, err := ExtractMACAddressFromXML(vmXML, networkBridge)
		if err != nil {
			klog.Warningf("Could not extract MAC from VM '%s', skipping - %v", vmName, err)
			continue
		}

		klog.V(4).Infof("GetVMNameByMACMatch: VM '%s' has MAC '%s'", vmName, vmMAC)
		klog.V(2).Infof("VM %s has MAC %s", vmName, vmMAC)
		klog.V(4).Infof("Comparing VM MAC %s with target MAC %s", vmMAC, nodeMAC)

		// Check if this VM's MAC matches the node's MAC
		if vmMAC == nodeMAC {
			klog.V(2).Infof("GetVMNameByMACMatch: Found matching VM '%s' with MAC '%s'", vmName, vmMAC)
			klog.V(2).Infof("Found matching VM: %s (MAC: %s)", vmName, vmMAC)
			return vmName, nil
		}
	}

	klog.ErrorS(nil, "GetVMNameByMACMatch: No VM found with MAC", "mac", nodeMAC, "node", nodeName)
	return "", fmt.Errorf("no VM found with MAC address %s for node %s", nodeMAC, nodeName)
}

// GetVMNetworkInfo retrieves the UUID and MAC address for a VM's network interface
//
// Parameters:
//   - vmName: Name of the VM to get network info for
//   - networkBridge: The network bridge name to extract MAC address from
//   - sshConfig: SSH configuration for connecting to the hypervisor
//   - knownHostsPath: Path to the known_hosts file for SSH
//
// Returns:
//   - string: The VM's UUID
//   - string: The MAC address for the specified network bridge
//   - error: Any error that occurred during retrieval
func GetVMNetworkInfo(vmName string, networkBridge string, sshConfig *SSHConfig, knownHostsPath string) (string, string, error) {
	klog.V(4).Infof("GetVMNetworkInfo: Getting network info for VM '%s' on bridge '%s'", vmName, networkBridge)

	newUUID, err := VirshGetVMUUID(vmName, sshConfig, knownHostsPath)
	if err != nil {
		klog.ErrorS(err, "GetVMNetworkInfo failed to get UUID", "vm", vmName)
		return "", "", fmt.Errorf("failed to get VM UUID: %v", err)
	}

	newXMLOutput, err := VirshDumpXML(vmName, sshConfig, knownHostsPath)
	if err != nil {
		klog.ErrorS(err, "GetVMNetworkInfo failed to get XML", "vm", vmName)
		return "", "", fmt.Errorf("failed to get VM XML: %v", err)
	}

	newMACAddress, err := ExtractMACAddressFromXML(newXMLOutput, networkBridge)
	if err != nil {
		klog.ErrorS(err, "GetVMNetworkInfo failed to extract MAC", "vm", vmName)
		return "", "", fmt.Errorf("failed to find MAC address in VM XML: %v", err)
	}

	klog.V(2).Infof("GetVMNetworkInfo: Successfully retrieved info for VM '%s': UUID=%s, MAC=%s", vmName, newUUID, newMACAddress)
	return newUUID, newMACAddress, nil
}

// RecreateVMFromXML recreates a VM from its XML configuration.
// This is typically used during node replacement or disaster recovery scenarios.
//
// The function performs the following steps:
//  1. Validates VM name to prevent command injection
//  2. Checks if the VM already exists (skips recreation if it does)
//  3. Creates a temporary XML file on the hypervisor (/tmp/<vmName>.xml)
//  4. Defines the VM in libvirt using the XML configuration
//  5. Starts the VM
//  6. Enables autostart so the VM starts automatically on hypervisor boot
//  7. Cleans up the temporary XML file
//
// Security: The VM name is validated to prevent shell command injection attacks.
//
// Parameters:
//   - vmName: Name of the VM to recreate (must not contain shell metacharacters)
//   - xmlContent: The complete libvirt XML configuration for the VM
//   - sshConfig: SSH configuration for connecting to the hypervisor
//   - knownHostsPath: Path to the known_hosts file for SSH
//
// Returns:
//   - error: Any error that occurred during recreation (nil if VM already exists)
func RecreateVMFromXML(vmName, xmlContent string, sshConfig *SSHConfig, knownHostsPath string) error {
	klog.V(2).Infof("RecreateVMFromXML: Starting recreation of VM '%s'", vmName)

	// Validate VM name to prevent command injection
	if strings.ContainsAny(vmName, ";&|$`\\\"'<>()[]{}!*?~") {
		klog.ErrorS(nil, "RecreateVMFromXML: Invalid VM name contains shell metacharacters", "vmName", vmName)
		return fmt.Errorf("invalid VM name contains shell metacharacters: %s", vmName)
	}

	// Check if VM already exists using the dedicated function
	_, err := VirshVMExists(vmName, sshConfig, knownHostsPath)
	if err == nil {
		klog.V(2).Infof("RecreateVMFromXML: VM '%s' already exists, skipping recreation", vmName)
		klog.V(2).Infof("VM %s already exists, skipping recreation", vmName)
		return nil
	}
	klog.V(4).Infof("RecreateVMFromXML: VM '%s' does not exist, proceeding with recreation", vmName)

	// Create a temporary file on the hypervisor with the XML content
	createXMLCommand := fmt.Sprintf(`cat > /tmp/%s.xml <<'XML_EOF'
%s
XML_EOF`, vmName, xmlContent)

	klog.V(4).Infof("RecreateVMFromXML: Creating temporary XML file /tmp/%s.xml", vmName)
	_, _, err = ExecuteSSHCommand(createXMLCommand, sshConfig, knownHostsPath)
	if err != nil {
		klog.ErrorS(err, "RecreateVMFromXML failed to create XML file")
		return fmt.Errorf("failed to create XML file on hypervisor: %v", err)
	}

	// Redefine the VM using the backed up XML (using helper function)
	klog.V(4).Infof("RecreateVMFromXML: Defining VM '%s' from XML", vmName)
	err = VirshDefineVM(fmt.Sprintf("/tmp/%s.xml", vmName), sshConfig, knownHostsPath)
	if err != nil {
		klog.ErrorS(err, "RecreateVMFromXML failed to define VM")
		return fmt.Errorf("failed to define VM: %v", err)
	}

	// Start the VM (using helper function)
	klog.V(4).Infof("RecreateVMFromXML: Starting VM '%s'", vmName)
	err = VirshStartVM(vmName, sshConfig, knownHostsPath)
	if err != nil {
		klog.ErrorS(err, "RecreateVMFromXML failed to start VM")
		return fmt.Errorf("failed to start VM: %v", err)
	}

	// Enable autostart (using helper function)
	klog.V(4).Infof("RecreateVMFromXML: Enabling autostart for VM '%s'", vmName)
	err = VirshAutostartVM(vmName, sshConfig, knownHostsPath)
	if err != nil {
		klog.Warningf("Failed to enable autostart (non-fatal) - %v", err)
	}

	// Clean up temporary XML file
	klog.V(4).Infof("RecreateVMFromXML: Cleaning up temporary XML file /tmp/%s.xml", vmName)
	_, _, err = ExecuteSSHCommand(fmt.Sprintf("rm -f /tmp/%s.xml", vmName), sshConfig, knownHostsPath)
	if err != nil {
		klog.Warningf("Failed to clean up XML file (non-fatal) - %v", err)
	}

	klog.V(2).Infof("RecreateVMFromXML: Successfully recreated VM '%s'", vmName)
	klog.V(2).Infof("Recreated VM: %s", vmName)
	return nil
}

// WaitForVMToStart waits for a VM to reach running state with retry logic.
// This polls the VM state periodically until it reports as "running" or the timeout is exceeded.
//
// The function performs two checks:
//  1. Verifies the VM exists in the virsh VM list
//  2. Checks that the VM's domain state is "running" (not just defined or paused)
//
// Parameters:
//   - vmName: Name of the VM to wait for
//   - sshConfig: SSH configuration for connecting to the hypervisor
//   - knownHostsPath: Path to the known_hosts file for SSH
//
// Returns:
//   - error: Error if VM doesn't start within timeout period (vmStartTimeout) or if any operation fails
func WaitForVMToStart(vmName string, sshConfig *SSHConfig, knownHostsPath string) error {
	klog.V(2).Infof("WaitForVMToStart: Starting wait for VM '%s' to reach running state", vmName)
	klog.V(2).Infof("Waiting for VM %s to start...", vmName)

	err := RetryOperationWithTimeout(func() error {
		klog.V(4).Infof("WaitForVMToStart: Checking if VM '%s' is running (retry iteration)", vmName)

		// Check if VM exists using VirshVMExists helper
		_, err := VirshVMExists(vmName, sshConfig, knownHostsPath)
		if err != nil {
			klog.V(4).Infof("WaitForVMToStart: VM '%s' not found in VM list - %v", vmName, err)
			return fmt.Errorf("VM %s not yet running: %v", vmName, err)
		}

		// Check if VM is actually running (not just defined)
		statusOutput, err := VirshCommand(fmt.Sprintf("domstate %s", vmName), sshConfig, knownHostsPath)
		if err != nil {
			klog.ErrorS(err, "WaitForVMToStart failed to check VM state", "vm", vmName)
			return fmt.Errorf("failed to check VM %s state: %v", vmName, err)
		}

		statusOutput = strings.TrimSpace(statusOutput)
		klog.V(4).Infof("WaitForVMToStart: VM '%s' current state: %s", vmName, statusOutput)

		if !strings.Contains(statusOutput, "running") {
			return fmt.Errorf("VM %s is not running, current state: %s", vmName, statusOutput)
		}

		klog.V(2).Infof("WaitForVMToStart: VM '%s' is confirmed running", vmName)
		klog.V(2).Infof("VM %s is now running", vmName)
		return nil
	}, vmStartTimeout, vmStartPollInterval, fmt.Sprintf("VM %s startup", vmName))

	if err != nil {
		klog.ErrorS(err, "WaitForVMToStart timeout or error", "vm", vmName)
	} else {
		klog.V(2).Infof("WaitForVMToStart: Successfully confirmed VM '%s' is running", vmName)
	}

	return err
}

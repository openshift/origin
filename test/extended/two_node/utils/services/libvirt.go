// Package services provides virsh/libvirt utilities: VM lifecycle, inspection, network config, and recreation via SSH.
package services

import (
	"encoding/xml"
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/test/extended/two_node/utils/core"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// Domain represents a libvirt domain (virtual machine) configuration.
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

type VMState string

const (
	VMStateUnknown VMState = "unknown"
	VMStateRunning VMState = "running"
	VMStateShutOff VMState = "shut off"
)

var VMStateList = []VMState{
	VMStateUnknown,
	VMStateRunning,
	VMStateShutOff,
}

// Constants for virsh commands
const (
	virshCommand          = "virsh"
	virshListAllName      = "list --all --name"
	virshConnectionOption = "-c qemu:///system"
)

// VerifyVirsh checks if virsh is available by executing 'virsh version'.
//
//	output, err := VerifyVirsh(sshConfig, knownHostsPath)
func VerifyVirsh(sshConfig *core.SSHConfig, knownHostsPath string) (string, error) {
	e2e.Logf("VerifyVirsh: Checking virsh availability on %s", sshConfig.IP)
	output, err := VirshCommand("version", sshConfig, knownHostsPath)
	if err != nil {
		e2e.Logf("ERROR: VerifyVirsh failed on host %s: %v", sshConfig.IP, err)
	} else {
		e2e.Logf("VerifyVirsh: Success - %s", output)
	}
	return output, err
}

// VirshCommand executes a virsh command on the remote hypervisor via SSH.
//
//	output, err := VirshCommand("list --all", sshConfig, knownHostsPath)
func VirshCommand(command string, sshConfig *core.SSHConfig, knownHostsPath string) (string, error) {
	fullCommand := fmt.Sprintf("%s %s %s", virshCommand, virshConnectionOption, command)
	e2e.Logf("VirshCommand: Executing '%s' on %s", fullCommand, sshConfig.IP)
	output, _, err := core.ExecuteSSHCommand(fullCommand, sshConfig, knownHostsPath)
	if err != nil {
		e2e.Logf("ERROR: VirshCommand failed: %v, command: %s, host: %s", err, fullCommand, sshConfig.IP)
	} else {
		e2e.Logf("VirshCommand: Success - output length: %d bytes", len(output))
	}
	return output, err
}

// VirshDumpXML retrieves the XML configuration of a VM.
//
//	xml, err := VirshDumpXML("master-0", sshConfig, knownHostsPath)
func VirshDumpXML(vmName string, sshConfig *core.SSHConfig, knownHostsPath string) (string, error) {
	e2e.Logf("VirshDumpXML: Getting XML for VM '%s'", vmName)
	output, err := VirshCommand(fmt.Sprintf("dumpxml %s", vmName), sshConfig, knownHostsPath)
	if err != nil {
		e2e.Logf("ERROR: VirshDumpXML failed for VM %s: %v", vmName, err)
	} else {
		e2e.Logf("VirshDumpXML: Success for VM '%s' - XML length: %d bytes", vmName, len(output))
	}
	return output, err
}

// VirshListAllVMs lists all VMs (running and stopped) on the hypervisor.
//
//	vmList, err := VirshListAllVMs(sshConfig, knownHostsPath)
func VirshListAllVMs(sshConfig *core.SSHConfig, knownHostsPath string) (string, error) {
	e2e.Logf("VirshListAllVMs: Listing all VMs on %s", sshConfig.IP)
	output, err := VirshCommand(virshListAllName, sshConfig, knownHostsPath)
	if err != nil {
		e2e.Logf("ERROR: VirshListAllVMs failed on host %s: %v", sshConfig.IP, err)
	} else {
		vmCount := len(strings.Fields(output))
		e2e.Logf("VirshListAllVMs: Found %d VMs", vmCount)
	}
	return output, err
}

// VirshVMExists checks if a VM with the given name exists on the hypervisor.
//
//	output, err := VirshVMExists("master-0", sshConfig, knownHostsPath)
func VirshVMExists(vmName string, sshConfig *core.SSHConfig, knownHostsPath string) (string, error) {
	e2e.Logf("VirshVMExists: Checking if VM '%s' exists", vmName)
	output, err := VirshCommand(fmt.Sprintf("%s | grep -q %s", virshListAllName, vmName), sshConfig, knownHostsPath)
	if err != nil {
		e2e.Logf("VirshVMExists: VM '%s' does not exist or grep failed - %v", vmName, err)
	} else {
		e2e.Logf("VirshVMExists: VM '%s' exists", vmName)
	}
	return output, err
}

// VirshGetVMUUID retrieves the UUID of a VM.
//
//	uuid, err := VirshGetVMUUID("master-0", sshConfig, knownHostsPath)
func VirshGetVMUUID(vmName string, sshConfig *core.SSHConfig, knownHostsPath string) (string, error) {
	e2e.Logf("VirshGetVMUUID: Getting UUID for VM '%s'", vmName)
	output, err := VirshCommand(fmt.Sprintf("domuuid %s", vmName), sshConfig, knownHostsPath)
	uuid := strings.TrimSpace(output)
	if err != nil {
		e2e.Logf("ERROR: VirshGetVMUUID failed for VM %s: %v", vmName, err)
	} else {
		e2e.Logf("VirshGetVMUUID: VM '%s' has UUID: %s", vmName, uuid)
	}
	return uuid, err
}

// VirshShutdownVM gracefully shuts down a running VM (allows guest OS to shutdown cleanly).
//
//	err := VirshShutdownVM("master-0", sshConfig, knownHostsPath)
func VirshShutdownVM(vmName string, sshConfig *core.SSHConfig, knownHostsPath string) error {
	e2e.Logf("VirshShutdownVM: Gracefully shutting down VM '%s'", vmName)
	_, err := VirshCommand(fmt.Sprintf("shutdown %s", vmName), sshConfig, knownHostsPath)
	if err != nil {
		e2e.Logf("ERROR: VirshShutdownVM failed for VM %s: %v", vmName, err)
	} else {
		e2e.Logf("VirshShutdownVM: Successfully initiated shutdown for VM '%s'", vmName)
	}
	return err
}

// VirshUndefineVM undefines a VM (removes libvirt config, not disk images).
//
//	err := VirshUndefineVM("master-0", sshConfig, knownHostsPath)
func VirshUndefineVM(vmName string, sshConfig *core.SSHConfig, knownHostsPath string) error {
	e2e.Logf("VirshUndefineVM: Undefining VM '%s' (including NVRAM)", vmName)
	_, err := VirshCommand(fmt.Sprintf("undefine %s --nvram", vmName), sshConfig, knownHostsPath)
	if err != nil {
		e2e.Logf("ERROR: VirshUndefineVM failed for VM '%s': %v", vmName, err)
	} else {
		e2e.Logf("VirshUndefineVM: Successfully undefined VM '%s'", vmName)
	}
	return err
}

// VirshDestroyVM forcefully stops a running VM (equivalent to power-off).
//
//	err := VirshDestroyVM("master-0", sshConfig, knownHostsPath)
func VirshDestroyVM(vmName string, sshConfig *core.SSHConfig, knownHostsPath string) error {
	e2e.Logf("VirshDestroyVM: Forcefully stopping VM '%s'", vmName)
	_, err := VirshCommand(fmt.Sprintf("destroy %s", vmName), sshConfig, knownHostsPath)
	if err != nil {
		e2e.Logf("ERROR: VirshDestroyVM failed for VM '%s': %v", vmName, err)
	} else {
		e2e.Logf("VirshDestroyVM: Successfully destroyed VM '%s'", vmName)
	}
	return err
}

// VirshDefineVM defines a new VM from an XML configuration file.
//
//	err := VirshDefineVM("/tmp/master-0.xml", sshConfig, knownHostsPath)
func VirshDefineVM(xmlFilePath string, sshConfig *core.SSHConfig, knownHostsPath string) error {
	e2e.Logf("VirshDefineVM: Defining VM from XML file '%s'", xmlFilePath)
	_, err := VirshCommand(fmt.Sprintf("define %s", xmlFilePath), sshConfig, knownHostsPath)
	if err != nil {
		e2e.Logf("ERROR: VirshDefineVM failed for XML file %s: %v", xmlFilePath, err)
	} else {
		e2e.Logf("VirshDefineVM: Successfully defined VM from '%s'", xmlFilePath)
	}
	return err
}

// VirshStartVM starts a defined VM.
//
//	err := VirshStartVM("master-0", sshConfig, knownHostsPath)
func VirshStartVM(vmName string, sshConfig *core.SSHConfig, knownHostsPath string) error {
	e2e.Logf("VirshStartVM: Starting VM '%s'", vmName)
	_, err := VirshCommand(fmt.Sprintf("start %s", vmName), sshConfig, knownHostsPath)
	if err != nil {
		e2e.Logf("ERROR: VirshStartVM failed for VM %s: %v", vmName, err)
	} else {
		e2e.Logf("VirshStartVM: Successfully started VM '%s'", vmName)
	}
	return err
}

// VirshAutostartVM enables autostart for a VM (starts on hypervisor boot).
//
//	err := VirshAutostartVM("master-0", sshConfig, knownHostsPath)
func VirshAutostartVM(vmName string, sshConfig *core.SSHConfig, knownHostsPath string) error {
	e2e.Logf("VirshAutostartVM: Enabling autostart for VM '%s'", vmName)
	_, err := VirshCommand(fmt.Sprintf("autostart %s", vmName), sshConfig, knownHostsPath)
	if err != nil {
		e2e.Logf("ERROR: VirshAutostartVM failed for VM %s: %v", vmName, err)
	} else {
		e2e.Logf("VirshAutostartVM: Successfully enabled autostart for VM '%s'", vmName)
	}
	return err
}

// ExtractMACAddressFromXML extracts the MAC address for a network bridge from VM XML.
//
//	mac, err := ExtractMACAddressFromXML(xmlContent, "ostestpr")
func ExtractMACAddressFromXML(xmlContent string, networkBridge string) (string, error) {
	e2e.Logf("ExtractMACAddressFromXML: Extracting MAC for bridge '%s'", networkBridge)

	var domain Domain
	err := xml.Unmarshal([]byte(xmlContent), &domain)
	if err != nil {
		e2e.Logf("ERROR: ExtractMACAddressFromXML failed to parse domain XML: %v", err)
		return "", fmt.Errorf("failed to parse domain XML: %v", err)
	}

	e2e.Logf("ExtractMACAddressFromXML: Parsed domain '%s', checking %d interfaces", domain.Name, len(domain.Devices.Interfaces))

	// Look for the interface with ostestpr bridge
	for _, iface := range domain.Devices.Interfaces {
		e2e.Logf("ExtractMACAddressFromXML: Checking interface with bridge '%s', MAC '%s'", iface.Source.Bridge, iface.MAC.Address)
		if iface.Source.Bridge == networkBridge {
			e2e.Logf("ExtractMACAddressFromXML: Found MAC address '%s' for bridge '%s'", iface.MAC.Address, networkBridge)
			e2e.Logf("Found %s interface with MAC: %s", networkBridge, iface.MAC.Address)
			return iface.MAC.Address, nil
		}
	}

	e2e.Logf("ERROR: ExtractMACAddressFromXML: No interface found for bridge %s", networkBridge)
	return "", fmt.Errorf("no %s interface found in domain XML", networkBridge)
}

// GetVMNameByMACMatch finds the VM name with a specific MAC address by searching all VMs.
//
//	vmName, err := GetVMNameByMACMatch("master-0", "52:54:00:12:34:56", "ostestpr", sshConfig, knownHostsPath)
func GetVMNameByMACMatch(nodeName, nodeMAC string, networkBridge string, sshConfig *core.SSHConfig, knownHostsPath string) (string, error) {
	e2e.Logf("GetVMNameByMACMatch: Searching for VM with MAC '%s' on bridge '%s' (node: %s)", nodeMAC, networkBridge, nodeName)

	// Get list of all VMs using SSH to hypervisor
	vmListOutput, err := VirshListAllVMs(sshConfig, knownHostsPath)
	e2e.Logf("VirshListAllVMs output: %s", vmListOutput)
	if err != nil {
		e2e.Logf("ERROR: GetVMNameByMACMatch failed to get VM list: %v", err)
		return "", fmt.Errorf("failed to get VM list: %v", err)
	}

	vmNames := strings.Fields(vmListOutput)
	e2e.Logf("GetVMNameByMACMatch: Found %d VMs to check: %v", len(vmNames), vmNames)
	e2e.Logf("Found VMs: %v", vmNames)

	// Check each VM to find the one with matching MAC address
	for i, vmName := range vmNames {
		if vmName == "" {
			e2e.Logf("GetVMNameByMACMatch: Skipping empty VM name at index %d", i)
			continue
		}

		e2e.Logf("GetVMNameByMACMatch: Checking VM %d/%d: '%s'", i+1, len(vmNames), vmName)

		// Get VM XML configuration using SSH to hypervisor
		vmXML, err := VirshDumpXML(vmName, sshConfig, knownHostsPath)
		e2e.Logf("Getting XML for VM: %s", vmName)
		if err != nil {
			e2e.Logf("WARNING: Could not get XML for VM '%s', skipping - %v", vmName, err)
			continue
		}

		// Extract MAC address from VM XML for the ostestpr bridge
		vmMAC, err := ExtractMACAddressFromXML(vmXML, networkBridge)
		if err != nil {
			e2e.Logf("WARNING: Could not extract MAC from VM '%s', skipping - %v", vmName, err)
			continue
		}

		e2e.Logf("GetVMNameByMACMatch: VM '%s' has MAC '%s'", vmName, vmMAC)
		e2e.Logf("VM %s has MAC %s", vmName, vmMAC)
		e2e.Logf("Comparing VM MAC %s with target MAC %s", vmMAC, nodeMAC)

		// Check if this VM's MAC matches the node's MAC
		if vmMAC == nodeMAC {
			e2e.Logf("GetVMNameByMACMatch: Found matching VM '%s' with MAC '%s'", vmName, vmMAC)
			e2e.Logf("Found matching VM: %s (MAC: %s)", vmName, vmMAC)
			return vmName, nil
		}
	}

	e2e.Logf("ERROR: GetVMNameByMACMatch: No VM found with MAC %s for node %s", nodeMAC, nodeName)
	return "", fmt.Errorf("no VM found with MAC address %s for node %s", nodeMAC, nodeName)
}

// GetVMNetworkInfo retrieves the UUID and MAC address for a VM's network interface.
//
//	uuid, mac, err := GetVMNetworkInfo("master-0", "ostestpr", sshConfig, knownHostsPath)
func GetVMNetworkInfo(vmName string, networkBridge string, sshConfig *core.SSHConfig, knownHostsPath string) (string, string, error) {
	e2e.Logf("GetVMNetworkInfo: Getting network info for VM '%s' on bridge '%s'", vmName, networkBridge)

	newUUID, err := VirshGetVMUUID(vmName, sshConfig, knownHostsPath)
	if err != nil {
		e2e.Logf("ERROR: GetVMNetworkInfo failed to get UUID for VM %s: %v", vmName, err)
		return "", "", fmt.Errorf("failed to get VM UUID: %v", err)
	}

	newXMLOutput, err := VirshDumpXML(vmName, sshConfig, knownHostsPath)
	if err != nil {
		e2e.Logf("ERROR: GetVMNetworkInfo failed to get XML for VM %s: %v", vmName, err)
		return "", "", fmt.Errorf("failed to get VM XML: %v", err)
	}

	newMACAddress, err := ExtractMACAddressFromXML(newXMLOutput, networkBridge)
	if err != nil {
		e2e.Logf("ERROR: GetVMNetworkInfo failed to extract MAC for VM %s: %v", vmName, err)
		return "", "", fmt.Errorf("failed to find MAC address in VM XML: %v", err)
	}

	e2e.Logf("GetVMNetworkInfo: Successfully retrieved info for VM '%s': UUID=%s, MAC=%s", vmName, newUUID, newMACAddress)
	return newUUID, newMACAddress, nil
}

// WaitForVMState waits for a VM to reach a given state by polling domstate.
func WaitForVMState(vmName string, vmState VMState, timeout time.Duration, pollInterval time.Duration, sshConfig *core.SSHConfig, knownHostsPath string) error {
	e2e.Logf("WaitForVMState: Starting wait for VM '%s' to reach state %s (timeout: %v)", vmName, vmState, timeout)

	err := core.RetryWithOptions(func() error {
		state, err := GetVMState(vmName, sshConfig, knownHostsPath)
		if err != nil {
			return err
		}

		if state != vmState {
			return fmt.Errorf("VM %s state is not '%s'", vmName, vmState)
		}

		e2e.Logf("WaitForVMState: VM '%s' has reached state '%s'", vmName, vmState)
		return nil
	}, core.RetryOptions{
		Timeout:      timeout,
		PollInterval: pollInterval,
	}, fmt.Sprintf("VM %s state check", vmName))

	if err != nil {
		e2e.Logf("ERROR: WaitForVMState timeout or error for VM '%s': %v", vmName, err)
	} else {
		e2e.Logf("WaitForVMState: Successfully confirmed VM '%s' is '%s'", vmName, vmState)
	}

	return err
}

// GetVMState returns the current state of the VM.
func GetVMState(vmName string, sshConfig *core.SSHConfig, knownHostsPath string) (VMState, error) {
	e2e.Logf("GetVMState: Checking VM '%s' state", vmName)

	// Check if VM exists using VirshVMExists helper
	_, err := VirshVMExists(vmName, sshConfig, knownHostsPath)
	if err != nil {
		e2e.Logf("GetVMState: VM '%s' not found in VM list: %v", vmName, err)
		return VMStateUnknown, fmt.Errorf("VM '%s' does not exist yet: %v", vmName, err)
	}

	// Check VM state (not just defined)
	statusOutput, err := VirshCommand(fmt.Sprintf("domstate %s", vmName), sshConfig, knownHostsPath)
	if err != nil {
		e2e.Logf("ERROR: GetVMState failed to check VM '%s' state: %v", vmName, err)
		return VMStateUnknown, fmt.Errorf("failed to check VM %s state: %v", vmName, err)
	}

	statusOutput = strings.TrimSpace(statusOutput)
	e2e.Logf("GetVMState: VM '%s' current state: %s", vmName, statusOutput)

	for _, state := range VMStateList {
		if strings.Contains(statusOutput, string(state)) {
			return state, nil
		}
	}
	return VMStateUnknown, fmt.Errorf("VM '%s' unexpected status output '%s'", vmName, statusOutput)
}

// FindVMByNodeName finds a VM that corresponds to an OpenShift node
// This uses a simple name-based correlation approach
func FindVMByNodeName(nodeName string, sshConfig *core.SSHConfig, knownHostsPath string) (string, error) {
	vmListOutput, err := VirshListAllVMs(sshConfig, knownHostsPath)
	if err != nil {
		return "", fmt.Errorf("failed to list VMs: %w", err)
	}

	vmNames := strings.Fields(vmListOutput)
	// Try different naming patterns commonly used in OpenShift test environments
	possibleVMNames := []string{
		nodeName,                                // exact match
		fmt.Sprintf("%s.example.com", nodeName), // FQDN
		strings.Replace(nodeName, "-", "_", -1), // underscores instead of dashes
	}
	for _, vmName := range vmNames {
		for _, possibleName := range possibleVMNames {
			if vmName == possibleName || strings.Contains(vmName, possibleName) || strings.Contains(possibleName, vmName) {
				e2e.Logf("Matched VM '%s' to node '%s' using pattern '%s'", vmName,
					nodeName, possibleName)
				return vmName, nil
			}
		}
	}

	return "", fmt.Errorf("no suitable VM found for node %s among VMs: %v", nodeName, vmNames)
}

// Package services provides virsh/libvirt utilities: VM lifecycle, inspection, network config, and recreation via SSH.
package services

import (
	"encoding/xml"
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/test/extended/two_node/utils/core"
	"k8s.io/klog/v2"
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
	klog.V(4).Infof("VerifyVirsh: Checking virsh availability on %s", sshConfig.IP)
	output, err := VirshCommand("version", sshConfig, knownHostsPath)
	if err != nil {
		klog.ErrorS(err, "VerifyVirsh failed", "host", sshConfig.IP)
	} else {
		klog.V(2).Infof("VerifyVirsh: Success - %s", output)
	}
	return output, err
}

// VirshCommand executes a virsh command on the remote hypervisor via SSH.
//
//	output, err := VirshCommand("list --all", sshConfig, knownHostsPath)
func VirshCommand(command string, sshConfig *core.SSHConfig, knownHostsPath string) (string, error) {
	fullCommand := fmt.Sprintf("%s %s %s", virshCommand, virshConnectionOption, command)
	klog.V(4).Infof("VirshCommand: Executing '%s' on %s", fullCommand, sshConfig.IP)
	output, _, err := core.ExecuteSSHCommand(fullCommand, sshConfig, knownHostsPath)
	if err != nil {
		klog.ErrorS(err, "VirshCommand failed", "command", fullCommand, "host", sshConfig.IP)
	} else {
		klog.V(4).Infof("VirshCommand: Success - output length: %d bytes", len(output))
	}
	return output, err
}

// VirshDumpXML retrieves the XML configuration of a VM.
//
//	xml, err := VirshDumpXML("master-0", sshConfig, knownHostsPath)
func VirshDumpXML(vmName string, sshConfig *core.SSHConfig, knownHostsPath string) (string, error) {
	klog.V(4).Infof("VirshDumpXML: Getting XML for VM '%s'", vmName)
	output, err := VirshCommand(fmt.Sprintf("dumpxml %s", vmName), sshConfig, knownHostsPath)
	if err != nil {
		klog.ErrorS(err, "VirshDumpXML failed", "vm", vmName)
	} else {
		klog.V(4).Infof("VirshDumpXML: Success for VM '%s' - XML length: %d bytes", vmName, len(output))
	}
	return output, err
}

// VirshListAllVMs lists all VMs (running and stopped) on the hypervisor.
//
//	vmList, err := VirshListAllVMs(sshConfig, knownHostsPath)
func VirshListAllVMs(sshConfig *core.SSHConfig, knownHostsPath string) (string, error) {
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

// VirshVMExists checks if a VM with the given name exists on the hypervisor.
//
//	output, err := VirshVMExists("master-0", sshConfig, knownHostsPath)
func VirshVMExists(vmName string, sshConfig *core.SSHConfig, knownHostsPath string) (string, error) {
	klog.V(4).Infof("VirshVMExists: Checking if VM '%s' exists", vmName)
	output, err := VirshCommand(fmt.Sprintf("%s | grep -q %s", virshListAllName, vmName), sshConfig, knownHostsPath)
	if err != nil {
		klog.V(4).Infof("VirshVMExists: VM '%s' does not exist or grep failed - %v", vmName, err)
	} else {
		klog.V(2).Infof("VirshVMExists: VM '%s' exists", vmName)
	}
	return output, err
}

// VirshGetVMUUID retrieves the UUID of a VM.
//
//	uuid, err := VirshGetVMUUID("master-0", sshConfig, knownHostsPath)
func VirshGetVMUUID(vmName string, sshConfig *core.SSHConfig, knownHostsPath string) (string, error) {
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

// VirshUndefineVM undefines a VM (removes libvirt config, not disk images).
//
//	err := VirshUndefineVM("master-0", sshConfig, knownHostsPath)
func VirshUndefineVM(vmName string, sshConfig *core.SSHConfig, knownHostsPath string) error {
	klog.V(2).Infof("VirshUndefineVM: Undefining VM '%s' (including NVRAM)", vmName)
	_, err := VirshCommand(fmt.Sprintf("undefine %s --nvram", vmName), sshConfig, knownHostsPath)
	if err != nil {
		klog.ErrorS(err, "VirshUndefineVM failed", "vm", vmName)
	} else {
		klog.V(2).Infof("VirshUndefineVM: Successfully undefined VM '%s'", vmName)
	}
	return err
}

// VirshDestroyVM forcefully stops a running VM (equivalent to power-off).
//
//	err := VirshDestroyVM("master-0", sshConfig, knownHostsPath)
func VirshDestroyVM(vmName string, sshConfig *core.SSHConfig, knownHostsPath string) error {
	klog.V(2).Infof("VirshDestroyVM: Forcefully stopping VM '%s'", vmName)
	_, err := VirshCommand(fmt.Sprintf("destroy %s", vmName), sshConfig, knownHostsPath)
	if err != nil {
		klog.ErrorS(err, "VirshDestroyVM failed", "vm", vmName)
	} else {
		klog.V(2).Infof("VirshDestroyVM: Successfully destroyed VM '%s'", vmName)
	}
	return err
}

// VirshDefineVM defines a new VM from an XML configuration file.
//
//	err := VirshDefineVM("/tmp/master-0.xml", sshConfig, knownHostsPath)
func VirshDefineVM(xmlFilePath string, sshConfig *core.SSHConfig, knownHostsPath string) error {
	klog.V(2).Infof("VirshDefineVM: Defining VM from XML file '%s'", xmlFilePath)
	_, err := VirshCommand(fmt.Sprintf("define %s", xmlFilePath), sshConfig, knownHostsPath)
	if err != nil {
		klog.ErrorS(err, "VirshDefineVM failed", "xmlFile", xmlFilePath)
	} else {
		klog.V(2).Infof("VirshDefineVM: Successfully defined VM from '%s'", xmlFilePath)
	}
	return err
}

// VirshStartVM starts a defined VM.
//
//	err := VirshStartVM("master-0", sshConfig, knownHostsPath)
func VirshStartVM(vmName string, sshConfig *core.SSHConfig, knownHostsPath string) error {
	klog.V(2).Infof("VirshStartVM: Starting VM '%s'", vmName)
	_, err := VirshCommand(fmt.Sprintf("start %s", vmName), sshConfig, knownHostsPath)
	if err != nil {
		klog.ErrorS(err, "VirshStartVM failed", "vm", vmName)
	} else {
		klog.V(2).Infof("VirshStartVM: Successfully started VM '%s'", vmName)
	}
	return err
}

// VirshAutostartVM enables autostart for a VM (starts on hypervisor boot).
//
//	err := VirshAutostartVM("master-0", sshConfig, knownHostsPath)
func VirshAutostartVM(vmName string, sshConfig *core.SSHConfig, knownHostsPath string) error {
	klog.V(2).Infof("VirshAutostartVM: Enabling autostart for VM '%s'", vmName)
	_, err := VirshCommand(fmt.Sprintf("autostart %s", vmName), sshConfig, knownHostsPath)
	if err != nil {
		klog.ErrorS(err, "VirshAutostartVM failed", "vm", vmName)
	} else {
		klog.V(2).Infof("VirshAutostartVM: Successfully enabled autostart for VM '%s'", vmName)
	}
	return err
}

// ExtractMACAddressFromXML extracts the MAC address for a network bridge from VM XML.
//
//	mac, err := ExtractMACAddressFromXML(xmlContent, "ostestpr")
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

// GetVMNameByMACMatch finds the VM name with a specific MAC address by searching all VMs.
//
//	vmName, err := GetVMNameByMACMatch("master-0", "52:54:00:12:34:56", "ostestpr", sshConfig, knownHostsPath)
func GetVMNameByMACMatch(nodeName, nodeMAC string, networkBridge string, sshConfig *core.SSHConfig, knownHostsPath string) (string, error) {
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

// GetVMNetworkInfo retrieves the UUID and MAC address for a VM's network interface.
//
//	uuid, mac, err := GetVMNetworkInfo("master-0", "ostestpr", sshConfig, knownHostsPath)
func GetVMNetworkInfo(vmName string, networkBridge string, sshConfig *core.SSHConfig, knownHostsPath string) (string, string, error) {
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

// WaitForVMState waits for a VM to reach a given state by polling domstate.
func WaitForVMState(vmName string, vmState VMState, timeout time.Duration, pollInterval time.Duration, sshConfig *core.SSHConfig, knownHostsPath string) error {
	klog.V(2).Infof("WaitForVMState: Starting wait for VM '%s' to reach state %s", vmName, vmState)

	err := core.RetryWithOptions(func() error {
		klog.V(4).Infof("WaitForVMState: Checking VM '%s' state (retry iteration)", vmName)

		// Check if VM exists using VirshVMExists helper
		_, err := VirshVMExists(vmName, sshConfig, knownHostsPath)
		if err != nil {
			klog.V(4).Infof("WaitForVMState: VM '%s' not found in VM list - %v", vmName, err)
			return fmt.Errorf("VM %s state is not '%s' yet: %v", vmName, vmState, err)
		}

		// Check VM state (not just defined)
		statusOutput, err := VirshCommand(fmt.Sprintf("domstate %s", vmName), sshConfig, knownHostsPath)
		if err != nil {
			klog.ErrorS(err, "WaitForVMState failed to check VM state", "vm", vmName)
			return fmt.Errorf("failed to check VM %s state: %v", vmName, err)
		}

		statusOutput = strings.TrimSpace(statusOutput)
		klog.V(4).Infof("WaitForVMState: VM '%s' current state: %s", vmName, statusOutput)

		if !strings.Contains(statusOutput, string(vmState)) {
			return fmt.Errorf("VM %s is not '%s', current state: %s", vmName, vmState, statusOutput)
		}

		klog.V(2).Infof("WaitForVMState: VM '%s' has reached state '%s'", vmName, vmState)
		return nil
	}, core.RetryOptions{
		Timeout:      timeout,
		PollInterval: pollInterval,
	}, fmt.Sprintf("VM %s state check", vmName))

	if err != nil {
		klog.ErrorS(err, "WaitForVMState timeout or error", "vm", vmName)
	} else {
		klog.V(2).Infof("WaitForVMState: Successfully confirmed VM '%s' is '%s'", vmName, vmState)
	}

	return err
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
				klog.V(2).Infof("Matched VM '%s' to node '%s' using pattern '%s'", vmName,
					nodeName, possibleName)
				return vmName, nil
			}
		}
	}

	return "", fmt.Errorf("no suitable VM found for node %s among VMs: %v", nodeName, vmNames)
}

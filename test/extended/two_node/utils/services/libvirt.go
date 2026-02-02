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
	virshConnectionOption = "-c qemu:///system"
)

// VirshListFlag represents optional flags for the virsh list command.
type VirshListFlag string

const (
	// VirshListFlagAll includes inactive domains in the output (--all)
	VirshListFlagAll VirshListFlag = "--all"
	// VirshListFlagName outputs only domain names, one per line (--name)
	VirshListFlagName VirshListFlag = "--name"
)

// VerifyVirsh checks if virsh is available by executing 'virsh version'.
//
//	output, err := VerifyVirsh(sshConfig, knownHostsPath)
func VerifyVirsh(sshConfig *core.SSHConfig, knownHostsPath string) (string, error) {
	output, err := VirshCommand("version", sshConfig, knownHostsPath)
	if err != nil {
		e2e.Logf("VerifyVirsh failed on %s: %v", sshConfig.IP, err)
	}
	return output, err
}

// VirshCommand executes a virsh command on the remote hypervisor via SSH.
//
//	output, err := VirshCommand("list --all", sshConfig, knownHostsPath)
func VirshCommand(command string, sshConfig *core.SSHConfig, knownHostsPath string) (string, error) {
	fullCommand := fmt.Sprintf("%s %s %s", virshCommand, virshConnectionOption, command)
	output, _, err := core.ExecuteSSHCommand(fullCommand, sshConfig, knownHostsPath)
	if err != nil {
		e2e.Logf("VirshCommand failed: %v, command: %s, host: %s", err, fullCommand, sshConfig.IP)
	}
	return output, err
}

// VirshDumpXML retrieves the XML configuration of a VM.
//
//	xml, err := VirshDumpXML("master-0", sshConfig, knownHostsPath)
func VirshDumpXML(vmName string, sshConfig *core.SSHConfig, knownHostsPath string) (string, error) {
	output, err := VirshCommand(fmt.Sprintf("dumpxml %s", vmName), sshConfig, knownHostsPath)
	if err != nil {
		e2e.Logf("VirshDumpXML failed for VM %s: %v", vmName, err)
	}
	return output, err
}

// VirshList lists VMs on the hypervisor with configurable output format.
// Pass VirshListFlagAll to include inactive domains, VirshListFlagName to output only names.
//
// Examples:
//
//	vmList, err := VirshList(sshConfig, knownHostsPath, VirshListFlagAll, VirshListFlagName)  // names only
//	vmList, err := VirshList(sshConfig, knownHostsPath, VirshListFlagAll)                    // table with state
func VirshList(sshConfig *core.SSHConfig, knownHostsPath string, flags ...VirshListFlag) (string, error) {
	cmd := "list"
	for _, flag := range flags {
		cmd += " " + string(flag)
	}
	output, err := VirshCommand(cmd, sshConfig, knownHostsPath)
	if err != nil {
		e2e.Logf("VirshList failed on %s: %v", sshConfig.IP, err)
	}
	return output, err
}

// VirshVMExists checks if a VM with the given name exists on the hypervisor.
//
//	output, err := VirshVMExists("master-0", sshConfig, knownHostsPath)
func VirshVMExists(vmName string, sshConfig *core.SSHConfig, knownHostsPath string) (string, error) {
	output, err := VirshCommand(fmt.Sprintf("list --all --name | grep -q %s", vmName), sshConfig, knownHostsPath)
	// err is expected if VM does not exist (grep returns non-zero)
	return output, err
}

// VirshGetVMUUID retrieves the UUID of a VM.
//
//	uuid, err := VirshGetVMUUID("master-0", sshConfig, knownHostsPath)
func VirshGetVMUUID(vmName string, sshConfig *core.SSHConfig, knownHostsPath string) (string, error) {
	output, err := VirshCommand(fmt.Sprintf("domuuid %s", vmName), sshConfig, knownHostsPath)
	uuid := strings.TrimSpace(output)
	if err != nil {
		e2e.Logf("VirshGetVMUUID failed for VM %s: %v", vmName, err)
	}
	return uuid, err
}

// VirshShutdownVM gracefully shuts down a running VM (allows guest OS to shutdown cleanly).
//
//	err := VirshShutdownVM("master-0", sshConfig, knownHostsPath)
func VirshShutdownVM(vmName string, sshConfig *core.SSHConfig, knownHostsPath string) error {
	_, err := VirshCommand(fmt.Sprintf("shutdown %s", vmName), sshConfig, knownHostsPath)
	if err != nil {
		e2e.Logf("VirshShutdownVM failed for VM %s: %v", vmName, err)
	}
	return err
}

// VirshUndefineVM undefines a VM (removes libvirt config, not disk images).
//
//	err := VirshUndefineVM("master-0", sshConfig, knownHostsPath)
func VirshUndefineVM(vmName string, sshConfig *core.SSHConfig, knownHostsPath string) error {
	_, err := VirshCommand(fmt.Sprintf("undefine %s --nvram", vmName), sshConfig, knownHostsPath)
	if err != nil {
		e2e.Logf("VirshUndefineVM failed for VM %s: %v", vmName, err)
	}
	return err
}

// VirshDestroyVM forcefully stops a running VM (equivalent to power-off).
//
//	err := VirshDestroyVM("master-0", sshConfig, knownHostsPath)
func VirshDestroyVM(vmName string, sshConfig *core.SSHConfig, knownHostsPath string) error {
	_, err := VirshCommand(fmt.Sprintf("destroy %s", vmName), sshConfig, knownHostsPath)
	if err != nil {
		e2e.Logf("VirshDestroyVM failed for VM %s: %v", vmName, err)
	}
	return err
}

// VirshDefineVM defines a new VM from an XML configuration file.
//
//	err := VirshDefineVM("/tmp/master-0.xml", sshConfig, knownHostsPath)
func VirshDefineVM(xmlFilePath string, sshConfig *core.SSHConfig, knownHostsPath string) error {
	_, err := VirshCommand(fmt.Sprintf("define %s", xmlFilePath), sshConfig, knownHostsPath)
	if err != nil {
		e2e.Logf("VirshDefineVM failed for %s: %v", xmlFilePath, err)
	}
	return err
}

// VirshStartVM starts a defined VM.
//
//	err := VirshStartVM("master-0", sshConfig, knownHostsPath)
func VirshStartVM(vmName string, sshConfig *core.SSHConfig, knownHostsPath string) error {
	_, err := VirshCommand(fmt.Sprintf("start %s", vmName), sshConfig, knownHostsPath)
	if err != nil {
		e2e.Logf("VirshStartVM failed for VM %s: %v", vmName, err)
	}
	return err
}

// VirshAutostartVM enables autostart for a VM (starts on hypervisor boot).
//
//	err := VirshAutostartVM("master-0", sshConfig, knownHostsPath)
func VirshAutostartVM(vmName string, sshConfig *core.SSHConfig, knownHostsPath string) error {
	_, err := VirshCommand(fmt.Sprintf("autostart %s", vmName), sshConfig, knownHostsPath)
	if err != nil {
		e2e.Logf("VirshAutostartVM failed for VM %s: %v", vmName, err)
	}
	return err
}

// ExtractMACAddressFromXML extracts the MAC address for a network bridge from VM XML.
//
//	mac, err := ExtractMACAddressFromXML(xmlContent, "ostestpr")
func ExtractMACAddressFromXML(xmlContent string, networkBridge string) (string, error) {
	var domain Domain
	err := xml.Unmarshal([]byte(xmlContent), &domain)
	if err != nil {
		e2e.Logf("ExtractMACAddressFromXML: failed to parse domain XML: %v", err)
		return "", fmt.Errorf("failed to parse domain XML: %v", err)
	}

	for _, iface := range domain.Devices.Interfaces {
		if iface.Source.Bridge == networkBridge {
			return iface.MAC.Address, nil
		}
	}

	e2e.Logf("ExtractMACAddressFromXML: no %s interface found in domain XML for %s", networkBridge, domain.Name)
	return "", fmt.Errorf("no %s interface found in domain XML for %s", networkBridge, domain.Name)
}

// GetVMNameByMACMatch finds the VM name with a specific MAC address by searching all VMs.
//
//	vmName, err := GetVMNameByMACMatch("master-0", "52:54:00:12:34:56", "ostestpr", sshConfig, knownHostsPath)
func GetVMNameByMACMatch(nodeName, nodeMAC string, networkBridge string, sshConfig *core.SSHConfig, knownHostsPath string) (string, error) {
	vmListOutput, err := VirshList(sshConfig, knownHostsPath, VirshListFlagAll, VirshListFlagName)
	if err != nil {
		e2e.Logf("GetVMNameByMACMatch: failed to get VM list: %v", err)
		return "", fmt.Errorf("failed to get VM list: %v", err)
	}

	vmNames := strings.Fields(vmListOutput)
	for _, vmName := range vmNames {
		if vmName == "" {
			continue
		}

		vmXML, err := VirshDumpXML(vmName, sshConfig, knownHostsPath)
		if err != nil {
			// Skip VMs we can't get XML for (may be transient or permission issues)
			continue
		}

		vmMAC, err := ExtractMACAddressFromXML(vmXML, networkBridge)
		if err != nil {
			// Skip VMs without the expected bridge interface
			continue
		}

		if vmMAC == nodeMAC {
			e2e.Logf("Found VM '%s' matching node %s (MAC: %s)", vmName, nodeName, vmMAC)
			return vmName, nil
		}
	}

	e2e.Logf("GetVMNameByMACMatch: no VM found with MAC %s for node %s among %d VMs", nodeMAC, nodeName, len(vmNames))
	return "", fmt.Errorf("no VM found with MAC address %s for node %s", nodeMAC, nodeName)
}

// GetVMNetworkInfo retrieves the UUID and MAC address for a VM's network interface.
//
//	uuid, mac, err := GetVMNetworkInfo("master-0", "ostestpr", sshConfig, knownHostsPath)
func GetVMNetworkInfo(vmName string, networkBridge string, sshConfig *core.SSHConfig, knownHostsPath string) (string, string, error) {
	newUUID, err := VirshGetVMUUID(vmName, sshConfig, knownHostsPath)
	if err != nil {
		e2e.Logf("GetVMNetworkInfo: failed to get UUID for VM %s: %v", vmName, err)
		return "", "", fmt.Errorf("failed to get VM UUID: %v", err)
	}

	newXMLOutput, err := VirshDumpXML(vmName, sshConfig, knownHostsPath)
	if err != nil {
		e2e.Logf("GetVMNetworkInfo: failed to get XML for VM %s: %v", vmName, err)
		return "", "", fmt.Errorf("failed to get VM XML: %v", err)
	}

	newMACAddress, err := ExtractMACAddressFromXML(newXMLOutput, networkBridge)
	if err != nil {
		e2e.Logf("GetVMNetworkInfo: failed to extract MAC for VM %s: %v", vmName, err)
		return "", "", fmt.Errorf("failed to find MAC address in VM XML: %v", err)
	}

	return newUUID, newMACAddress, nil
}

// WaitForVMState waits for a VM to reach a given state by polling domstate.
func WaitForVMState(vmName string, vmState VMState, timeout time.Duration, pollInterval time.Duration, sshConfig *core.SSHConfig, knownHostsPath string) error {
	err := core.RetryWithOptions(func() error {
		state, err := GetVMState(vmName, sshConfig, knownHostsPath)
		if err != nil {
			return err
		}

		if state != vmState {
			return fmt.Errorf("VM %s state is '%s', waiting for '%s'", vmName, state, vmState)
		}

		return nil
	}, core.RetryOptions{
		Timeout:      timeout,
		PollInterval: pollInterval,
	}, fmt.Sprintf("VM %s state check", vmName))

	if err != nil {
		e2e.Logf("WaitForVMState: timeout waiting for VM '%s' to reach state '%s': %v", vmName, vmState, err)
	} else {
		e2e.Logf("VM '%s' reached state '%s'", vmName, vmState)
	}

	return err
}

// GetVMState returns the current state of the VM.
func GetVMState(vmName string, sshConfig *core.SSHConfig, knownHostsPath string) (VMState, error) {
	// Check if VM exists using VirshVMExists helper
	_, err := VirshVMExists(vmName, sshConfig, knownHostsPath)
	if err != nil {
		return VMStateUnknown, fmt.Errorf("VM '%s' does not exist: %v", vmName, err)
	}

	// Check VM state (not just defined)
	statusOutput, err := VirshCommand(fmt.Sprintf("domstate %s", vmName), sshConfig, knownHostsPath)
	if err != nil {
		e2e.Logf("GetVMState: failed to get state for VM '%s': %v", vmName, err)
		return VMStateUnknown, fmt.Errorf("failed to check VM %s state: %v", vmName, err)
	}

	statusOutput = strings.TrimSpace(statusOutput)

	for _, state := range VMStateList {
		if strings.Contains(statusOutput, string(state)) {
			return state, nil
		}
	}

	e2e.Logf("GetVMState: VM '%s' has unexpected state: %s", vmName, statusOutput)
	return VMStateUnknown, fmt.Errorf("VM '%s' unexpected status output '%s'", vmName, statusOutput)
}

// FindVMByNodeName finds a VM that corresponds to an OpenShift node
// This uses a simple name-based correlation approach
func FindVMByNodeName(nodeName string, sshConfig *core.SSHConfig, knownHostsPath string) (string, error) {
	vmListOutput, err := VirshList(sshConfig, knownHostsPath, VirshListFlagAll, VirshListFlagName)
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

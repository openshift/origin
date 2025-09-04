package utils

import (
	"encoding/xml"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
)

// XML structs for libvirt domain XML parsing
type Domain struct {
	XMLName xml.Name `xml:"domain"`
	Name    string   `xml:"name"`
	UUID    string   `xml:"uuid"`
	Devices Devices  `xml:"devices"`
}

type Devices struct {
	Interfaces []Interface `xml:"interface"`
}

type Interface struct {
	Type   string `xml:"type,attr"`
	MAC    MAC    `xml:"mac"`
	Source Source `xml:"source"`
}

type MAC struct {
	Address string `xml:"address,attr"`
}

type Source struct {
	Bridge string `xml:"bridge,attr"`
}

// Constants for virsh commands
const (
	virshCommand = "virsh"
	virshListAllName = "list --all --name"
	virshConnectionOption = "-c qemu:///system"
)

// Virsh utility functions
func VerifyVirsh(sshConfig *SSHConfig) (string, error) {
	return VirshCommand("version", sshConfig)
}

func VirshCommand(command string, sshConfig *SSHConfig) (string, error) {
	return ExecuteSSHCommand(fmt.Sprintf("%s %s %s", virshCommand, virshConnectionOption, command), sshConfig)
}

func VirshDumpXML(vmName string, sshConfig *SSHConfig) (string, error) {
	return VirshCommand(fmt.Sprintf("dumpxml %s", vmName), sshConfig)
}

func VirshListAllVMs(sshConfig *SSHConfig) (string, error) {
	return VirshCommand(virshListAllName, sshConfig)
}

func VirshVMExists(vmName string, sshConfig *SSHConfig) (string, error) {
	return VirshCommand(fmt.Sprintf("%s | grep -q %s", virshListAllName, vmName), sshConfig)
}

func VirshGetVMUUID(vmName string, sshConfig *SSHConfig) (string, error) {
	output, err := VirshCommand(fmt.Sprintf("domuuid %s", vmName), sshConfig)
	return strings.TrimSpace(output), err
}

func VirshUndefineVM(vmName string, sshConfig *SSHConfig) error {
	_, err := VirshCommand(fmt.Sprintf("undefine %s --nvram", vmName), sshConfig)
	return err
}

func VirshDestroyVM(vmName string, sshConfig *SSHConfig) error {
	_, err := VirshCommand(fmt.Sprintf("destroy %s", vmName), sshConfig)
	return err
}

func VirshDefineVM(xmlFilePath string, sshConfig *SSHConfig) error {
	_, err := VirshCommand(fmt.Sprintf("define %s", xmlFilePath), sshConfig)
	return err
}

func VirshStartVM(vmName string, sshConfig *SSHConfig) error {
	_, err := VirshCommand(fmt.Sprintf("start %s", vmName), sshConfig)
	return err
}

func VirshAutostartVM(vmName string, sshConfig *SSHConfig) error {
	_, err := VirshCommand(fmt.Sprintf("autostart %s", vmName), sshConfig)
	return err
}

// XML parsing functions
func ExtractIPFromVMXML(xmlContent, networkName string) (string, error) {
	var domain Domain
	err := xml.Unmarshal([]byte(xmlContent), &domain)
	if err != nil {
		g.GinkgoT().Logf("Warning: Failed to parse domain XML: %v", err)
		return "", fmt.Errorf("failed to parse domain XML: %v", err)
	}

	// Look for the interface with the specified network
	for _, iface := range domain.Devices.Interfaces {
		if iface.Source.Bridge == networkName {
			// Note: IP addresses are typically not stored in the domain XML
			// They are assigned dynamically by DHCP. This function might need
			// to be updated to get IP from a different source.
			g.GinkgoT().Logf("Found interface for network %s, but IP addresses are not stored in domain XML", networkName)
			return "", fmt.Errorf("interface found for network %s, but IP addresses are not stored in domain XML", networkName)
		}
	}

	return "", fmt.Errorf("no interface found for network %s", networkName)
}

func ExtractMACFromVMXML(xmlContent, networkName string) (string, error) {
	var domain Domain
	err := xml.Unmarshal([]byte(xmlContent), &domain)
	if err != nil {
		g.GinkgoT().Logf("Warning: Failed to parse domain XML: %v", err)
		return "", fmt.Errorf("failed to parse domain XML: %v", err)
	}

	// Look for the interface with the specified network
	for _, iface := range domain.Devices.Interfaces {
		if iface.Source.Bridge == networkName {
			g.GinkgoT().Logf("Found %s interface with MAC: %s", networkName, iface.MAC.Address)
			return iface.MAC.Address, nil
		}
	}

	return "", fmt.Errorf("no interface found for network %s", networkName)
}

func ExtractMACAddressFromXML(xmlContent string, networkBridge string) (string, error) {
	var domain Domain
	err := xml.Unmarshal([]byte(xmlContent), &domain)
	if err != nil {
		return "", fmt.Errorf("failed to parse domain XML: %v", err)
	}

	// Look for the interface with ostestpr bridge
	for _, iface := range domain.Devices.Interfaces {
		if iface.Source.Bridge == networkBridge {
			g.GinkgoT().Logf("Found %s interface with MAC: %s", networkBridge, iface.MAC.Address)
			return iface.MAC.Address, nil
		}
	}

	return "", fmt.Errorf("no %s interface found in domain XML", networkBridge)
}

// VM management functions
func GetVMNameByMACMatch(nodeName, nodeMAC string, networkBridge string, sshConfig *SSHConfig) (string, error) {
	// Get list of all VMs using SSH to hypervisor
	vmListOutput, err := VirshListAllVMs(sshConfig)
	g.GinkgoT().Logf("[DEBUG] VirshListAllVMs output: %s", vmListOutput)
	if err != nil {
		return "", fmt.Errorf("failed to get VM list: %v", err)
	}

	vmNames := strings.Fields(vmListOutput)
	g.GinkgoT().Logf("Found VMs: %v", vmNames)

	// Check each VM to find the one with matching MAC address
	for _, vmName := range vmNames {
		if vmName == "" {
			continue
		}

		// Get VM XML configuration using SSH to hypervisor
		vmXML, err := VirshDumpXML(vmName, sshConfig)
	g.GinkgoT().Logf("[DEBUG] Getting XML for VM: %s", vmName)
		if err != nil {
			g.GinkgoT().Logf("Warning: Could not get XML for VM %s: %v", vmName, err)
			continue
		}

		// Extract MAC address from VM XML for the ostestpr bridge
		vmMAC, err := ExtractMACAddressFromXML(vmXML, networkBridge)
		if err != nil {
			g.GinkgoT().Logf("Warning: Could not extract MAC address from VM %s XML: %v", vmName, err)
			continue
		}

		g.GinkgoT().Logf("VM %s has MAC %s", vmName, vmMAC)
	g.GinkgoT().Logf("[DEBUG] Comparing VM MAC %s with target MAC %s", vmMAC, nodeMAC)

		// Check if this VM's MAC matches the node's MAC
		if vmMAC == nodeMAC {
			g.GinkgoT().Logf("Found matching VM: %s (MAC: %s)", vmName, vmMAC)
			return vmName, nil
		}
	}

	return "", fmt.Errorf("no VM found with MAC address %s for node %s", nodeMAC, nodeName)
}

func GetVMNetworkInfo(vmName string, networkBridge string, sshConfig *SSHConfig) (string, string, error) {
	newUUID, err := VirshGetVMUUID(vmName, sshConfig)
	if err != nil {
		return "", "", fmt.Errorf("failed to get VM UUID: %v", err)
	}

	newXMLOutput, err := VirshDumpXML(vmName, sshConfig)
	if err != nil {
		return "", "", fmt.Errorf("failed to get VM XML: %v", err)
	}

	newMACAddress, err := ExtractMACAddressFromXML(newXMLOutput, networkBridge)
	if err != nil {
		return "", "", fmt.Errorf("failed to find MAC address in VM XML: %v", err)
	}

	return newUUID, newMACAddress, nil
}

func WaitForVMToStart(vmName string, sshConfig *SSHConfig) error {
	g.GinkgoT().Logf("Waiting for VM %s to start...", vmName)

	return retryOperationWithTimeout(func() error {
		// Check if VM is running using constant
		_, err := ExecuteSSHCommand(fmt.Sprintf("%s | grep %s", virshListAllName, vmName), sshConfig)
		if err != nil {
			return fmt.Errorf("VM %s not yet running: %v", vmName, err)
		}

		// Check if VM is actually running (not just defined)
		statusOutput, err := ExecuteSSHCommand(fmt.Sprintf("virsh domstate %s", vmName), sshConfig)
		if err != nil {
			return fmt.Errorf("failed to check VM %s state: %v", vmName, err)
		}

		if !strings.Contains(statusOutput, "running") {
			return fmt.Errorf("VM %s is not running, current state: %s", vmName, strings.TrimSpace(statusOutput))
		}

		g.GinkgoT().Logf("VM %s is now running", vmName)
		return nil
	}, 2*time.Minute, 15*time.Second, fmt.Sprintf("VM %s startup", vmName))
}

func RecreateVMFromXML(vmName, xmlContent string, sshConfig *SSHConfig) error {
	// Check if VM already exists
	_, err := ExecuteSSHCommand(fmt.Sprintf("%s | grep -q %s", virshListAllName, vmName), sshConfig)
	if err == nil {
		g.GinkgoT().Logf("VM %s already exists, skipping recreation", vmName)
		return nil
	}

	// Create a temporary file on the hypervisor with the XML content
	createXMLCommand := fmt.Sprintf(`cat > /tmp/%s.xml <<'XML_EOF'
%s
XML_EOF`, vmName, xmlContent)

	_, err = ExecuteSSHCommand(createXMLCommand, sshConfig)
	if err != nil {
		return fmt.Errorf("failed to create XML file on hypervisor: %v", err)
	}

	// Redefine the VM using the backed up XML
	_, err = ExecuteSSHCommand(fmt.Sprintf("virsh define /tmp/%s.xml", vmName), sshConfig)
	if err != nil {
		return fmt.Errorf("failed to define VM: %v", err)
	}

	// Start the VM
	_, err = ExecuteSSHCommand(fmt.Sprintf("virsh start %s", vmName), sshConfig)
	if err != nil {
		return fmt.Errorf("failed to start VM: %v", err)
	}

	// Enable autostart
	_, err = ExecuteSSHCommand(fmt.Sprintf("virsh autostart %s", vmName), sshConfig)
	if err != nil {
		g.GinkgoT().Logf("Warning: Failed to enable autostart for VM: %v", err)
	}

	// Clean up temporary XML file
	_, err = ExecuteSSHCommand(fmt.Sprintf("rm -f /tmp/%s.xml", vmName), sshConfig)
	if err != nil {
		g.GinkgoT().Logf("Warning: Failed to clean up temporary XML file: %v", err)
	}

	g.GinkgoT().Logf("Recreated VM: %s", vmName)
	return nil
}

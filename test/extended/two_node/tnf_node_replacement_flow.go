// TNF node replacement: setup, backup/recovery, VM destroy, etcd/pacemaker, and API/OVN delete path.
package two_node

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	metal3v1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/origin/test/extended/two_node/utils"
	"github.com/openshift/origin/test/extended/two_node/utils/apis"
	"github.com/openshift/origin/test/extended/two_node/utils/core"
	"github.com/openshift/origin/test/extended/two_node/utils/services"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"sigs.k8s.io/yaml"
)

func setupTestEnvironment(testConfig *TNFTestConfig, oc *exutil.CLI) {
	// Get hypervisor configuration from test context
	if !exutil.HasHypervisorConfig() {
		services.PrintHypervisorConfigUsage()
		o.Expect(fmt.Errorf("no hypervisor configuration available")).To(o.BeNil(), "Hypervisor configuration is required. See usage message above for configuration options.")
	}

	config := exutil.GetHypervisorConfig()
	testConfig.Hypervisor.Config.IP = config.HypervisorIP
	testConfig.Hypervisor.Config.User = config.SSHUser
	testConfig.Hypervisor.Config.PrivateKeyPath = config.PrivateKeyPath

	e2e.Logf("Using hypervisor configuration from test context:")
	e2e.Logf("  Hypervisor IP: %s", testConfig.Hypervisor.Config.IP)
	e2e.Logf("  SSH User: %s", testConfig.Hypervisor.Config.User)
	e2e.Logf("  Private Key Path: %s", testConfig.Hypervisor.Config.PrivateKeyPath)

	// Validate hypervisor IP address
	err := core.ValidateIPAddress(testConfig.Hypervisor.Config.IP)
	o.Expect(err).To(o.BeNil(), "Invalid hypervisor IP address: %v", err)

	// Validate that the private key file exists and has secure permissions
	if _, err := os.Stat(testConfig.Hypervisor.Config.PrivateKeyPath); os.IsNotExist(err) {
		o.Expect(err).To(o.BeNil(), "Private key file does not exist at path: %s", testConfig.Hypervisor.Config.PrivateKeyPath)
	}

	// Validate SSH key permissions for security
	err = core.ValidateSSHKeyPermissions(testConfig.Hypervisor.Config.PrivateKeyPath)
	o.Expect(err).To(o.BeNil(), "SSH private key validation failed: %v", err)

	knownHostsPath, err := core.PrepareLocalKnownHostsFile(&testConfig.Hypervisor.Config)
	o.Expect(err).To(o.BeNil(), "Expected to prepare local known hosts file without error")
	testConfig.Hypervisor.KnownHostsPath = knownHostsPath

	// Verify hypervisor connectivity and virsh availability
	err = services.VerifyHypervisorAvailability(&testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
	o.Expect(err).To(o.BeNil(), "Expected to verify hypervisor connectivity without error")

	// Set target and surviving node names dynamically (random selection)
	testConfig.TargetNode.Name, testConfig.SurvivingNode.Name = getRandomControlPlaneNode(oc)

	// Validate node names conform to Kubernetes conventions
	err = core.ValidateNodeName(testConfig.TargetNode.Name)
	o.Expect(err).To(o.BeNil(), "Invalid target node name: %v", err)
	err = core.ValidateNodeName(testConfig.SurvivingNode.Name)
	o.Expect(err).To(o.BeNil(), "Invalid surviving node name: %v", err)

	// Set dynamic resource names based on target node
	setDynamicResourceNames(testConfig, oc)

	// Get IP addresses for both nodes
	testConfig.TargetNode.IP, testConfig.SurvivingNode.IP = getNodeIPs(oc, testConfig.TargetNode.Name, testConfig.SurvivingNode.Name)

	// Validate node IP addresses
	err = core.ValidateIPAddress(testConfig.TargetNode.IP)
	o.Expect(err).To(o.BeNil(), "Invalid target node IP address: %v", err)
	err = core.ValidateIPAddress(testConfig.SurvivingNode.IP)
	o.Expect(err).To(o.BeNil(), "Invalid surviving node IP address: %v", err)

	// Compute Redfish IP from target node IP (gateway IP, works for both IPv4 and IPv6)
	// IPv4 example: 192.168.111.20 -> 192.168.111.1
	// IPv6 example: fd00::20 -> fd00::1
	testConfig.Execution.RedfishIP, err = computeGatewayIP(testConfig.TargetNode.IP)
	o.Expect(err).To(o.BeNil(), "Expected to compute Redfish gateway IP from target node IP %s: %v", testConfig.TargetNode.IP, err)
	e2e.Logf("Computed Redfish IP from target node IP %s: %s", testConfig.TargetNode.IP, testConfig.Execution.RedfishIP)

	// Prepare known hosts file for the surviving node
	// Note: We don't prepare the target node's known_hosts here because its SSH key will change
	// after reprovisioning. It will be prepared in restorePacemakerCluster after the node is ready.
	survivingNodeKnownHostsPath, err := core.PrepareRemoteKnownHostsFile(testConfig.SurvivingNode.IP, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
	o.Expect(err).To(o.BeNil(), "Expected to prepare surviving node known hosts file without error")
	testConfig.SurvivingNode.KnownHostsPath = survivingNodeKnownHostsPath

	e2e.Logf("Target node for replacement: %s (IP: %s)", testConfig.TargetNode.Name, testConfig.TargetNode.IP)
	e2e.Logf("Surviving node: %s (IP: %s)", testConfig.SurvivingNode.Name, testConfig.SurvivingNode.IP)
	e2e.Logf("Target node MAC: %s", testConfig.TargetNode.MAC)
	e2e.Logf("Target VM for replacement: %s", testConfig.TargetNode.VMName)
	e2e.Logf("Target machine name: %s", testConfig.TargetNode.MachineName)
	e2e.Logf("Redfish IP (gateway): %s", testConfig.Execution.RedfishIP)

	e2e.Logf("Test environment setup complete. Hypervisor IP: %s", testConfig.Hypervisor.Config.IP)
	e2e.Logf("setupTestEnvironment completed, testConfig.TargetNode.VMName: %s", testConfig.TargetNode.VMName)
}

// getRandomControlPlaneNode returns a random control plane node for replacement and the surviving node
func getRandomControlPlaneNode(oc *exutil.CLI) (string, string) {
	controlPlaneNodes, err := utils.GetNodes(oc, utils.LabelNodeRoleControlPlane)
	o.Expect(err).To(o.BeNil(), "Expected to get control plane nodes without error")

	// Ensure we have at least 2 control plane nodes
	o.Expect(len(controlPlaneNodes.Items)).To(o.BeNumerically(">=", 2), "Expected at least 2 control plane nodes for replacement test")

	// Select a random node using the same approach as other TNF recovery tests
	randomIndex := rand.Intn(len(controlPlaneNodes.Items))
	selectedNode := controlPlaneNodes.Items[randomIndex].Name
	core.ExpectNotEmpty(selectedNode, "Expected selected control plane node name to not be empty")

	// Validate node name format for security
	err = core.ValidateNodeName(selectedNode)
	e2e.Logf("Validate node name: %v", err)

	o.Expect(err).To(o.BeNil(), "Target node name validation failed: %v", err)

	// Find the surviving node (the other control plane node)
	var survivingNode string
	for i, node := range controlPlaneNodes.Items {
		if i != randomIndex {
			survivingNode = node.Name
			break
		}
	}

	// Validate that the surviving node name is not empty
	core.ExpectNotEmpty(survivingNode, "Expected surviving control plane node name to not be empty")

	// Validate surviving node name format for security
	err = core.ValidateNodeName(survivingNode)
	o.Expect(err).To(o.BeNil(), "Surviving node name validation failed: %v", err)

	e2e.Logf("Randomly selected control plane node for replacement: %s (index: %d)", selectedNode, randomIndex)
	e2e.Logf("Surviving control plane node: %s", survivingNode)

	return selectedNode, survivingNode
}

// setDynamicResourceNames sets all dynamic resource names based on the target node
func setDynamicResourceNames(testConfig *TNFTestConfig, oc *exutil.CLI) {
	// Set dynamic resource names based on target node
	testConfig.EtcdResources.PeerSecretName = fmt.Sprintf("%s-%s", etcdPeerSecretBaseName, testConfig.TargetNode.Name)
	testConfig.EtcdResources.ServingSecretName = fmt.Sprintf("%s-%s", etcdServingSecretBaseName, testConfig.TargetNode.Name)
	testConfig.EtcdResources.ServingMetricsSecretName = fmt.Sprintf("%s-%s", etcdServingMetricsSecretBaseName, testConfig.TargetNode.Name)
	testConfig.Jobs.AuthJobName = fmt.Sprintf("%s-%s", tnfAuthJobBaseName, testConfig.TargetNode.Name)
	testConfig.Jobs.AfterSetupJobName = fmt.Sprintf("%s-%s", tnfAfterSetupJobBaseName, testConfig.TargetNode.Name)
	// Update-setup jobs are created by CEO during node replacement - one per node
	testConfig.Jobs.UpdateSetupJobTargetName = fmt.Sprintf("%s-%s", tnfUpdateSetupJobBaseName, testConfig.TargetNode.Name)
	testConfig.Jobs.UpdateSetupJobSurvivorName = fmt.Sprintf("%s-%s", tnfUpdateSetupJobBaseName, testConfig.SurvivingNode.Name)

	bmcSecretName, err := apis.FindBMCSecretByNodeName(oc, machineAPINamespace, testConfig.TargetNode.Name)
	o.Expect(err).To(o.BeNil(), "Expected to find BMC secret for node %s", testConfig.TargetNode.Name)
	testConfig.TargetNode.BMCSecretName = bmcSecretName

	bmhName, err := apis.FindBMHByNodeName(oc, machineAPINamespace, testConfig.TargetNode.Name)
	o.Expect(err).To(o.BeNil(), "Expected to find BareMetalHost for node %s", testConfig.TargetNode.Name)
	testConfig.TargetNode.BMHName = bmhName

	// Get the MAC address of the target node from its BareMetalHost
	testConfig.TargetNode.MAC = getNodeMACAddress(oc, testConfig.TargetNode.Name)
	e2e.Logf("Found targetNodeMAC: %s for node: %s", testConfig.TargetNode.MAC, testConfig.TargetNode.Name)

	// Find the corresponding VM name by matching MAC addresses
	testConfig.TargetNode.VMName, err = services.GetVMNameByMACMatch(testConfig.TargetNode.Name, testConfig.TargetNode.MAC, virshProvisioningBridge, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
	e2e.Logf("GetVMNameByMACMatch returned: testConfig.TargetNode.VMName=%s, err=%v", testConfig.TargetNode.VMName, err)
	o.Expect(err).To(o.BeNil(), "Expected to find VM name for node %s with MAC %s: %v", testConfig.TargetNode.Name, testConfig.TargetNode.MAC, err)

	// Validate VM name is safe for shell commands
	err = core.ValidateResourceName(testConfig.TargetNode.VMName, "VM")
	o.Expect(err).To(o.BeNil(), "Invalid VM name: %v", err)

	// Ensure we found a valid VM name
	core.ExpectNotEmpty(testConfig.TargetNode.VMName, "Expected to find a valid VM name for node %s with MAC %s", testConfig.TargetNode.Name, testConfig.TargetNode.MAC)

	// Extract and store the machine name from the BMH consumerRef
	testConfig.TargetNode.MachineName = extractMachineNameFromBMH(oc, testConfig.TargetNode.Name)

	// Validate machine name is safe for shell commands
	err = core.ValidateResourceName(testConfig.TargetNode.MachineName, "machine")
	o.Expect(err).To(o.BeNil(), "Invalid machine name: %v", err)

	// Extract the machine hash from the machine name
	// Machine name format: {cluster}-{hash}-{role}-{index} (e.g., "ostest-abc123-master-0").
	// The cluster segment can contain dashes (e.g. infra ID); the hash is the third-from-last token.
	machineNameParts := strings.Split(testConfig.TargetNode.MachineName, "-")
	if len(machineNameParts) >= 4 {
		testConfig.TargetNode.MachineHash = machineNameParts[len(machineNameParts)-3]
		e2e.Logf("Extracted machine hash: %s from machine name: %s", testConfig.TargetNode.MachineHash, testConfig.TargetNode.MachineName)
	} else {
		e2e.Logf("WARNING: Unable to extract machine hash from machine name: %s (need at least 4 dash-separated segments; got %d)",
			testConfig.TargetNode.MachineName, len(machineNameParts))
	}

	// Record the surviving VM's libvirt disk paths before any destroy/recreate. If target dumpxml or
	// virsh vol-path ever resolves to the same file as the survivor, moving it aside would brick the survivor.
	testConfig.SurvivingNode.MAC = getNodeMACAddress(oc, testConfig.SurvivingNode.Name)
	survVM, survErr := services.GetVMNameByMACMatch(testConfig.SurvivingNode.Name, testConfig.SurvivingNode.MAC, virshProvisioningBridge, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
	o.Expect(survErr).To(o.BeNil(), "Expected to find VM name for surviving node %s with MAC %s", testConfig.SurvivingNode.Name, testConfig.SurvivingNode.MAC)
	testConfig.SurvivingNode.VMName = survVM
	err = core.ValidateResourceName(testConfig.SurvivingNode.VMName, "VM")
	o.Expect(err).To(o.BeNil(), "Invalid surviving VM name: %v", err)

	survPaths, survPathErr := hypervisorDiskPathSetForLiveVM(testConfig, testConfig.SurvivingNode.VMName)
	o.Expect(survPathErr).To(o.BeNil(), "Expected to enumerate libvirt disk paths for surviving VM %s: %v", testConfig.SurvivingNode.VMName, survPathErr)
	o.Expect(survPaths).NotTo(o.BeEmpty(), "surviving VM %s must have at least one resolvable disk path (safety guard)", testConfig.SurvivingNode.VMName)
	testConfig.Execution.SurvivorLibvirtDiskPaths = survPaths
	e2e.Logf("Surviving libvirt VM=%s disk path guard: %d unique path(s) recorded", testConfig.SurvivingNode.VMName, len(survPaths))
}

// getNodeIPs retrieves the IP addresses for the target and surviving nodes
func getNodeIPs(oc *exutil.CLI, targetNodeName, survivingNodeName string) (string, string) {
	// Get target node IP
	targetNodeIP, err := getNodeInternalIP(oc, targetNodeName)
	o.Expect(err).To(o.BeNil(), "Expected to get target node IP without error")
	core.ExpectNotEmpty(targetNodeIP, "Expected target node IP to not be empty")

	// Validate target node IP address
	err = core.ValidateIPAddress(targetNodeIP)
	o.Expect(err).To(o.BeNil(), "Target node IP validation failed: %v", err)

	// Get surviving node IP
	survivingNodeIP, err := getNodeInternalIP(oc, survivingNodeName)
	o.Expect(err).To(o.BeNil(), "Expected to get surviving node IP without error")
	core.ExpectNotEmpty(survivingNodeIP, "Expected surviving node IP to not be empty")

	// Validate surviving node IP address
	err = core.ValidateIPAddress(survivingNodeIP)
	o.Expect(err).To(o.BeNil(), "Surviving node IP validation failed: %v", err)

	e2e.Logf("Target node %s IP: %s", targetNodeName, targetNodeIP)
	e2e.Logf("Surviving node %s IP: %s", survivingNodeName, survivingNodeIP)

	return targetNodeIP, survivingNodeIP
}

// getNodeInternalIP gets the internal IP address of a node using JSON output for robust parsing
func getNodeInternalIP(oc *exutil.CLI, nodeName string) (string, error) {
	// Get node details in JSON format
	nodes, err := oc.AdminKubeClient().CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get node %s details: %v", nodeName, err)
	}

	// Find the InternalIP address from the node's status
	for _, addr := range nodes.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			if addr.Address == "" {
				return "", fmt.Errorf("node %s has empty internal IP address", nodeName)
			}
			e2e.Logf("Found internal IP %s for node %s", addr.Address, nodeName)
			return addr.Address, nil
		}
	}

	return "", fmt.Errorf("could not find internal IP address for node %s", nodeName)
}

// computeGatewayIP derives the BMC/Redfish gateway address for BMH templates from the target node's internal IP.
//
// The tests assume a dev-scripts / libvirt baremetal-IPI style lab layout: nodes and the gateway sit on a shared
// prefix where the gateway is the predictable ".1" / "::1" style address on that segment (IPv4: last octet 1;
// IPv6: last two bytes set to 0 and 1). That matches typical dev-scripts bridges; it is not a general algorithm
// for arbitrary production IPv6 networks.
//
// To support arbitrary IPv6 subnets you would need the real next hop or BMC-reachable address from the
// environment—e.g. default route or gateway from the node (ip -6 route, NM state), BareMetalHost status,
// hypervisor libvirt network XML, or an explicit test setting (env var / fixture) supplied by the job.
//
// For IPv4: 192.168.111.20 -> 192.168.111.1
// For IPv6 (lab heuristic): fd00::20 -> fd00::1
func computeGatewayIP(nodeIP string) (string, error) {
	// Parse the IP address to validate and determine if it's IPv4 or IPv6
	ip := net.ParseIP(nodeIP)
	if ip == nil {
		return "", fmt.Errorf("invalid IP address: %s", nodeIP)
	}

	// Check if it's IPv4 or IPv6
	if ip.To4() != nil {
		// IPv4: Set last byte to 1
		// Example: 192.168.111.20 -> 192.168.111.1
		ip = ip.To4()
		ip[3] = 1
		return ip.String(), nil
	}

	// IPv6: Set last two bytes to 1
	// Example: fd00::20 -> fd00::1
	// Example: fd00:0:0:0:0:0:0:20 -> fd00:0:0:0:0:0:0:1
	ip[len(ip)-2] = 0
	ip[len(ip)-1] = 1
	return ip.String(), nil
}

// getNodeMACAddress retrieves the MAC address for a node from its BareMetalHost
func getNodeMACAddress(oc *exutil.CLI, nodeName string) string {
	// Find the BareMetalHost name using pattern matching (handles FQDNs)
	bmhName, err := apis.FindBMHByNodeName(oc, machineAPINamespace, nodeName)
	o.Expect(err).To(o.BeNil(), "Expected to find BareMetalHost for node %s", nodeName)

	// Get the BareMetalHost YAML to extract the MAC address
	bmh, err := apis.GetBMH(oc, bmhName, machineAPINamespace)
	o.Expect(err).To(o.BeNil(), "Expected to get BareMetalHost without error")

	// Extract the MAC address from the BareMetalHost spec
	macAddress := bmh.Spec.BootMACAddress
	core.ExpectNotEmpty(macAddress, "Expected BareMetalHost %s to have a BootMACAddress", bmhName)

	e2e.Logf("Found MAC address %s for node %s", macAddress, nodeName)
	return macAddress
}

// extractMachineNameFromBMH extracts the machine name from BareMetalHost's consumerRef
func extractMachineNameFromBMH(oc *exutil.CLI, nodeName string) string {
	// Find the BareMetalHost name using pattern matching (handles FQDNs)
	bmhName, err := apis.FindBMHByNodeName(oc, machineAPINamespace, nodeName)
	o.Expect(err).To(o.BeNil(), "Expected to find BareMetalHost for node %s", nodeName)

	// Get the BareMetalHost YAML to extract the machine name
	bmh, err := apis.GetBMH(oc, bmhName, machineAPINamespace)
	o.Expect(err).To(o.BeNil(), "Expected to get BareMetalHost without error")

	// Extract the machine name from consumerRef
	o.Expect(bmh.Spec.ConsumerRef).ToNot(o.BeNil(), "Expected BareMetalHost to have a consumerRef")
	core.ExpectNotEmpty(bmh.Spec.ConsumerRef.Name, "Expected consumerRef to have a name")

	machineName := bmh.Spec.ConsumerRef.Name
	e2e.Logf("Found machine name: %s", machineName)
	return machineName
}

// initNodeReplacementLogDir creates ARTIFACT_DIR/node-replacement-logs/<timestamp>/ and sets
// testConfig.Execution.NodeReplacementLogDir. Used so CEO log capture and BMH/Machine delete diagnostics
// share one directory per spec run. No-op if ARTIFACT_DIR is unset or testConfig is nil.
func initNodeReplacementLogDir(testConfig *TNFTestConfig) {
	if testConfig == nil {
		return
	}
	artifactDir := os.Getenv("ARTIFACT_DIR")
	if artifactDir == "" {
		return
	}
	ts := time.Now().Format("20060102-150405")
	dir := filepath.Join(artifactDir, nodeReplacementLogsRootDirName, ts)
	if err := os.MkdirAll(dir, 0755); err != nil {
		e2e.Logf("node replacement logs: mkdir %s: %v", dir, err)
		return
	}
	testConfig.Execution.NodeReplacementLogDir = dir
	e2e.Logf("node replacement logs directory: %s", dir)
}

// startCEOLogCapture arranges for CEO logs to be captured at the end of the test (via the returned
// stop function, call with defer). A single `oc logs` fetch at teardown reads from the deployment's
// current pod(s), so output still covers recovery after the target node (and any CEO pod on it) is gone.
// logDir should be testConfig.Execution.NodeReplacementLogDir (timestamped node-replacement-logs path).
// No-op if logDir is empty.
func startCEOLogCapture(logDir string) (stop func()) {
	if logDir == "" {
		return func() {}
	}
	logPath := filepath.Join(logDir, "cluster-etcd-operator-node-replacement.log")
	return func() {
		cmd := exec.Command("oc", "logs", "-n", "openshift-etcd-operator", "deployment/etcd-operator", "--timestamps", "--all-containers=true")
		out, err := cmd.CombinedOutput()
		if err != nil {
			e2e.Logf("Failed to capture CEO logs at end of test: %v", err)
			return
		}
		if err := os.WriteFile(logPath, out, 0644); err != nil {
			e2e.Logf("Failed to write CEO log file %s: %v", logPath, err)
			return
		}
		e2e.Logf("Captured CEO logs to %s", logPath)
	}
}

// ========================================
// AfterEach Functions
// ========================================

// recoverClusterFromBackup restores the cluster from GlobalBackupDir (AfterEach runs whenever a backup directory exists).
//
// Numbered steps below are integers only. Step 0 restores the BMC Secret so credentials exist before VM recovery.
// Steps 8 and 9 run only when the current spec has already failed
// (g.CurrentSpecReport().Failed()); they are not part of the successful replacement flow.
//
// When the spec has already failed, steps 8–9 repair OVN before the rest of recovery continues:
//
//	PreReplacementChassisID is set at deleteNodeReferences entry (and refreshed before Node delete) so Step 8 runs
//	even when the spec failed before Node removal (e.g. BMO wait or BMH/Machine delete timeout).
//	- Step 8: Clear k8s.ovn.org/node-chassis-id when it still matches the pre-delete chassis while local OVS
//	  kept that system-id (for example unchanged guest disk). OVN-K re-annotates from local OVS until the
//	  annotation is cleared and ovnkube-node on that node is recycled so SB registration matches the host.
//	- Step 9: Restart ovnkube-node on both nodes and ovnkube-control-plane pods, then settle, so SB/NB views
//	  and dataplane state resync (stale remote chassis rows, lagging port_bindings, flaky east-west).
//
// Steps 10–11 always run: static-pod revision bump forces kube-apiserver/KCM/scheduler installers on all
// control-plane nodes when manifests are missing under /etc/kubernetes/manifests; pacemaker cleanup clears
// fencing/cleanup state on the survivor.
//
// recoveryAbort reports whether ctx is already cancelled or past its deadline (recovery budget exhausted).
func recoveryAbort(ctx context.Context, recoveryTimeout time.Duration, step string) bool {
	if err := ctx.Err(); err != nil {
		e2e.Logf("ERROR: Cluster recovery aborted before %s (overall deadline %v): %v", step, recoveryTimeout, err)
		return true
	}
	return false
}

// Overall recovery timeout: 20 minutes.
func recoverClusterFromBackup(testConfig *TNFTestConfig, oc *exutil.CLI) error {
	e2e.Logf("Starting cluster recovery from backup directory: %s", testConfig.Execution.GlobalBackupDir)

	// Mark that recovery is using the backup
	testConfig.Execution.BackupUsedForRecovery = true

	// Set up overall recovery timeout
	const recoveryTimeout = 20 * time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), recoveryTimeout)
	defer cancel()

	// Monitor timeout in a goroutine
	done := make(chan struct{})
	defer close(done)

	go func() {
		select {
		case <-ctx.Done():
			e2e.Logf("ERROR: Recovery timeout exceeded (%v) - recovery may be incomplete", recoveryTimeout)
		case <-done:
			// Recovery completed
		}
	}()

	defer func() {
		if r := recover(); r != nil {
			e2e.Logf("ERROR: Recovery failed with panic: %v", r)
		}
		// Clean up backup directory after recovery attempt
		if testConfig.Execution.GlobalBackupDir != "" {
			e2e.Logf("Cleaning up backup directory after recovery: %s", testConfig.Execution.GlobalBackupDir)
			os.RemoveAll(testConfig.Execution.GlobalBackupDir)
			testConfig.Execution.GlobalBackupDir = ""
		}
	}()

	// Step 0: Restore BMC credentials before long-running VM work so BMO never sees a host without a Secret while we recover.
	e2e.Logf("Step 0: Ensuring BMC secret from backup")
	if err := recreateBMCSecret(testConfig, oc, ctx); err != nil {
		e2e.Logf("ERROR: Failed to recreate BMC secret from backup (early): %v", err)
		return nil
	}

	// Step 1: Recreate the VM from backup
	if recoveryAbort(ctx, recoveryTimeout, "step 1") {
		return ctx.Err()
	}
	e2e.Logf("Step 1: Recreating VM from backup")
	if err := recoverVMFromBackup(testConfig, ctx); err != nil {
		e2e.Logf("ERROR: Failed to recover VM %s from backup at %s: %v",
			testConfig.TargetNode.VMName, testConfig.Execution.GlobalBackupDir, err)
		return nil
	}

	// Step 2: Best-effort restore etcd member Secrets from backup. recoverEtcdSecretsFromBackup does not
	// return errors: missing backups or apply failures are logged inside it; CEO recreates member secrets when the node rejoins.
	if recoveryAbort(ctx, recoveryTimeout, "step 2") {
		return ctx.Err()
	}
	e2e.Logf("Step 2: Recreating etcd secrets from backup")
	recoverEtcdSecretsFromBackup(testConfig, oc, ctx)

	// Step 3: Recreate BMH from backup (no detached so Ironic can provision). Machine is created after BMH provisions.
	if recoveryAbort(ctx, recoveryTimeout, "step 3") {
		return ctx.Err()
	}
	e2e.Logf("Step 3: Recreating BMH from backup")
	if err := recoverBMHFromBackup(testConfig, oc, ctx); err != nil {
		e2e.Logf("ERROR: Failed to recover BMH %s from backup: %v", testConfig.TargetNode.BMHName, err)
		return nil
	}

	// Step 4: Wait for BMH provisioning.
	if recoveryAbort(ctx, recoveryTimeout, "step 4") {
		return ctx.Err()
	}
	e2e.Logf("Step 4: Waiting for BMH provisioning")
	if err := waitForBMHProvisioning(ctx, testConfig, oc); err != nil {
		e2e.Logf("ERROR: BMH provisioning wait failed during recovery: %v", err)
		return nil
	}

	// Step 5: Apply detached annotation now that the host is provisioned.
	if recoveryAbort(ctx, recoveryTimeout, "step 5") {
		return ctx.Err()
	}
	e2e.Logf("Step 5: Applying detached annotation after provisioning")
	reapplyDetachedAnnotation(testConfig, oc, ctx)

	// Step 6: Recreate Machine from backup (after BMH has provisioned).
	if recoveryAbort(ctx, recoveryTimeout, "step 6") {
		return ctx.Err()
	}
	e2e.Logf("Step 6: Recreating Machine from backup")
	if err := recoverMachineFromBackup(testConfig, oc, ctx); err != nil {
		e2e.Logf("ERROR: Failed to recover Machine %s from backup: %v", testConfig.TargetNode.MachineName, err)
		return nil
	}

	// Step 7: Wait for node to register; approve node-bootstrapper CSR when machine-approver does not (reused node name).
	if recoveryAbort(ctx, recoveryTimeout, "step 7") {
		return ctx.Err()
	}
	e2e.Logf("Step 7: Waiting for node-bootstrapper CSR and approving if machine-approver has not (reused node name)")
	if err := apis.WaitForAndApproveNodeBootstrapperCSR(ctx, oc, testConfig.TargetNode.Name, csrApprovalWaitTimeout); err != nil {
		e2e.Logf("ERROR: Failed to approve node-bootstrapper CSR for node %s during recovery: %v", testConfig.TargetNode.Name, err)
		return nil
	}

	// Step 8 (failed spec only): Clear stale node-chassis-id / mismatched OVS identity on the replacement Node.
	if recoveryAbort(ctx, recoveryTimeout, "step 8") {
		return ctx.Err()
	}
	if g.CurrentSpecReport().Failed() {
		e2e.Logf("Step 8: Clear stale OVN chassis annotation on replacement node if needed (failed spec only)")
		clearStaleChassisAnnotationOnReplacementIfNeeded(oc, testConfig.TargetNode.Name, testConfig.Execution.PreReplacementChassisID)
	}

	// Step 9 (failed spec only): Full OVN-K recycle on both nodes so SB/NB and the dataplane resync.
	if recoveryAbort(ctx, recoveryTimeout, "step 9") {
		return ctx.Err()
	}
	if g.CurrentSpecReport().Failed() {
		e2e.Logf("Step 9: Restart OVN-Kubernetes on both nodes and settle (failed spec only; after step 8)")
		if err := recoverOVNKForNodeReplacement(oc, testConfig.SurvivingNode.Name, testConfig.TargetNode.Name); err != nil {
			e2e.Logf("WARNING: OVN-K recovery during backup recovery failed: %v", err)
		} else {
			logReplacementOVNChassisStaleIfPresent(oc, testConfig.TargetNode.Name, testConfig.Execution.PreReplacementChassisID,
				"after OVN-K recovery during backup recovery")
			e2e.Logf("Waiting %v after OVN-K recovery for data plane to settle", ovnkubeRestartSettleWait)
			select {
			case <-ctx.Done():
				e2e.Logf("ERROR: Recovery context cancelled during OVN-K settle wait: %v", ctx.Err())
				return ctx.Err()
			case <-time.After(ovnkubeRestartSettleWait):
			}
		}
	}

	// Step 10: Trace→Normal operator log level bump triggers a new revision so static-pod installers re-materialize
	// apiserver, KCM, and scheduler manifests on control-plane nodes that lack them after recovery.
	if recoveryAbort(ctx, recoveryTimeout, "step 10") {
		return ctx.Err()
	}
	e2e.Logf("Step 10: Bumping kube-apiserver / kube-controller-manager / kube-scheduler revision (static pod installers on recovered node)")
	forceStaticPodRevisionBump(oc)

	// Step 11: Clear pacemaker fencing/cleanup state on the survivor after disruptive work.
	if recoveryAbort(ctx, recoveryTimeout, "step 11") {
		return ctx.Err()
	}
	e2e.Logf("Step 11: Cleaning up pacemaker resources on survivor node")
	e2e.Logf("Running pcs resource cleanup on survivor: %s", testConfig.SurvivingNode.Name)
	_, _, err := services.PcsResourceCleanup(testConfig.SurvivingNode.IP, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath, testConfig.SurvivingNode.KnownHostsPath)
	if err != nil {
		e2e.Logf("WARNING: Failed to run pcs resource cleanup: %v", err)
	}

	e2e.Logf("Running pcs stonith cleanup on survivor: %s", testConfig.SurvivingNode.Name)
	_, _, err = services.PcsStonithCleanup(testConfig.SurvivingNode.IP, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath, testConfig.SurvivingNode.KnownHostsPath)
	if err != nil {
		e2e.Logf("WARNING: Failed to run pcs stonith cleanup: %v", err)
	}

	e2e.Logf("Waiting %v for pacemaker cleanup to settle", pacemakerCleanupWaitTime)
	select {
	case <-ctx.Done():
		e2e.Logf("ERROR: Recovery context cancelled during pacemaker settle wait: %v", ctx.Err())
		return ctx.Err()
	case <-time.After(pacemakerCleanupWaitTime):
	}

	e2e.Logf("Cluster recovery process completed")
	return nil
}

// ========================================
// Recovery Functions (called by recoverClusterFromBackup)
// ========================================

// recoverVMFromBackup recreates the VM from the backed up XML
func recoverVMFromBackup(testConfig *TNFTestConfig, ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	// Check if the specific VM already exists
	_, err := services.VirshVMExists(testConfig.TargetNode.VMName, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
	if err == nil {
		e2e.Logf("VM %s already exists, skipping recreation", testConfig.TargetNode.VMName)
		return nil
	}

	core.ExpectNotEmpty(testConfig.TargetNode.VMName, "Expected testConfig.TargetNode.VMName to be set before recreating VM")
	// Read the backed up XML
	xmlFile := filepath.Join(testConfig.Execution.GlobalBackupDir, testConfig.TargetNode.VMName+".xml")
	xmlContent, err := os.ReadFile(xmlFile)
	if err != nil {
		return fmt.Errorf("failed to read XML backup: %v", err)
	}
	xmlOutput := string(xmlContent)

	// Restore backing disk(s) from .tnf-backup if present.
	// Supports both file and volume-backed disks by resolving volume refs via virsh vol-path.
	diskRefs, extractErr := services.ExtractDiskSourceRefs(xmlOutput)
	if extractErr == nil {
		sshConfig := &testConfig.Hypervisor.Config
		knownHostsPath := testConfig.Hypervisor.KnownHostsPath
		seenDisk := make(map[string]struct{})
		for _, ref := range diskRefs {
			path, pathErr := resolveDiskSourceRefPath(ref, sshConfig, knownHostsPath)
			if pathErr != nil {
				e2e.Logf("WARNING: failed to resolve disk source (type=%s pool=%q volume=%q file=%q): %v (continuing)",
					ref.Type, ref.Pool, ref.Volume, ref.FilePath, pathErr)
				continue
			}
			if _, dup := seenDisk[path]; dup {
				continue
			}
			seenDisk[path] = struct{}{}
			mustNotCollideWithSurvivorDisk(testConfig, path)
			backupPath := path + backingDiskBackupSuffix
			qBackup := quotePathForShell(backupPath)
			qPath := quotePathForShell(path)
			restoreCmd := fmt.Sprintf("[ -f %s ] && sudo mv -f %s %s", qBackup, qBackup, qPath)
			_, _, restoreErr := core.ExecuteSSHCommand(restoreCmd, sshConfig, knownHostsPath)
			if restoreErr != nil {
				e2e.Logf("WARNING: failed to restore backing disk from %s to %s: %v (continuing)", backupPath, path, restoreErr)
			} else {
				e2e.Logf("Restored backing disk from %s to %s for recovery", backupPath, path)
			}
		}
	}

	// Create XML file on the hypervisor using secure method
	xmlPath := fmt.Sprintf(vmXMLFilePattern, testConfig.TargetNode.VMName)
	err = core.CreateRemoteFile(xmlPath, xmlOutput, core.StandardFileMode, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
	if err != nil {
		return fmt.Errorf("failed to create XML file on hypervisor: %w", err)
	}

	// Redefine the VM using the backed up XML
	err = services.VirshDefineVM(xmlPath, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
	if err != nil {
		return fmt.Errorf("failed to define VM: %v", err)
	}

	// Start the VM
	err = services.VirshStartVM(testConfig.TargetNode.VMName, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
	if err != nil {
		return fmt.Errorf("failed to start VM: %v", err)
	}

	// Enable autostart
	err = services.VirshAutostartVM(testConfig.TargetNode.VMName, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
	if err != nil {
		e2e.Logf("WARNING: Failed to enable autostart for VM: %v", err)
	}

	// Clean up temporary XML file
	xmlPath = fmt.Sprintf(vmXMLFilePattern, testConfig.TargetNode.VMName)
	err = core.DeleteRemoteTempFile(xmlPath, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
	if err != nil {
		e2e.Logf("WARNING: Failed to clean up temporary XML file: %v", err)
	}

	e2e.Logf("Recreated VM: %s", testConfig.TargetNode.VMName)
	if err := ctx.Err(); err != nil {
		return err
	}
	return services.WaitForVMState(testConfig.TargetNode.VMName, services.VMStateRunning, vmLibvirtRunningTimeout, utils.ThirtySecondPollInterval, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
}

// recoverEtcdSecretsFromBackup best-effort restores etcd member Secrets from on-disk backup
// (same helper as BMC: createSecretFromBackupIfNeeded). It returns nothing: missing backup files or
// restore failures are logged and skipped. The Cluster Etcd Operator recreates member secrets when
// the replacement control-plane node is admitted back into etcd.
func recoverEtcdSecretsFromBackup(testConfig *TNFTestConfig, oc *exutil.CLI, ctx context.Context) {
	etcdSecrets := []string{
		testConfig.EtcdResources.PeerSecretName,
		testConfig.EtcdResources.ServingSecretName,
		testConfig.EtcdResources.ServingMetricsSecretName,
	}

	for _, secretName := range etcdSecrets {
		if err := ctx.Err(); err != nil {
			e2e.Logf("WARNING: Skipping remaining etcd secret restores (recovery context done): %v", err)
			return
		}
		secretFile := filepath.Join(testConfig.Execution.GlobalBackupDir, secretName+".yaml")
		if _, err := os.Stat(secretFile); os.IsNotExist(err) {
			// Not an error: CEO will recreate this member secret after the node rejoins.
			e2e.Logf("WARNING: Backup file for etcd secret %s not found", secretName)
			continue
		}
		if err := createSecretFromBackupIfNeeded(oc, services.EtcdNamespace, testConfig.Execution.GlobalBackupDir, secretName); err != nil {
			// Non-fatal for the same reason as a missing backup file (see function comment).
			e2e.Logf("WARNING: Failed to restore etcd secret %s from backup: %v", secretName, err)
			continue
		}
	}
}

// bmhIsDeleting reports whether the BareMetalHost is being removed: API deletion in progress or Metal3 deleting state.
func bmhIsDeleting(bmh *metal3v1alpha1.BareMetalHost) bool {
	if bmh == nil {
		return false
	}
	if bmh.DeletionTimestamp != nil && !bmh.DeletionTimestamp.IsZero() {
		return true
	}
	return bmh.Status.Provisioning.State == metal3v1alpha1.StateDeleting
}

// bareMetalHostExistsWithTransientRetry retries apis.BareMetalHostExists on transient API errors until a definitive result or apiMax elapses.
// If apiMax elapses while errors remain transient: when onTimeoutAssumeExists, returns (true, nil) so recovery skips create; otherwise (false, nil) to attempt create.
func bareMetalHostExistsWithTransientRetry(oc *exutil.CLI, bmhName, namespace string, apiStart time.Time, apiMax, sleepOnTransient time.Duration, onTimeoutAssumeExists bool) (bool, error) {
	for {
		if time.Since(apiStart) > apiMax {
			if onTimeoutAssumeExists {
				e2e.Logf("WARNING: BareMetalHost %s existence check exceeded %v after transient API errors; assuming still present (skipping recreate)", bmhName, apiMax)
				return true, nil
			}
			e2e.Logf("WARNING: BareMetalHost %s existence check exceeded %v after transient API errors; proceeding as if absent (create may return AlreadyExists)", bmhName, apiMax)
			return false, nil
		}
		exists, err := apis.BareMetalHostExists(oc, bmhName, namespace)
		if err == nil {
			return exists, nil
		}
		if utils.IsTransientKubernetesAPIError(err) {
			e2e.Logf("WARNING: transient error checking BareMetalHost %s existence: %v; sleeping %v and retrying", bmhName, err, sleepOnTransient)
			time.Sleep(sleepOnTransient)
			continue
		}
		return false, err
	}
}

// getBareMetalHostRecover retries apis.GetBMH on transient errors. NotFound is returned as-is.
// If apiMax elapses while errors remain transient, logs a warning and returns (nil, true, nil): caller should skip BMH recreation.
func getBareMetalHostRecover(oc *exutil.CLI, bmhName, namespace string, apiStart time.Time, apiMax, sleepOnTransient time.Duration) (*metal3v1alpha1.BareMetalHost, bool, error) {
	for {
		if time.Since(apiStart) > apiMax {
			e2e.Logf("WARNING: BareMetalHost %s get exceeded %v after transient API errors; skipping BMH recreation step", bmhName, apiMax)
			return nil, true, nil
		}
		bmh, err := apis.GetBMH(oc, bmhName, namespace)
		if err == nil {
			return bmh, false, nil
		}
		if apierrors.IsNotFound(err) {
			return nil, false, err
		}
		if utils.IsTransientKubernetesAPIError(err) {
			e2e.Logf("WARNING: transient error getting BareMetalHost %s: %v; sleeping %v and retrying", bmhName, err, sleepOnTransient)
			time.Sleep(sleepOnTransient)
			continue
		}
		return nil, false, err
	}
}

// getBareMetalHostWaitRecover retries apis.GetBMH during BMH delete-wait. On transient timeout, returns assumeStillDeleting=true (caller continues wait).
func getBareMetalHostWaitRecover(oc *exutil.CLI, bmhName, namespace string, apiStart time.Time, apiMax, sleepOnTransient time.Duration) (bmh *metal3v1alpha1.BareMetalHost, assumeStillDeleting bool, err error) {
	for {
		if time.Since(apiStart) > apiMax {
			e2e.Logf("WARNING: BareMetalHost %s get during delete-wait exceeded %v after transient API errors; assuming still deleting", bmhName, apiMax)
			return nil, true, nil
		}
		bmh, err := apis.GetBMH(oc, bmhName, namespace)
		if err == nil {
			return bmh, false, nil
		}
		if apierrors.IsNotFound(err) {
			return nil, false, err
		}
		if utils.IsTransientKubernetesAPIError(err) {
			e2e.Logf("WARNING: transient error getting BareMetalHost %s during delete-wait: %v; sleeping %v and retrying", bmhName, err, sleepOnTransient)
			time.Sleep(sleepOnTransient)
			continue
		}
		return nil, false, err
	}
}

// recoverBMHFromBackup recreates BMC secret and BMH from backup. The BMH is created without the detached
// annotation so Ironic can provision; detached is applied later after provisioning. Call waitForBMHProvisioning
// and reapplyDetachedAnnotation before recreating the Machine.
func recoverBMHFromBackup(testConfig *TNFTestConfig, oc *exutil.CLI, recoverCtx context.Context) error {
	waitForBaremetalOperatorWebhookReady(oc, baremetalWebhookWaitTimeout)
	if err := recreateBMCSecret(testConfig, oc, recoverCtx); err != nil {
		return fmt.Errorf("failed to recreate BMC secret: %v", err)
	}
	dyn, err := dynamic.NewForConfig(oc.AdminConfig())
	if err != nil {
		return fmt.Errorf("create dynamic client: %w", err)
	}

	bmhFile := filepath.Join(testConfig.Execution.GlobalBackupDir, testConfig.TargetNode.BMHName+".yaml")
	apiRecoverStart := time.Now()
	apiRecoverMax := recoverBMHTerminatingMaxWait
	transientSleep := recoverBMHTerminatingPollInterval

	bmhExists, err := bareMetalHostExistsWithTransientRetry(oc, testConfig.TargetNode.BMHName, machineAPINamespace, apiRecoverStart, apiRecoverMax, transientSleep, false)
	if err != nil {
		return fmt.Errorf("check BareMetalHost existence: %w", err)
	}
	if bmhExists {
		bmh, giveUp, getErr := getBareMetalHostRecover(oc, testConfig.TargetNode.BMHName, machineAPINamespace, apiRecoverStart, apiRecoverMax, transientSleep)
		if giveUp {
			return nil
		}
		if getErr != nil {
			if apierrors.IsNotFound(getErr) {
				bmhExists = false
			} else {
				return fmt.Errorf("get BareMetalHost for recovery: %w", getErr)
			}
		} else if bmhIsDeleting(bmh) {
			e2e.Logf("BareMetalHost %s is deleting (deletionTimestamp=%v provisioningState=%s); waiting up to %v with %d checks every %v before recreate",
				testConfig.TargetNode.BMHName, bmh.DeletionTimestamp, bmh.Status.Provisioning.State,
				recoverBMHTerminatingMaxWait, recoverBMHTerminatingMaxChecks, recoverBMHTerminatingPollInterval)
			waitStart := time.Now()
			for check := 1; check <= recoverBMHTerminatingMaxChecks; check++ {
				if err := recoverCtx.Err(); err != nil {
					return err
				}
				if time.Since(waitStart) > recoverBMHTerminatingMaxWait {
					return fmt.Errorf("recovery wait for BareMetalHost %s exceeded %v (check %d/%d)",
						testConfig.TargetNode.BMHName, recoverBMHTerminatingMaxWait, check, recoverBMHTerminatingMaxChecks)
				}
				exists, pollErr := bareMetalHostExistsWithTransientRetry(oc, testConfig.TargetNode.BMHName, machineAPINamespace, apiRecoverStart, apiRecoverMax, transientSleep, true)
				if pollErr != nil {
					return fmt.Errorf("check BareMetalHost existence during recovery wait: %w", pollErr)
				}
				if !exists {
					e2e.Logf("BareMetalHost %s removed from API during recovery wait (check %d/%d)", testConfig.TargetNode.BMHName, check, recoverBMHTerminatingMaxChecks)
					bmhExists = false
					break
				}
				var assumeStillDeleting bool
				bmh, assumeStillDeleting, pollErr = getBareMetalHostWaitRecover(oc, testConfig.TargetNode.BMHName, machineAPINamespace, apiRecoverStart, apiRecoverMax, transientSleep)
				if pollErr != nil {
					if apierrors.IsNotFound(pollErr) {
						e2e.Logf("BareMetalHost %s not found during recovery wait (check %d/%d)", testConfig.TargetNode.BMHName, check, recoverBMHTerminatingMaxChecks)
						bmhExists = false
						break
					}
					return fmt.Errorf("get BareMetalHost during recovery wait: %w", pollErr)
				}
				if assumeStillDeleting {
					e2e.Logf("BareMetalHost %s delete-wait check %d/%d: treating as still deleting after API flake handling", testConfig.TargetNode.BMHName, check, recoverBMHTerminatingMaxChecks)
				} else if !bmhIsDeleting(bmh) {
					e2e.Logf("BareMetalHost %s exists and is no longer deleting (check %d/%d); skipping recreation", testConfig.TargetNode.BMHName, check, recoverBMHTerminatingMaxChecks)
					// BMO may have removed the BMC Secret while reconciling a stuck delete; re-ensure from backup.
					if err := recreateBMCSecret(testConfig, oc, recoverCtx); err != nil {
						return fmt.Errorf("re-ensure BMC secret after BMH left deleting state: %w", err)
					}
					return nil
				} else {
					e2e.Logf("BareMetalHost %s still deleting (check %d/%d); deletionTimestamp=%v provisioningState=%s",
						testConfig.TargetNode.BMHName, check, recoverBMHTerminatingMaxChecks, bmh.DeletionTimestamp, bmh.Status.Provisioning.State)
				}
				if check == recoverBMHTerminatingMaxChecks {
					return fmt.Errorf("BareMetalHost %s still deleting after %d checks every %v (max wait %v)",
						testConfig.TargetNode.BMHName, recoverBMHTerminatingMaxChecks, recoverBMHTerminatingPollInterval, recoverBMHTerminatingMaxWait)
				}
				select {
				case <-recoverCtx.Done():
					return recoverCtx.Err()
				case <-time.After(recoverBMHTerminatingPollInterval):
				}
			}
		} else {
			e2e.Logf("BareMetalHost %s already exists, skipping recreation", testConfig.TargetNode.BMHName)
			// Secret can be missing while BMH remains (e.g. partial delete); ensure credentials exist for the next reconcile.
			if err := recreateBMCSecret(testConfig, oc, recoverCtx); err != nil {
				return fmt.Errorf("re-ensure BMC secret when BMH already exists: %w", err)
			}
			return nil
		}
	}
	if !bmhExists {
		// Long delete waits or controller churn can drop the Secret before Create; restore again from backup.
		if err := recreateBMCSecret(testConfig, oc, recoverCtx); err != nil {
			return fmt.Errorf("re-ensure BMC secret before BareMetalHost create: %w", err)
		}
		bmhBytes, readErr := os.ReadFile(bmhFile)
		if readErr != nil {
			return fmt.Errorf("read BareMetalHost file: %w", readErr)
		}
		var bmhU unstructured.Unstructured
		if err := utils.DecodeObject(string(bmhBytes), &bmhU); err != nil {
			return fmt.Errorf("decode BareMetalHost YAML: %w", err)
		}
		// Clear server-assigned fields so Create() creates a new resource (API server will assign new UID and resourceVersion).
		bmhU.SetResourceVersion("")
		bmhU.SetUID("")
		// Remove detached annotation so BMO/Ironic perform provisioning (deploy RHCOS); backup had it set on the already-provisioned host.
		if ann := bmhU.GetAnnotations(); ann != nil {
			delete(ann, bmhDetachedAnnotationKey)
			bmhU.SetAnnotations(ann)
		}
		// Clear status so BMO treats the host as needing provisioning.
		delete(bmhU.Object, "status")
		err = core.RetryWithOptions(func() error {
			_, createErr := dyn.Resource(apis.BMHGVR).Namespace(machineAPINamespace).Create(recoverCtx, &bmhU, metav1.CreateOptions{})
			return createErr
		}, core.RetryOptions{
			Timeout:      etcdThreeMinutePollTimeout,
			PollInterval: utils.ThirtySecondPollInterval,
			MaxRetries:   10,
			ShouldRetry: func(err error) bool {
				return services.IsRetryableEtcdError(err) || utils.IsTransientKubernetesAPIError(err)
			},
		}, fmt.Sprintf("create BareMetalHost %s", testConfig.TargetNode.BMHName))
		if err != nil {
			return fmt.Errorf("failed to recreate BareMetalHost after retries: %v", err)
		}
		e2e.Logf("Recreated BareMetalHost: %s", testConfig.TargetNode.BMHName)
	}
	return nil
}

// recoverMachineFromBackup recreates the Machine from backup. Call after BMH has been provisioned and
// detached annotation applied. Sets spec.providerID from the current VM UUID so the Machine matches the
// provisioned host (providerID is derived from the VM UUID that Ironic uses to identify the host).
func recoverMachineFromBackup(testConfig *TNFTestConfig, oc *exutil.CLI, recoverCtx context.Context) error {
	dyn, err := dynamic.NewForConfig(oc.AdminConfig())
	if err != nil {
		return fmt.Errorf("create dynamic client: %w", err)
	}

	machineFile := filepath.Join(testConfig.Execution.GlobalBackupDir, testConfig.TargetNode.MachineName+"-machine.yaml")
	exists, err := apis.MachineExists(oc, testConfig.TargetNode.MachineName, machineAPINamespace)
	if err != nil {
		return fmt.Errorf("check Machine existence: %w", err)
	}
	if exists {
		e2e.Logf("Machine %s already exists, skipping recreation", testConfig.TargetNode.MachineName)
		return nil
	}
	machineBytes, readErr := os.ReadFile(machineFile)
	if readErr != nil {
		return fmt.Errorf("read Machine file: %w", readErr)
	}
	var u unstructured.Unstructured
	if err := utils.DecodeObject(string(machineBytes), &u); err != nil {
		return fmt.Errorf("decode Machine YAML: %w", err)
	}
	// Clear server-assigned fields so Create() creates a new resource (API server will assign new UID and resourceVersion).
	u.SetResourceVersion("")
	u.SetUID("")
	// Clear status so the controller sets nodeRef from the provisioned node.
	delete(u.Object, "status")
	// Set spec.providerID from the current VM UUID. The backup's providerID referred to the pre-destroy VM;
	// after recovery we recreated the VM from backup (same or new UUID) and the BMH was re-provisioned,
	// so the Machine must use the providerID that matches the current host identity (VM UUID).
	vmUUID, _, err := services.GetVMNetworkInfo(testConfig.TargetNode.VMName, virshProvisioningBridge, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
	if err != nil {
		return fmt.Errorf("get VM UUID for providerID: %w", err)
	}
	providerID := fmt.Sprintf("baremetalhost:///%s/%s/%s", machineAPINamespace, testConfig.TargetNode.BMHName, vmUUID)
	if spec, ok := u.Object["spec"].(map[string]interface{}); ok {
		spec["providerID"] = providerID
	}
	e2e.Logf("Setting Machine spec.providerID to %s (VM UUID: %s)", providerID, vmUUID)
	err = core.RetryWithOptions(func() error {
		_, createErr := dyn.Resource(apis.MachineGVR).Namespace(machineAPINamespace).Create(recoverCtx, &u, metav1.CreateOptions{})
		return createErr
	}, core.RetryOptions{
		Timeout:      etcdThreeMinutePollTimeout,
		PollInterval: utils.ThirtySecondPollInterval,
		MaxRetries:   10,
		ShouldRetry:  services.IsRetryableEtcdError,
	}, fmt.Sprintf("create Machine %s", testConfig.TargetNode.MachineName))
	if err != nil {
		return fmt.Errorf("failed to recreate Machine after retries: %v", err)
	}
	e2e.Logf("Recreated Machine: %s", testConfig.TargetNode.MachineName)
	return nil
}

// redfishServiceRootURL returns the Redfish service root URL for curl probes (same gateway/port as BMH/fencing).
func redfishServiceRootURL(redfishIP string) string {
	return fmt.Sprintf("https://%s/redfish/v1/", net.JoinHostPort(redfishIP, redfishPort))
}

// waitForRedfishRootReachableFromSurvivingNode runs curl against the Redfish service root from the surviving
// control-plane node (two-hop SSH via the hypervisor). Fencing and tnf-fencing-job use the same path from nodes;
// this avoids restoring BMC credentials while the virtual BMC is still unreachable from the cluster.
// The parent ctx cancels the wait (e.g. recovery overall deadline) in addition to redfishAPIReachableTimeout.
func waitForRedfishRootReachableFromSurvivingNode(ctx context.Context, testConfig *TNFTestConfig) error {
	if strings.TrimSpace(testConfig.Execution.RedfishIP) == "" {
		return fmt.Errorf("RedfishIP is empty; cannot probe Redfish reachability")
	}
	if strings.TrimSpace(testConfig.SurvivingNode.IP) == "" {
		return fmt.Errorf("surviving node IP is empty; cannot SSH for Redfish probe")
	}
	localKH := testConfig.Hypervisor.KnownHostsPath
	remoteKH := testConfig.SurvivingNode.KnownHostsPath
	if localKH == "" || remoteKH == "" {
		return fmt.Errorf("known_hosts paths not set (local=%q remote=%q)", localKH, remoteKH)
	}
	sshCfg := &testConfig.Hypervisor.Config
	url := redfishServiceRootURL(testConfig.Execution.RedfishIP)
	e2e.Logf("Waiting until Redfish root %s responds from surviving node %s (%s)", url, testConfig.SurvivingNode.Name, testConfig.SurvivingNode.IP)

	deadline := time.Now().Add(redfishAPIReachableTimeout)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return err
		}
		// Accept typical success/auth responses; reject connection/TLS failures (curl non-zero exit).
		quotedURL := strconv.Quote(url)
		cmd := fmt.Sprintf(`code=$(curl -g -k -sS -o /dev/null -w "%%{http_code}" --connect-timeout 10 --max-time 30 %s) || exit 1; echo "$code" | grep -Eq '^(200|401|403|404)$'`, quotedURL)
		stdout, stderr, err := core.ExecuteRemoteSSHCommand(testConfig.SurvivingNode.IP, cmd, sshCfg, localKH, remoteKH)
		if err == nil {
			e2e.Logf("Redfish probe succeeded from surviving node (stdout=%q)", strings.TrimSpace(stdout))
			return nil
		}
		e2e.Logf("Redfish probe from surviving node failed (retrying): %v stderr=%q stdout=%q", err, stderr, stdout)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(redfishAPIReachablePollInterval):
		}
	}
	return fmt.Errorf("timed out waiting for Redfish root %s reachable from surviving node", url)
}

// recreateBMCSecret recreates the BMC secret from backup via createSecretFromBackupIfNeeded (retries + verify).
func recreateBMCSecret(testConfig *TNFTestConfig, oc *exutil.CLI, ctx context.Context) error {
	if err := waitForRedfishRootReachableFromSurvivingNode(ctx, testConfig); err != nil {
		return err
	}
	return createSecretFromBackupIfNeeded(oc, machineAPINamespace, testConfig.Execution.GlobalBackupDir, testConfig.TargetNode.BMCSecretName)
}

// ========================================
// Main Test Functions (in order of execution)
// ========================================

// backupTargetNodeConfiguration backs up all necessary resources for node replacement
func backupTargetNodeConfiguration(testConfig *TNFTestConfig, oc *exutil.CLI) string {
	// Create backup directory
	var err error
	backupDir, err := os.MkdirTemp("", backupDirName)
	o.Expect(err).To(o.BeNil(), "Expected to create backup directory without error")

	ctx := context.Background()

	// Backup BMC secret via API
	bmcSecret, err := oc.AdminKubeClient().CoreV1().Secrets(machineAPINamespace).Get(ctx, testConfig.TargetNode.BMCSecretName, metav1.GetOptions{})
	o.Expect(err).To(o.BeNil(), "Expected to get BMC secret without error")
	bmcSecretBytes, err := yaml.Marshal(bmcSecret)
	o.Expect(err).To(o.BeNil(), "Expected to marshal BMC secret")
	bmcSecretFile := filepath.Join(backupDir, testConfig.TargetNode.BMCSecretName+".yaml")
	o.Expect(os.WriteFile(bmcSecretFile, bmcSecretBytes, core.SecureFileMode)).To(o.Succeed(), "Expected to write BMC secret backup")

	// Backup BareMetalHost via API
	bmh, err := apis.GetBMH(oc, testConfig.TargetNode.BMHName, machineAPINamespace)
	o.Expect(err).To(o.BeNil(), "Expected to get BareMetalHost without error")
	bmhBytes, err := yaml.Marshal(bmh)
	o.Expect(err).To(o.BeNil(), "Expected to marshal BareMetalHost")
	bmhFile := filepath.Join(backupDir, testConfig.TargetNode.BMHName+".yaml")
	o.Expect(os.WriteFile(bmhFile, bmhBytes, core.SecureFileMode)).To(o.Succeed(), "Expected to write BareMetalHost backup")

	// Backup Machine via API
	machineYAML, err := apis.GetMachineYAML(oc, testConfig.TargetNode.MachineName, machineAPINamespace)
	o.Expect(err).To(o.BeNil(), "Expected to get machine without error")
	machineFile := filepath.Join(backupDir, fmt.Sprintf("%s-machine.yaml", testConfig.TargetNode.MachineName))
	o.Expect(os.WriteFile(machineFile, machineYAML, core.SecureFileMode)).To(o.Succeed(), "Expected to write machine backup")

	// Etcd member Secrets: backup is strict like BMC/BMH/Machine — Get/marshal/write failures mean the API or backup
	// path is unhealthy; fail now instead of a broken partial backup. During recoverClusterFromBackup, restoring
	// those files is still best-effort (CEO recreates member secrets when the node rejoins; see recoverEtcdSecretsFromBackup).
	etcdSecrets := []string{
		testConfig.EtcdResources.PeerSecretName,
		testConfig.EtcdResources.ServingSecretName,
		testConfig.EtcdResources.ServingMetricsSecretName,
	}
	for _, secretName := range etcdSecrets {
		secret, err := oc.AdminKubeClient().CoreV1().Secrets(services.EtcdNamespace).Get(ctx, secretName, metav1.GetOptions{})
		o.Expect(err).To(o.BeNil(), "Expected to get etcd secret %s for backup", secretName)
		secretBytes, err := yaml.Marshal(secret)
		o.Expect(err).To(o.BeNil(), "Expected to marshal etcd secret %s for backup", secretName)
		secretFile := filepath.Join(backupDir, secretName+".yaml")
		o.Expect(os.WriteFile(secretFile, secretBytes, core.SecureFileMode)).To(o.Succeed(), "Expected to write etcd secret %s backup", secretName)
		e2e.Logf("Backed up etcd secret: %s", secretName)
	}

	e2e.Logf("About to validate testConfig.TargetNode.VMName, current value: %s", testConfig.TargetNode.VMName)
	// Validate that testConfig.TargetNode.VMName is set
	if testConfig.TargetNode.VMName == "" {
		e2e.Logf("testConfig.TargetNode.VMName bytes: %v", []byte(testConfig.TargetNode.VMName))
		e2e.Logf("ERROR: testConfig.TargetNode.VMName is empty! This should have been set in setupTestEnvironment")
		e2e.Logf("testConfig.TargetNode.Name: %s", testConfig.TargetNode.Name)
		e2e.Logf("testConfig.SurvivingNode.Name: %s", testConfig.SurvivingNode.Name)
		core.ExpectNotEmpty(testConfig.TargetNode.VMName, "Expected testConfig.TargetNode.VMName to be set before backing up VM configuration")
	}
	// Get XML dump of VM using SSH to hypervisor
	xmlOutput, err := services.VirshDumpXML(testConfig.TargetNode.VMName, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
	o.Expect(err).To(o.BeNil(), "Expected to get XML dump without error")

	xmlFile := filepath.Join(backupDir, testConfig.TargetNode.VMName+".xml")
	err = os.WriteFile(xmlFile, []byte(xmlOutput), core.SecureFileMode)
	o.Expect(err).To(o.BeNil(), "Expected to write XML dump to file without error")

	return backupDir
}

// destroyVM destroys the target VM using SSH to hypervisor
func destroyVM(testConfig *TNFTestConfig) {
	core.ExpectNotEmpty(testConfig.TargetNode.VMName, "Expected testConfig.TargetNode.VMName to be set before destroying VM")
	e2e.Logf("Destroying VM: %s", testConfig.TargetNode.VMName)

	// Undefine the VM first to prevent STONITH/fencing from restarting it
	e2e.Logf("Undefining VM %s to prevent fence recovery", testConfig.TargetNode.VMName)
	err := services.VirshUndefineVM(testConfig.TargetNode.VMName, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
	o.Expect(err).To(o.BeNil(), "Expected to undefine VM without error")
	e2e.Logf("VM %s undefined successfully", testConfig.TargetNode.VMName)

	// Destroy (stop) the VM
	e2e.Logf("Destroying (stopping) VM %s", testConfig.TargetNode.VMName)
	err = services.VirshDestroyVM(testConfig.TargetNode.VMName, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
	o.Expect(err).To(o.BeNil(), "Expected to destroy VM without error")
	e2e.Logf("VM %s destroyed successfully", testConfig.TargetNode.VMName)
}

// waitForEtcdToStop observes etcd stop on the surviving node
func waitForEtcdToStop(testConfig *TNFTestConfig) {
	e2e.Logf("Waiting for etcd to stop on surviving node: %s", testConfig.SurvivingNode.Name)

	// Check that etcd has stopped on the survivor before proceeding
	err := waitForEtcdResourceToStop(testConfig, etcdThreeMinutePollTimeout)
	if err != nil {
		// Log warning but don't fail - proceed with restoration anyway
		e2e.Logf("WARNING: etcd did not stop on surviving node %s within timeout: %v", testConfig.SurvivingNode.Name, err)
		e2e.Logf("WARNING: Proceeding with quorum restoration anyway")
	} else {
		e2e.Logf("etcd has stopped on surviving node %s", testConfig.SurvivingNode.Name)
	}
}

// restoreEtcdQuorum restores etcd quorum on the surviving node using a two-phase approach.
//
// Two-Phase Recovery Strategy:
//
// Phase 1: stonith confirm --force (Preferred)
//   - Confirms to Pacemaker that the failed node has been fenced
//   - This is the semantically correct way to handle manual fencing confirmation
//   - The VM was destroyed (undefine + destroy), so actual fencing will fail with
//     "Unable to get PowerState" errors - this is EXPECTED behavior
//   - stonith confirm tells Pacemaker "trust me, this node is dead" so it can proceed
//   - Waits up to etcdPhase1StartAfterStonithTimeout for etcd to start after confirmation (polling every 30s)
//
// Phase 2: STONITH disable + cleanup (Fallback)
//   - Used when Phase 1 does not bring etcd up on the survivor.
//   - Disables STONITH (safe here because the second node is destroyed).
//   - Runs pcs resource cleanup up to stonithCleanupMaxAttempts times (stonithCleanupRoundTimeout per attempt).
//   - Re-enables STONITH in defer regardless of success/failure.
//
// Note: Disabling STONITH is not generally recommended, but is safe in this specific
// scenario because we have verified the second node is destroyed and cannot cause split-brain.
func restoreEtcdQuorum(testConfig *TNFTestConfig, oc *exutil.CLI) {
	e2e.Logf("Restoring etcd quorum on surviving node: %s", testConfig.SurvivingNode.Name)

	// Try Phase 1: stonith confirm approach
	if tryStonithConfirm(testConfig, oc) {
		return // Success
	}

	// Fall back to Phase 2: STONITH disable + cleanup approach
	tryStonithDisableCleanup(testConfig, oc)
}

// tryStonithConfirm attempts to restore etcd quorum by confirming fencing completion
// This tells Pacemaker that the failed node has been successfully fenced,
// allowing it to proceed with normal recovery without waiting for actual fencing.
// Flow: verify VM destroyed → confirm fencing → wait for etcd to start → verify
// Returns true if successful, false if should fall back to Phase 2
func tryStonithConfirm(testConfig *TNFTestConfig, oc *exutil.CLI) bool {
	e2e.Logf("Phase 1: Attempting stonith confirm for failed node %s", testConfig.TargetNode.Name)

	// Step 1: Verify the VM no longer exists before confirming fencing
	// This is a safety check - we must not confirm fencing if the VM is still present
	e2e.Logf("Verifying VM %s no longer exists via 'virsh list --all'", testConfig.TargetNode.VMName)
	vmList, err := services.VirshList(&testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath, services.VirshListFlagAll, services.VirshListFlagName)
	if err != nil {
		e2e.Logf("WARNING: Failed to list VMs on hypervisor: %v", err)
		e2e.Logf("Will proceed to Phase 2 (STONITH disable + cleanup) fallback")
		return false
	}

	// Check if the target VM is in the list
	vmLines := strings.Split(strings.TrimSpace(vmList), "\n")
	for _, vmName := range vmLines {
		vmName = strings.TrimSpace(vmName)
		if vmName == testConfig.TargetNode.VMName {
			e2e.Logf("ERROR: VM %s still exists! Cannot confirm fencing for a VM that is still present.", testConfig.TargetNode.VMName)
			e2e.Logf("VM list output:\n%s", vmList)
			e2e.Logf("Will proceed to Phase 2 (STONITH disable + cleanup) fallback")
			return false
		}
	}
	e2e.Logf("Confirmed: VM %s does not exist (destruction verified)", testConfig.TargetNode.VMName)
	e2e.Logf("Using 'pcs stonith confirm --force' to tell Pacemaker the node is confirmed dead")

	// Log current pacemaker status before stonith confirm
	logPacemakerStatus(testConfig, "before stonith confirm")

	// Step 2: Confirm the fencing of the target node
	// This tells Pacemaker "trust me, this node is dead" and allows recovery to proceed
	output, stderr, err := services.PcsStonithConfirm(
		testConfig.TargetNode.Name,
		testConfig.SurvivingNode.IP,
		&testConfig.Hypervisor.Config,
		testConfig.Hypervisor.KnownHostsPath,
		testConfig.SurvivingNode.KnownHostsPath,
	)
	if err != nil {
		e2e.Logf("WARNING: Failed to run stonith confirm for %s on %s: %v", testConfig.TargetNode.Name, testConfig.SurvivingNode.Name, err)
		e2e.Logf("stdout: %s, stderr: %s", output, stderr)
		e2e.Logf("Will proceed to Phase 2 (STONITH disable + cleanup) fallback")
		return false
	}
	e2e.Logf("Successfully ran stonith confirm for %s, output: %s", testConfig.TargetNode.Name, output)

	e2e.Logf("Waiting %v for Pacemaker to process the stonith confirmation", stonithConfirmSettleWait)
	time.Sleep(stonithConfirmSettleWait)

	// Log pacemaker status after stonith confirm
	logPacemakerStatus(testConfig, "after stonith confirm")

	// Step 3: Wait for etcd to start after stonith confirm (poll every 30s; cap etcdPhase1StartAfterStonithTimeout).
	// After stonith confirm, Pacemaker should proceed with recovery and start etcd.
	e2e.Logf("Waiting up to %v for etcd to start after stonith confirm (checking every 30s)", etcdPhase1StartAfterStonithTimeout)
	err = waitForEtcdToStart(testConfig, etcdPhase1StartAfterStonithTimeout, utils.ThirtySecondPollInterval)
	if err != nil {
		e2e.Logf("WARNING: etcd did not start within %v after stonith confirm: %v", etcdPhase1StartAfterStonithTimeout, err)
		e2e.Logf("Will proceed to Phase 2 (STONITH disable + cleanup) fallback")
		return false
	}

	e2e.Logf("SUCCESS: etcd started on surviving node %s after stonith confirm", testConfig.SurvivingNode.Name)
	logPacemakerStatus(testConfig, "verification after stonith confirm")
	e2e.Logf("Successfully restored etcd quorum on surviving node: %s", testConfig.SurvivingNode.Name)
	waitForAPIResponsive(oc, etcdThreeMinutePollTimeout)

	return true // Success
}

// tryStonithDisableCleanup attempts to restore etcd using STONITH disable + resource cleanup
// This is the fallback approach when pcs debug-restart fails
func tryStonithDisableCleanup(testConfig *TNFTestConfig, oc *exutil.CLI) {
	e2e.Logf("Phase 2: Using STONITH disable + resource cleanup fallback approach")

	// Disable STONITH
	e2e.Logf("Disabling STONITH on surviving node %s", testConfig.SurvivingNode.Name)
	output, stderr, err := services.PcsStonithDisable(testConfig.SurvivingNode.IP, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath, testConfig.SurvivingNode.KnownHostsPath)
	if err != nil {
		e2e.Logf("ERROR: Failed to disable STONITH on %s: %v, stderr: %s", testConfig.SurvivingNode.Name, err, stderr)
		o.Expect(err).To(o.BeNil(), "Failed to disable STONITH on %s: %v, output: %s", testConfig.SurvivingNode.Name, err, output)
	}
	e2e.Logf("Successfully disabled STONITH on %s", testConfig.SurvivingNode.Name)

	// Ensure STONITH is re-enabled at the end, regardless of success or failure
	defer func() {
		e2e.Logf("Ensuring STONITH is re-enabled on surviving node %s", testConfig.SurvivingNode.Name)
		output, stderr, err := services.PcsStonithEnable(testConfig.SurvivingNode.IP, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath, testConfig.SurvivingNode.KnownHostsPath)
		if err != nil {
			e2e.Logf("WARNING: Failed to re-enable STONITH on %s: %v, stderr: %s", testConfig.SurvivingNode.Name, err, stderr)
			e2e.Logf("STONITH re-enable output: %s", output)
		} else {
			e2e.Logf("Successfully re-enabled STONITH on %s", testConfig.SurvivingNode.Name)
		}
	}()

	// Verify STONITH is actually disabled by checking pcs property
	e2e.Logf("Verifying STONITH is disabled by checking pcs property")
	propertyOutput, propertyStderr, propertyErr := services.PcsProperty(testConfig.SurvivingNode.IP, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath, testConfig.SurvivingNode.KnownHostsPath)
	if propertyErr != nil {
		e2e.Logf("WARNING: Failed to get pcs property on %s: %v, stderr: %s", testConfig.SurvivingNode.Name, propertyErr, propertyStderr)
	} else {
		e2e.Logf("Current pcs property configuration:\n%s", propertyOutput)
		// Check STONITH status (both "stonith-enabled: false" and "stonith-enabled=false" formats)
		if strings.Contains(propertyOutput, "stonith-enabled") && strings.Contains(propertyOutput, "false") {
			e2e.Logf("CONFIRMED: STONITH is disabled (stonith-enabled=false)")
		} else if strings.Contains(propertyOutput, "stonith-enabled") && strings.Contains(propertyOutput, "true") {
			e2e.Logf("WARNING: STONITH appears to still be enabled! Expected false but found true")
		} else {
			e2e.Logf("INFO: stonith-enabled property not found in output (may be using default)")
		}
	}

	e2e.Logf("Running pcs resource cleanup every minute (up to %d attempts) until etcd starts on surviving node %s", stonithCleanupMaxAttempts, testConfig.SurvivingNode.Name)

	maxAttempts := stonithCleanupMaxAttempts
	var lastErr error
	etcdStarted := false

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		e2e.Logf("Attempt %d/%d: Running pcs resource cleanup on surviving node %s", attempt, maxAttempts, testConfig.SurvivingNode.Name)
		output, stderr, err := services.PcsResourceCleanup(testConfig.SurvivingNode.IP, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath, testConfig.SurvivingNode.KnownHostsPath)
		if err != nil {
			e2e.Logf("WARNING: Failed to run pcs resource cleanup on %s (attempt %d/%d): %v, stderr: %s", testConfig.SurvivingNode.Name, attempt, maxAttempts, err, stderr)
			// Continue to check if etcd starts anyway
		} else {
			e2e.Logf("Successfully ran pcs resource cleanup on %s (attempt %d/%d), output: %s", testConfig.SurvivingNode.Name, attempt, maxAttempts, output)
		}

		// Wait up to 1 minute for etcd to start after this cleanup attempt
		e2e.Logf("Checking if etcd starts within 1 minute (attempt %d/%d)", attempt, maxAttempts)
		lastErr = waitForEtcdToStart(testConfig, stonithCleanupRoundTimeout, utils.FiveSecondPollInterval)
		if lastErr == nil {
			e2e.Logf("SUCCESS: etcd started on surviving node %s after %d cleanup attempt(s)", testConfig.SurvivingNode.Name, attempt)
			etcdStarted = true
			break
		}

		e2e.Logf("etcd has not started yet on %s after attempt %d/%d: %v", testConfig.SurvivingNode.Name, attempt, maxAttempts, lastErr)

		// If this wasn't the last attempt, wait won't happen again (cleanup runs every minute)
		// The stonithCleanupRoundTimeout in waitForEtcdToStart already provides the delay between attempts
	}

	// If etcd didn't start after all attempts, gather debug info and fail
	if !etcdStarted {
		e2e.Logf("ERROR: etcd did not start on %s after %d cleanup attempts over 3 minutes", testConfig.SurvivingNode.Name, maxAttempts)

		// Get pacemaker journal logs to help with debugging
		e2e.Logf("Getting pacemaker journal logs for debugging")
		journalOutput, _, journalErr := services.PcsJournal(pacemakerJournalLines, testConfig.SurvivingNode.IP, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath, testConfig.SurvivingNode.KnownHostsPath)
		if journalErr != nil {
			e2e.Logf("WARNING: Failed to get pacemaker journal logs on %s: %v", testConfig.SurvivingNode.Name, journalErr)
		} else {
			e2e.Logf("Last %d lines of pacemaker journal on %s:\n%s", pacemakerJournalLines, testConfig.SurvivingNode.Name, journalOutput)
		}

		// Get pacemaker status (--full for readable logs)
		pcsStatusOutput, _, pcsErr := services.PcsStatusFull(testConfig.SurvivingNode.IP, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath, testConfig.SurvivingNode.KnownHostsPath)
		if pcsErr != nil {
			e2e.Logf("WARNING: Failed to get pacemaker status on %s: %v", testConfig.SurvivingNode.Name, pcsErr)
		} else {
			e2e.Logf("Pacemaker status on %s:\n%s", testConfig.SurvivingNode.Name, pcsStatusOutput)
		}

		o.Expect(lastErr).To(o.BeNil(), "Expected etcd to start on surviving node %s within 3 minutes after resource cleanup attempts", testConfig.SurvivingNode.Name)
	}

	e2e.Logf("SUCCESS: etcd has started on surviving node %s", testConfig.SurvivingNode.Name)
	logPacemakerStatus(testConfig, "verification after STONITH cleanup")
	e2e.Logf("Successfully restored etcd quorum on surviving node: %s", testConfig.SurvivingNode.Name)
	waitForAPIResponsive(oc, etcdThreeMinutePollTimeout)
	// Stabilize baremetal operator before deletes: BMO may have been on the destroyed node and rescheduled
	// to the survivor; wait for webhook endpoints so deletes and later BMH create succeed.
	waitForBaremetalOperatorWebhookReady(oc, baremetalWebhookWaitTimeout)
}

// capturePreReplacementNodeIdentity records k8s.ovn.org/node-chassis-id and the Node UID on testConfig.Execution for
// recoverClusterFromBackup Step 8 (clear stale chassis) and for OVN SB chassis-del. Call at deleteNodeReferences entry
// so a failure before Node delete (e.g. waitForBaremetalOperatorDeploymentReady, BMH/Machine delete) still leaves
// PreReplacementChassisID set; call again immediately before Node delete to refresh values for SB cleanup.
func capturePreReplacementNodeIdentity(oc *exutil.CLI, testConfig *TNFTestConfig, phase string) {
	nodeName := testConfig.TargetNode.Name
	ctx, cancel := context.WithTimeout(context.Background(), shortK8sClientTimeout)
	defer cancel()
	node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		e2e.Logf("[chassis provision observe] %s: could not get node %s: %v", phase, nodeName, err)
		return
	}
	testConfig.Execution.PreReplacementChassisID = nodeOVNChassisIDFromNode(node)
	testConfig.Execution.PreReplacementNodeUID = string(node.UID)
	e2e.Logf("[chassis provision observe] %s: node-chassis-id=%q node.uid=%s (for recovery Step 8 + SB chassis-del; after same-name reprovision expect new uid)",
		phase, testConfig.Execution.PreReplacementChassisID, testConfig.Execution.PreReplacementNodeUID)
}

// deleteNodeReferences deletes OpenShift resources for the target node: BMH, Machine, Node; clears OVN SB chassis;
// drops Etcd and KubeAPIServer operator nodeStatuses for same-name replacement; deletes stale app=installer Pods;
// removes etcd TLS Secrets last so a spec failure earlier does not leave Secrets gone while the Node still exists
// (recoverClusterFromBackup restores those files best-effort; CEO may not recreate immediately if the Node API object remains).
func deleteNodeReferences(testConfig *TNFTestConfig, oc *exutil.CLI) {
	e2e.Logf("Deleting OpenShift resources for node: %s", testConfig.TargetNode.Name)

	capturePreReplacementNodeIdentity(oc, testConfig, "deleteNodeReferences entry")

	// Ensure BMO is ready to process deletes (reconciler running). We already waited for webhook earlier;
	// this waits for the deployment to have a ready replica so the controller can remove finalizers after Ironic cleanup.
	// After node loss BMO may have been on the destroyed node and needs time to reschedule onto the survivor.
	waitForBaremetalOperatorDeploymentReady(oc, baremetalOperatorDeploymentWaitTimeout)

	// Delete BMH then Machine first (order matters: Machine controller waits for BMH to be released).
	// BMO/Ironic need time to deprovision; deleteOcResourceWithRetry polls up to bmhMachineDeleteWaitTimeout (no force-delete).
	// Webhook/BMO readiness was waited above.
	err := deleteOcResourceWithRetry(oc, bmhResourceType, testConfig.TargetNode.BMHName, machineAPINamespace, testConfig)
	o.Expect(err).To(o.BeNil(), "Expected to delete BareMetalHost without error")

	err = deleteOcResourceWithRetry(oc, machineResourceType, testConfig.TargetNode.MachineName, machineAPINamespace, testConfig)
	o.Expect(err).To(o.BeNil(), "Expected to delete machine without error")

	// Delete the Node after BMH/Machine so the name is free when the replacement registers and CSRs are approved.
	// Etcd member TLS Secrets are removed at the end of this function (after Node removal and operator/OVN cleanup).
	nodeName := testConfig.TargetNode.Name
	capturePreReplacementNodeIdentity(oc, testConfig, "immediately before Node delete")
	logNodeOVNChassisWebhookDiag(oc, nodeName, "immediately before Node delete (pre-replacement)")
	err = core.RetryWithOptions(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), shortK8sClientTimeout)
		defer cancel()
		err := oc.AdminKubeClient().CoreV1().Nodes().Delete(ctx, nodeName, metav1.DeleteOptions{})
		if apierrors.IsNotFound(err) {
			return nil // already deleted, idempotent success
		}
		return err
	}, core.RetryOptions{
		MaxRetries:   maxDeleteRetries,
		PollInterval: utils.FiveSecondPollInterval,
	}, fmt.Sprintf("delete node %s", nodeName))
	o.Expect(err).To(o.BeNil(), "Expected to delete Node %s", nodeName)
	e2e.Logf("Node %s delete accepted; waiting until it is fully removed from the API (no Terminating object)", nodeName)
	waitUntilNodeFullyRemovedFromAPI(oc, nodeName, etcdThreeMinutePollTimeout)
	e2e.Logf("Node %s fully removed from API (name free for replacement; avoids stale k8s.ovn.org/node-chassis-id on same name)", nodeName)

	// OVN: remove the deleted node's chassis from SB via the surviving node's ovnkube-node/sbdb (do not delete that pod
	// here — that would drop the sbdb sidecar and block chassis-del). Poll until SB shows no matching Chassis rows before
	// reprovisioning. If the spec later fails and backup recovery runs, recoverClusterFromBackup steps 8–9 perform stale
	// annotation cleanup and a full OVN-K restart for SB/NB/dataplane resync (see that function’s header).
	o.Expect(deleteStaleOVNChassisFromSouthbound(oc, testConfig.SurvivingNode.Name, testConfig.TargetNode.Name, testConfig.Execution.PreReplacementChassisID)).To(o.Succeed(),
		"SB must show no Chassis for deleted hostname (and no Chassis named pre-replace node-chassis-id when captured) before replacement; see logs prefixed [OVN chassis cleanup]")

	logSurvivorOVSAfterChassisCleanup(testConfig)
	e2e.Logf("[post-node-delete OVN] Survivor %s: SB chassis-del complete (survivor ovnkube-node left running so sbdb remains available for chassis-del)", testConfig.SurvivingNode.Name)

	// Drop cluster-etcd-operator's nodeStatuses row for this node name. StaticPodOperatorStatus.nodeStatuses is keyed
	// only by nodeName; after same-name replacement the new Node has a new UID but status can still report the prior
	// currentRevision, so static-pod materialization may not re-run on the new host.
	etcdNodeStatusCleanupStart := time.Now()
	o.Expect(removeEtcdClusterOperatorNodeStatusForDeletedNode(oc, nodeName)).To(o.Succeed(),
		"remove etcd.operator.openshift.io/%s status.nodeStatuses entry for deleted node %s so CEO re-installs on replacement",
		etcdClusterOperatorCRName, nodeName)
	e2e.Logf("[stage timing] Etcd operator nodeStatus cleanup (post-Node-delete): %v (Get/UpdateStatus per attempt: %v, max conflict retries: %d)",
		time.Since(etcdNodeStatusCleanupStart), shortK8sClientTimeout, etcdOperatorNodeStatusCleanupMaxAttempts)

	// Drop cluster-kube-apiserver-operator's nodeStatuses row for this node name (same StaticPodOperatorStatus listMapKey=nodeName issue as Etcd).
	kasNodeStatusCleanupStart := time.Now()
	o.Expect(removeKubeAPIServerClusterOperatorNodeStatusForDeletedNode(oc, nodeName)).To(o.Succeed(),
		"remove kubeapiserver.operator.openshift.io/%s status.nodeStatuses entry for deleted node %s so KAO re-installs on replacement",
		kubeAPIServerOperatorCRName, nodeName)
	e2e.Logf("[stage timing] KubeAPIServer operator nodeStatus cleanup (post-Node-delete): %v (Get/UpdateStatus per attempt: %v, max conflict retries: %d)",
		time.Since(kasNodeStatusCleanupStart), shortK8sClientTimeout, etcdOperatorNodeStatusCleanupMaxAttempts)

	// Remove Completed (or stuck) installer Pods for this node so static-pod operators can schedule fresh installers
	// on the replacement (matches spec.nodeName or pod name suffix -<nodeName>, e.g. installer-2-retry-1-master-0).
	installerPodCleanupStart := time.Now()
	nInstaller, errInstaller := deleteStaleControlPlaneInstallerPodsForNode(oc, nodeName)
	o.Expect(errInstaller).To(o.Succeed(), "delete stale app=installer pods for removed node %s", nodeName)
	e2e.Logf("[stage timing] Static-pod installer pod cleanup (post-Node-delete): %v (deleted %d pod(s), list/delete timeout: %v per call)",
		time.Since(installerPodCleanupStart), nInstaller, shortK8sClientTimeout)

	// Delete etcd member TLS Secrets last: if the spec fails before this point, Secrets remain for the still-existing
	// Node (or for recovery). Best-effort restore in recoverClusterFromBackup is then less critical for quorum.
	etcdSecrets := []string{
		testConfig.EtcdResources.PeerSecretName,
		testConfig.EtcdResources.ServingSecretName,
		testConfig.EtcdResources.ServingMetricsSecretName,
	}
	for _, secretName := range etcdSecrets {
		err := core.RetryWithOptions(func() error {
			ctx, cancel := context.WithTimeout(context.Background(), shortK8sClientTimeout)
			defer cancel()
			return oc.AdminKubeClient().CoreV1().Secrets(services.EtcdNamespace).Delete(ctx, secretName, metav1.DeleteOptions{})
		}, core.RetryOptions{
			MaxRetries:   maxDeleteRetries,
			PollInterval: utils.FiveSecondPollInterval,
		}, fmt.Sprintf("delete secret %s", secretName))
		o.Expect(err).To(o.BeNil(), "Expected to delete %s secret without error", secretName)
	}

	e2e.Logf("OpenShift resources for node %s deleted successfully", testConfig.TargetNode.Name)
}

// waitUntilNodeFullyRemovedFromAPI polls until Get(nodeName) returns NotFound.
// After Nodes().Delete, the object can remain in Terminating with finalizers; the replacement
// kubelet may re-register the same node name. If the old Node still exists (even Terminating),
// controllers can leave stale annotations (e.g. k8s.ovn.org/node-chassis-id) and OVN/network-node-identity
// can reject updates. Waiting ensures the next Node object for this name is a clean create.
func waitUntilNodeFullyRemovedFromAPI(oc *exutil.CLI, nodeName string, timeout time.Duration) {
	err := core.PollUntil(func() (bool, error) {
		ctx, cancel := context.WithTimeout(context.Background(), shortK8sClientTimeout)
		defer cancel()
		node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			e2e.Logf("waitUntilNodeFullyRemovedFromAPI: get node %s: %v", nodeName, err)
			return false, nil
		}
		chassis := nodeOVNChassisIDFromNode(node)
		e2e.Logf("Node %s still present in API (Terminating or finalizers); waiting — [node-chassis-id diag] uid=%s terminating=%v chassis-id=%q",
			nodeName, node.UID, node.DeletionTimestamp != nil, chassis)
		return false, nil
	}, timeout, utils.FiveSecondPollInterval, fmt.Sprintf("node %s to be fully removed from API", nodeName))
	o.Expect(err).To(o.BeNil(), "Expected node %s to be fully removed within %v so replacement gets a clean Node (avoids stale OVN chassis-id)", nodeName, timeout)
}

// removeEtcdClusterOperatorNodeStatusForDeletedNode removes the status.nodeStatuses entry for deletedNodeName on the
// cluster-scoped etcd.operator.openshift.io Etcd (name cluster). Call only after the Node object is fully removed so
// a replacement registering the same node name is not treated as already at currentRevision.
//
// Implementation: Get the Etcd, drop the matching NodeStatus from the slice, Etcds().UpdateStatus (full status body).
// Merge/json patch of nodeStatuses alone is unreliable for this CR; replacing status via UpdateStatus matches a full
// status subresource write (same effect as kubectl/oc replace with --subresource=status and a complete status document).
func removeEtcdClusterOperatorNodeStatusForDeletedNode(oc *exutil.CLI, deletedNodeName string) error {
	etcdClient := oc.AdminOperatorClient().OperatorV1().Etcds()
	var lastErr error
	for attempt := 1; attempt <= etcdOperatorNodeStatusCleanupMaxAttempts; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), shortK8sClientTimeout)
		etcdObj, err := etcdClient.Get(ctx, etcdClusterOperatorCRName, metav1.GetOptions{})
		cancel()
		if err != nil {
			return fmt.Errorf("get etcd.operator.openshift.io %s: %w", etcdClusterOperatorCRName, err)
		}
		ns := etcdObj.Status.NodeStatuses
		if len(ns) == 0 {
			e2e.Logf("[etcd nodeStatus cleanup] etcd %s has no status.nodeStatuses; skipping", etcdClusterOperatorCRName)
			return nil
		}
		filtered := make([]operatorv1.NodeStatus, 0, len(ns))
		removed := false
		for i := range ns {
			if ns[i].NodeName == deletedNodeName {
				removed = true
				e2e.Logf("[etcd nodeStatus cleanup] dropping nodeStatus for nodeName=%q (same-name replacement)", deletedNodeName)
				continue
			}
			filtered = append(filtered, ns[i])
		}
		if !removed {
			e2e.Logf("[etcd nodeStatus cleanup] no nodeStatus for nodeName=%q on etcd %s; nothing to remove", deletedNodeName, etcdClusterOperatorCRName)
			return nil
		}
		etcdObj.Status.NodeStatuses = filtered
		ctx2, cancel2 := context.WithTimeout(context.Background(), shortK8sClientTimeout)
		_, lastErr = etcdClient.UpdateStatus(ctx2, etcdObj, metav1.UpdateOptions{})
		cancel2()
		if lastErr == nil {
			e2e.Logf("[etcd nodeStatus cleanup] Etcds().UpdateStatus(%s): removed status.nodeStatuses for %s",
				etcdClusterOperatorCRName, deletedNodeName)
			return nil
		}
		if apierrors.IsConflict(lastErr) && attempt < etcdOperatorNodeStatusCleanupMaxAttempts {
			e2e.Logf("[etcd nodeStatus cleanup] conflict on UpdateStatus (attempt %d/%d): %v", attempt, etcdOperatorNodeStatusCleanupMaxAttempts, lastErr)
			time.Sleep(time.Second)
			continue
		}
		return fmt.Errorf("UpdateStatus etcd %s after removing node %q: %w", etcdClusterOperatorCRName, deletedNodeName, lastErr)
	}
	if lastErr != nil {
		return lastErr
	}
	return nil
}

// removeKubeAPIServerClusterOperatorNodeStatusForDeletedNode removes the status.nodeStatuses entry for deletedNodeName on the
// cluster-scoped kubeapiserver.operator.openshift.io KubeAPIServer (name cluster). Call only after the Node object is fully removed so
// a replacement registering the same node name is not treated as already at currentRevision.
//
// Implementation matches removeEtcdClusterOperatorNodeStatusForDeletedNode: Get the KubeAPIServer, drop the matching NodeStatus from the slice,
// KubeAPIServers().UpdateStatus (full status body).
func removeKubeAPIServerClusterOperatorNodeStatusForDeletedNode(oc *exutil.CLI, deletedNodeName string) error {
	kasClient := oc.AdminOperatorClient().OperatorV1().KubeAPIServers()
	var lastErr error
	for attempt := 1; attempt <= etcdOperatorNodeStatusCleanupMaxAttempts; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), shortK8sClientTimeout)
		kasObj, err := kasClient.Get(ctx, kubeAPIServerOperatorCRName, metav1.GetOptions{})
		cancel()
		if err != nil {
			return fmt.Errorf("get kubeapiserver.operator.openshift.io %s: %w", kubeAPIServerOperatorCRName, err)
		}
		ns := kasObj.Status.NodeStatuses
		if len(ns) == 0 {
			e2e.Logf("[kube-apiserver nodeStatus cleanup] kubeapiserver %s has no status.nodeStatuses; skipping", kubeAPIServerOperatorCRName)
			return nil
		}
		filtered := make([]operatorv1.NodeStatus, 0, len(ns))
		removed := false
		for i := range ns {
			if ns[i].NodeName == deletedNodeName {
				removed = true
				e2e.Logf("[kube-apiserver nodeStatus cleanup] dropping nodeStatus for nodeName=%q (same-name replacement)", deletedNodeName)
				continue
			}
			filtered = append(filtered, ns[i])
		}
		if !removed {
			e2e.Logf("[kube-apiserver nodeStatus cleanup] no nodeStatus for nodeName=%q on kubeapiserver %s; nothing to remove", deletedNodeName, kubeAPIServerOperatorCRName)
			return nil
		}
		kasObj.Status.NodeStatuses = filtered
		ctx2, cancel2 := context.WithTimeout(context.Background(), shortK8sClientTimeout)
		_, lastErr = kasClient.UpdateStatus(ctx2, kasObj, metav1.UpdateOptions{})
		cancel2()
		if lastErr == nil {
			e2e.Logf("[kube-apiserver nodeStatus cleanup] KubeAPIServers().UpdateStatus(%s): removed status.nodeStatuses for %s",
				kubeAPIServerOperatorCRName, deletedNodeName)
			return nil
		}
		if apierrors.IsConflict(lastErr) && attempt < etcdOperatorNodeStatusCleanupMaxAttempts {
			e2e.Logf("[kube-apiserver nodeStatus cleanup] conflict on UpdateStatus (attempt %d/%d): %v", attempt, etcdOperatorNodeStatusCleanupMaxAttempts, lastErr)
			time.Sleep(time.Second)
			continue
		}
		return fmt.Errorf("UpdateStatus kubeapiserver %s after removing node %q: %w", kubeAPIServerOperatorCRName, deletedNodeName, lastErr)
	}
	if lastErr != nil {
		return lastErr
	}
	return nil
}

// deleteStaleControlPlaneInstallerPodsForNode deletes app=installer Pods in static-pod operator namespaces that
// still refer to deletedNodeName: either scheduled on that node (spec.nodeName) or named with suffix -<nodeName>.
func deleteStaleControlPlaneInstallerPodsForNode(oc *exutil.CLI, deletedNodeName string) (deleted int, err error) {
	client := oc.AdminKubeClient().CoreV1()
	suffix := "-" + deletedNodeName
	for _, ns := range staticPodOperatorInstallerNamespaces {
		ctx, cancel := context.WithTimeout(context.Background(), shortK8sClientTimeout)
		pods, listErr := client.Pods(ns).List(ctx, metav1.ListOptions{LabelSelector: "app=installer"})
		cancel()
		if listErr != nil {
			return deleted, fmt.Errorf("list installer pods in namespace %s: %w", ns, listErr)
		}
		for i := range pods.Items {
			p := &pods.Items[i]
			if p.Spec.NodeName != deletedNodeName && !strings.HasSuffix(p.Name, suffix) {
				continue
			}
			ctx2, cancel2 := context.WithTimeout(context.Background(), shortK8sClientTimeout)
			delErr := client.Pods(ns).Delete(ctx2, p.Name, metav1.DeleteOptions{})
			cancel2()
			if delErr != nil && !apierrors.IsNotFound(delErr) {
				return deleted, fmt.Errorf("delete pod %s/%s: %w", ns, p.Name, delErr)
			}
			deleted++
			e2e.Logf("[installer pod cleanup] deleted %s/%s (nodeName=%q spec.nodeName=%q)", ns, p.Name, deletedNodeName, p.Spec.NodeName)
		}
	}
	if deleted == 0 {
		e2e.Logf("[installer pod cleanup] no matching app=installer pods for node %q in static-pod operator namespaces", deletedNodeName)
	}
	return deleted, nil
}

func podSpecHasContainer(pod *corev1.Pod, containerName string) bool {
	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Name == containerName {
			return true
		}
	}
	return false
}

// waitUntilSouthboundShowsNoChassisForDeletedNode polls ovn-sbctl until this SB view has no Chassis whose hostname
// matches the deleted Node name and (when preReplaceChassisName is set) no Chassis whose name equals that string
// (the usual k8s.ovn.org/node-chassis-id / Chassis.name). Logs each non-ready poll for post-mortem proof.
func waitUntilSouthboundShowsNoChassisForDeletedNode(
	execSbctl func(sbctlArgs ...string) (string, error),
	targetNodeName, preReplaceChassisName, proofPodDesc string,
) error {
	desc := fmt.Sprintf("SB view %s: no Chassis for hostname=%q", proofPodDesc, targetNodeName)
	if preReplaceChassisName != "" {
		desc = fmt.Sprintf("%s and no Chassis named %q", desc, preReplaceChassisName)
	}
	return core.PollUntil(func() (bool, error) {
		byHostOut, byHostErr := execSbctl("ovn-sbctl", "--columns=name,hostname", "--no-headings", "find", "chassis", fmt.Sprintf("hostname=%q", targetNodeName))
		if byHostErr != nil {
			e2e.Logf("[OVN chassis cleanup] proof poll %s: find chassis hostname=%q: %v", proofPodDesc, targetNodeName, byHostErr)
			return false, nil
		}
		hostTrim := strings.TrimSpace(byHostOut)
		if hostTrim != "" {
			e2e.Logf("[OVN chassis cleanup] proof poll %s: SB still lists Chassis for hostname=%q: %q", proofPodDesc, targetNodeName, hostTrim)
			return false, nil
		}
		if preReplaceChassisName != "" {
			byNameOut, byNameErr := execSbctl("ovn-sbctl", "--columns=name,hostname", "--no-headings", "find", "chassis", fmt.Sprintf("name=%q", preReplaceChassisName))
			if byNameErr != nil {
				e2e.Logf("[OVN chassis cleanup] proof poll %s: find chassis name=%q: %v", proofPodDesc, preReplaceChassisName, byNameErr)
				return false, nil
			}
			nameTrim := strings.TrimSpace(byNameOut)
			if nameTrim != "" {
				e2e.Logf("[OVN chassis cleanup] proof poll %s: SB still lists Chassis named %q (pre-replace node-chassis-id): %q", proofPodDesc, preReplaceChassisName, nameTrim)
				return false, nil
			}
		}
		suffix := ""
		if preReplaceChassisName != "" {
			suffix = fmt.Sprintf(" and no Chassis named %q", preReplaceChassisName)
		}
		e2e.Logf("[OVN chassis cleanup] proof OK %s: no SB Chassis for hostname=%q%s (SB clean before replacement provisioning; node-chassis-id on new Node still follows local OVS system-id)",
			proofPodDesc, targetNodeName, suffix)
		return true, nil
	}, ovnSBChassisAbsentWaitTimeout, ovnSBChassisAbsentPollInterval, desc)
}

// deleteStaleOVNChassisFromSouthbound removes stale OVN Southbound Chassis rows for the deleted node.
//
// What is the stale record?
//   - OVN-K registers each node as a row in the SB Chassis table. chassis-add NAME creates Chassis.name=NAME;
//     the Node annotation k8s.ovn.org/node-chassis-id is that same NAME (not necessarily the row _uuid).
//   - After Node delete, that row can linger; a replacement with the same node name may collide.
//
// How ovn-sbctl deletes:
//   - chassis-del CHASSIS removes the row whose Chassis.name equals CHASSIS (see ovn-sbctl(8) chassis-add/chassis-del).
//   - Passing a row _uuid when name differs yields: "no chassis named <uuid>" — wrong identifier, not a missing row.
//     Prefer Chassis.name from `ovn-sbctl --columns=name --no-headings find chassis hostname="<node>"`, or chassis-del
//     using k8s.ovn.org/node-chassis-id from the Node annotation when that value is the chassis name.
//   - --if-exists makes delete idempotent (no error if the chassis is already gone).
//
// We run cleanup against ovnkube-control-plane first (central SB), then surviving node's ovnkube-node.
// Returns an error if no SB exec path succeeds or if post-delete SB proof polling times out.
func deleteStaleOVNChassisFromSouthbound(oc *exutil.CLI, survivingNodeName, targetNodeName, chassisID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), shortK8sClientTimeout)
	defer cancel()

	// sbctl runs inside podName/container (sbdb).
	runChassisCleanup := func(podName, container string) error {
		proofPodDesc := fmt.Sprintf("pod=%s/%s", podName, container)
		execSbctl := func(sbctlArgs ...string) (string, error) {
			args := append([]string{"-n", ovnKubernetesNamespace, podName, "-c", container, "--"}, sbctlArgs...)
			out, err := oc.AsAdmin().Run("exec").Args(args...).Output()
			return string(out), err
		}
		// 1) Any Chassis with hostname == deleted Node name: delete by Chassis.name (not _uuid).
		// ovn-sbctl requires global options (e.g. --columns) before the command word "find"; otherwise
		// "--columns" is parsed as part of the table condition and fails (observed with ovn-sbctl 25.x).
		findOut, findErr := execSbctl("ovn-sbctl", "--columns=name", "--no-headings", "find", "chassis", fmt.Sprintf("hostname=%q", targetNodeName))
		if findErr != nil {
			e2e.Logf("[OVN chassis cleanup] find chassis hostname=%q on pod %s: %v (will still try annotation name)", targetNodeName, podName, findErr)
		} else {
			var uniq []string
			seen := make(map[string]struct{})
			for _, line := range strings.Split(strings.TrimSpace(findOut), "\n") {
				for _, tok := range strings.Fields(line) {
					tok = strings.TrimSpace(tok)
					if tok == "" {
						continue
					}
					if _, ok := seen[tok]; ok {
						continue
					}
					seen[tok] = struct{}{}
					uniq = append(uniq, tok)
				}
			}
			if len(uniq) == 0 {
				e2e.Logf("[OVN chassis cleanup] No SB Chassis row with hostname=%q on pod %s (already removed or not registered yet)", targetNodeName, podName)
			}
			for _, chName := range uniq {
				_, delErr := execSbctl("ovn-sbctl", "--if-exists", "chassis-del", chName)
				if delErr != nil {
					return fmt.Errorf("chassis-del --if-exists %q after hostname=%q find: %w", chName, targetNodeName, delErr)
				}
				e2e.Logf("[OVN chassis cleanup] chassis-del --if-exists %q (hostname=%q) on pod %s", chName, targetNodeName, podName)
			}
		}
		// 2) Delete by pre-replacement k8s.ovn.org/node-chassis-id (= Chassis.name in normal OVN-K registration).
		if chassisID != "" {
			_, delErr := execSbctl("ovn-sbctl", "--if-exists", "chassis-del", chassisID)
			if delErr != nil {
				return fmt.Errorf("chassis-del --if-exists %q: %w", chassisID, delErr)
			}
			e2e.Logf("[OVN chassis cleanup] chassis-del --if-exists %q (node annotation name) on pod %s", chassisID, podName)
		}
		// 3) Proof: poll until this SB view shows no Chassis for the deleted hostname (and no Chassis named
		// pre-replace id when known). chassis-del can succeed while find still lists rows until SB converges.
		if err := waitUntilSouthboundShowsNoChassisForDeletedNode(execSbctl, targetNodeName, chassisID, proofPodDesc); err != nil {
			return fmt.Errorf("SB proof after chassis-del on %s: %w", proofPodDesc, err)
		}
		return nil
	}

	// Try central (ovnkube-control-plane) first.
	cpPods, err := oc.AdminKubeClient().CoreV1().Pods(ovnKubernetesNamespace).List(ctx, metav1.ListOptions{LabelSelector: "app=ovnkube-control-plane"})
	if err == nil && len(cpPods.Items) > 0 {
		for i := range cpPods.Items {
			if cpPods.Items[i].Status.Phase != corev1.PodRunning {
				continue
			}
			podName := cpPods.Items[i].Name
			if !podSpecHasContainer(&cpPods.Items[i], ovnkubeNodeSBDBContainer) {
				e2e.Logf("[OVN chassis cleanup] Central pod %q has no %q container; skipping (use surviving ovnkube-node)", podName, ovnkubeNodeSBDBContainer)
				continue
			}
			if err := runChassisCleanup(podName, ovnkubeNodeSBDBContainer); err != nil {
				e2e.Logf("[OVN chassis cleanup] Central pod %q: %v; trying next central pod or surviving node", podName, err)
				continue
			}
			e2e.Logf("[OVN chassis cleanup] Completed SB chassis cleanup via central pod %s", podName)
			return nil
		}
	} else {
		e2e.Logf("[OVN chassis cleanup] No ovnkube-control-plane pods; trying surviving node ovnkube-node")
	}

	pods, err := oc.AdminKubeClient().CoreV1().Pods(ovnKubernetesNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=ovnkube-node",
		FieldSelector: "spec.nodeName=" + survivingNodeName,
	})
	if err != nil || len(pods.Items) == 0 {
		return fmt.Errorf("no ovnkube-node on surviving node %q (%v); cannot run chassis-del or SB proof for hostname=%q",
			survivingNodeName, err, targetNodeName)
	}
	var podName string
	for i := range pods.Items {
		if pods.Items[i].Status.Phase == corev1.PodRunning {
			podName = pods.Items[i].Name
			break
		}
	}
	if podName == "" {
		return fmt.Errorf("no running ovnkube-node on surviving node %q; cannot run chassis-del or SB proof for hostname=%q",
			survivingNodeName, targetNodeName)
	}
	if err := runChassisCleanup(podName, ovnkubeNodeSBDBContainer); err != nil {
		return fmt.Errorf("surviving node pod %q chassis cleanup / SB proof: %w", podName, err)
	}
	e2e.Logf("[OVN chassis cleanup] Completed SB chassis cleanup via surviving node pod %s", podName)
	return nil
}

const ovnNodeChassisIDAnnotation = "k8s.ovn.org/node-chassis-id"

// ovsSystemIDGetCmd returns OVS's stable host identity; OVN-K registers SB Chassis / node-chassis-id from this value.
const ovsSystemIDGetCmd = "sudo ovs-vsctl --if-exists get Open_vSwitch . external_ids:system-id"

// ovnStaleChassisLogPrefix tags diagnostics for Node k8s.ovn.org/node-chassis-id vs host OVS system-id.
const ovnStaleChassisLogPrefix = "[OVN stale chassis]"

package two_node

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	metal3v1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/test/extended/two_node/utils"
	"github.com/openshift/origin/test/extended/two_node/utils/apis"
	"github.com/openshift/origin/test/extended/two_node/utils/core"
	"github.com/openshift/origin/test/extended/two_node/utils/services"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
)

// Constants
const (
	backupDirName = "tnf-node-replacement-backup"

	// OpenShift namespaces
	machineAPINamespace = "openshift-machine-api"
	etcdNamespace       = "openshift-etcd"

	// Timeouts
	fifteenSecondTimeout = 15 * time.Second
	oneMinuteTimeout     = 1 * time.Minute
	threeMinuteTimeout   = 3 * time.Minute
	fiveMinuteTimeout    = 5 * time.Minute
	tenMinuteTimeout     = 10 * time.Minute

	// Poll intervals
	fiveSecondPollInterval    = 5 * time.Second
	fifteenSecondPollInterval = 15 * time.Second
	thirtySecondPollInterval  = 30 * time.Second

	// Retry configuration
	maxDeleteRetries = 3

	// Pacemaker configuration
	pacemakerCleanupWaitTime = 15 * time.Second
	pacemakerJournalLines    = 25 // Number of journal lines to display for debugging

	// Provisioning timeouts
	bmhProvisioningTimeout      = 15 * time.Minute
	bmhProvisioningPollInterval = 30 * time.Second

	// Resource types
	secretResourceType  = "secret"
	bmhResourceType     = "bmh"
	machineResourceType = "machines.machine.openshift.io"

	// Output formats
	yamlOutputFormat = "yaml"

	// Annotations
	bmhDetachedAnnotation = "baremetalhost.metal3.io/detached=''"

	// Base names for dynamic resource names
	etcdPeerSecretBaseName           = "etcd-peer"
	etcdServingSecretBaseName        = "etcd-serving"
	etcdServingMetricsSecretBaseName = "etcd-serving-metrics"
	tnfAuthJobBaseName               = "tnf-auth-job"
	tnfAfterSetupJobBaseName         = "tnf-after-setup-job"

	// Virsh commands
	virshProvisioningBridge = "ostestpr"

	// Template paths (relative to test/extended/ - framework FixturePath will prefix automatically)
	templateBaseDir     = "testdata/two_node"
	bmhTemplatePath     = templateBaseDir + "/baremetalhost-template.yaml"
	machineTemplatePath = templateBaseDir + "/machine-template.yaml"

	// File patterns
	vmXMLFilePattern = "/tmp/%s.xml"
)

// Variables

// TNFTestConfig holds all test configuration and state
// This struct groups related variables to avoid global variable shadowing and improve maintainability
// HypervisorConnection contains SSH connection details for the hypervisor
type HypervisorConnection struct {
	Config         core.SSHConfig
	KnownHostsPath string
}

// NodeInfo contains information about a cluster node
type NodeInfo struct {
	Name           string
	IP             string
	VMName         string // Hypervisor VM name
	MachineName    string // OpenShift Machine name
	MachineHash    string // Machine name hash component
	BMCSecretName  string // BMC secret name
	BMHName        string // BareMetalHost name
	MAC            string // MAC address
	KnownHostsPath string // SSH known_hosts file path
}

// EtcdResources contains etcd-related Kubernetes resource names
type EtcdResources struct {
	PeerSecretName           string
	ServingSecretName        string
	ServingMetricsSecretName string
}

// JobTracking contains test job names
type JobTracking struct {
	AuthJobName       string
	AfterSetupJobName string
}

// TestExecution tracks test state and configuration
type TestExecution struct {
	GlobalBackupDir              string
	HasAttemptedNodeProvisioning bool
	BackupUsedForRecovery        bool   // Set to true if recovery used the backup
	RedfishIP                    string // Gateway IP for BMC access
}

// TNFTestConfig contains all configuration for two-node test execution
type TNFTestConfig struct {
	Hypervisor    HypervisorConnection
	TargetNode    NodeInfo
	SurvivingNode NodeInfo
	EtcdResources EtcdResources
	Jobs          JobTracking
	Execution     TestExecution
}

// ========================================
// Core Test Logic
// ========================================

var _ = g.Describe("[sig-etcd][apigroup:config.openshift.io][OCPFeatureGate:DualReplica][Suite:openshift/two-node][Slow][Serial][Disruptive][Requires:HypervisorSSHConfig] TNF", func() {
	var (
		testConfig TNFTestConfig
		oc         = exutil.NewCLIWithoutNamespace("").AsAdmin()
	)
	defer g.GinkgoRecover()
	g.BeforeEach(func() {
		// Set klog verbosity to 2 for detailed logging if not already set by user
		if vFlag := flag.Lookup("v"); vFlag != nil {
			// Only set if the flag hasn't been explicitly set by the user (still has default value)
			if vFlag.Value.String() == "0" {
				if err := flag.Set("v", "2"); err != nil {
					klog.Warningf("Failed to set klog verbosity: %v", err)
				} else {
					klog.Infof("Set klog verbosity to 2 for detailed logging")
				}
			} else {
				klog.Infof("Using user-specified klog verbosity: %s", vFlag.Value.String())
			}
		}

		// Skip if cluster topology doesn't match
		utils.SkipIfNotTopology(oc, configv1.DualReplicaTopologyMode)

		// Check cluster health before running disruptive test
		klog.V(2).Infof("Checking cluster health before running disruptive node replacement test")
		if err := utils.IsClusterHealthy(oc); err != nil {
			e2eskipper.Skipf("Skipping test because cluster is not healthy: %v", err)
		}
		klog.V(2).Infof("Cluster health check passed: all operators healthy and all nodes ready")

		setupTestEnvironment(&testConfig, oc)
	})

	g.AfterEach(func() {
		// Always attempt recovery if we have backup data
		if testConfig.Execution.GlobalBackupDir != "" {
			g.By("Attempting cluster recovery from backup")
			recoverClusterFromBackup(&testConfig, oc)
		}
		// Clean up target node known_hosts only if it was created (after reprovisioning)
		if testConfig.TargetNode.KnownHostsPath != "" {
			core.CleanupRemoteKnownHostsFile(&testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath, testConfig.TargetNode.KnownHostsPath)
		}
		core.CleanupRemoteKnownHostsFile(&testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath, testConfig.SurvivingNode.KnownHostsPath)
		core.CleanupLocalKnownHostsFile(&testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)

		// Wait for cluster operators to become healthy (regardless of test success/failure)
		g.By("Waiting for cluster operators to become healthy")
		e2e.Logf("Waiting up to 10 minutes for all cluster operators to become healthy")
		err := core.PollUntil(func() (bool, error) {
			if err := utils.IsClusterHealthy(oc); err != nil {
				klog.V(4).Infof("Cluster not yet healthy: %v", err)
				return false, nil
			}
			e2e.Logf("All cluster operators are healthy")
			return true, nil
		}, tenMinuteTimeout, thirtySecondPollInterval, "cluster operators to become healthy")
		if err != nil {
			e2e.Logf("WARNING: Cluster operators did not become healthy within 10 minutes: %v", err)
			e2e.Logf("This may indicate the cluster is still recovering from the disruptive test")
		}
	})

	g.It("should recover from an in-place node replacement", g.Label("Size:L"), func() {

		g.By("Backing up the target node's configuration")
		backupDir := backupTargetNodeConfiguration(&testConfig, oc)
		testConfig.Execution.GlobalBackupDir = backupDir // Store globally for recovery
		defer func() {
			// Only clean up backup if it wasn't used for recovery
			if backupDir != "" && !testConfig.Execution.BackupUsedForRecovery {
				klog.V(2).Infof("Test completed successfully, cleaning up backup directory: %s", backupDir)
				os.RemoveAll(backupDir)
			}
		}()

		g.By("Destroying the target VM")
		destroyVM(&testConfig)

		// Wait for etcd to stop on the surviving node
		g.By("Waiting for etcd to stop on the surviving node")
		waitForEtcdToStop(&testConfig, oc)

		// Restore etcd quorum on the survivor using a two-phase approach:
		// Phase 1: pcs debug-stop/start with verification (3 min timeout)
		// Phase 2: STONITH disable + cleanup fallback (3 attempts × 1 min)
		g.By("Restoring etcd quorum on surviving node")
		restoreEtcdQuorum(&testConfig, oc)

		g.By("Deleting OpenShift node references")
		deleteNodeReferences(&testConfig, oc)

		g.By("Recreating the target VM using backed up configuration")
		recreateTargetVM(&testConfig, backupDir)

		g.By("Provisioning the target node with Ironic")
		provisionTargetNodeWithIronic(&testConfig, oc)

		g.By("Waiting for the replacement node to appear in the cluster")
		waitForNodeRecovery(&testConfig, oc, tenMinuteTimeout, thirtySecondPollInterval)

		g.By("Restoring pacemaker cluster configuration")
		restorePacemakerCluster(&testConfig, oc)

		g.By("Verifying the cluster is fully restored")
		verifyRestoredCluster(&testConfig, oc)

		g.By("Successfully completed node replacement process")
		klog.V(2).Infof("Node replacement process completed. Backup files created in: %s", backupDir)
	})
})

// ========================================
// BeforeEach Functions
// ========================================

// setupTestEnvironment validates prerequisites and gathers required information
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

	klog.V(2).Infof("Using hypervisor configuration from test context:")
	klog.V(2).Infof("  Hypervisor IP: %s", testConfig.Hypervisor.Config.IP)
	klog.V(2).Infof("  SSH User: %s", testConfig.Hypervisor.Config.User)
	klog.V(2).Infof("  Private Key Path: %s", testConfig.Hypervisor.Config.PrivateKeyPath)

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
	klog.V(2).Infof("Computed Redfish IP from target node IP %s: %s", testConfig.TargetNode.IP, testConfig.Execution.RedfishIP)

	// Prepare known hosts file for the surviving node
	// Note: We don't prepare the target node's known_hosts here because its SSH key will change
	// after reprovisioning. It will be prepared in restorePacemakerCluster after the node is ready.
	survivingNodeKnownHostsPath, err := core.PrepareRemoteKnownHostsFile(testConfig.SurvivingNode.IP, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
	o.Expect(err).To(o.BeNil(), "Expected to prepare surviving node known hosts file without error")
	testConfig.SurvivingNode.KnownHostsPath = survivingNodeKnownHostsPath

	klog.V(2).Infof("Target node for replacement: %s (IP: %s)", testConfig.TargetNode.Name, testConfig.TargetNode.IP)
	klog.V(2).Infof("Surviving node: %s (IP: %s)", testConfig.SurvivingNode.Name, testConfig.SurvivingNode.IP)
	klog.V(2).Infof("Target node MAC: %s", testConfig.TargetNode.MAC)
	klog.V(2).Infof("Target VM for replacement: %s", testConfig.TargetNode.VMName)
	klog.V(2).Infof("Target machine name: %s", testConfig.TargetNode.MachineName)
	klog.V(2).Infof("Redfish IP (gateway): %s", testConfig.Execution.RedfishIP)

	klog.V(2).Infof("Test environment setup complete. Hypervisor IP: %s", testConfig.Hypervisor.Config.IP)
	klog.V(4).Infof("setupTestEnvironment completed, testConfig.TargetNode.VMName: %s", testConfig.TargetNode.VMName)
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
	klog.V(2).Infof("Validate node name: %v", err)

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

	klog.V(2).Infof("Randomly selected control plane node for replacement: %s (index: %d)", selectedNode, randomIndex)
	klog.V(2).Infof("Surviving control plane node: %s", survivingNode)

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
	testConfig.TargetNode.BMCSecretName = findObjectByNamePattern(oc, secretResourceType, machineAPINamespace, testConfig.TargetNode.Name, "bmc-secret")
	testConfig.TargetNode.BMHName = findObjectByNamePattern(oc, bmhResourceType, machineAPINamespace, testConfig.TargetNode.Name, "")

	// Get the MAC address of the target node from its BareMetalHost
	testConfig.TargetNode.MAC = getNodeMACAddress(oc, testConfig.TargetNode.Name)
	klog.V(4).Infof("Found targetNodeMAC: %s for node: %s", testConfig.TargetNode.MAC, testConfig.TargetNode.Name)

	// Find the corresponding VM name by matching MAC addresses
	var err error
	testConfig.TargetNode.VMName, err = services.GetVMNameByMACMatch(testConfig.TargetNode.Name, testConfig.TargetNode.MAC, virshProvisioningBridge, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
	klog.V(4).Infof("GetVMNameByMACMatch returned: testConfig.TargetNode.VMName=%s, err=%v", testConfig.TargetNode.VMName, err)
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
	// Machine name format: {cluster}-{hash}-{role}-{index} (e.g., "ostest-abc123-master-0")
	machineNameParts := strings.Split(testConfig.TargetNode.MachineName, "-")
	if len(machineNameParts) >= 4 {
		testConfig.TargetNode.MachineHash = machineNameParts[1]
		klog.V(2).Infof("Extracted machine hash: %s from machine name: %s", testConfig.TargetNode.MachineHash, testConfig.TargetNode.MachineName)
	} else {
		klog.Warningf("Unable to extract machine hash from machine name: %s (unexpected format)", testConfig.TargetNode.MachineName)
	}
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

	klog.V(2).Infof("Target node %s IP: %s", targetNodeName, targetNodeIP)
	klog.V(2).Infof("Surviving node %s IP: %s", survivingNodeName, survivingNodeIP)

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
			klog.V(4).Infof("Found internal IP %s for node %s", addr.Address, nodeName)
			return addr.Address, nil
		}
	}

	return "", fmt.Errorf("could not find internal IP address for node %s", nodeName)
}

// computeGatewayIP computes the gateway IP address from a node IP address.
// This follows the same logic as libvirt bridges, where the gateway is the first
// address in the subnet.
//
// For IPv4: 192.168.111.20 -> 192.168.111.1
// For IPv6: fd00::20 -> fd00::1
//
// Parameters:
//   - nodeIP: The IP address of a node (IPv4 or IPv6)
//
// Returns:
//   - string: The gateway IP address
//   - error: Any error that occurred during computation
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
	// Find the BareMetalHost name using regex pattern matching
	bmhName := findObjectByNamePattern(oc, bmhResourceType, machineAPINamespace, nodeName, "")

	// Get the BareMetalHost YAML to extract the MAC address
	bmh, err := apis.GetBMH(oc, bmhName, machineAPINamespace)
	o.Expect(err).To(o.BeNil(), "Expected to get BareMetalHost without error")

	// Extract the MAC address from the BareMetalHost spec
	macAddress := bmh.Spec.BootMACAddress
	core.ExpectNotEmpty(macAddress, "Expected BareMetalHost %s to have a BootMACAddress", bmhName)

	klog.V(2).Infof("Found MAC address %s for node %s", macAddress, nodeName)
	return macAddress
}

// extractMachineNameFromBMH extracts the machine name from BareMetalHost's consumerRef
func extractMachineNameFromBMH(oc *exutil.CLI, nodeName string) string {
	// Find the BareMetalHost name using regex pattern matching
	bmhName := findObjectByNamePattern(oc, bmhResourceType, machineAPINamespace, nodeName, "")

	// Get the BareMetalHost YAML to extract the machine name
	bmh, err := apis.GetBMH(oc, bmhName, machineAPINamespace)
	o.Expect(err).To(o.BeNil(), "Expected to get BareMetalHost without error")

	// Extract the machine name from consumerRef
	o.Expect(bmh.Spec.ConsumerRef).ToNot(o.BeNil(), "Expected BareMetalHost to have a consumerRef")
	core.ExpectNotEmpty(bmh.Spec.ConsumerRef.Name, "Expected consumerRef to have a name")

	machineName := bmh.Spec.ConsumerRef.Name
	klog.V(2).Infof("Found machine name: %s", machineName)
	return machineName
}

// ========================================
// AfterEach Functions
// ========================================

// recoverClusterFromBackup attempts to recover the cluster from backup if the test fails
// Has an overall 30-minute timeout to prevent indefinite hanging
func recoverClusterFromBackup(testConfig *TNFTestConfig, oc *exutil.CLI) {
	klog.V(2).Infof("Starting cluster recovery from backup directory: %s", testConfig.Execution.GlobalBackupDir)

	// Mark that recovery is using the backup
	testConfig.Execution.BackupUsedForRecovery = true

	// Set up overall recovery timeout (30 minutes)
	const recoveryTimeout = 30 * time.Minute
	ctx, cancel := context.WithTimeout(context.Background(), recoveryTimeout)
	defer cancel()

	// Monitor timeout in a goroutine
	done := make(chan struct{})
	defer close(done)

	go func() {
		select {
		case <-ctx.Done():
			klog.Errorf("Recovery timeout exceeded (%v) - recovery may be incomplete", recoveryTimeout)
		case <-done:
			// Recovery completed
		}
	}()

	defer func() {
		if r := recover(); r != nil {
			klog.Errorf("Recovery failed with panic: %v", r)
		}
		// Clean up backup directory after recovery attempt
		if testConfig.Execution.GlobalBackupDir != "" {
			klog.V(2).Infof("Cleaning up backup directory after recovery: %s", testConfig.Execution.GlobalBackupDir)
			os.RemoveAll(testConfig.Execution.GlobalBackupDir)
			testConfig.Execution.GlobalBackupDir = ""
		}
	}()

	// Step 1: Recreate the VM from backup
	klog.V(2).Infof("Step 1: Recreating VM from backup")
	if err := recoverVMFromBackup(testConfig); err != nil {
		klog.Errorf("Failed to recover VM %s from backup at %s: %v",
			testConfig.TargetNode.VMName, testConfig.Execution.GlobalBackupDir, err)
		return
	}

	// Step 2: Recreate etcd secrets from backup
	klog.V(2).Infof("Step 2: Recreating etcd secrets from backup")
	if err := recoverEtcdSecretsFromBackup(testConfig, oc); err != nil {
		klog.Errorf("Failed to recover etcd secrets (%s, %s, %s) from backup: %v",
			testConfig.EtcdResources.PeerSecretName,
			testConfig.EtcdResources.ServingSecretName,
			testConfig.EtcdResources.ServingMetricsSecretName, err)
		return
	}

	// Step 3: Recreate BMH and Machine
	klog.V(2).Infof("Step 3: Recreating BMH and Machine from backup")
	if err := recoverBMHAndMachineFromBackup(testConfig, oc); err != nil {
		klog.Errorf("Failed to recover BMH %s and Machine %s from backup: %v",
			testConfig.TargetNode.BMHName, testConfig.TargetNode.MachineName, err)
		return
	}

	// Step 4: Clean up pacemaker resources before CSR approval
	klog.V(2).Infof("Step 4: Cleaning up pacemaker resources on survivor node")
	e2e.Logf("Cleaning up pacemaker resources and STONITH on survivor node")
	klog.V(2).Infof("Running pcs resource cleanup on survivor: %s", testConfig.SurvivingNode.Name)
	_, _, err := services.PcsResourceCleanup(testConfig.SurvivingNode.IP, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath, testConfig.SurvivingNode.KnownHostsPath)
	if err != nil {
		klog.Warningf("Failed to run pcs resource cleanup: %v", err)
	}

	klog.V(2).Infof("Running pcs stonith cleanup on survivor: %s", testConfig.SurvivingNode.Name)
	_, _, err = services.PcsStonithCleanup(testConfig.SurvivingNode.IP, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath, testConfig.SurvivingNode.KnownHostsPath)
	if err != nil {
		klog.Warningf("Failed to run pcs stonith cleanup: %v", err)
	}

	klog.V(2).Infof("Waiting %v for pacemaker cleanup to settle", pacemakerCleanupWaitTime)
	time.Sleep(pacemakerCleanupWaitTime)

	// Step 5: Approve CSRs only if we attempted node provisioning and target node is not yet ready
	if !testConfig.Execution.HasAttemptedNodeProvisioning {
		klog.V(2).Infof("Step 5: Skipping CSR approval (no node provisioning was attempted)")
	} else if utils.IsNodeReady(oc, testConfig.TargetNode.Name) {
		klog.V(2).Infof("Step 5: Skipping CSR approval (target node %s is already Ready)", testConfig.TargetNode.Name)
	} else {
		klog.V(2).Infof("Step 5: Approving CSRs for cluster recovery (target node %s not ready)", testConfig.TargetNode.Name)
		approvedCount := apis.ApproveCSRs(oc, tenMinuteTimeout, thirtySecondPollInterval, 2)
		klog.V(2).Infof("Cluster recovery CSR approval complete: approved %d CSRs", approvedCount)
	}

	klog.V(2).Infof("Cluster recovery process completed")
}

// ========================================
// Recovery Functions (called by recoverClusterFromBackup)
// ========================================

// recoverVMFromBackup recreates the VM from the backed up XML
func recoverVMFromBackup(testConfig *TNFTestConfig) error {
	// Check if the specific VM already exists
	_, err := services.VirshVMExists(testConfig.TargetNode.VMName, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
	if err == nil {
		klog.V(2).Infof("VM %s already exists, skipping recreation", testConfig.TargetNode.VMName)
		return nil
	}

	core.ExpectNotEmpty(testConfig.TargetNode.VMName, "Expected testConfig.TargetNode.VMName to be set before recreating VM")
	// Read the backed up XML
	xmlFile := filepath.Join(testConfig.Execution.GlobalBackupDir, testConfig.TargetNode.VMName+".xml")
	xmlContent, err := os.ReadFile(xmlFile)
	if err != nil {
		return fmt.Errorf("failed to read XML backup: %v", err)
	}

	// Create XML file on the hypervisor using secure method
	xmlPath := fmt.Sprintf(vmXMLFilePattern, testConfig.TargetNode.VMName)
	err = core.CreateRemoteFile(xmlPath, string(xmlContent), core.StandardFileMode, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
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
		klog.Warningf("Failed to enable autostart for VM: %v", err)
	}

	// Clean up temporary XML file
	xmlPath = fmt.Sprintf(vmXMLFilePattern, testConfig.TargetNode.VMName)
	err = core.DeleteRemoteTempFile(xmlPath, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
	if err != nil {
		klog.Warningf("Failed to clean up temporary XML file: %v", err)
	}

	klog.V(2).Infof("Recreated VM: %s", testConfig.TargetNode.VMName)
	return services.WaitForVMState(testConfig.TargetNode.VMName, services.VMStateRunning, tenMinuteTimeout, thirtySecondPollInterval, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
}

// recoverEtcdSecretsFromBackup recreates etcd secrets from backup with retry logic
func recoverEtcdSecretsFromBackup(testConfig *TNFTestConfig, oc *exutil.CLI) error {
	etcdSecrets := []string{
		testConfig.EtcdResources.PeerSecretName,
		testConfig.EtcdResources.ServingSecretName,
		testConfig.EtcdResources.ServingMetricsSecretName,
	}

	for _, secretName := range etcdSecrets {
		secretFile := filepath.Join(testConfig.Execution.GlobalBackupDir, secretName+".yaml")
		if _, err := os.Stat(secretFile); os.IsNotExist(err) {
			klog.Warningf("Backup file for etcd secret %s not found", secretName)
			continue
		}

		// Check if the secret already exists
		_, err := oc.AdminKubeClient().CoreV1().Secrets(etcdNamespace).Get(context.Background(), secretName, metav1.GetOptions{})
		if err == nil {
			klog.V(2).Infof("Etcd secret %s already exists, skipping recreation", secretName)
			continue
		}

		// Retry the secret creation with etcd learner error handling
		err = core.RetryWithOptions(func() error {
			_, err := oc.AsAdmin().Run("create").Args("-f", secretFile).Output()
			return err
		}, core.RetryOptions{
			Timeout:      fiveMinuteTimeout,
			PollInterval: thirtySecondPollInterval,
			MaxRetries:   10,
			ShouldRetry:  services.IsRetryableEtcdError,
		}, fmt.Sprintf("create etcd secret %s", secretName))

		if err != nil {
			klog.Warningf("Failed to recreate etcd secret %s after retries: %v", secretName, err)
			continue
		}
		klog.V(2).Infof("Recreated etcd secret: %s", secretName)
	}

	return nil
}

// recoverBMHAndMachineFromBackup recreates BMH and Machine from backup with retry logic
func recoverBMHAndMachineFromBackup(testConfig *TNFTestConfig, oc *exutil.CLI) error {

	err := recreateBMCSecret(testConfig, oc)
	if err != nil {
		return fmt.Errorf("failed to recreate BMC secret: %v", err)
	}

	// Recreate Machine with retry
	machineFile := filepath.Join(testConfig.Execution.GlobalBackupDir, testConfig.TargetNode.MachineName+"-machine.yaml")

	// Check if Machine already exists
	_, err = oc.AsAdmin().MachineConfigurationClient().MachineconfigurationV1().MachineConfigs().Get(context.Background(), testConfig.TargetNode.MachineName, metav1.GetOptions{})
	if err != nil {
		// Retry Machine creation
		err = core.RetryWithOptions(func() error {
			_, err := oc.AsAdmin().Run("create").Args("-f", machineFile).Output()
			return err
		}, core.RetryOptions{
			Timeout:      fiveMinuteTimeout,
			PollInterval: thirtySecondPollInterval,
			MaxRetries:   10,
			ShouldRetry:  services.IsRetryableEtcdError,
		}, fmt.Sprintf("create Machine %s", testConfig.TargetNode.MachineName))

		if err != nil {
			return fmt.Errorf("failed to recreate Machine after retries: %v", err)
		}
		klog.V(2).Infof("Recreated Machine: %s", testConfig.TargetNode.MachineName)
	} else {
		klog.V(2).Infof("Machine %s already exists, skipping recreation", testConfig.TargetNode.MachineName)
	}

	return nil
}

// recreateBMCSecret recreates the BMC secret from backup
func recreateBMCSecret(testConfig *TNFTestConfig, oc *exutil.CLI) error {
	// Recreate BMC secret with retry
	bmcSecretFile := filepath.Join(testConfig.Execution.GlobalBackupDir, testConfig.TargetNode.BMCSecretName+".yaml")

	// Check if BMC secret already exists
	_, err := oc.AdminKubeClient().CoreV1().Secrets(machineAPINamespace).Get(context.Background(), testConfig.TargetNode.BMCSecretName, metav1.GetOptions{})
	if err != nil {
		// Retry BMC secret creation
		err = core.RetryWithOptions(func() error {
			_, err := oc.AsAdmin().Run("create").Args("-f", bmcSecretFile).Output()
			return err
		}, core.RetryOptions{
			Timeout:      fiveMinuteTimeout,
			PollInterval: thirtySecondPollInterval,
			MaxRetries:   10,
			ShouldRetry:  services.IsRetryableEtcdError,
		}, fmt.Sprintf("create BMC secret %s", testConfig.TargetNode.BMCSecretName))

		if err != nil {
			return fmt.Errorf("failed to recreate BMC secret after retries: %v", err)
		}
		klog.V(2).Infof("Recreated BMC secret: %s", testConfig.TargetNode.BMCSecretName)
	} else {
		klog.V(2).Infof("BMC secret %s already exists, skipping recreation", testConfig.TargetNode.BMCSecretName)
	}

	return nil
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

	// Download backup of BMC secret
	bmcSecretOutput, err := oc.AsAdmin().Run("get").Args(secretResourceType, testConfig.TargetNode.BMCSecretName, "-n", machineAPINamespace, "-o", yamlOutputFormat).Output()
	o.Expect(err).To(o.BeNil(), "Expected to get BMC secret without error")
	bmcSecretFile := filepath.Join(backupDir, testConfig.TargetNode.BMCSecretName+".yaml")
	err = os.WriteFile(bmcSecretFile, []byte(bmcSecretOutput), core.SecureFileMode)
	o.Expect(err).To(o.BeNil(), "Expected to write BMC secret backup without error")

	// Download backup of BareMetalHost
	bmhOutput, err := oc.AsAdmin().Run("get").Args(bmhResourceType, testConfig.TargetNode.BMHName, "-n", machineAPINamespace, "-o", yamlOutputFormat).Output()
	o.Expect(err).To(o.BeNil(), "Expected to get BareMetalHost without error")
	bmhFile := filepath.Join(backupDir, testConfig.TargetNode.BMHName+".yaml")
	err = os.WriteFile(bmhFile, []byte(bmhOutput), core.SecureFileMode)
	o.Expect(err).To(o.BeNil(), "Expected to write BareMetalHost backup without error")

	// Backup machine definition using the stored testConfig.TargetNode.MachineName
	machineOutput, err := oc.AsAdmin().Run("get").Args(machineResourceType, testConfig.TargetNode.MachineName, "-n", machineAPINamespace, "-o", yamlOutputFormat).Output()
	o.Expect(err).To(o.BeNil(), "Expected to get machine without error")
	machineFile := filepath.Join(backupDir, fmt.Sprintf("%s-machine.yaml", testConfig.TargetNode.MachineName))
	err = os.WriteFile(machineFile, []byte(machineOutput), core.SecureFileMode)
	o.Expect(err).To(o.BeNil(), "Expected to write machine backup without error")

	// Backup etcd secrets
	etcdSecrets := []string{
		testConfig.EtcdResources.PeerSecretName,
		testConfig.EtcdResources.ServingSecretName,
		testConfig.EtcdResources.ServingMetricsSecretName,
	}

	for _, secretName := range etcdSecrets {
		// Get the secret if it exists
		secretOutput, err := oc.AsAdmin().Run("get").Args(secretResourceType, secretName, "-n", etcdNamespace, "-o", yamlOutputFormat).Output()
		if err != nil {
			klog.Warningf("Could not backup etcd secret %s: %v", secretName, err)
			continue
		}

		secretFile := filepath.Join(backupDir, secretName+".yaml")
		err = os.WriteFile(secretFile, []byte(secretOutput), core.SecureFileMode)
		o.Expect(err).To(o.BeNil(), "Expected to write etcd secret %s backup without error", secretName)
		klog.V(2).Infof("Backed up etcd secret: %s", secretName)
	}

	klog.V(4).Infof("About to validate testConfig.TargetNode.VMName, current value: %s", testConfig.TargetNode.VMName)
	// Validate that testConfig.TargetNode.VMName is set
	if testConfig.TargetNode.VMName == "" {
		klog.V(2).Infof("testConfig.TargetNode.VMName bytes: %v", []byte(testConfig.TargetNode.VMName))
		klog.V(2).Infof("ERROR: testConfig.TargetNode.VMName is empty! This should have been set in setupTestEnvironment")
		klog.V(2).Infof("testConfig.TargetNode.Name: %s", testConfig.TargetNode.Name)
		klog.V(2).Infof("testConfig.SurvivingNode.Name: %s", testConfig.SurvivingNode.Name)
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
func waitForEtcdToStop(testConfig *TNFTestConfig, oc *exutil.CLI) {
	e2e.Logf("Waiting for etcd to stop on surviving node: %s", testConfig.SurvivingNode.Name)

	// Check that etcd has stopped on the survivor before proceeding
	err := waitForEtcdResourceToStop(testConfig, fiveMinuteTimeout)
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
// Phase 1: pcs debug-stop/start (Preferred)
//   - Stops etcd cleanly using pcs debug-stop
//   - Verifies resources stopped via pcs status and podman ps
//   - Starts etcd using pcs debug-start
//   - Waits up to 3 minutes for etcd to start (polling every 30s)
//
// Phase 2: STONITH disable + cleanup (Fallback)
//   - Falls back when Phase 1 fails due to OCPBUGS-65540
//   - Bug Link: https://issues.redhat.com/browse/OCPBUGS-65540
//   - Issue: Survivor fails to start because failed node is already marked as a learner
//   - Disables STONITH (safe because we know the second node is destroyed)
//   - Runs pcs resource cleanup up to 3 times (1 minute each, polling every 5s)
//   - Re-enables STONITH in cleanup regardless of success/failure
//
// Note: Disabling STONITH is not generally recommended, but is safe in this specific
// scenario because we have verified the second node is destroyed and cannot cause split-brain.
func restoreEtcdQuorum(testConfig *TNFTestConfig, oc *exutil.CLI) {
	e2e.Logf("Restoring etcd quorum on surviving node: %s", testConfig.SurvivingNode.Name)

	// Try Phase 1: pcs debug-stop/start approach
	if tryPcsDebugRestart(testConfig, oc) {
		return // Success
	}

	// Fall back to Phase 2: STONITH disable + cleanup approach
	tryStonithDisableCleanup(testConfig, oc)
}

// tryPcsDebugRestart attempts to restore etcd using pcs debug-stop/start
// Flow: stop → verify stopped (pcs + podman) → start → verify started
// Returns true if successful, false if should fall back to Phase 2
func tryPcsDebugRestart(testConfig *TNFTestConfig, oc *exutil.CLI) bool {
	e2e.Logf("Phase 1: Attempting pcs debug-stop/start on surviving node %s", testConfig.SurvivingNode.Name)

	// Step 1: Stop etcd
	output, _, err := services.PcsDebugStop(testConfig.SurvivingNode.IP, false, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath, testConfig.SurvivingNode.KnownHostsPath)
	if err != nil {
		e2e.Logf("WARNING: Failed to run pcs debug-stop on %s: %v, output: %s", testConfig.SurvivingNode.Name, err, output)
		e2e.Logf("Will proceed to Phase 2 (STONITH disable + cleanup) fallback")
		return false
	}
	e2e.Logf("Successfully ran pcs debug-stop on %s", testConfig.SurvivingNode.Name)

	// Step 2: Verify etcd resource has stopped via pcs resource status
	e2e.Logf("Verifying etcd resource has stopped via pcs resource status")
	err = waitForEtcdResourceToStop(testConfig, oneMinuteTimeout)
	if err != nil {
		e2e.Logf("WARNING: etcd resource did not stop after pcs debug-stop on %s: %v", testConfig.SurvivingNode.Name, err)
		e2e.Logf("Will proceed to Phase 2 (STONITH disable + cleanup) fallback")
		return false
	}
	e2e.Logf("Verified etcd resource has stopped on %s", testConfig.SurvivingNode.Name)

	// Step 3: Verify etcd container is not running via podman ps
	e2e.Logf("Verifying etcd container is not running via podman ps")
	podmanCmd := "sudo podman ps --format '{{.Names}}'"
	podmanOutput, _, err := core.ExecuteRemoteSSHCommand(testConfig.SurvivingNode.IP, podmanCmd, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath, testConfig.SurvivingNode.KnownHostsPath)
	if err != nil {
		e2e.Logf("WARNING: Failed to check running containers on %s: %v", testConfig.SurvivingNode.Name, err)
		// Continue anyway - pcs status is authoritative
	} else if strings.Contains(podmanOutput, "etcd") {
		e2e.Logf("WARNING: etcd container still appears to be running on %s after debug-stop", testConfig.SurvivingNode.Name)
		e2e.Logf("Running containers: %s", podmanOutput)
		e2e.Logf("Will proceed to Phase 2 (STONITH disable + cleanup) fallback")
		return false
	} else {
		e2e.Logf("Confirmed: etcd container is not running on %s", testConfig.SurvivingNode.Name)
	}

	// Step 4: Start etcd
	output, _, err = services.PcsDebugStart(testConfig.SurvivingNode.IP, false, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath, testConfig.SurvivingNode.KnownHostsPath)
	if err != nil {
		e2e.Logf("WARNING: Failed to run pcs debug-start on %s: %v, output: %s", testConfig.SurvivingNode.Name, err, output)
		e2e.Logf("Will proceed to Phase 2 (STONITH disable + cleanup) fallback")
		return false
	}
	e2e.Logf("Successfully ran pcs debug-start on %s, output: %s", testConfig.SurvivingNode.Name, output)

	// Step 5: Wait up to 3 minutes for etcd to start (checking every 30 seconds)
	e2e.Logf("Waiting up to 3 minutes for etcd to start after pcs debug-start (checking every 30s)")
	err = waitForEtcdToStart(testConfig, threeMinuteTimeout, thirtySecondPollInterval)
	if err != nil {
		e2e.Logf("WARNING: etcd did not start within 3 minutes after pcs debug-start: %v", err)
		e2e.Logf("Will proceed to Phase 2 (STONITH disable + cleanup) fallback")
		return false
	}

	e2e.Logf("SUCCESS: etcd started on surviving node %s after pcs debug-start", testConfig.SurvivingNode.Name)
	logPacemakerStatus(testConfig, "verification after pcs debug-start")
	e2e.Logf("Successfully restored etcd quorum on surviving node: %s", testConfig.SurvivingNode.Name)
	waitForAPIResponsive(oc, fiveMinuteTimeout)

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

	// Run pcs resource cleanup every minute for up to 3 minutes until etcd starts
	e2e.Logf("Running pcs resource cleanup every minute (up to 3 minutes) until etcd starts on surviving node %s", testConfig.SurvivingNode.Name)

	maxAttempts := 3 // 3 attempts over 3 minutes (one per minute)
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
		lastErr = waitForEtcdToStart(testConfig, oneMinuteTimeout, fiveSecondPollInterval)
		if lastErr == nil {
			e2e.Logf("SUCCESS: etcd started on surviving node %s after %d cleanup attempt(s)", testConfig.SurvivingNode.Name, attempt)
			etcdStarted = true
			break
		}

		e2e.Logf("etcd has not started yet on %s after attempt %d/%d: %v", testConfig.SurvivingNode.Name, attempt, maxAttempts, lastErr)

		// If this wasn't the last attempt, wait won't happen again (cleanup runs every minute)
		// The oneMinuteTimeout in waitForEtcdToStart already provides the delay between attempts
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

		// Get pacemaker status
		pcsStatusOutput, _, pcsErr := services.PcsStatus(testConfig.SurvivingNode.IP, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath, testConfig.SurvivingNode.KnownHostsPath)
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
	waitForAPIResponsive(oc, fiveMinuteTimeout)
}

// deleteNodeReferences deletes OpenShift resources related to the target node
func deleteNodeReferences(testConfig *TNFTestConfig, oc *exutil.CLI) {
	klog.V(2).Infof("Deleting OpenShift resources for node: %s", testConfig.TargetNode.Name)

	// Delete old etcd certificates using dynamic names with retry and timeout
	etcdSecrets := []string{
		testConfig.EtcdResources.PeerSecretName,
		testConfig.EtcdResources.ServingSecretName,
		testConfig.EtcdResources.ServingMetricsSecretName,
	}

	for _, secretName := range etcdSecrets {
		err := core.RetryWithOptions(func() error {
			ctx, cancel := context.WithTimeout(context.Background(), fifteenSecondTimeout)
			defer cancel()
			return oc.AdminKubeClient().CoreV1().Secrets(etcdNamespace).Delete(ctx, secretName, metav1.DeleteOptions{})
		}, core.RetryOptions{
			MaxRetries:   maxDeleteRetries,
			PollInterval: fiveSecondPollInterval,
		}, fmt.Sprintf("delete secret %s", secretName))
		o.Expect(err).To(o.BeNil(), "Expected to delete %s secret without error", secretName)
	}

	// Delete BareMetalHost entry with retry and timeout
	err := deleteOcResourceWithRetry(oc, bmhResourceType, testConfig.TargetNode.BMHName, machineAPINamespace)
	o.Expect(err).To(o.BeNil(), "Expected to delete BareMetalHost without error")

	// Delete machine entry with retry and timeout
	err = deleteOcResourceWithRetry(oc, machineResourceType, testConfig.TargetNode.MachineName, machineAPINamespace)
	o.Expect(err).To(o.BeNil(), "Expected to delete machine without error")

	klog.V(2).Infof("OpenShift resources for node %s deleted successfully", testConfig.TargetNode.Name)
}

// recreateTargetVM recreates the target VM using backed up configuration
func recreateTargetVM(testConfig *TNFTestConfig, backupDir string) {
	core.ExpectNotEmpty(testConfig.TargetNode.VMName, "Expected testConfig.TargetNode.VMName to be set before recreating VM")
	// Read the backed up XML
	xmlFile := filepath.Join(backupDir, testConfig.TargetNode.VMName+".xml")
	xmlContent, err := os.ReadFile(xmlFile)
	o.Expect(err).To(o.BeNil(), "Expected to read XML backup without error")
	xmlOutput := string(xmlContent)

	// Create XML file on the hypervisor using secure method
	xmlPath := fmt.Sprintf(vmXMLFilePattern, testConfig.TargetNode.VMName)
	err = core.CreateRemoteFile(xmlPath, xmlOutput, core.StandardFileMode, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
	o.Expect(err).To(o.BeNil(), "Expected to create XML file on hypervisor without error")

	// Redefine the VM using the backed up XML
	err = services.VirshDefineVM(xmlPath, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
	o.Expect(err).To(o.BeNil(), "Expected to define VM without error")

	// Start the VM with autostart enabled
	err = services.VirshStartVM(testConfig.TargetNode.VMName, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
	o.Expect(err).To(o.BeNil(), "Expected to start VM without error")

	err = services.VirshAutostartVM(testConfig.TargetNode.VMName, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
	o.Expect(err).To(o.BeNil(), "Expected to enable autostart for VM without error")

	// Clean up temporary XML file
	err = core.DeleteRemoteTempFile(xmlPath, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
	o.Expect(err).To(o.BeNil(), "Expected to clean up temporary XML file without error")

	err = services.WaitForVMState(testConfig.TargetNode.VMName, services.VMStateRunning, tenMinuteTimeout, thirtySecondPollInterval, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
	o.Expect(err).To(o.BeNil(), "Expected to wait for VM to start without error")
}

// provisionTargetNodeWithIronic handles the Ironic provisioning process
func provisionTargetNodeWithIronic(testConfig *TNFTestConfig, oc *exutil.CLI) {
	core.ExpectNotEmpty(testConfig.TargetNode.VMName, "Expected testConfig.TargetNode.VMName to be set before provisioning with Ironic")

	// Set flag to indicate we're attempting node provisioning
	testConfig.Execution.HasAttemptedNodeProvisioning = true

	recreateBMCSecret(testConfig, oc)
	newUUID, newMACAddress, err := services.GetVMNetworkInfo(testConfig.TargetNode.VMName, virshProvisioningBridge, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
	o.Expect(err).To(o.BeNil(), "Expected to get VM network info: %v", err)
	updateAndCreateBMH(testConfig, oc, newUUID, newMACAddress)
	waitForBMHProvisioning(testConfig, oc)
	reapplyDetachedAnnotation(testConfig, oc)
	recreateMachine(testConfig, oc)
}

// waitForNodeRecovery monitors for the replacement node to appear in the cluster and become Ready
func waitForNodeRecovery(testConfig *TNFTestConfig, oc *exutil.CLI, timeout time.Duration, pollInterval time.Duration) {
	err := core.PollUntil(func() (bool, error) {
		// Check if the target node exists and is Ready
		if utils.IsNodeReady(oc, testConfig.TargetNode.Name) {
			klog.V(2).Infof("Node %s is now Ready", testConfig.TargetNode.Name)
			return true, nil
		}

		// Node doesn't exist or is not Ready yet
		klog.V(4).Infof("Node %s is not Ready yet, continuing to poll", testConfig.TargetNode.Name)
		return false, nil
	}, timeout, pollInterval, fmt.Sprintf("node %s to be Ready", testConfig.TargetNode.Name))

	o.Expect(err).To(o.BeNil(), "Expected replacement node %s to appear and become Ready", testConfig.TargetNode.Name)
}

// restorePacemakerCluster restores the pacemaker cluster configuration
func restorePacemakerCluster(testConfig *TNFTestConfig, oc *exutil.CLI) {
	// Prepare known hosts file for the target node now that it has been reprovisioned
	// The SSH key changed during reprovisioning, so we need to scan it again
	klog.V(2).Infof("Preparing known_hosts for reprovisioned target node: %s", testConfig.TargetNode.IP)
	targetNodeKnownHostsPath, err := core.PrepareRemoteKnownHostsFile(testConfig.TargetNode.IP, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
	o.Expect(err).To(o.BeNil(), "Expected to prepare target node known hosts file after reprovisioning without error")
	testConfig.TargetNode.KnownHostsPath = targetNodeKnownHostsPath

	// Delete old jobs to allow new ones to be created
	e2e.Logf("Deleting old TNF auth job")
	err = services.DeleteAuthJob(testConfig.Jobs.AuthJobName, oc)
	o.Expect(err).To(o.BeNil(), "Expected to delete auth job %s without error", testConfig.Jobs.AuthJobName)

	e2e.Logf("Deleting old TNF after-setup job")
	err = services.DeleteAfterSetupJob(testConfig.Jobs.AfterSetupJobName, oc)
	o.Expect(err).To(o.BeNil(), "Expected to delete after-setup job %s without error", testConfig.Jobs.AfterSetupJobName)

	// Wait for the auth job to complete before proceeding with pacemaker operations
	e2e.Logf("Waiting for TNF auth job to complete")
	err = services.WaitForJobCompletion(testConfig.Jobs.AuthJobName, etcdNamespace, tenMinuteTimeout, fifteenSecondPollInterval, oc)
	o.Expect(err).To(o.BeNil(), "Expected auth job %s to complete without error", testConfig.Jobs.AuthJobName)

	// Waiting for CEO to recreate /var/lib/etcd/revision.json
	e2e.Logf("Waiting for CEO to recreate /var/lib/etcd/revision.json")
	err = services.WaitForEtcdRevisionCreation(testConfig.TargetNode.IP, tenMinuteTimeout, thirtySecondPollInterval, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath, testConfig.TargetNode.KnownHostsPath, oc)
	o.Expect(err).To(o.BeNil(), "Expected to wait for etcd revision creation without error")

	// Now that authentication is complete, we can proceed with pacemaker cluster operations
	e2e.Logf("Cycling removed node in pacemaker cluster")
	err = services.CycleRemovedNode(testConfig.TargetNode.Name, testConfig.TargetNode.IP, testConfig.SurvivingNode.IP, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath, testConfig.SurvivingNode.KnownHostsPath)
	o.Expect(err).To(o.BeNil(), "Expected to cycle removed node without error")

	// Verify both nodes are online in the pacemaker cluster
	e2e.Logf("Verifying both nodes are online in pacemaker cluster")
	nodeNames := []string{testConfig.TargetNode.Name, testConfig.SurvivingNode.Name}
	err = services.WaitForNodesOnline(nodeNames, testConfig.SurvivingNode.IP, tenMinuteTimeout, thirtySecondPollInterval, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath, testConfig.SurvivingNode.KnownHostsPath)
	o.Expect(err).To(o.BeNil(), "Expected both nodes %v to be online in pacemaker cluster", nodeNames)
	klog.V(2).Infof("Both nodes %v are online in pacemaker cluster", nodeNames)
}

// verifyRestoredCluster verifies that the cluster is fully restored and healthy
func verifyRestoredCluster(testConfig *TNFTestConfig, oc *exutil.CLI) {
	klog.V(2).Infof("Verifying cluster restoration: checking node status and cluster operators")

	// Step 1: Verify both nodes are in Ready state
	e2e.Logf("Verifying both nodes are in Ready state")

	// Check target node
	o.Expect(utils.IsNodeReady(oc, testConfig.TargetNode.Name)).To(o.BeTrue(), "Expected target node %s to be in Ready state", testConfig.TargetNode.Name)
	klog.V(2).Infof("Target node %s is Ready", testConfig.TargetNode.Name)

	// Check surviving node
	o.Expect(utils.IsNodeReady(oc, testConfig.SurvivingNode.Name)).To(o.BeTrue(), "Expected surviving node %s to be in Ready state", testConfig.SurvivingNode.Name)
	klog.V(2).Infof("Surviving node %s is Ready", testConfig.SurvivingNode.Name)

	// Step 2: Verify all cluster operators are available (not degraded or progressing)
	e2e.Logf("Verifying all cluster operators are available")
	coOutput, err := utils.MonitorClusterOperators(oc, tenMinuteTimeout, fifteenSecondPollInterval)
	o.Expect(err).To(o.BeNil(), "Expected all cluster operators to be available")
	klog.V(2).Infof("All cluster operators are available and healthy")

	// Log final status
	klog.V(2).Infof("Cluster verification completed successfully:")
	klog.V(2).Infof("  - Target node %s is Ready", testConfig.TargetNode.Name)
	klog.V(2).Infof("  - Surviving node %s is Ready", testConfig.SurvivingNode.Name)
	klog.V(2).Infof("  - All cluster operators are available")
	klog.V(2).Infof("\nFinal cluster operators status:\n%s", coOutput)
}

// ========================================
// Helper Functions for Main Test
// ========================================

// deleteOcResourceWithRetry deletes an OpenShift resource using oc with retry logic.
// This helper reduces duplication in resource deletion code.
func deleteOcResourceWithRetry(oc *exutil.CLI, resourceType, resourceName, namespace string) error {
	return core.RetryWithOptions(func() error {
		_, err := oc.AsAdmin().Run("delete").Args(resourceType, resourceName, "-n", namespace).Output()
		return err
	}, core.RetryOptions{
		MaxRetries:   maxDeleteRetries,
		PollInterval: fiveSecondPollInterval,
		Timeout:      oneMinuteTimeout,
	}, fmt.Sprintf("delete %s %s", resourceType, resourceName))
}

// logPacemakerStatus logs the pacemaker cluster status for verification purposes.
// This is a non-fatal operation - if it fails, a warning is logged but execution continues.
//
// Parameters:
//   - context: describes why the status is being checked (e.g., "verification after pcs debug-start")
//
// Example usage:
//
//	logPacemakerStatus(testConfig, "verification after pcs debug-start")
func logPacemakerStatus(testConfig *TNFTestConfig, context string) {
	e2e.Logf("Getting pacemaker status on %s for %s", testConfig.SurvivingNode.Name, context)
	pcsStatusOutput, _, err := services.PcsStatus(testConfig.SurvivingNode.IP, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath, testConfig.SurvivingNode.KnownHostsPath)
	if err != nil {
		e2e.Logf("WARNING: Failed to get pacemaker status on %s: %v", testConfig.SurvivingNode.Name, err)
	} else {
		e2e.Logf("Pacemaker status on %s:\n%s", testConfig.SurvivingNode.Name, pcsStatusOutput)
	}
}

// waitForAPIResponsive waits for the Kubernetes API to become responsive.
// This function will cause a test failure if the API does not respond within the timeout.
//
// Primary use case: Verifying API restoration after etcd quorum restoration.
//
// Parameters:
//   - timeout: maximum time to wait for API responsiveness
//
// The function polls every 15 seconds until the API responds or timeout is reached.
func waitForAPIResponsive(oc *exutil.CLI, timeout time.Duration) {
	e2e.Logf("Waiting for the Kubernetes API to be responsive (timeout: %v)", timeout)
	err := core.PollUntil(func() (bool, error) {
		if utils.IsAPIResponding(oc) {
			klog.V(2).Infof("Kubernetes API is responding")
			return true, nil
		}
		klog.V(4).Infof("Kubernetes API not yet responding, continuing to poll")
		return false, nil
	}, timeout, fifteenSecondPollInterval, "Kubernetes API to be responsive")
	o.Expect(err).To(o.BeNil(), "Expected Kubernetes API to be responsive within timeout")
}

// waitForEtcdResourceToStop waits for etcd resource to stop on the surviving node
func waitForEtcdResourceToStop(testConfig *TNFTestConfig, timeout time.Duration) error {
	e2e.Logf("Waiting for etcd resource to stop on surviving node: %s (timeout: %v)", testConfig.SurvivingNode.Name, timeout)

	return core.RetryWithOptions(func() error {
		// Check etcd resource status on the surviving node
		e2e.Logf("Polling etcd resource status on node %s", testConfig.SurvivingNode.Name)
		output, _, err := services.PcsResourceStatus(testConfig.SurvivingNode.Name, testConfig.SurvivingNode.IP, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath, testConfig.SurvivingNode.KnownHostsPath)
		if err != nil {
			e2e.Logf("Failed to get etcd resource status on %s: %v, output: %s", testConfig.SurvivingNode.Name, err, output)
			return fmt.Errorf("failed to get etcd resource status on %s: %v, output: %s", testConfig.SurvivingNode.Name, err, output)
		}

		e2e.Logf("Etcd resource status on %s:\n%s", testConfig.SurvivingNode.Name, output)

		// Check if etcd is stopped (not started) on the surviving node
		// We expect to see "Stopped: [ master-X ]" or no "Started:" line for the survivor
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.Contains(line, "Started:") && strings.Contains(line, testConfig.SurvivingNode.Name) {
				e2e.Logf("etcd is still started on surviving node %s (found line: %s)", testConfig.SurvivingNode.Name, line)
				return fmt.Errorf("etcd is still started on surviving node %s", testConfig.SurvivingNode.Name)
			}
		}

		// If we get here, etcd is not started on the surviving node
		e2e.Logf("etcd has stopped on surviving node: %s", testConfig.SurvivingNode.Name)
		return nil
	}, core.RetryOptions{
		Timeout:      fiveMinuteTimeout,
		PollInterval: fiveSecondPollInterval,
	}, fmt.Sprintf("etcd stop on %s", testConfig.SurvivingNode.Name))
}

// waitForEtcdToStart waits for etcd to start on the surviving node
func waitForEtcdToStart(testConfig *TNFTestConfig, timeout, pollInterval time.Duration) error {
	e2e.Logf("Waiting for etcd to start on surviving node: %s (timeout: %v, poll interval: %v)", testConfig.SurvivingNode.Name, timeout, pollInterval)

	return core.RetryWithOptions(func() error {
		// Check etcd resource status on the surviving node
		e2e.Logf("Polling etcd resource status on node %s", testConfig.SurvivingNode.Name)
		output, _, err := services.PcsResourceStatus(testConfig.SurvivingNode.Name, testConfig.SurvivingNode.IP, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath, testConfig.SurvivingNode.KnownHostsPath)
		if err != nil {
			e2e.Logf("Failed to get etcd resource status on %s: %v, output: %s", testConfig.SurvivingNode.Name, err, output)
			return fmt.Errorf("failed to get etcd resource status on %s: %v, output: %s", testConfig.SurvivingNode.Name, err, output)
		}

		e2e.Logf("Etcd resource status on %s:\n%s", testConfig.SurvivingNode.Name, output)

		// Check if etcd is started on the surviving node
		// We expect to see "Started: [ master-X ]" for the survivor
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.Contains(line, "Started:") && strings.Contains(line, testConfig.SurvivingNode.Name) {
				e2e.Logf("etcd has started on surviving node: %s (found line: %s)", testConfig.SurvivingNode.Name, line)
				return nil
			}
		}

		e2e.Logf("etcd is not started yet on surviving node %s", testConfig.SurvivingNode.Name)
		return fmt.Errorf("etcd is not started on surviving node %s", testConfig.SurvivingNode.Name)
	}, core.RetryOptions{
		Timeout:      timeout,
		PollInterval: pollInterval,
	}, fmt.Sprintf("etcd start on %s", testConfig.SurvivingNode.Name))
}

// updateAndCreateBMH creates a new BareMetalHost from template
func updateAndCreateBMH(testConfig *TNFTestConfig, oc *exutil.CLI, newUUID, newMACAddress string) {
	klog.V(2).Infof("Creating BareMetalHost with UUID: %s, MAC: %s", newUUID, newMACAddress)

	// Create BareMetalHost from template with placeholder substitution
	err := core.CreateResourceFromTemplate(oc, bmhTemplatePath, map[string]string{
		"{BMH_NAME}":         testConfig.TargetNode.BMHName,
		"{REDFISH_IP}":       testConfig.Execution.RedfishIP,
		"{UUID}":             newUUID,
		"{CREDENTIALS_NAME}": testConfig.TargetNode.BMCSecretName,
		"{BOOT_MAC_ADDRESS}": newMACAddress,
	})
	o.Expect(err).To(o.BeNil(), "Expected to create BareMetalHost without error")

	klog.V(2).Infof("Successfully created BareMetalHost: %s", testConfig.TargetNode.BMHName)
}

// waitForBMHProvisioning waits for the BareMetalHost to be provisioned
func waitForBMHProvisioning(testConfig *TNFTestConfig, oc *exutil.CLI) {
	klog.V(2).Infof("Waiting for BareMetalHost %s to be provisioned...", testConfig.TargetNode.BMHName)

	maxWaitTime := bmhProvisioningTimeout
	pollInterval := bmhProvisioningPollInterval

	err := core.PollUntil(func() (bool, error) {
		// Get the specific BareMetalHost in YAML format
		bmh, err := apis.GetBMH(oc, testConfig.TargetNode.BMHName, machineAPINamespace)
		if err != nil {
			klog.V(4).Infof("Error getting BareMetalHost %s: %v", testConfig.TargetNode.BMHName, err)
			return false, nil // Continue polling on errors
		}

		// Check the provisioning state
		currentState := string(bmh.Status.Provisioning.State)
		klog.V(4).Infof("BareMetalHost %s current state: %s", testConfig.TargetNode.BMHName, currentState)

		// Log additional status information
		if bmh.Status.ErrorMessage != "" {
			klog.V(4).Infof("BareMetalHost %s error message: %s", testConfig.TargetNode.BMHName, bmh.Status.ErrorMessage)
		}

		// Check if BMH is in provisioned state
		if currentState == string(metal3v1alpha1.StateProvisioned) {
			klog.V(2).Infof("BareMetalHost %s is provisioned", testConfig.TargetNode.BMHName)
			return true, nil
		}

		// Not yet provisioned, continue polling
		return false, nil
	}, maxWaitTime, pollInterval, fmt.Sprintf("BareMetalHost %s provisioning", testConfig.TargetNode.BMHName))

	o.Expect(err).To(o.BeNil(), "Expected BareMetalHost %s to be provisioned", testConfig.TargetNode.BMHName)
}

// reapplyDetachedAnnotation reapplies the detached annotation to the BareMetalHost
func reapplyDetachedAnnotation(testConfig *TNFTestConfig, oc *exutil.CLI) {
	klog.V(2).Infof("Applying detached annotation to BareMetalHost: %s", testConfig.TargetNode.BMHName)

	// Apply the detached annotation to the specific BMH
	_, err := oc.AsAdmin().Run("annotate").Args(
		bmhResourceType, testConfig.TargetNode.BMHName,
		"-n", machineAPINamespace,
		bmhDetachedAnnotation,
		"--overwrite",
	).Output()
	o.Expect(err).To(o.BeNil(), "Expected to apply detached annotation to BMH %s without error", testConfig.TargetNode.BMHName)

	klog.V(2).Infof("Successfully applied detached annotation to BareMetalHost: %s", testConfig.TargetNode.BMHName)
}

// recreateMachine recreates the Machine resource from template
func recreateMachine(testConfig *TNFTestConfig, oc *exutil.CLI) {
	klog.V(2).Infof("Recreating Machine: %s", testConfig.TargetNode.MachineName)

	// Check if the machine already exists
	_, err := oc.AsAdmin().Run("get").Args(machineResourceType, testConfig.TargetNode.MachineName, "-n", machineAPINamespace).Output()
	if err == nil {
		klog.V(2).Infof("Machine %s already exists, skipping recreation", testConfig.TargetNode.MachineName)
		return
	}

	// Create Machine from template with placeholder substitution
	err = core.CreateResourceFromTemplate(oc, machineTemplatePath, map[string]string{
		"{BMH_NAME}":     testConfig.TargetNode.BMHName,
		"{MACHINE_NAME}": testConfig.TargetNode.MachineName,
		"{MACHINE_HASH}": testConfig.TargetNode.MachineHash,
	})
	o.Expect(err).To(o.BeNil(), "Expected to create Machine without error")

	klog.V(2).Infof("Successfully recreated Machine: %s", testConfig.TargetNode.MachineName)
}

// ========================================
// Remaining Utility Functions
// ========================================

// findObjectByNamePattern finds an object by regex pattern matching
func findObjectByNamePattern(oc *exutil.CLI, resourceType, namespace, nodeName, suffix string) string {
	// List all objects of the specified type in the namespace
	objectsOutput, err := oc.AsAdmin().Run("get").Args(resourceType, "-n", namespace, "-o", "name").Output()
	o.Expect(err).To(o.BeNil(), "Expected to list %s objects without error", resourceType)

	// Create regex pattern based on whether suffix is provided
	var pattern string
	if suffix == "" {
		// For objects without suffix (like BareMetalHost): *-{nodeName}
		pattern = fmt.Sprintf(`.*-%s$`, regexp.QuoteMeta(nodeName))
	} else {
		// For objects with suffix (like BMC secrets): *-{nodeName}-{suffix}
		pattern = fmt.Sprintf(`.*-%s-%s$`, regexp.QuoteMeta(nodeName), regexp.QuoteMeta(suffix))
	}

	regex, err := regexp.Compile(pattern)
	o.Expect(err).To(o.BeNil(), "Expected to compile regex pattern without error")

	// Search through the objects
	lines := strings.Split(objectsOutput, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Extract object name by finding the last "/" and taking everything after it
		// This handles both simple resource types (secret/name) and API group types (baremetalhost.metal3.io/name)
		lastSlashIndex := strings.LastIndex(line, "/")
		if lastSlashIndex == -1 {
			continue // Skip lines without "/"
		}

		objectName := line[lastSlashIndex+1:]
		if regex.MatchString(objectName) {
			klog.V(2).Infof("Found %s: %s", resourceType, objectName)
			return objectName // Return just the name without the type prefix
		}
	}

	// Fail the test if no match is found
	if suffix == "" {
		o.Expect(fmt.Sprintf("no %s found matching pattern *-%s", resourceType, nodeName)).To(o.BeEmpty(), "Expected to find %s matching pattern *-%s", resourceType, nodeName)
	} else {
		o.Expect(fmt.Sprintf("no %s found matching pattern *-%s-%s", resourceType, nodeName, suffix)).To(o.BeEmpty(), "Expected to find %s matching pattern *-%s-%s", resourceType, nodeName, suffix)
	}
	return ""
}

package two_node

import (
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
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/klog/v2"
)

// Constants
const (
	backupDirName = "tnf-node-replacement-backup"

	// OpenShift namespaces
	machineAPINamespace = "openshift-machine-api"
	etcdNamespace       = "openshift-etcd"

	// Timeouts
	oneMinuteTimeout  = 1 * time.Minute
	fiveMinuteTimeout = 5 * time.Minute
	tenMinuteTimeout  = 10 * time.Minute

	// Poll intervals
	fiveSecondPollInterval    = 5 * time.Second
	fifteenSecondPollInterval = 15 * time.Second
	thirtySecondPollInterval  = 30 * time.Second

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

	// Template paths
	templateBaseDir = "test/extended/testdata/two_node"
	bmhTemplatePath = templateBaseDir + "/baremetalhost-template.yaml"

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

// etcdMemberListResponse represents the JSON response from etcdctl member list -w json
type etcdMemberListResponse struct {
	Header  etcdResponseHeader `json:"header"`
	Members []etcdMember       `json:"members"`
}

// etcdResponseHeader represents the header in etcd responses
type etcdResponseHeader struct {
	ClusterID uint64 `json:"cluster_id"`
	MemberID  uint64 `json:"member_id"`
	RaftTerm  int    `json:"raft_term"`
}

// etcdMember represents a single etcd member
type etcdMember struct {
	ID         uint64   `json:"ID"`
	Name       string   `json:"name"`
	PeerURLs   []string `json:"peerURLs"`
	ClientURLs []string `json:"clientURLs"`
	IsLearner  bool     `json:"isLearner"`
}

// clusterOperatorList represents the JSON response from oc get co -o json
type clusterOperatorList struct {
	APIVersion string                `json:"apiVersion"`
	Items      []clusterOperatorItem `json:"items"`
	Kind       string                `json:"kind"`
}

// clusterOperatorItem represents a single ClusterOperator in the list
type clusterOperatorItem struct {
	Metadata clusterOperatorMetadata `json:"metadata"`
	Status   clusterOperatorStatus   `json:"status"`
}

// clusterOperatorMetadata represents the metadata of a ClusterOperator
type clusterOperatorMetadata struct {
	Name string `json:"name"`
}

// clusterOperatorStatus represents the status of a ClusterOperator
type clusterOperatorStatus struct {
	Conditions []clusterOperatorCondition `json:"conditions"`
}

// clusterOperatorCondition represents a condition in a ClusterOperator status
type clusterOperatorCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// Global test configuration instance
var (
	oc = utils.CreateCLI(utils.CLIPrivilegeAdmin)
)

// ========================================
// Core Test Logic
// ========================================

var _ = g.Describe("[sig-etcd][apigroup:config.openshift.io][OCPFeatureGate:DualReplica][Suite:openshift/two-node][Disruptive][Requires:HypervisorSSHConfig] TNF", func() {
	var testConfig TNFTestConfig
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

		utils.SkipIfNotTopology(oc, configv1.DualReplicaTopologyMode)
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
	})

	g.It("should recover from an in-place node replacement", func() {

		g.By("Backing up the target node's configuration")
		backupDir := backupTargetNodeConfiguration(&testConfig, oc)
		testConfig.Execution.GlobalBackupDir = backupDir // Store globally for recovery
		defer func() {
			if backupDir != "" && testConfig.Execution.GlobalBackupDir == "" {
				// Only clean up if recovery didn't need it
				os.RemoveAll(backupDir)
			}
		}()

		g.By("Destroying the target VM")
		destroyVM(&testConfig)

		g.By("Manually restoring etcd quorum on the survivor")
		restoreEtcdQuorumOnSurvivor(&testConfig, oc)

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
	nodeOutput, err := oc.AsAdmin().Run("get").Args("node", nodeName, "-o", "json").Output()
	if err != nil {
		return "", fmt.Errorf("failed to get node %s details: %v", nodeName, err)
	}

	// Parse the JSON into a Node struct
	var node corev1.Node
	if err := utils.UnmarshalJSON(nodeOutput, &node); err != nil {
		return "", fmt.Errorf("failed to parse node JSON for %s: %v", nodeName, err)
	}

	// Find the InternalIP address from the node's status
	for _, addr := range node.Status.Addresses {
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
	bmhOutput, err := oc.AsAdmin().Run("get").Args(bmhResourceType, bmhName, "-n", machineAPINamespace, "-o", yamlOutputFormat).Output()
	o.Expect(err).To(o.BeNil(), "Expected to get BareMetalHost without error")

	// Parse the YAML into a BareMetalHost object
	var bmh metal3v1alpha1.BareMetalHost
	decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(bmhOutput), 4096)
	err = decoder.Decode(&bmh)
	o.Expect(err).To(o.BeNil(), "Expected to parse BareMetalHost YAML without error")

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
	bmhOutput, err := oc.AsAdmin().Run("get").Args(bmhResourceType, bmhName, "-n", machineAPINamespace, "-o", yamlOutputFormat).Output()
	o.Expect(err).To(o.BeNil(), "Expected to get BareMetalHost without error")

	// Parse the YAML into a BareMetalHost object
	var bmh metal3v1alpha1.BareMetalHost
	decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(bmhOutput), 4096)
	err = decoder.Decode(&bmh)
	o.Expect(err).To(o.BeNil(), "Expected to parse BareMetalHost YAML without error")

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
func recoverClusterFromBackup(testConfig *TNFTestConfig, oc *exutil.CLI) {
	klog.V(2).Infof("Starting cluster recovery from backup directory: %s", testConfig.Execution.GlobalBackupDir)

	defer func() {
		if r := recover(); r != nil {
			klog.Errorf("Recovery failed with panic: %v", r)
		}
		// Clean up backup directory after recovery attempt
		if testConfig.Execution.GlobalBackupDir != "" {
			os.RemoveAll(testConfig.Execution.GlobalBackupDir)
			testConfig.Execution.GlobalBackupDir = ""
		}
	}()

	// Step 1: Recreate the VM from backup
	klog.V(2).Infof("Step 1: Recreating VM from backup")
	if err := recoverVMFromBackup(testConfig); err != nil {
		klog.Errorf("Failed to recover VM: %v", err)
		return
	}

	// Step 2: Promote etcd learner member to prevent stalling
	klog.V(2).Infof("Step 2: Promoting etcd learner member to prevent stalling")
	if err := promoteEtcdLearnerMember(testConfig); err != nil {
		klog.Warningf("Failed to promote etcd learner member: %v", err)
		// Don't return here, continue with recovery as this is not critical
	}

	// Step 3: Recreate etcd secrets from backup
	klog.V(2).Infof("Step 3: Recreating etcd secrets from backup")
	if err := recoverEtcdSecretsFromBackup(testConfig, oc); err != nil {
		klog.Errorf("Failed to recover etcd secrets: %v", err)
		return
	}

	// Step 4: Recreate BMH and Machine
	klog.V(2).Infof("Step 4: Recreating BMH and Machine from backup")
	if err := recoverBMHAndMachineFromBackup(testConfig, oc); err != nil {
		klog.Errorf("Failed to recover BMH and Machine: %v", err)
		return
	}

	// Step 5: Clean up pacemaker resources before CSR approval
	klog.V(2).Infof("Step 5: Cleaning up pacemaker resources on survivor node")
	g.By("Cleaning up pacemaker resources and STONITH on survivor node")
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

	klog.V(2).Infof("Waiting 15 seconds for pacemaker cleanup to settle")
	time.Sleep(15 * time.Second)

	// Step 6: Approve CSRs only if we attempted node provisioning and target node is not yet ready
	if !testConfig.Execution.HasAttemptedNodeProvisioning {
		klog.V(2).Infof("Step 6: Skipping CSR approval (no node provisioning was attempted)")
	} else if utils.IsNodeReady(oc, testConfig.TargetNode.Name) {
		klog.V(2).Infof("Step 6: Skipping CSR approval (target node %s is already Ready)", testConfig.TargetNode.Name)
	} else {
		klog.V(2).Infof("Step 6: Approving CSRs for cluster recovery (target node %s not ready)", testConfig.TargetNode.Name)
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
	return services.WaitForVMToStart(testConfig.TargetNode.VMName, tenMinuteTimeout, thirtySecondPollInterval, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
}

// promoteEtcdLearnerMember promotes the etcd learner member to voter status
func promoteEtcdLearnerMember(testConfig *TNFTestConfig) error {
	klog.V(2).Infof("Attempting to promote etcd learner member on surviving node: %s (IP: %s)", testConfig.SurvivingNode.Name, testConfig.SurvivingNode.IP)

	return core.RetryWithOptions(func() error {
		// First, get the list of etcd members to find the learner
		memberListCmd := `sudo podman exec -it etcd etcdctl member list -w json`
		output, _, err := core.ExecuteRemoteSSHCommand(testConfig.SurvivingNode.IP, memberListCmd, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath, testConfig.SurvivingNode.KnownHostsPath)
		if err != nil {
			return fmt.Errorf("failed to get etcd member list on %s: %v", testConfig.SurvivingNode.IP, err)
		}

		klog.V(4).Infof("Etcd member list output: %s", output)

		// Parse the JSON output to find the learner member
		learnerMemberID, learnerName, err := findLearnerMemberID(output)
		if err != nil {
			return fmt.Errorf("failed to find learner member ID: %v", err)
		}

		if learnerMemberID == "" {
			klog.V(2).Infof("No learner member found, all members are already voters")
			return nil // No learner to promote, this is success
		}

		klog.V(2).Infof("Found learner member: ID=%s, Name=%s", learnerMemberID, learnerName)

		// Promote the learner member
		promoteCmd := fmt.Sprintf(`sudo podman exec -it etcd etcdctl member promote %s`, learnerMemberID)
		promoteOutput, _, err := core.ExecuteRemoteSSHCommand(testConfig.SurvivingNode.IP, promoteCmd, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath, testConfig.SurvivingNode.KnownHostsPath)
		if err != nil {
			return fmt.Errorf("failed to promote etcd learner member %s on %s: %v, output: %s", learnerMemberID, testConfig.SurvivingNode.IP, err, promoteOutput)
		}

		klog.V(4).Infof("Successfully promoted etcd learner member %s: %s", learnerMemberID, promoteOutput)
		return nil
	}, core.RetryOptions{
		Timeout:      tenMinuteTimeout,
		PollInterval: thirtySecondPollInterval,
	}, "promote etcd learner member")
}

// findLearnerMemberID parses the etcd member list JSON output to find the learner member ID and name
func findLearnerMemberID(memberListJSON string) (string, string, error) {
	// Parse the JSON output
	var memberList etcdMemberListResponse
	err := utils.UnmarshalJSON(memberListJSON, &memberList)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse etcd member list JSON: %v", err)
	}

	// Find learner members
	for _, member := range memberList.Members {
		if member.IsLearner {
			// Convert member ID to hex string format (etcdctl expects hex format)
			memberIDHex := fmt.Sprintf("%x", member.ID)
			klog.V(2).Infof("Found learner member: ID=%s (hex: %s), Name=%s", fmt.Sprintf("%d", member.ID), memberIDHex, member.Name)
			return memberIDHex, member.Name, nil
		}
	}

	// No learner found
	klog.V(2).Infof("No learner member found in member list")
	return "", "", nil
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
		_, err := oc.AsAdmin().Run("get").Args(secretResourceType, secretName, "-n", etcdNamespace).Output()
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
	_, err = oc.AsAdmin().Run("get").Args(machineResourceType, testConfig.TargetNode.MachineName, "-n", machineAPINamespace).Output()
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
	_, err := oc.AsAdmin().Run("get").Args(secretResourceType, testConfig.TargetNode.BMCSecretName, "-n", machineAPINamespace).Output()
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
	klog.V(2).Infof("Destroying VM: %s", testConfig.TargetNode.VMName)

	// Undefine and destroy VM using SSH to hypervisor
	err := services.VirshUndefineVM(testConfig.TargetNode.VMName, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
	o.Expect(err).To(o.BeNil(), "Expected to undefine VM without error")

	err = services.VirshDestroyVM(testConfig.TargetNode.VMName, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
	o.Expect(err).To(o.BeNil(), "Expected to destroy VM without error")

	klog.V(2).Infof("VM %s destroyed successfully", testConfig.TargetNode.VMName)
}

// restoreEtcdQuorumOnSurvivor restores etcd quorum on the surviving node
func restoreEtcdQuorumOnSurvivor(testConfig *TNFTestConfig, oc *exutil.CLI) {
	klog.V(2).Infof("Restoring etcd quorum on surviving node: %s", testConfig.SurvivingNode.Name)

	// Wait a minute after node deletion to allow etcd to stop naturally
	g.By("Waiting a minute for etcd to stop naturally after node deletion")
	time.Sleep(oneMinuteTimeout)

	// Check that etcd has stopped on the survivor before proceeding
	g.By("Verifying that etcd has stopped on the surviving node")
	err := waitForEtcdToStop(testConfig)
	o.Expect(err).To(o.BeNil(), "Expected etcd to stop on surviving node %s within timeout", testConfig.SurvivingNode.Name)

	// SSH to hypervisor, then to surviving node to run pcs debug-start
	// We need to chain the SSH commands: host -> hypervisor -> surviving node
	output, _, err := services.PcsDebugRestart(testConfig.SurvivingNode.IP, false, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath, testConfig.SurvivingNode.KnownHostsPath)
	if err != nil {
		o.Expect(err).To(o.BeNil(), fmt.Sprintf("Failed to restore etcd quorum on %s: %v, output: %s", testConfig.SurvivingNode.Name, err, output))
	}

	// Verify that etcd has started on the survivor after debug-start
	g.By("Verifying that etcd has started on the surviving node after debug-start")
	err = waitForEtcdToStart(testConfig)
	if err != nil {
		// If we get here, etcd is not started on the surviving node
		// Get pacemaker journal logs to help with debugging
		journalLines := 25
		klog.V(2).Infof("Etcd is not started on %s, getting pacemaker journal logs for debugging", testConfig.SurvivingNode.Name)
		journalOutput, _, journalErr := services.PcsJournal(journalLines, testConfig.SurvivingNode.IP, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath, testConfig.SurvivingNode.KnownHostsPath)
		if journalErr != nil {
			klog.Warningf("Failed to get pacemaker journal logs on %s: %v", testConfig.SurvivingNode.Name, journalErr)
		} else {
			klog.V(4).Infof("Last %d lines of pacemaker journal on %s:\n%s", journalLines, testConfig.SurvivingNode.Name, journalOutput)
		}
	}
	o.Expect(err).To(o.BeNil(), "Expected etcd to start on surviving node %s within timeout", testConfig.SurvivingNode.Name)

	// Log pacemaker status to check if etcd has been started on the survivor
	pcsStatusOutput, _, err := services.PcsStatus(testConfig.SurvivingNode.IP, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath, testConfig.SurvivingNode.KnownHostsPath)
	if err != nil {
		klog.Warningf("Failed to get pacemaker status on survivor %s: %v", testConfig.SurvivingNode.IP, err)
	} else {
		klog.V(4).Infof("Pacemaker status on survivor %s:\n%s", testConfig.SurvivingNode.IP, pcsStatusOutput)
	}

	klog.V(2).Infof("Successfully restored etcd quorum on surviving node: %s", testConfig.SurvivingNode.Name)

	// Wait for the API to be responsive before proceeding with OpenShift operations
	g.By("Waiting for the Kubernetes API to be responsive after VM destruction")
	err = core.PollUntil(func() (bool, error) {
		if utils.IsAPIResponding(oc) {
			klog.V(2).Infof("Kubernetes API is responding")
			return true, nil
		}
		klog.V(4).Infof("Kubernetes API not yet responding, continuing to poll")
		return false, nil
	}, fiveMinuteTimeout, fifteenSecondPollInterval, "Kubernetes API to be responsive")
	o.Expect(err).To(o.BeNil(), "Expected Kubernetes API to be responsive within timeout")
}

// deleteNodeReferences deletes OpenShift resources related to the target node
func deleteNodeReferences(testConfig *TNFTestConfig, oc *exutil.CLI) {
	klog.V(2).Infof("Deleting OpenShift resources for node: %s", testConfig.TargetNode.Name)

	// Delete old etcd certificates using dynamic names
	_, err := oc.AsAdmin().Run("delete").Args(secretResourceType, testConfig.EtcdResources.PeerSecretName, "-n", etcdNamespace).Output()
	o.Expect(err).To(o.BeNil(), "Expected to delete %s secret without error", testConfig.EtcdResources.PeerSecretName)

	_, err = oc.AsAdmin().Run("delete").Args(secretResourceType, testConfig.EtcdResources.ServingSecretName, "-n", etcdNamespace).Output()
	o.Expect(err).To(o.BeNil(), "Expected to delete %s secret without error", testConfig.EtcdResources.ServingSecretName)

	_, err = oc.AsAdmin().Run("delete").Args(secretResourceType, testConfig.EtcdResources.ServingMetricsSecretName, "-n", etcdNamespace).Output()
	o.Expect(err).To(o.BeNil(), "Expected to delete %s secret without error", testConfig.EtcdResources.ServingMetricsSecretName)

	// Delete BareMetalHost entry
	_, err = oc.AsAdmin().Run("delete").Args(bmhResourceType, testConfig.TargetNode.BMHName, "-n", machineAPINamespace).Output()
	o.Expect(err).To(o.BeNil(), "Expected to delete BareMetalHost without error")

	// Delete machine entry using the stored testConfig.TargetNode.MachineName
	_, err = oc.AsAdmin().Run("delete").Args(machineResourceType, testConfig.TargetNode.MachineName, "-n", machineAPINamespace).Output()
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
	xmlPath = fmt.Sprintf(vmXMLFilePattern, testConfig.TargetNode.VMName)
	err = core.DeleteRemoteTempFile(xmlPath, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
	o.Expect(err).To(o.BeNil(), "Expected to clean up temporary XML file without error")

	err = services.WaitForVMToStart(testConfig.TargetNode.VMName, tenMinuteTimeout, thirtySecondPollInterval, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath)
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
	g.By("Deleting old TNF auth job")
	err = services.DeleteAuthJob(testConfig.Jobs.AuthJobName, oc)
	o.Expect(err).To(o.BeNil(), "Expected to delete auth job %s without error", testConfig.Jobs.AuthJobName)

	g.By("Deleting old TNF after-setup job")
	err = services.DeleteAfterSetupJob(testConfig.Jobs.AfterSetupJobName, oc)
	o.Expect(err).To(o.BeNil(), "Expected to delete after-setup job %s without error", testConfig.Jobs.AfterSetupJobName)

	// Wait for the auth job to complete before proceeding with pacemaker operations
	g.By("Waiting for TNF auth job to complete")
	err = services.WaitForJobCompletion(testConfig.Jobs.AuthJobName, etcdNamespace, tenMinuteTimeout, fifteenSecondPollInterval, oc)
	o.Expect(err).To(o.BeNil(), "Expected auth job %s to complete without error", testConfig.Jobs.AuthJobName)

	// Waiting for CEO to recreate /var/lib/etcd/revision.json
	g.By("Waiting for CEO to recreate /var/lib/etcd/revision.json")
	err = services.WaitForEtcdRevisionCreation(testConfig.TargetNode.IP, tenMinuteTimeout, thirtySecondPollInterval, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath, testConfig.TargetNode.KnownHostsPath, oc)
	o.Expect(err).To(o.BeNil(), "Expected to wait for etcd revision creation without error")

	// Now that authentication is complete, we can proceed with pacemaker cluster operations
	g.By("Cycling removed node in pacemaker cluster")
	err = services.CycleRemovedNode(testConfig.TargetNode.Name, testConfig.TargetNode.IP, testConfig.SurvivingNode.IP, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath, testConfig.SurvivingNode.KnownHostsPath)
	o.Expect(err).To(o.BeNil(), "Expected to cycle removed node without error")

	// Verify both nodes are online in the pacemaker cluster
	g.By("Verifying both nodes are online in pacemaker cluster")
	nodeNames := []string{testConfig.TargetNode.Name, testConfig.SurvivingNode.Name}
	err = services.WaitForNodesOnline(nodeNames, testConfig.SurvivingNode.IP, tenMinuteTimeout, thirtySecondPollInterval, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath, testConfig.SurvivingNode.KnownHostsPath)
	o.Expect(err).To(o.BeNil(), "Expected both nodes %v to be online in pacemaker cluster", nodeNames)
	klog.V(2).Infof("Both nodes %v are online in pacemaker cluster", nodeNames)
}

// verifyRestoredCluster verifies that the cluster is fully restored and healthy
func verifyRestoredCluster(testConfig *TNFTestConfig, oc *exutil.CLI) {
	klog.V(2).Infof("Verifying cluster restoration: checking node status and cluster operators")

	// Step 1: Verify both nodes are in Ready state
	g.By("Verifying both nodes are in Ready state")

	// Check target node
	o.Expect(utils.IsNodeReady(oc, testConfig.TargetNode.Name)).To(o.BeTrue(), "Expected target node %s to be in Ready state", testConfig.TargetNode.Name)
	klog.V(2).Infof("Target node %s is Ready", testConfig.TargetNode.Name)

	// Check surviving node
	o.Expect(utils.IsNodeReady(oc, testConfig.SurvivingNode.Name)).To(o.BeTrue(), "Expected surviving node %s to be in Ready state", testConfig.SurvivingNode.Name)
	klog.V(2).Infof("Surviving node %s is Ready", testConfig.SurvivingNode.Name)

	// Step 2: Verify all cluster operators are available (not degraded or progressing)
	g.By("Verifying all cluster operators are available")
	coOutput, err := monitorClusterOperators(oc, tenMinuteTimeout, fifteenSecondPollInterval)
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

// waitForEtcdToStop waits for etcd to stop on the surviving node
func waitForEtcdToStop(testConfig *TNFTestConfig) error {
	klog.V(2).Infof("Waiting for etcd to stop on surviving node: %s", testConfig.SurvivingNode.Name)

	return core.RetryWithOptions(func() error {
		// Check etcd resource status on the surviving node
		output, _, err := services.PcsResourceStatus(testConfig.SurvivingNode.Name, testConfig.SurvivingNode.IP, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath, testConfig.SurvivingNode.KnownHostsPath)
		if err != nil {
			return fmt.Errorf("failed to get etcd resource status on %s: %v, output: %s", testConfig.SurvivingNode.Name, err, output)
		}

		klog.V(4).Infof("Etcd resource status on %s:\n%s", testConfig.SurvivingNode.Name, output)

		// Check if etcd is stopped (not started) on the surviving node
		// We expect to see "Stopped: [ master-X ]" or no "Started:" line for the survivor
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.Contains(line, "Started:") && strings.Contains(line, testConfig.SurvivingNode.Name) {
				return fmt.Errorf("etcd is still started on surviving node %s", testConfig.SurvivingNode.Name)
			}
		}

		// If we get here, etcd is not started on the surviving node
		klog.V(2).Infof("Etcd has stopped on surviving node: %s", testConfig.SurvivingNode.Name)
		return nil
	}, core.RetryOptions{
		Timeout:      fiveMinuteTimeout,
		PollInterval: fiveSecondPollInterval,
	}, fmt.Sprintf("etcd stop on %s", testConfig.SurvivingNode.Name))
}

// waitForEtcdToStart waits for etcd to start on the surviving node
func waitForEtcdToStart(testConfig *TNFTestConfig) error {
	klog.V(2).Infof("Waiting for etcd to start on surviving node: %s", testConfig.SurvivingNode.Name)

	return core.RetryWithOptions(func() error {
		// Check etcd resource status on the surviving node
		output, _, err := services.PcsResourceStatus(testConfig.SurvivingNode.Name, testConfig.SurvivingNode.IP, &testConfig.Hypervisor.Config, testConfig.Hypervisor.KnownHostsPath, testConfig.SurvivingNode.KnownHostsPath)
		if err != nil {
			return fmt.Errorf("failed to get etcd resource status on %s: %v, output: %s", testConfig.SurvivingNode.Name, err, output)
		}

		klog.V(4).Infof("Etcd resource status on %s:\n%s", testConfig.SurvivingNode.Name, output)

		// Check if etcd is started on the surviving node
		// We expect to see "Started: [ master-X ]" for the survivor
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.Contains(line, "Started:") && strings.Contains(line, testConfig.SurvivingNode.Name) {
				klog.V(2).Infof("Etcd has started on surviving node: %s", testConfig.SurvivingNode.Name)
				return nil
			}
		}

		return fmt.Errorf("etcd is not started on surviving node %s", testConfig.SurvivingNode.Name)
	}, core.RetryOptions{
		Timeout:      fiveMinuteTimeout,
		PollInterval: fiveSecondPollInterval,
	}, fmt.Sprintf("etcd start on %s", testConfig.SurvivingNode.Name))
}

// monitorClusterOperators monitors cluster operators and ensures they are all available
func monitorClusterOperators(oc *exutil.CLI, timeout time.Duration, pollInterval time.Duration) (string, error) {
	err := core.PollUntil(func() (bool, error) {
		// Get cluster operators status in JSON format
		coOutput, err := oc.AsAdmin().Run("get").Args("co", "-o", "json").Output()
		if err != nil {
			klog.V(4).Infof("Error getting cluster operators: %v", err)
			return false, nil // Temporary error, continue polling
		}

		// Parse the JSON response
		var coList clusterOperatorList
		if err := utils.UnmarshalJSON(coOutput, &coList); err != nil {
			klog.V(4).Infof("Failed to parse ClusterOperator list JSON: %v", err)
			return false, nil // Parse error, continue polling
		}

		// Check each operator's conditions
		allHealthy := true
		var degradedOperators []string
		var progressingOperators []string

		for _, co := range coList.Items {
			isDegraded := false
			isProgressing := false

			// Check conditions
			for _, condition := range co.Status.Conditions {
				if condition.Type == "Degraded" && condition.Status == "True" {
					isDegraded = true
					degradedOperators = append(degradedOperators, fmt.Sprintf("%s: %s (reason: %s)", co.Metadata.Name, condition.Message, condition.Reason))
				}
				if condition.Type == "Progressing" && condition.Status == "True" {
					isProgressing = true
					progressingOperators = append(progressingOperators, fmt.Sprintf("%s: %s (reason: %s)", co.Metadata.Name, condition.Message, condition.Reason))
				}
			}

			if isDegraded || isProgressing {
				allHealthy = false
			}
		}

		// Log current status
		klog.V(4).Infof("Cluster operators status check: All healthy: %v, Degraded count: %d, Progressing count: %d",
			allHealthy, len(degradedOperators), len(progressingOperators))

		if len(degradedOperators) > 0 {
			klog.V(4).Infof("Degraded operators: %v", degradedOperators)
		}
		if len(progressingOperators) > 0 {
			klog.V(4).Infof("Progressing operators: %v", progressingOperators)
		}

		// If all operators are healthy, we're done
		if allHealthy {
			klog.V(2).Infof("All cluster operators are healthy (not degraded or progressing)!")
			return true, nil // All healthy, stop polling
		}

		// Log the current operator status for debugging
		if klog.V(4).Enabled() {
			wideOutput, _ := oc.AsAdmin().Run("get").Args("co", "-o", "wide").Output()
			klog.V(4).Infof("Current cluster operators status:\n%s", wideOutput)
		}

		return false, nil // Not all healthy yet, continue polling
	}, timeout, pollInterval, "cluster operators to be healthy")

	// Get final wide output for display purposes
	wideOutput, _ := oc.AsAdmin().Run("get").Args("co", "-o", "wide").Output()

	if err != nil {
		klog.V(4).Infof("Final cluster operators status after timeout:\n%s", wideOutput)
		return wideOutput, fmt.Errorf("cluster operators did not become healthy: %w", err)
	}

	return wideOutput, nil
}

// updateAndCreateBMH creates a new BareMetalHost from template
func updateAndCreateBMH(testConfig *TNFTestConfig, oc *exutil.CLI, newUUID, newMACAddress string) {
	klog.V(2).Infof("Creating BareMetalHost with UUID: %s, MAC: %s", newUUID, newMACAddress)

	// Create BareMetalHost from template with placeholder substitution
	err := core.CreateResourceFromTemplate(oc, bmhTemplatePath, map[string]string{
		"{NAME}":             testConfig.TargetNode.BMHName,
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
		bmhOutput, err := oc.AsAdmin().Run("get").Args(bmhResourceType, testConfig.TargetNode.BMHName, "-n", machineAPINamespace, "-o", yamlOutputFormat).Output()
		if err != nil {
			klog.V(4).Infof("Error getting BareMetalHost %s: %v", testConfig.TargetNode.BMHName, err)
			return false, nil // Continue polling on errors
		}

		// Parse the YAML into a BareMetalHost object
		var bmh metal3v1alpha1.BareMetalHost
		decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(bmhOutput), 4096)
		err = decoder.Decode(&bmh)
		if err != nil {
			klog.V(4).Infof("Error parsing BareMetalHost YAML: %v", err)
			return false, nil // Continue polling on parse errors
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
	templatePath := filepath.Join("test", "extended", "testdata", "two_node", "machine-template.yaml")
	err = core.CreateResourceFromTemplate(oc, templatePath, map[string]string{
		"{NODE_NAME}":    testConfig.TargetNode.BMHName,
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

	// Throw an error if no match is found
	if suffix == "" {
		o.Expect("").To(o.BeEmpty(), "Expected to find %s matching pattern *-%s", resourceType, nodeName)
	} else {
		o.Expect("").To(o.BeEmpty(), "Expected to find %s matching pattern *-%s-%s", resourceType, nodeName, suffix)
	}
	return ""
}

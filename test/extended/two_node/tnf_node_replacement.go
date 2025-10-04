package two_node

import (
	"encoding/json"
	"fmt"
	"math/rand"
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
	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/klog/v2"
)

// Constants
const (
	backupDirName = "tnf-node-replacement-backup"

	// OpenShift namespaces
	machineAPINamespace = "openshift-machine-api"
	etcdNamespace = "openshift-etcd"

	// Timeouts and intervals
	nodeRecoveryTimeout = 10 * time.Minute
	nodeRecoveryPollInterval = 15 * time.Second
	csrApprovalTimeout = 5 * time.Minute
	csrApprovalPollInterval = 10 * time.Second
	clusterOperatorTimeout = 5 * time.Minute
	clusterOperatorPollInterval = 10 * time.Second
	etcdStopWaitTime = 30 * time.Second
	etcdStatusCheckTimeout = 2 * time.Minute
	etcdStatusCheckPollInterval = 5 * time.Second

	// Expected counts
	expectedCSRCount = 2

	// Resource types
	secretResourceType = "secret"
	bmhResourceType = "bmh"
	machineResourceType = "machines.machine.openshift.io"

	// Output formats
	yamlOutputFormat = "yaml"

	// Node states
	nodeReadyState = "Ready"

	// BMH states
	bmhProvisionedState = "provisioned"

	// Cluster operator states
	coDegradedState = "Degraded=True"
	coProgressingState = "Progressing=True"

	// Base names for dynamic resource names
	etcdPeerSecretBaseName = "etcd-peer"
	etcdServingSecretBaseName = "etcd-serving"
	etcdServingMetricsSecretBaseName = "etcd-serving-metrics"
	tnfAuthJobBaseName = "tnf-auth-job"
	tnfAfterSetupJobBaseName = "tnf-after-setup-job"

	// Virsh commands
	virshProvisioningBridge = "ostestpr"

	// Additional constants for pacemaker operations
	pacemakerQuorumTimeout = 5 * time.Minute
	pacemakerQuorumPollInterval = 10 * time.Second
)

// Variables

// TNFTestConfig holds all test configuration and state
// This struct groups related variables to avoid global variable shadowing and improve maintainability
type TNFTestConfig struct {
	HypervisorConfig utils.SSHConfig

	// Node configuration
	TargetNodeName string
	TargetNodeIP string
	TargetVMName string
	TargetMachineName string
	TargetMachineHash string
	TargetBMCSecretName string
	TargetBMHName string
	TargetNodeMAC string
	SurvivingNodeName string
	SurvivingNodeIP string

	// Dynamic resource names
	EtcdPeerSecretName string
	EtcdServingSecretName string
	EtcdServingMetricsSecretName string
	TNFAuthJobName string
	TNFAfterSetupJobName string

	// Backup and recovery
	GlobalBackupDir string

	// Test execution tracking
	HasAttemptedNodeProvisioning bool

	// Known hosts file paths
	HypervisorKnownHostsPath string
	TargetNodeKnownHostsPath string
	SurvivingNodeKnownHostsPath string
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

// Global test configuration instance
var (
	oc = createCLI(admin)
)

// Main test function
var _ = g.Describe("[sig-etcd][apigroup:config.openshift.io][OCPFeatureGate:DualReplica][Suite:openshift/two-node][Disruptive][Requires:HypervisorSSHConfig] TNF", func() {
	var testConfig TNFTestConfig
	defer g.GinkgoRecover()

	g.BeforeEach(func() {
		skipIfNotTopology(oc, configv1.DualReplicaTopologyMode)
		setupTestEnvironment(&testConfig, oc)
	})

	g.AfterEach(func() {
		// Always attempt recovery if we have backup data
		if testConfig.GlobalBackupDir != "" {
			g.By("Attempting cluster recovery from backup")
			recoverClusterFromBackup(&testConfig, oc)
		}
		// Clean up target node known_hosts only if it was created (after reprovisioning)
		if testConfig.TargetNodeKnownHostsPath != "" {
			utils.CleanupRemoteKnownHostsFile(&testConfig.HypervisorConfig, testConfig.HypervisorKnownHostsPath, testConfig.TargetNodeKnownHostsPath)
		}
		utils.CleanupRemoteKnownHostsFile(&testConfig.HypervisorConfig, testConfig.HypervisorKnownHostsPath, testConfig.SurvivingNodeKnownHostsPath)
		utils.CleanupLocalKnownHostsFile(&testConfig.HypervisorConfig, testConfig.HypervisorKnownHostsPath)
	})

	g.It("should recover from an in-place node replacement", func() {

		g.By("Backing up the target node's configuration")
		backupDir := backupTargetNodeConfiguration(&testConfig, oc)
		testConfig.GlobalBackupDir = backupDir // Store globally for recovery
		defer func() {
			if backupDir != "" && testConfig.GlobalBackupDir == "" {
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
		recreateTargetVM(&testConfig, oc, backupDir)

		g.By("Provisioning the target node with Ironic")
		provisionTargetNodeWithIronic(&testConfig, oc)

		g.By("Approving certificate signing requests for the new node")
		approveCSRs(oc)

		g.By("Waiting for the replacement node to appear in the cluster")
		waitForNodeRecovery(&testConfig, oc)

		g.By("Restoring pacemaker cluster configuration")
		restorePacemakerCluster(&testConfig, oc)

		g.By("Verifying the cluster is fully restored")
		verifyRestoredCluster(&testConfig, oc)

		g.By("Successfully completed node replacement process")
		klog.V(2).Infof("Node replacement process completed. Backup files created in: %s", backupDir)
	})
})

// Step functions that run all of the steps

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

// backupTargetNodeConfiguration backs up all necessary resources for node replacement
func backupTargetNodeConfiguration(testConfig *TNFTestConfig, oc *exutil.CLI) string {
	// Create backup directory
	var err error
	backupDir, err := os.MkdirTemp("", backupDirName)
	o.Expect(err).To(o.BeNil(), "Expected to create backup directory without error")

	// Download backup of BMC secret
	bmcSecretOutput, err := oc.AsAdmin().Run("get").Args(secretResourceType, testConfig.TargetBMCSecretName, "-n", machineAPINamespace, "-o", yamlOutputFormat).Output()
	o.Expect(err).To(o.BeNil(), "Expected to get BMC secret without error")
	bmcSecretFile := filepath.Join(backupDir, testConfig.TargetBMCSecretName+".yaml")
	err = os.WriteFile(bmcSecretFile, []byte(bmcSecretOutput), 0644)
	o.Expect(err).To(o.BeNil(), "Expected to write BMC secret backup without error")

	// Download backup of BareMetalHost
	bmhOutput, err := oc.AsAdmin().Run("get").Args(bmhResourceType, testConfig.TargetBMHName, "-n", machineAPINamespace, "-o", yamlOutputFormat).Output()
	o.Expect(err).To(o.BeNil(), "Expected to get BareMetalHost without error")
	bmhFile := filepath.Join(backupDir, testConfig.TargetBMHName+".yaml")
	err = os.WriteFile(bmhFile, []byte(bmhOutput), 0644)
	o.Expect(err).To(o.BeNil(), "Expected to write BareMetalHost backup without error")

	// Backup machine definition using the stored testConfig.TargetMachineName
	machineOutput, err := oc.AsAdmin().Run("get").Args(machineResourceType, testConfig.TargetMachineName, "-n", machineAPINamespace, "-o", yamlOutputFormat).Output()
	o.Expect(err).To(o.BeNil(), "Expected to get machine without error")
	machineFile := filepath.Join(backupDir, fmt.Sprintf("%s-machine.yaml", testConfig.TargetMachineName))
	err = os.WriteFile(machineFile, []byte(machineOutput), 0644)
	o.Expect(err).To(o.BeNil(), "Expected to write machine backup without error")

	// Backup etcd secrets
	etcdSecrets := []string{
		testConfig.EtcdPeerSecretName,
		testConfig.EtcdServingSecretName,
		testConfig.EtcdServingMetricsSecretName,
	}

	for _, secretName := range etcdSecrets {
		// Get the secret if it exists
		secretOutput, err := oc.AsAdmin().Run("get").Args(secretResourceType, secretName, "-n", etcdNamespace, "-o", yamlOutputFormat).Output()
		if err != nil {
			klog.Warningf("Could not backup etcd secret %s: %v", secretName, err)
			continue
		}

		secretFile := filepath.Join(backupDir, secretName+".yaml")
		err = os.WriteFile(secretFile, []byte(secretOutput), 0644)
		o.Expect(err).To(o.BeNil(), "Expected to write etcd secret %s backup without error", secretName)
		klog.V(2).Infof("Backed up etcd secret: %s", secretName)
	}

	klog.V(4).Infof("About to validate testConfig.TargetVMName, current value: %s", testConfig.TargetVMName)
	// Validate that testConfig.TargetVMName is set
	if testConfig.TargetVMName == "" {
		klog.V(2).Infof("testConfig.TargetVMName bytes: %v", []byte(testConfig.TargetVMName))
		klog.V(2).Infof("ERROR: testConfig.TargetVMName is empty! This should have been set in setupTestEnvironment")
		klog.V(2).Infof("testConfig.TargetNodeName: %s", testConfig.TargetNodeName)
		klog.V(2).Infof("testConfig.SurvivingNodeName: %s", testConfig.SurvivingNodeName)
		o.Expect(testConfig.TargetVMName).ToNot(o.BeEmpty(), "Expected testConfig.TargetVMName to be set before backing up VM configuration")
	}
	// Get XML dump of VM using SSH to hypervisor
	xmlOutput, err := utils.VirshDumpXML(testConfig.TargetVMName, &testConfig.HypervisorConfig, testConfig.HypervisorKnownHostsPath)
	o.Expect(err).To(o.BeNil(), "Expected to get XML dump without error")

	xmlFile := filepath.Join(backupDir, testConfig.TargetVMName+".xml")
	err = os.WriteFile(xmlFile, []byte(xmlOutput), 0644)
	o.Expect(err).To(o.BeNil(), "Expected to write XML dump to file without error")

	return backupDir
}

// destroyVM destroys the target VM using SSH to hypervisor
func destroyVM(testConfig *TNFTestConfig) {
	o.Expect(testConfig.TargetVMName).ToNot(o.BeEmpty(), "Expected testConfig.TargetVMName to be set before destroying VM")
	klog.V(2).Infof("Destroying VM: %s", testConfig.TargetVMName)

	// Undefine and destroy VM using SSH to hypervisor
	err := utils.VirshUndefineVM(testConfig.TargetVMName, &testConfig.HypervisorConfig, testConfig.HypervisorKnownHostsPath)
	o.Expect(err).To(o.BeNil(), "Expected to undefine VM without error")

	err = utils.VirshDestroyVM(testConfig.TargetVMName, &testConfig.HypervisorConfig, testConfig.HypervisorKnownHostsPath)
	o.Expect(err).To(o.BeNil(), "Expected to destroy VM without error")

	klog.V(2).Infof("VM %s destroyed successfully", testConfig.TargetVMName)
}

// deleteNodeReferences deletes OpenShift resources related to the target node
func deleteNodeReferences(testConfig *TNFTestConfig, oc *exutil.CLI) {
	klog.V(2).Infof("Deleting OpenShift resources for node: %s", testConfig.TargetNodeName)

	// Delete old etcd certificates using dynamic names
	_, err := oc.AsAdmin().Run("delete").Args(secretResourceType, testConfig.EtcdPeerSecretName, "-n", etcdNamespace).Output()
	o.Expect(err).To(o.BeNil(), "Expected to delete %s secret without error", testConfig.EtcdPeerSecretName)

	_, err = oc.AsAdmin().Run("delete").Args(secretResourceType, testConfig.EtcdServingSecretName, "-n", etcdNamespace).Output()
	o.Expect(err).To(o.BeNil(), "Expected to delete %s secret without error", testConfig.EtcdServingSecretName)

	_, err = oc.AsAdmin().Run("delete").Args(secretResourceType, testConfig.EtcdServingMetricsSecretName, "-n", etcdNamespace).Output()
	o.Expect(err).To(o.BeNil(), "Expected to delete %s secret without error", testConfig.EtcdServingMetricsSecretName)

	// Delete BareMetalHost entry
	_, err = oc.AsAdmin().Run("delete").Args(bmhResourceType, testConfig.TargetBMHName, "-n", machineAPINamespace).Output()
	o.Expect(err).To(o.BeNil(), "Expected to delete BareMetalHost without error")

	// Delete machine entry using the stored testConfig.TargetMachineName
	_, err = oc.AsAdmin().Run("delete").Args(machineResourceType, testConfig.TargetMachineName, "-n", machineAPINamespace).Output()
	o.Expect(err).To(o.BeNil(), "Expected to delete machine without error")

	klog.V(2).Infof("OpenShift resources for node %s deleted successfully", testConfig.TargetNodeName)
}

// restoreEtcdQuorumOnSurvivor restores etcd quorum on the surviving node
func restoreEtcdQuorumOnSurvivor(testConfig *TNFTestConfig, oc *exutil.CLI) {
	klog.V(2).Infof("Restoring etcd quorum on surviving node: %s", testConfig.SurvivingNodeName)

	// Wait 30 seconds after node deletion to allow etcd to stop naturally
	g.By("Waiting 30 seconds for etcd to stop naturally after node deletion")
	time.Sleep(etcdStopWaitTime)

	// Check that etcd has stopped on the survivor before proceeding
	g.By("Verifying that etcd has stopped on the surviving node")
	err := waitForEtcdToStop(testConfig)
	o.Expect(err).To(o.BeNil(), "Expected etcd to stop on surviving node %s within timeout", testConfig.SurvivingNodeName)

	// SSH to hypervisor, then to surviving node to run pcs debug-start
	// We need to chain the SSH commands: host -> hypervisor -> surviving node
	output, _, err := utils.PcsDebugRestart(testConfig.SurvivingNodeIP, &testConfig.HypervisorConfig, testConfig.HypervisorKnownHostsPath, testConfig.SurvivingNodeKnownHostsPath)
	if err != nil {
		o.Expect(err).To(o.BeNil(), fmt.Sprintf("Failed to restore etcd quorum on %s: %v, output: %s", testConfig.SurvivingNodeName, err, output))
	}

	// Verify that etcd has started on the survivor after debug-start
	g.By("Verifying that etcd has started on the surviving node after debug-start")
	err = waitForEtcdToStart(testConfig)
	o.Expect(err).To(o.BeNil(), "Expected etcd to start on surviving node %s within timeout", testConfig.SurvivingNodeName)

	// Log pacemaker status to check if etcd has been started on the survivor
	pcsStatusOutput, _, err := utils.PcsStatus(testConfig.SurvivingNodeIP, &testConfig.HypervisorConfig, testConfig.HypervisorKnownHostsPath, testConfig.SurvivingNodeKnownHostsPath)
	if err != nil {
		klog.Warningf("Failed to get pacemaker status on survivor %s: %v", testConfig.SurvivingNodeIP, err)
	} else {
		klog.V(4).Infof("Pacemaker status on survivor %s:\n%s", testConfig.SurvivingNodeIP, pcsStatusOutput)
	}

	klog.V(2).Infof("Successfully restored etcd quorum on surviving node: %s", testConfig.SurvivingNodeName)

	// Wait for pacemaker to restore quorum before proceeding with OpenShift API operations
	g.By("Waiting for pacemaker to restore quorum after VM destruction")
	output, err = monitorClusterOperators(oc)
	o.Expect(err).To(o.BeNil(), "Expected pacemaker to restore quorum within timeout")
	klog.V(2).Infof("Cluster operators status:\n%s", output)
}

// recreateTargetVM recreates the target VM using backed up configuration
func recreateTargetVM(testConfig *TNFTestConfig, oc *exutil.CLI, backupDir string) {
	o.Expect(testConfig.TargetVMName).ToNot(o.BeEmpty(), "Expected testConfig.TargetVMName to be set before recreating VM")
	// Read the backed up XML
	xmlFile := filepath.Join(backupDir, testConfig.TargetVMName+".xml")
	xmlContent, err := os.ReadFile(xmlFile)
	o.Expect(err).To(o.BeNil(), "Expected to read XML backup without error")
	xmlOutput := string(xmlContent)

	// Create a temporary file on the hypervisor with the XML content
	// First, create the XML file on the hypervisor
	createXMLCommand := fmt.Sprintf(`cat > /tmp/%s.xml <<'XML_EOF'
%s
XML_EOF`, testConfig.TargetVMName, xmlOutput)

	_, _, err = utils.ExecuteSSHCommand(createXMLCommand, &testConfig.HypervisorConfig, testConfig.HypervisorKnownHostsPath)
	o.Expect(err).To(o.BeNil(), "Expected to create XML file on hypervisor without error")

	// Redefine the VM using the backed up XML
	err = utils.VirshDefineVM(fmt.Sprintf("/tmp/%s.xml", testConfig.TargetVMName), &testConfig.HypervisorConfig, testConfig.HypervisorKnownHostsPath)
	o.Expect(err).To(o.BeNil(), "Expected to define VM without error")

	// Start the VM with autostart enabled
	err = utils.VirshStartVM(testConfig.TargetVMName, &testConfig.HypervisorConfig, testConfig.HypervisorKnownHostsPath)
	o.Expect(err).To(o.BeNil(), "Expected to start VM without error")

	err = utils.VirshAutostartVM(testConfig.TargetVMName, &testConfig.HypervisorConfig, testConfig.HypervisorKnownHostsPath)
	o.Expect(err).To(o.BeNil(), "Expected to enable autostart for VM without error")

	// Clean up temporary XML file
	_, _, err = utils.ExecuteSSHCommand(fmt.Sprintf("rm -f /tmp/%s.xml", testConfig.TargetVMName), &testConfig.HypervisorConfig, testConfig.HypervisorKnownHostsPath)
	o.Expect(err).To(o.BeNil(), "Expected to clean up temporary XML file without error")
}

// provisionTargetNodeWithIronic handles the Ironic provisioning process
func provisionTargetNodeWithIronic(testConfig *TNFTestConfig, oc *exutil.CLI) {
	o.Expect(testConfig.TargetVMName).ToNot(o.BeEmpty(), "Expected testConfig.TargetVMName to be set before provisioning with Ironic")

	// Set flag to indicate we're attempting node provisioning
	testConfig.HasAttemptedNodeProvisioning = true

	recreateBMCSecret(testConfig, oc)
	newUUID, newMACAddress, err := utils.GetVMNetworkInfo(testConfig.TargetVMName, virshProvisioningBridge, &testConfig.HypervisorConfig, testConfig.HypervisorKnownHostsPath)
	o.Expect(err).To(o.BeNil(), "Expected to get VM network info: %v", err)
	updateAndCreateBMH(testConfig, oc, newUUID, newMACAddress)
	waitForBMHProvisioning(testConfig, oc)
	reapplyDetachedAnnotation(testConfig, oc)
	recreateMachine(testConfig, oc)
}

// approveCSRs monitors and approves Certificate Signing Requests
func approveCSRs(oc *exutil.CLI) {
	// Monitor CSRs and approve them as they appear
	maxCSRWaitTime := csrApprovalTimeout
	csrPollInterval := csrApprovalPollInterval
	csrStartTime := time.Now()
	approvedCount := 0
	targetApprovedCount := expectedCSRCount

	for time.Since(csrStartTime) < maxCSRWaitTime && approvedCount < targetApprovedCount {
		// Get pending CSRs
		csrOutput, err := oc.AsAdmin().Run("get").Args("csr", "-o", "json").Output()
		if err == nil {
			// Extract CSR names that need approval (status is empty)
			pendingCSRs := []string{}
			lines := strings.Split(csrOutput, "\n")
			for _, line := range lines {
				if strings.Contains(line, "\"name\"") && strings.Contains(line, "\"status\": {}") {
					// Extract CSR name from JSON
					start := strings.Index(line, "\"name\": \"") + 8
					end := strings.Index(line[start:], "\"") + start
					if start > 7 && end > start {
						csrName := line[start:end]
						pendingCSRs = append(pendingCSRs, csrName)
					}
				}
			}

			// Approve pending CSRs
			for _, csrName := range pendingCSRs {
				klog.V(2).Infof("Approving CSR: %s", csrName)
				_, err = oc.AsAdmin().Run("adm").Args("certificate", "approve", csrName).Output()
				if err == nil {
					approvedCount++
					klog.V(2).Infof("Approved CSR %s (total approved: %d)", csrName, approvedCount)
				}
			}
		}

		if approvedCount < targetApprovedCount {
			klog.V(2).Infof("Waiting for more CSRs to approve... (approved: %d/%d, elapsed: %v)", approvedCount, targetApprovedCount, time.Since(csrStartTime))
			time.Sleep(csrPollInterval)
		}
	}

	// Verify we have approved the expected number of CSRs
	o.Expect(approvedCount).To(o.BeNumerically(">=", targetApprovedCount), fmt.Sprintf("Expected to approve at least %d CSRs, but only approved %d", targetApprovedCount, approvedCount))
	klog.V(2).Infof("Successfully approved %d CSRs", approvedCount)
}

// waitForNodeRecovery monitors for the replacement node to appear in the cluster
func waitForNodeRecovery(testConfig *TNFTestConfig, oc *exutil.CLI) {
	maxWaitTime := nodeRecoveryTimeout
	pollInterval := nodeRecoveryPollInterval
	startTime := time.Now()

	for time.Since(startTime) < maxWaitTime {
		// Check if the target node exists
		_, err := oc.AsAdmin().Run("get").Args("node", testConfig.TargetNodeName).Output()
		if err == nil {
			klog.V(2).Infof("Replacement node %s has appeared in the cluster", testConfig.TargetNodeName)

			// Wait a bit more for the node to be fully ready
			time.Sleep(30 * time.Second)

			// Verify the node is in Ready state
			nodeOutput, err := oc.AsAdmin().Run("get").Args("node", testConfig.TargetNodeName, "-o", "wide").Output()
			if err == nil {
				klog.V(4).Infof("Node status: %s", nodeOutput)
				if strings.Contains(nodeOutput, nodeReadyState) {
					klog.V(2).Infof("Node %s is now Ready", testConfig.TargetNodeName)
					return
				}
			}
		}

		klog.V(2).Infof("Waiting for replacement node %s to appear... (elapsed: %v)", testConfig.TargetNodeName, time.Since(startTime))
		time.Sleep(pollInterval)
	}

	// If we reach here, the timeout was exceeded
	o.Expect(false).To(o.BeTrue(), fmt.Sprintf("Replacement node %s did not appear within %v timeout", testConfig.TargetNodeName, maxWaitTime))
}

// restorePacemakerCluster restores the pacemaker cluster configuration
func restorePacemakerCluster(testConfig *TNFTestConfig, oc *exutil.CLI) {
	// Prepare known hosts file for the target node now that it has been reprovisioned
	// The SSH key changed during reprovisioning, so we need to scan it again
	klog.V(2).Infof("Preparing known_hosts for reprovisioned target node: %s", testConfig.TargetNodeIP)
	targetNodeKnownHostsPath, err := utils.PrepareRemoteKnownHostsFile(testConfig.TargetNodeIP, &testConfig.HypervisorConfig, testConfig.HypervisorKnownHostsPath)
	o.Expect(err).To(o.BeNil(), "Expected to prepare target node known hosts file after reprovisioning without error")
	testConfig.TargetNodeKnownHostsPath = targetNodeKnownHostsPath

	utils.DeleteNodeJobs(testConfig.TNFAuthJobName, testConfig.TNFAfterSetupJobName, oc)
	utils.RestoreEtcdRevision(testConfig.TargetNodeName, testConfig.TargetNodeIP, &testConfig.HypervisorConfig, testConfig.HypervisorKnownHostsPath, testConfig.TargetNodeKnownHostsPath, oc)
	utils.CycleRemovedNode(testConfig.TargetNodeName, testConfig.TargetNodeIP, testConfig.SurvivingNodeIP, &testConfig.HypervisorConfig, testConfig.HypervisorKnownHostsPath, testConfig.SurvivingNodeKnownHostsPath)
}

// verifyRestoredCluster verifies that the cluster is fully restored and healthy
func verifyRestoredCluster(testConfig *TNFTestConfig, oc *exutil.CLI) {
	klog.V(2).Infof("Verifying cluster restoration: checking node status and cluster operators")

	// Step 1: Verify both nodes are in Ready state
	g.By("Verifying both nodes are in Ready state")

	// Check target node
	targetNodeOutput, err := oc.AsAdmin().Run("get").Args("node", testConfig.TargetNodeName, "-o", "wide").Output()
	o.Expect(err).To(o.BeNil(), "Expected to get target node %s without error", testConfig.TargetNodeName)
	o.Expect(targetNodeOutput).To(o.ContainSubstring(nodeReadyState), "Expected target node %s to be in Ready state", testConfig.TargetNodeName)
	klog.V(2).Infof("Target node %s is Ready", testConfig.TargetNodeName)

	// Check surviving node
	survivingNodeOutput, err := oc.AsAdmin().Run("get").Args("node", testConfig.SurvivingNodeName, "-o", "wide").Output()
	o.Expect(err).To(o.BeNil(), "Expected to get surviving node %s without error", testConfig.SurvivingNodeName)
	o.Expect(survivingNodeOutput).To(o.ContainSubstring(nodeReadyState), "Expected surviving node %s to be in Ready state", testConfig.SurvivingNodeName)
	klog.V(2).Infof("Surviving node %s is Ready", testConfig.SurvivingNodeName)

	// Step 2: Verify all cluster operators are available (not degraded or progressing)
	g.By("Verifying all cluster operators are available")
	coOutput, err := monitorClusterOperators(oc)
	o.Expect(err).To(o.BeNil(), "Expected all cluster operators to be available")
	klog.V(2).Infof("All cluster operators are available and healthy")

	// Log final status
	klog.V(2).Infof("Cluster verification completed successfully:")
	klog.V(2).Infof("  - Target node %s is Ready", testConfig.TargetNodeName)
	klog.V(2).Infof("  - Surviving node %s is Ready", testConfig.SurvivingNodeName)
	klog.V(2).Infof("  - All cluster operators are available")
	klog.V(2).Infof("\nFinal cluster operators status:\n%s", coOutput)
}

// monitorClusterOperators monitors cluster operators and ensures they are all available
func monitorClusterOperators(oc *exutil.CLI) (string, error){
	maxWaitTime := clusterOperatorTimeout
	pollInterval := clusterOperatorPollInterval
	startTime := time.Now()

	for time.Since(startTime) < maxWaitTime {
		// Get cluster operators status
		coOutput, err := oc.AsAdmin().Run("get").Args("co", "-o", "wide").Output()
		if err != nil {
			klog.V(2).Infof("Error getting cluster operators: %v", err)
			time.Sleep(pollInterval)
			continue
		}

		// Parse the output to check operator statuses
		lines := strings.Split(coOutput, "\n")
		allAvailable := true
		hasDegraded := false
		hasProgressing := false

		for _, line := range lines {
			// Skip header line
			if strings.Contains(line, "NAME") && strings.Contains(line, "VERSION") {
				continue
			}

			// Skip empty lines
			if strings.TrimSpace(line) == "" {
				continue
			}

			// Check for degraded or progressing operators
			if strings.Contains(line, coDegradedState) {
				hasDegraded = true
				allAvailable = false
				klog.V(2).Infof("Found degraded operator: %s", line)
			}
			if strings.Contains(line, coProgressingState) {
				hasProgressing = true
				allAvailable = false
				klog.V(2).Infof("Found progressing operator: %s", line)
			}
		}

		// Log current status
		klog.V(2).Infof("Cluster operators status check (elapsed: %v):", time.Since(startTime))
		klog.V(2).Infof("All available: %v, Has degraded: %v, Has progressing: %v", allAvailable, hasDegraded, hasProgressing)

		// If all operators are available, we're done
		if allAvailable {
			klog.V(2).Infof("All cluster operators are available!")
			return coOutput, nil
		}

		// Log the current operator status for debugging
		klog.V(4).Infof("Current cluster operators status:\n%s", coOutput)

		// Wait before next check
		time.Sleep(pollInterval)
	}

	// If we reach here, the timeout was exceeded
	// Get final status for debugging
	finalCoOutput, err := oc.AsAdmin().Run("get").Args("co", "-o", "wide").Output()
	if err == nil {
		klog.V(4).Infof("Final cluster operators status after timeout:\n%s", finalCoOutput)
	}

	return finalCoOutput, fmt.Errorf("cluster operators did not become available within %v timeout", maxWaitTime)
}

// Utility functions

func getNodeMACAddress(oc *exutil.CLI, nodeName string) string {
	// Find the BareMetalHost name using regex pattern matching
	bmhName := findObjectByNamePattern(oc, bmhResourceType, machineAPINamespace, nodeName, "")

	// Get the BareMetalHost YAML to extract the MAC address
	bmhOutput, err := kubectlGetResource(oc, bmhResourceType, bmhName, machineAPINamespace, yamlOutputFormat)
	o.Expect(err).To(o.BeNil(), "Expected to get BareMetalHost without error")

	// Parse the YAML into a BareMetalHost object
	var bmh metal3v1alpha1.BareMetalHost
	decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(bmhOutput), 4096)
	err = decoder.Decode(&bmh)
	o.Expect(err).To(o.BeNil(), "Expected to parse BareMetalHost YAML without error")

	// Extract the MAC address from the BareMetalHost spec
	macAddress := bmh.Spec.BootMACAddress
	o.Expect(macAddress).ToNot(o.BeEmpty(), "Expected BareMetalHost %s to have a BootMACAddress", bmhName)

	klog.V(2).Infof("Found MAC address %s for node %s", macAddress, nodeName)
	return macAddress
}

// setupTestEnvironment validates prerequisites and gathers required information
func setupTestEnvironment(testConfig *TNFTestConfig, oc *exutil.CLI) {
	// Get hypervisor configuration from test context
	if !exutil.HasHypervisorConfig() {
		printHypervisorConfigUsage()
		o.Expect(fmt.Errorf("no hypervisor configuration available")).To(o.BeNil(), "Hypervisor configuration is required. See usage message above for configuration options.")
	}

	config := exutil.GetHypervisorConfig()
	testConfig.HypervisorConfig.IP = config.HypervisorIP
	testConfig.HypervisorConfig.User = config.SSHUser
	testConfig.HypervisorConfig.PrivateKeyPath = config.PrivateKey

	klog.V(2).Infof("Using hypervisor configuration from test context:")
	klog.V(2).Infof("  Hypervisor IP: %s", testConfig.HypervisorConfig.IP)
	klog.V(2).Infof("  SSH User: %s", testConfig.HypervisorConfig.User)
	klog.V(2).Infof("  Private Key Path: %s", testConfig.HypervisorConfig.PrivateKeyPath)

	// Validate that the private key file exists
	if _, err := os.Stat(testConfig.HypervisorConfig.PrivateKeyPath); os.IsNotExist(err) {
		o.Expect(err).To(o.BeNil(), "Private key file does not exist at path: %s", testConfig.HypervisorConfig.PrivateKeyPath)
	}

	knownHostsPath, err := utils.PrepareLocalKnownHostsFile(&testConfig.HypervisorConfig)
	o.Expect(err).To(o.BeNil(), "Expected to prepare local known hosts file without error")
	testConfig.HypervisorKnownHostsPath = knownHostsPath

	// Verify hypervisor connectivity and virsh availability
	err = utils.VerifyHypervisorAvailability(&testConfig.HypervisorConfig, testConfig.HypervisorKnownHostsPath)
	o.Expect(err).To(o.BeNil(), "Expected to verify hypervisor connectivity without error")

	// Set target and surviving node names dynamically (random selection)
	testConfig.TargetNodeName, testConfig.SurvivingNodeName = getRandomControlPlaneNode(oc)

	// Set dynamic resource names based on target node
	setDynamicResourceNames(testConfig, oc)

	// Get IP addresses for both nodes
	testConfig.TargetNodeIP, testConfig.SurvivingNodeIP = getNodeIPs(oc, testConfig.TargetNodeName, testConfig.SurvivingNodeName)

	// Prepare known hosts file for the surviving node
	// Note: We don't prepare the target node's known_hosts here because its SSH key will change
	// after reprovisioning. It will be prepared in restorePacemakerCluster after the node is ready.
	survivingNodeKnownHostsPath, err := utils.PrepareRemoteKnownHostsFile(testConfig.SurvivingNodeIP, &testConfig.HypervisorConfig, testConfig.HypervisorKnownHostsPath)
	o.Expect(err).To(o.BeNil(), "Expected to prepare surviving node known hosts file without error")
	testConfig.SurvivingNodeKnownHostsPath = survivingNodeKnownHostsPath

	klog.V(2).Infof("Target node for replacement: %s (IP: %s)", testConfig.TargetNodeName, testConfig.TargetNodeIP)
	klog.V(2).Infof("Surviving node: %s (IP: %s)", testConfig.SurvivingNodeName, testConfig.SurvivingNodeIP)
	klog.V(2).Infof("Target node MAC: %s", testConfig.TargetNodeMAC)
	klog.V(2).Infof("Target VM for replacement: %s", testConfig.TargetVMName)
	klog.V(2).Infof("Target machine name: %s", testConfig.TargetMachineName)

	klog.V(2).Infof("Test environment setup complete. Hypervisor IP: %s", testConfig.HypervisorConfig.IP)
	klog.V(4).Infof("setupTestEnvironment completed, testConfig.TargetVMName: %s", testConfig.TargetVMName)
}

// getRandomControlPlaneNode returns a random control plane node for replacement and the surviving node
func getRandomControlPlaneNode(oc *exutil.CLI) (string, string) {
	controlPlaneNodes, err := getNodes(oc, labelNodeRoleControlPlane)
	o.Expect(err).To(o.BeNil(), "Expected to get control plane nodes without error")

	// Ensure we have at least 2 control plane nodes
	o.Expect(len(controlPlaneNodes.Items)).To(o.BeNumerically(">=", 2), "Expected at least 2 control plane nodes for replacement test")

	// Select a random node using the same approach as other TNF recovery tests
	randomIndex := rand.Intn(len(controlPlaneNodes.Items))
	selectedNode := controlPlaneNodes.Items[randomIndex].Name

	// Validate that the selected node name is not empty
	o.Expect(selectedNode).ToNot(o.BeEmpty(), "Expected selected control plane node name to not be empty")

	// Find the surviving node (the other control plane node)
	var survivingNode string
	for i, node := range controlPlaneNodes.Items {
		if i != randomIndex {
			survivingNode = node.Name
			break
		}
	}

	// Validate that the surviving node name is not empty
	o.Expect(survivingNode).ToNot(o.BeEmpty(), "Expected surviving control plane node name to not be empty")

	klog.V(2).Infof("Randomly selected control plane node for replacement: %s (index: %d)", selectedNode, randomIndex)
	klog.V(2).Infof("Surviving control plane node: %s", survivingNode)

	return selectedNode, survivingNode
}

// getNodeIPs retrieves the IP addresses for the target and surviving nodes
func getNodeIPs(oc *exutil.CLI, targetNodeName, survivingNodeName string) (string, string) {
	// Get target node IP
	targetNodeIP, err := getNodeInternalIP(oc, targetNodeName)
	o.Expect(err).To(o.BeNil(), "Expected to get target node IP without error")
	o.Expect(targetNodeIP).ToNot(o.BeEmpty(), "Expected target node IP to not be empty")

	// Get surviving node IP
	survivingNodeIP, err := getNodeInternalIP(oc, survivingNodeName)
	o.Expect(err).To(o.BeNil(), "Expected to get surviving node IP without error")
	o.Expect(survivingNodeIP).ToNot(o.BeEmpty(), "Expected surviving node IP to not be empty")

	klog.V(2).Infof("Target node %s IP: %s", targetNodeName, targetNodeIP)
	klog.V(2).Infof("Surviving node %s IP: %s", survivingNodeName, survivingNodeIP)

	return targetNodeIP, survivingNodeIP
}

// getNodeInternalIP gets the internal IP address of a node
func getNodeInternalIP(oc *exutil.CLI, nodeName string) (string, error) {
	// Get node details in wide format to see IP addresses
	nodeOutput, err := oc.AsAdmin().Run("get").Args("node", nodeName, "-o", "wide").Output()
	if err != nil {
		return "", fmt.Errorf("failed to get node %s details: %v", nodeName, err)
	}

	// Parse the output to extract the internal IP
	lines := strings.Split(nodeOutput, "\n")
	for _, line := range lines {
		// Skip header line
		if strings.Contains(line, "NAME") && strings.Contains(line, "INTERNAL-IP") {
			continue
		}

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Split by whitespace and get the INTERNAL-IP column (usually the 6th column)
		fields := strings.Fields(line)
		if len(fields) >= 6 {
			internalIP := fields[5] // INTERNAL-IP is typically the 6th column
			// Validate that it looks like an IP address
			if strings.Contains(internalIP, ".") && len(internalIP) > 7 {
				return internalIP, nil
			}
		}
	}

	return "", fmt.Errorf("could not find internal IP for node %s in output: %s", nodeName, nodeOutput)
}

// printHypervisorConfigUsage prints a detailed usage message for hypervisor configuration
func printHypervisorConfigUsage() {
	usageMessage := `
================================================================================
TNF Node Replacement Test - Missing Hypervisor Configuration
================================================================================

This test requires hypervisor SSH configuration to perform node replacement
operations. Please provide the configuration using the --with-hypervisor-json flag:

Example:
openshift-tests run openshift/two-node --with-hypervisor-json='{
  "IP": "192.168.111.1",
  "User": "root",
  "privateKey": "/path/to/private/key"
}'

Configuration Details:
- IP: IP address of the hypervisor host for SSH access
- User: Username for SSH connection (typically "root")
- privateKey: Local file path to the SSH private key

The test will use this configuration to:
- SSH into the hypervisor to manage VMs
- Perform node replacement operations
- Recover from node failures

Environment Variable Alternative:
You can also set the HYPERVISOR_CONFIG environment variable:
export HYPERVISOR_CONFIG='{"IP":"192.168.111.1","User":"root","privateKey":"/path/to/key"}'

For more information, see the test documentation or contact the test team.
================================================================================
`
	g.GinkgoT().Logf(usageMessage)
}

// extractMachineNameFromBMH extracts the machine name from BareMetalHost's consumerRef
func extractMachineNameFromBMH(oc *exutil.CLI, nodeName string) string {
	// Find the BareMetalHost name using regex pattern matching
	bmhName := findObjectByNamePattern(oc, bmhResourceType, machineAPINamespace, nodeName, "")

	// Get the BareMetalHost YAML to extract the machine name
	bmhOutput, err := kubectlGetResource(oc, bmhResourceType, bmhName, machineAPINamespace, yamlOutputFormat)
	o.Expect(err).To(o.BeNil(), "Expected to get BareMetalHost without error")

	// Parse the YAML into a BareMetalHost object
	var bmh metal3v1alpha1.BareMetalHost
	decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(bmhOutput), 4096)
	err = decoder.Decode(&bmh)
	o.Expect(err).To(o.BeNil(), "Expected to parse BareMetalHost YAML without error")

	// Extract the machine name from consumerRef
	o.Expect(bmh.Spec.ConsumerRef).ToNot(o.BeNil(), "Expected BareMetalHost to have a consumerRef")
	o.Expect(bmh.Spec.ConsumerRef.Name).ToNot(o.BeEmpty(), "Expected consumerRef to have a name")

	machineName := bmh.Spec.ConsumerRef.Name
	klog.V(2).Infof("Found machine name: %s", machineName)
	return machineName
}

// kubectlGetResource is a utility function to get Kubernetes resources
func kubectlGetResource(oc *exutil.CLI, resourceType, name, namespace, outputFormat string) (string, error) {
	args := []string{resourceType}
	if name != "" {
		args = append(args, name)
	}
	if namespace != "" {
		args = append(args, "-n", namespace)
	}
	if outputFormat != "" {
		args = append(args, "-o", outputFormat)
	}
	return oc.AsAdmin().Run("get").Args(args...).Output()
}

// kubectlCreateResource is a utility function to create Kubernetes resources from file
func kubectlCreateResource(oc *exutil.CLI, filePath string) error {
	_, err := oc.AsAdmin().Run("create").Args("-f", filePath).Output()
	return err
}

// recoverClusterFromBackup attempts to recover the cluster from backup if the test fails
func recoverClusterFromBackup(testConfig *TNFTestConfig, oc *exutil.CLI) {
	klog.V(2).Infof("Starting cluster recovery from backup directory: %s", testConfig.GlobalBackupDir)

	defer func() {
		if r := recover(); r != nil {
			klog.Errorf("Recovery failed with panic: %v", r)
		}
		// Clean up backup directory after recovery attempt
		if testConfig.GlobalBackupDir != "" {
			os.RemoveAll(testConfig.GlobalBackupDir)
			testConfig.GlobalBackupDir = ""
		}
	}()

	// Step 1: Recreate the VM from backup
	klog.V(2).Infof("Step 1: Recreating VM from backup")
	if err := recoverVMFromBackup(testConfig); err != nil {
		klog.Errorf("Failed to recover VM: %v", err)
		return
	}

	// Wait for VM to start
	klog.V(2).Infof("Waiting for VM to start...")
	time.Sleep(3 * time.Minute)

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

	// Step 5: Re-enable stonith on the surviving node
	klog.V(2).Infof("Step 5: Re-enabling stonith on the surviving node")
	if err := reenableStonith(testConfig); err != nil {
		klog.Warningf("Failed to re-enable stonith: %v", err)
		// Don't return here, continue with recovery as this is not critical
	}

	// Step 6: Approve CSRs only if we attempted node provisioning
	if testConfig.HasAttemptedNodeProvisioning {
		klog.V(2).Infof("Step 6: Approving CSRs for cluster recovery (node provisioning was attempted)")
		go func() {
			// Run CSR approval in background for 5 minutes
			timeout := time.After(5 * time.Minute)
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-timeout:
					return
				case <-ticker.C:
					approveAnyPendingCSRs(oc)
				}
			}
		}()

		klog.V(2).Infof("Cluster recovery initiated with CSR approval. Monitoring for 5 minutes...")
		time.Sleep(5 * time.Minute)
	} else {
		klog.V(2).Infof("Step 6: Skipping CSR approval (no node provisioning was attempted)")
	}

	klog.V(2).Infof("Cluster recovery process completed")
}

// recoverVMFromBackup recreates the VM from the backed up XML
func recoverVMFromBackup(testConfig *TNFTestConfig) error {
	// Check if the specific VM already exists
	_, err := utils.VirshVMExists(testConfig.TargetVMName, &testConfig.HypervisorConfig, testConfig.HypervisorKnownHostsPath)
	if err == nil {
		klog.V(2).Infof("VM %s already exists, skipping recreation", testConfig.TargetVMName)
		return nil
	}

	o.Expect(testConfig.TargetVMName).ToNot(o.BeEmpty(), "Expected testConfig.TargetVMName to be set before recreating VM")
	// Read the backed up XML
	xmlFile := filepath.Join(testConfig.GlobalBackupDir, testConfig.TargetVMName+".xml")
	xmlContent, err := os.ReadFile(xmlFile)
	if err != nil {
		return fmt.Errorf("failed to read XML backup: %v", err)
	}

	// Create a temporary file on the hypervisor with the XML content
	createXMLCommand := fmt.Sprintf(`cat > /tmp/%s.xml <<'XML_EOF'
%s
XML_EOF`, testConfig.TargetVMName, string(xmlContent))

	_, _, err = utils.ExecuteSSHCommand(createXMLCommand, &testConfig.HypervisorConfig, testConfig.HypervisorKnownHostsPath)
	if err != nil {
		return fmt.Errorf("failed to create XML file on hypervisor: %v", err)
	}

	// Redefine the VM using the backed up XML
	err = utils.VirshDefineVM(fmt.Sprintf("/tmp/%s.xml", testConfig.TargetVMName), &testConfig.HypervisorConfig, testConfig.HypervisorKnownHostsPath)
	if err != nil {
		return fmt.Errorf("failed to define VM: %v", err)
	}

	// Start the VM
	err = utils.VirshStartVM(testConfig.TargetVMName, &testConfig.HypervisorConfig, testConfig.HypervisorKnownHostsPath)
	if err != nil {
		return fmt.Errorf("failed to start VM: %v", err)
	}

	// Enable autostart
	err = utils.VirshAutostartVM(testConfig.TargetVMName, &testConfig.HypervisorConfig, testConfig.HypervisorKnownHostsPath)
	if err != nil {
		klog.Warningf("Failed to enable autostart for VM: %v", err)
	}

	// Clean up temporary XML file
	_, _, err = utils.ExecuteSSHCommand(fmt.Sprintf("rm -f /tmp/%s.xml", testConfig.TargetVMName), &testConfig.HypervisorConfig, testConfig.HypervisorKnownHostsPath)
	if err != nil {
		klog.Warningf("Failed to clean up temporary XML file: %v", err)
	}

	klog.V(2).Infof("Recreated VM: %s", testConfig.TargetVMName)
	return nil
}

// retryRecoveryOperation retries a recovery operation until it succeeds or times out
// This is needed because etcd learner promotion can cause intermittent API failures
func retryRecoveryOperation(operation func() error, operationName string) error {
	maxRetries := 10
	retryInterval := 30 * time.Second
	timeout := 5 * time.Minute

	startTime := time.Now()

	for i := 0; i < maxRetries && time.Since(startTime) < timeout; i++ {
		err := operation()
		if err == nil {
			klog.V(2).Infof("Recovery operation %s succeeded on attempt %d after %v", operationName, i+1, time.Since(startTime))
			return nil
		}

		// Check if this is an etcd learner error that we should retry
		if isEtcdLearnerError(err) {
			klog.V(2).Infof("Recovery operation %s failed on attempt %d due to etcd learner error (will retry): %v", operationName, i+1, err)
		} else {
			klog.V(2).Infof("Recovery operation %s failed on attempt %d with non-retryable error: %v", operationName, i+1, err)
			return err // Don't retry non-etcd learner errors
		}

		if i < maxRetries-1 && time.Since(startTime) < timeout {
			klog.V(2).Infof("Retrying recovery operation %s in %v...", operationName, retryInterval)
			time.Sleep(retryInterval)
		}
	}

	return fmt.Errorf("recovery operation %s failed after %d attempts over %v", operationName, maxRetries, time.Since(startTime))
}

// isEtcdLearnerError checks if an error is related to etcd learner restrictions
func isEtcdLearnerError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	// Common etcd learner error patterns
	learnerErrorPatterns := []string{
		"rpc error: code = Unavailable",
		"etcdserver: too many requests",
		"etcdserver: request timed out",
		"context deadline exceeded",
		"connection refused",
		"learner",
		"not a voter",
		"raft: not leader",
	}

	for _, pattern := range learnerErrorPatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	return false
}

// retryOperationWithTimeout retries an operation until it succeeds or times out
func retryOperationWithTimeout(operation func() error, timeout, pollInterval time.Duration, operationName string) error {
	startTime := time.Now()

	for time.Since(startTime) < timeout {
		err := operation()
		if err == nil {
			klog.V(2).Infof("Operation %s succeeded after %v", operationName, time.Since(startTime))
			return nil
		}

		klog.V(2).Infof("Operation %s failed, retrying in %v: %v", operationName, pollInterval, err)
		time.Sleep(pollInterval)
	}

	return fmt.Errorf("operation %s failed after %v timeout", operationName, timeout)
}

// recoverEtcdSecretsFromBackup recreates etcd secrets from backup with retry logic
func recoverEtcdSecretsFromBackup(testConfig *TNFTestConfig, oc *exutil.CLI) error {
	etcdSecrets := []string{
		testConfig.EtcdPeerSecretName,
		testConfig.EtcdServingSecretName,
		testConfig.EtcdServingMetricsSecretName,
	}

	for _, secretName := range etcdSecrets {
		secretFile := filepath.Join(testConfig.GlobalBackupDir, secretName+".yaml")
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
		err = retryRecoveryOperation(func() error {
			return kubectlCreateResource(oc, secretFile)
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
	machineFile := filepath.Join(testConfig.GlobalBackupDir, testConfig.TargetMachineName+"-machine.yaml")

	// Check if Machine already exists
	_, err = oc.AsAdmin().Run("get").Args(machineResourceType, testConfig.TargetMachineName, "-n", machineAPINamespace).Output()
	if err != nil {
		// Retry Machine creation
		err = retryRecoveryOperation(func() error {
			return kubectlCreateResource(oc, machineFile)
		}, fmt.Sprintf("create Machine %s", testConfig.TargetMachineName))

		if err != nil {
			return fmt.Errorf("failed to recreate Machine after retries: %v", err)
		}
		klog.V(2).Infof("Recreated Machine: %s", testConfig.TargetMachineName)
	} else {
		klog.V(2).Infof("Machine %s already exists, skipping recreation", testConfig.TargetMachineName)
	}

	return nil
}

// approveAnyPendingCSRs approves any pending CSRs found in the cluster with retry logic
func approveAnyPendingCSRs(oc *exutil.CLI) {
	// Get pending CSRs with retry
	var csrOutput string
	err := retryRecoveryOperation(func() error {
		var err error
		csrOutput, err = oc.AsAdmin().Run("get").Args("csr", "-o", "json").Output()
		return err
	}, "get pending CSRs")

	if err != nil {
		klog.Warningf("Failed to get CSRs after retries: %v", err)
		return
	}

	// Extract CSR names that need approval (status is empty)
	pendingCSRs := []string{}
	lines := strings.Split(csrOutput, "\n")
	for _, line := range lines {
		if strings.Contains(line, "\"name\"") && strings.Contains(line, "\"status\": {}") {
			// Extract CSR name from JSON
			start := strings.Index(line, "\"name\": \"") + 8
			end := strings.Index(line[start:], "\"") + start
			if start > 7 && end > start {
				csrName := line[start:end]
				pendingCSRs = append(pendingCSRs, csrName)
			}
		}
	}

	// Approve pending CSRs with retry
	for _, csrName := range pendingCSRs {
		klog.V(2).Infof("Approving CSR during recovery: %s", csrName)

		err = retryRecoveryOperation(func() error {
			_, err := oc.AsAdmin().Run("adm").Args("certificate", "approve", csrName).Output()
			return err
		}, fmt.Sprintf("approve CSR %s", csrName))

		if err == nil {
			klog.V(2).Infof("Approved CSR during recovery: %s", csrName)
		} else {
			klog.Warningf("Failed to approve CSR %s after retries: %v", csrName, err)
		}
	}
}

// waitForPacemakerQuorum waits for pacemaker to restore quorum after node deletion
func waitForPacemakerQuorum() error {
	klog.V(2).Infof("Waiting for API server to restore quorum (checking with oc status)...")

	return retryOperationWithTimeout(func() error {
		// Check if the API server is accessible by running a simple oc command
		// If we have quorum, the API server will be working. If not, the request will timeout.
		_, err := oc.AsAdmin().Run("status").Args().Output()
		if err != nil {
			return fmt.Errorf("API server not yet accessible (no quorum): %v", err)
		}

		klog.V(4).Infof("API server is accessible - quorum restored")
		return nil
	}, pacemakerQuorumTimeout, pacemakerQuorumPollInterval, "API server quorum restoration")
}

// promoteEtcdLearnerMember promotes the etcd learner member to voter status
func promoteEtcdLearnerMember(testConfig *TNFTestConfig) error {
	klog.V(2).Infof("Attempting to promote etcd learner member on surviving node: %s (IP: %s)", testConfig.SurvivingNodeName, testConfig.SurvivingNodeIP)

	return retryOperationWithTimeout(func() error {
		// First, get the list of etcd members to find the learner
		memberListCmd := `sudo podman exec -it etcd etcdctl member list -w json`
		output, _, err := utils.ExecuteRemoteSSHCommand(testConfig.SurvivingNodeIP, memberListCmd, &testConfig.HypervisorConfig, testConfig.HypervisorKnownHostsPath, testConfig.SurvivingNodeKnownHostsPath)
		if err != nil {
			return fmt.Errorf("failed to get etcd member list on %s: %v", testConfig.SurvivingNodeIP, err)
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
		promoteOutput, _, err := utils.ExecuteRemoteSSHCommand(testConfig.SurvivingNodeIP, promoteCmd, &testConfig.HypervisorConfig, testConfig.HypervisorKnownHostsPath, testConfig.SurvivingNodeKnownHostsPath)
		if err != nil {
			return fmt.Errorf("failed to promote etcd learner member %s on %s: %v, output: %s", learnerMemberID, testConfig.SurvivingNodeIP, err, promoteOutput)
		}

		klog.V(4).Infof("Successfully promoted etcd learner member %s: %s", learnerMemberID, promoteOutput)
		return nil
	}, 10*time.Minute, 30*time.Second, "promote etcd learner member")
}

// findLearnerMemberID parses the etcd member list JSON output to find the learner member ID and name
func findLearnerMemberID(memberListJSON string) (string, string, error) {
	// Parse the JSON output
	var memberList etcdMemberListResponse
	err := json.Unmarshal([]byte(memberListJSON), &memberList)
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

// Missing functions that need to be implemented
func recreateBMCSecret(testConfig *TNFTestConfig, oc *exutil.CLI) error {
		// Recreate BMC secret with retry
		bmcSecretFile := filepath.Join(testConfig.GlobalBackupDir, testConfig.TargetBMCSecretName+".yaml")

		// Check if BMC secret already exists
		_, err := oc.AsAdmin().Run("get").Args(secretResourceType, testConfig.TargetBMCSecretName, "-n", machineAPINamespace).Output()
		if err != nil {
			// Retry BMC secret creation
			err = retryRecoveryOperation(func() error {
				return kubectlCreateResource(oc, bmcSecretFile)
			}, fmt.Sprintf("create BMC secret %s", testConfig.TargetBMCSecretName))

			if err != nil {
				return fmt.Errorf("failed to recreate BMC secret after retries: %v", err)
			}
			klog.V(2).Infof("Recreated BMC secret: %s", testConfig.TargetBMCSecretName)
		} else {
			klog.V(2).Infof("BMC secret %s already exists, skipping recreation", testConfig.TargetBMCSecretName)
		}

		return nil
}

func updateAndCreateBMH(testConfig *TNFTestConfig, oc *exutil.CLI, newUUID, newMACAddress string) {
	klog.V(2).Infof("Creating BareMetalHost with UUID: %s, MAC: %s", newUUID, newMACAddress)

	// Read the BMH template from testdata
	templatePath := filepath.Join("test", "extended", "testdata", "two_node", "baremetalhost-template.yaml")
	templateContent, err := os.ReadFile(templatePath)
	o.Expect(err).To(o.BeNil(), "Expected to read BMH template without error")

	// Replace placeholders with actual values
	bmhContent := string(templateContent)
	bmhContent = strings.ReplaceAll(bmhContent, "{NAME}", testConfig.TargetBMHName)
	bmhContent = strings.ReplaceAll(bmhContent, "{IP}", testConfig.TargetNodeIP)
	bmhContent = strings.ReplaceAll(bmhContent, "{UUID}", newUUID)
	bmhContent = strings.ReplaceAll(bmhContent, "{CREDENTIALS_NAME}", testConfig.TargetBMCSecretName)
	bmhContent = strings.ReplaceAll(bmhContent, "{BOOT_MAC_ADDRESS}", newMACAddress)

	// Create a temporary file with the updated BMH content
	tmpFile, err := os.CreateTemp("", "bmh-*.yaml")
	o.Expect(err).To(o.BeNil(), "Expected to create temporary BMH file without error")
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(bmhContent)
	o.Expect(err).To(o.BeNil(), "Expected to write BMH content to temporary file without error")
	tmpFile.Close()

	// Create the BareMetalHost using oc create
	_, err = oc.AsAdmin().Run("create").Args("-f", tmpFile.Name()).Output()
	o.Expect(err).To(o.BeNil(), "Expected to create BareMetalHost without error")

	klog.V(2).Infof("Successfully created BareMetalHost: %s", testConfig.TargetBMHName)
}

func waitForBMHProvisioning(testConfig *TNFTestConfig, oc *exutil.CLI) {
	klog.V(2).Infof("Waiting for BareMetalHost %s to be provisioned...", testConfig.TargetBMHName)

	maxWaitTime := 15 * time.Minute
	pollInterval := 30 * time.Second
	startTime := time.Now()

	for time.Since(startTime) < maxWaitTime {
		// Get the specific BareMetalHost in YAML format
		bmhOutput, err := oc.AsAdmin().Run("get").Args(bmhResourceType, testConfig.TargetBMHName, "-n", machineAPINamespace, "-o", yamlOutputFormat).Output()
		if err != nil {
			klog.V(2).Infof("Error getting BareMetalHost %s: %v", testConfig.TargetBMHName, err)
			time.Sleep(pollInterval)
			continue
		}

		// Parse the YAML into a BareMetalHost object
		var bmh metal3v1alpha1.BareMetalHost
		decoder := yaml.NewYAMLOrJSONDecoder(strings.NewReader(bmhOutput), 4096)
		err = decoder.Decode(&bmh)
		if err != nil {
			klog.V(2).Infof("Error parsing BareMetalHost YAML: %v", err)
			time.Sleep(pollInterval)
			continue
		}

		// Check the provisioning state
		currentState := string(bmh.Status.Provisioning.State)
		klog.V(4).Infof("BareMetalHost %s current state: %s", testConfig.TargetBMHName, currentState)

		// Check if BMH is in provisioned state
		if currentState == bmhProvisionedState {
			klog.V(2).Infof("BareMetalHost %s is provisioned", testConfig.TargetBMHName)
			return
		}

		// Log additional status information
		if bmh.Status.ErrorMessage != "" {
			klog.V(2).Infof("BareMetalHost %s error message: %s", testConfig.TargetBMHName, bmh.Status.ErrorMessage)
		}

		klog.V(2).Infof("Waiting for BareMetalHost %s provisioning (current state: %s, elapsed: %v)",
			testConfig.TargetBMHName, currentState, time.Since(startTime))
		time.Sleep(pollInterval)
	}

	// If we reach here, the timeout was exceeded
	o.Expect(false).To(o.BeTrue(), fmt.Sprintf("BareMetalHost %s did not reach provisioned state within %v timeout", testConfig.TargetBMHName, maxWaitTime))
}

func reapplyDetachedAnnotation(testConfig *TNFTestConfig, oc *exutil.CLI) {
	klog.V(2).Infof("Applying detached annotation to BareMetalHost: %s", testConfig.TargetBMHName)

	// Apply the detached annotation to the specific BMH
	_, err := oc.AsAdmin().Run("annotate").Args(
		bmhResourceType, testConfig.TargetBMHName,
		"-n", machineAPINamespace,
		"baremetalhost.metal3.io/detached=true",
		"--overwrite",
	).Output()
	o.Expect(err).To(o.BeNil(), "Expected to apply detached annotation to BMH %s without error", testConfig.TargetBMHName)

	klog.V(2).Infof("Successfully applied detached annotation to BareMetalHost: %s", testConfig.TargetBMHName)
}

func recreateMachine(testConfig *TNFTestConfig, oc *exutil.CLI) {
	klog.V(2).Infof("Recreating Machine: %s", testConfig.TargetMachineName)

	// Check if the machine already exists
	_, err := oc.AsAdmin().Run("get").Args(machineResourceType, testConfig.TargetMachineName, "-n", machineAPINamespace).Output()
	if err == nil {
		klog.V(2).Infof("Machine %s already exists, skipping recreation", testConfig.TargetMachineName)
		return
	}

	// Read the Machine template from testdata
	templatePath := filepath.Join("test", "extended", "testdata", "two_node", "machine-template.yaml")
	templateContent, err := os.ReadFile(templatePath)
	o.Expect(err).To(o.BeNil(), "Expected to read Machine template without error")

	// Replace placeholders with actual values
	machineContent := string(templateContent)
	machineContent = strings.ReplaceAll(machineContent, "{NODE_NAME}", testConfig.TargetBMHName)
	machineContent = strings.ReplaceAll(machineContent, "{MACHINE_NAME}", testConfig.TargetMachineName)
	machineContent = strings.ReplaceAll(machineContent, "{MACHINE_HASH}", testConfig.TargetMachineHash)

	// Create a temporary file with the updated Machine content
	tmpFile, err := os.CreateTemp("", "machine-*.yaml")
	o.Expect(err).To(o.BeNil(), "Expected to create temporary Machine file without error")
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(machineContent)
	o.Expect(err).To(o.BeNil(), "Expected to write Machine content to temporary file without error")
	tmpFile.Close()

	// Create the Machine using oc create
	_, err = oc.AsAdmin().Run("create").Args("-f", tmpFile.Name()).Output()
	o.Expect(err).To(o.BeNil(), "Expected to create Machine without error")

	klog.V(2).Infof("Successfully recreated Machine: %s", testConfig.TargetMachineName)
}

func setDynamicResourceNames(testConfig *TNFTestConfig, oc *exutil.CLI) {
	// Set dynamic resource names based on target node
	testConfig.EtcdPeerSecretName = fmt.Sprintf("%s-%s", etcdPeerSecretBaseName, testConfig.TargetNodeName)
	testConfig.EtcdServingSecretName = fmt.Sprintf("%s-%s", etcdServingSecretBaseName, testConfig.TargetNodeName)
	testConfig.EtcdServingMetricsSecretName = fmt.Sprintf("%s-%s", etcdServingMetricsSecretBaseName, testConfig.TargetNodeName)
	testConfig.TNFAuthJobName = fmt.Sprintf("%s-%s", tnfAuthJobBaseName, testConfig.TargetNodeName)
	testConfig.TNFAfterSetupJobName = fmt.Sprintf("%s-%s", tnfAfterSetupJobBaseName, testConfig.TargetNodeName)
	testConfig.TargetBMCSecretName = findObjectByNamePattern(oc, secretResourceType, machineAPINamespace, testConfig.TargetNodeName, "bmc-secret")
	testConfig.TargetBMHName = findObjectByNamePattern(oc, bmhResourceType, machineAPINamespace, testConfig.TargetNodeName, "")

	// Get the MAC address of the target node from its BareMetalHost
	testConfig.TargetNodeMAC = getNodeMACAddress(oc, testConfig.TargetNodeName)
	klog.V(4).Infof("Found targetNodeMAC: %s for node: %s", testConfig.TargetNodeMAC, testConfig.TargetNodeName)

	// Find the corresponding VM name by matching MAC addresses
	var err error
	testConfig.TargetVMName, err = utils.GetVMNameByMACMatch(testConfig.TargetNodeName, testConfig.TargetNodeMAC, virshProvisioningBridge, &testConfig.HypervisorConfig, testConfig.HypervisorKnownHostsPath)
	klog.V(4).Infof("GetVMNameByMACMatch returned: testConfig.TargetVMName=%s, err=%v", testConfig.TargetVMName, err)
	o.Expect(err).To(o.BeNil(), "Expected to find VM name for node %s with MAC %s: %v", testConfig.TargetNodeName, testConfig.TargetNodeMAC, err)

	// Ensure we found a valid VM name
	o.Expect(testConfig.TargetVMName).ToNot(o.BeEmpty(), "Expected to find a valid VM name for node %s with MAC %s", testConfig.TargetNodeName, testConfig.TargetNodeMAC)

	// Extract and store the machine name from the BMH consumerRef
	testConfig.TargetMachineName = extractMachineNameFromBMH(oc, testConfig.TargetNodeName)

	// Extract the machine hash from the machine name
	// Machine name format: {cluster}-{hash}-{role}-{index} (e.g., "ostest-abc123-master-0")
	machineNameParts := strings.Split(testConfig.TargetMachineName, "-")
	if len(machineNameParts) >= 4 {
		testConfig.TargetMachineHash = machineNameParts[1]
		klog.V(2).Infof("Extracted machine hash: %s from machine name: %s", testConfig.TargetMachineHash, testConfig.TargetMachineName)
	} else {
		klog.Warningf("Unable to extract machine hash from machine name: %s (unexpected format)", testConfig.TargetMachineName)
	}
}

// waitForEtcdToStop waits for etcd to stop on the surviving node
func waitForEtcdToStop(testConfig *TNFTestConfig) error {
	klog.V(2).Infof("Waiting for etcd to stop on surviving node: %s", testConfig.SurvivingNodeName)

	return retryOperationWithTimeout(func() error {
		// Check etcd resource status on the surviving node
		output, _, err := utils.PcsResourceStatus(testConfig.SurvivingNodeName, testConfig.SurvivingNodeIP, &testConfig.HypervisorConfig, testConfig.HypervisorKnownHostsPath, testConfig.SurvivingNodeKnownHostsPath)
		if err != nil {
			return fmt.Errorf("failed to get etcd resource status on %s: %v, output: %s", testConfig.SurvivingNodeName, err, output)
		}

		klog.V(4).Infof("Etcd resource status on %s:\n%s", testConfig.SurvivingNodeName, output)

		// Check if etcd is stopped (not started) on the surviving node
		// We expect to see "Stopped: [ master-X ]" or no "Started:" line for the survivor
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.Contains(line, "Started:") && strings.Contains(line, testConfig.SurvivingNodeName) {
				return fmt.Errorf("etcd is still started on surviving node %s", testConfig.SurvivingNodeName)
			}
		}

		// If we get here, etcd is not started on the surviving node
		klog.V(2).Infof("Etcd has stopped on surviving node: %s", testConfig.SurvivingNodeName)
		return nil
	}, etcdStatusCheckTimeout, etcdStatusCheckPollInterval, fmt.Sprintf("etcd stop on %s", testConfig.SurvivingNodeName))
}

// waitForEtcdToStart waits for etcd to start on the surviving node
func waitForEtcdToStart(testConfig *TNFTestConfig) error {
	klog.V(2).Infof("Waiting for etcd to start on surviving node: %s", testConfig.SurvivingNodeName)

	return retryOperationWithTimeout(func() error {
		// Check etcd resource status on the surviving node
		output, _, err := utils.PcsResourceStatus(testConfig.SurvivingNodeName, testConfig.SurvivingNodeIP, &testConfig.HypervisorConfig, testConfig.HypervisorKnownHostsPath, testConfig.SurvivingNodeKnownHostsPath)
		if err != nil {
			return fmt.Errorf("failed to get etcd resource status on %s: %v, output: %s", testConfig.SurvivingNodeName, err, output)
		}

		klog.V(4).Infof("Etcd resource status on %s:\n%s", testConfig.SurvivingNodeName, output)

		// Check if etcd is started on the surviving node
		// We expect to see "Started: [ master-X ]" for the survivor
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.Contains(line, "Started:") && strings.Contains(line, testConfig.SurvivingNodeName) {
				klog.V(2).Infof("Etcd has started on surviving node: %s", testConfig.SurvivingNodeName)
				return nil
			}
		}

		// If we get here, etcd is not started on the surviving node
		// Get pacemaker journal logs to help with debugging
		klog.V(2).Infof("Etcd is not started on %s, getting pacemaker journal logs for debugging", testConfig.SurvivingNodeName)
		journalOutput, _, journalErr := utils.PcsJournal(25, testConfig.SurvivingNodeIP, &testConfig.HypervisorConfig, testConfig.HypervisorKnownHostsPath, testConfig.SurvivingNodeKnownHostsPath)
		if journalErr != nil {
			klog.Warningf("Failed to get pacemaker journal logs on %s: %v", testConfig.SurvivingNodeName, journalErr)
		} else {
			klog.V(4).Infof("Last 20 lines of pacemaker journal on %s:\n%s", testConfig.SurvivingNodeName, journalOutput)
		}

		return fmt.Errorf("etcd is not started on surviving node %s", testConfig.SurvivingNodeName)
	}, etcdStatusCheckTimeout, etcdStatusCheckPollInterval, fmt.Sprintf("etcd start on %s", testConfig.SurvivingNodeName))
}

// reenableStonith re-enables stonith on the surviving node
func reenableStonith(testConfig *TNFTestConfig) error {
	klog.V(2).Infof("Re-enabling stonith on surviving node: %s", testConfig.SurvivingNodeName)

	// Execute the stonith enable command on the surviving node
	output, _, err := utils.PcsEnableStonith(testConfig.SurvivingNodeIP, &testConfig.HypervisorConfig, testConfig.HypervisorKnownHostsPath, testConfig.SurvivingNodeKnownHostsPath)
	if err != nil {
		return fmt.Errorf("failed to re-enable stonith on %s: %v, output: %s", testConfig.SurvivingNodeName, err, output)
	}

	klog.V(2).Infof("Successfully re-enabled stonith on surviving node: %s", testConfig.SurvivingNodeName)
	klog.V(4).Infof("Stonith enable output: %s", output)
	return nil
}
package two_node

import (
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
)

// Constants
const (
	backupDirName = "tnf-node-replacement-backup"

	// Network configuration
	ostestbmNetwork = "ostestbm" // Network name for IP matching

	// OpenShift namespaces
	machineAPINamespace = "openshift-machine-api"
	etcdNamespace = "openshift-etcd"

	// Debug configuration
	debugNamespace = "default"

	// Timeouts and intervals
	nodeRecoveryTimeout = 10 * time.Minute
	nodeRecoveryPollInterval = 15 * time.Second
	bmhProvisioningTimeout = 10 * time.Minute
	bmhProvisioningPollInterval = 30 * time.Second
	csrApprovalTimeout = 5 * time.Minute
	csrApprovalPollInterval = 10 * time.Second
	clusterOperatorTimeout = 5 * time.Minute
	clusterOperatorPollInterval = 10 * time.Second
	vmStartTimeout = 2 * time.Minute
	vmStartPollInterval = 15 * time.Second
	recoveryOperationTimeout = 2 * time.Minute
	recoveryOperationPollInterval = 10 * time.Second
	etcdStopWaitTime = 30 * time.Second
	etcdStatusCheckTimeout = 2 * time.Minute
	etcdStatusCheckPollInterval = 5 * time.Second

	// Expected counts
	expectedCSRCount = 2

	// Network configuration
	etcdPort = "2379"

	// File permissions
	etcdDirPermissions = 0766
	etcdFilePermissions = 0644

	// Resource types
	secretResourceType = "secret"
	bmhResourceType = "bmh"
	machineResourceType = "machines.machine.openshift.io"
	nodeResourceType = "node"
	csrResourceType = "csr"
	coResourceType = "co"
	jobResourceType = "job"

	// Output formats
	yamlOutputFormat = "yaml"
	jsonOutputFormat = "json"
	nameOutputFormat = "name"
	wideOutputFormat = "wide"

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

	// File paths
	tmpDirPrefix = "/tmp/"
	backupTempPrefix = "tnf-node-replacement-backup"

	// Revision JSON template
	revisionJSONTemplate = `{"clusterId":"0","raftIndex":{"https://%s:%s":0},"maxRaftIndex":0,"created":""}`

	// Directory creation commands
	mkdirEtcdDir = "sudo mkdir /var/lib/etcd"
	chmodEtcdDir = "sudo chmod %o /var/lib/etcd"
	teeRevisionJSON = "echo '%s' | sudo tee -a /var/lib/etcd/revision.json"
	chmodRevisionJSON = "sudo chmod %o /var/lib/etcd/revision.json"

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
		utils.CleanupKnownHostsFile(&testConfig.HypervisorConfig)
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

		// g.By("Recreating the target VM using backed up configuration")
		// recreateTargetVM(oc, backupDir)

		// g.By("Provisioning the target node with Ironic")
		// provisionTargetNodeWithIronic(oc, backupDir)

		// g.By("Approving certificate signing requests for the new node")
		// approveCSRs(oc)

		// g.By("Waiting for the replacement node to appear in the cluster")
		// waitForNodeRecovery(oc)

		// g.By("Restoring pacemaker cluster configuration")
		// restorePacemakerCluster(oc)

		// g.By("Verifying the cluster is fully restored")
		// verifyRestoredCluster(oc)

		// g.By("Successfully completed node replacement process")
		// g.GinkgoT().Logf("Node replacement process completed. Backup files created in: %s", backupDir)
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
			g.GinkgoT().Logf("Found %s: %s", resourceType, objectName)
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
	bmcSecretOutput, err := oc.AsAdmin().Run("get").Args("secret", testConfig.TargetBMCSecretName, "-n", machineAPINamespace, "-o", "yaml").Output()
	o.Expect(err).To(o.BeNil(), "Expected to get BMC secret without error")
	bmcSecretFile := filepath.Join(backupDir, testConfig.TargetBMCSecretName+".yaml")
	err = os.WriteFile(bmcSecretFile, []byte(bmcSecretOutput), 0644)
	o.Expect(err).To(o.BeNil(), "Expected to write BMC secret backup without error")

	// Download backup of BareMetalHost
	bmhOutput, err := oc.AsAdmin().Run("get").Args("bmh", testConfig.TargetBMHName, "-n", machineAPINamespace, "-o", "yaml").Output()
	o.Expect(err).To(o.BeNil(), "Expected to get BareMetalHost without error")
	bmhFile := filepath.Join(backupDir, testConfig.TargetBMHName+".yaml")
	err = os.WriteFile(bmhFile, []byte(bmhOutput), 0644)
	o.Expect(err).To(o.BeNil(), "Expected to write BareMetalHost backup without error")

	// Backup machine definition using the stored testConfig.TargetMachineName
	machineOutput, err := oc.AsAdmin().Run("get").Args(machineResourceType, testConfig.TargetMachineName, "-n", machineAPINamespace, "-o", "yaml").Output()
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
		secretOutput, err := oc.AsAdmin().Run("get").Args("secret", secretName, "-n", etcdNamespace, "-o", "yaml").Output()
		if err != nil {
			g.GinkgoT().Logf("Warning: Could not backup etcd secret %s: %v", secretName, err)
			continue
		}

		secretFile := filepath.Join(backupDir, secretName+".yaml")
		err = os.WriteFile(secretFile, []byte(secretOutput), 0644)
		o.Expect(err).To(o.BeNil(), "Expected to write etcd secret %s backup without error", secretName)
		g.GinkgoT().Logf("Backed up etcd secret: %s", secretName)
	}

	g.GinkgoT().Logf("[DEBUG] About to validate testConfig.TargetVMName, current value: %s", testConfig.TargetVMName)
	// Validate that testConfig.TargetVMName is set
	if testConfig.TargetVMName == "" {
		g.GinkgoT().Logf("testConfig.TargetVMName bytes: %v", []byte(testConfig.TargetVMName))
		g.GinkgoT().Logf("ERROR: testConfig.TargetVMName is empty! This should have been set in setupTestEnvironment")
		g.GinkgoT().Logf("testConfig.TargetNodeName: %s", testConfig.TargetNodeName)
		g.GinkgoT().Logf("testConfig.SurvivingNodeName: %s", testConfig.SurvivingNodeName)
		o.Expect(testConfig.TargetVMName).ToNot(o.BeEmpty(), "Expected testConfig.TargetVMName to be set before backing up VM configuration")
	}
	// Get XML dump of VM using SSH to hypervisor
	xmlOutput, err := utils.VirshDumpXML(testConfig.TargetVMName, &testConfig.HypervisorConfig)
	o.Expect(err).To(o.BeNil(), "Expected to get XML dump without error")

	xmlFile := filepath.Join(backupDir, testConfig.TargetVMName+".xml")
	err = os.WriteFile(xmlFile, []byte(xmlOutput), 0644)
	o.Expect(err).To(o.BeNil(), "Expected to write XML dump to file without error")

	return backupDir
}

// destroyVM destroys the target VM using SSH to hypervisor
func destroyVM(testConfig *TNFTestConfig) {
	o.Expect(testConfig.TargetVMName).ToNot(o.BeEmpty(), "Expected testConfig.TargetVMName to be set before destroying VM")
	g.GinkgoT().Logf("Destroying VM: %s", testConfig.TargetVMName)

	// Undefine and destroy VM using SSH to hypervisor
	err := utils.VirshUndefineVM(testConfig.TargetVMName, &testConfig.HypervisorConfig)
	o.Expect(err).To(o.BeNil(), "Expected to undefine VM without error")

	err = utils.VirshDestroyVM(testConfig.TargetVMName, &testConfig.HypervisorConfig)
	o.Expect(err).To(o.BeNil(), "Expected to destroy VM without error")

	g.GinkgoT().Logf("VM %s destroyed successfully", testConfig.TargetVMName)
}

// deleteNodeReferences deletes OpenShift resources related to the target node
func deleteNodeReferences(testConfig *TNFTestConfig, oc *exutil.CLI) {
	g.GinkgoT().Logf("Deleting OpenShift resources for node: %s", testConfig.TargetNodeName)

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

	g.GinkgoT().Logf("OpenShift resources for node %s deleted successfully", testConfig.TargetNodeName)
}

// restoreEtcdQuorumOnSurvivor restores etcd quorum on the surviving node
func restoreEtcdQuorumOnSurvivor(testConfig *TNFTestConfig, oc *exutil.CLI) {
	g.GinkgoT().Logf("Restoring etcd quorum on surviving node: %s", testConfig.SurvivingNodeName)

	// Wait 30 seconds after node deletion to allow etcd to stop naturally
	g.By("Waiting 30 seconds for etcd to stop naturally after node deletion")
	time.Sleep(etcdStopWaitTime)

	// Check that etcd has stopped on the survivor before proceeding
	g.By("Verifying that etcd has stopped on the surviving node")
	err := waitForEtcdToStop(testConfig)
	o.Expect(err).To(o.BeNil(), "Expected etcd to stop on surviving node %s within timeout", testConfig.SurvivingNodeName)

	// SSH to hypervisor, then to surviving node to run pcs debug-start
	// We need to chain the SSH commands: host -> hypervisor -> surviving node
	debugStart := fmt.Sprintf(`ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=%s core@%s "%s"`, utils.GetSurvivingNodeKnownHostsPath(), testConfig.SurvivingNodeIP, pcsDisableStonith)

	output, err := utils.ExecuteSSHCommand(debugStart, &testConfig.HypervisorConfig)
	if err != nil {
		o.Expect(err).To(o.BeNil(), fmt.Sprintf("Failed to restore etcd quorum on %s: %v, output: %s", testConfig.SurvivingNodeName, err, output))
	}

	// Verify that etcd has started on the survivor after debug-start
	g.By("Verifying that etcd has started on the surviving node after debug-start")
	err = waitForEtcdToStart(testConfig)
	o.Expect(err).To(o.BeNil(), "Expected etcd to start on surviving node %s within timeout", testConfig.SurvivingNodeName)

	// Log pacemaker status to check if etcd has been started on the survivor
	pcsStatusOutput, err := utils.ExecuteSSHCommand(fmt.Sprintf(`ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=%s core@%s "sudo pcs status"`, utils.GetSurvivingNodeKnownHostsPath(), testConfig.SurvivingNodeIP), &testConfig.HypervisorConfig)
	if err != nil {
		g.GinkgoT().Logf("Warning: Failed to get pacemaker status on survivor %s: %v", testConfig.SurvivingNodeIP, err)
	} else {
		g.GinkgoT().Logf("[DEBUG] Pacemaker status on survivor %s:\n%s", testConfig.SurvivingNodeIP, pcsStatusOutput)
	}

	g.GinkgoT().Logf("Successfully restored etcd quorum on surviving node: %s", testConfig.SurvivingNodeName)

	// Wait for pacemaker to restore quorum before proceeding with OpenShift API operations
	g.By("Waiting for pacemaker to restore quorum after VM destruction")
	output, err = monitorClusterOperators(oc)
	o.Expect(err).To(o.BeNil(), "Expected pacemaker to restore quorum within timeout")
	g.GinkgoT().Logf("Cluster operators status:\n%s", output)
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

	_, err = utils.ExecuteSSHCommand(createXMLCommand, &testConfig.HypervisorConfig)
	o.Expect(err).To(o.BeNil(), "Expected to create XML file on hypervisor without error")

	// Redefine the VM using the backed up XML
	err = utils.VirshDefineVM(fmt.Sprintf("/tmp/%s.xml", testConfig.TargetVMName), &testConfig.HypervisorConfig)
	o.Expect(err).To(o.BeNil(), "Expected to define VM without error")

	// Start the VM with autostart enabled
	err = utils.VirshStartVM(testConfig.TargetVMName, &testConfig.HypervisorConfig)
	o.Expect(err).To(o.BeNil(), "Expected to start VM without error")

	err = utils.VirshAutostartVM(testConfig.TargetVMName, &testConfig.HypervisorConfig)
	o.Expect(err).To(o.BeNil(), "Expected to enable autostart for VM without error")

	// Clean up temporary XML file
	_, err = utils.ExecuteSSHCommand(fmt.Sprintf("rm -f /tmp/%s.xml", testConfig.TargetVMName), &testConfig.HypervisorConfig)
	o.Expect(err).To(o.BeNil(), "Expected to clean up temporary XML file without error")
}

// provisionTargetNodeWithIronic handles the Ironic provisioning process
func provisionTargetNodeWithIronic(testConfig *TNFTestConfig, oc *exutil.CLI) {
	o.Expect(testConfig.TargetVMName).ToNot(o.BeEmpty(), "Expected testConfig.TargetVMName to be set before provisioning with Ironic")

	// Set flag to indicate we're attempting node provisioning
	testConfig.HasAttemptedNodeProvisioning = true

	recreateBMCSecret(oc, testConfig.GlobalBackupDir)
	newUUID, newMACAddress, err := utils.GetVMNetworkInfo(testConfig.TargetVMName, virshProvisioningBridge, &testConfig.HypervisorConfig)
	o.Expect(err).To(o.BeNil(), "Expected to get VM network info: %v", err)
	updateAndCreateBMH(oc, testConfig.GlobalBackupDir, newUUID, newMACAddress)
	waitForBMHProvisioning(oc)
	reapplyDetachedAnnotation(oc)
	recreateMachine(oc, testConfig.GlobalBackupDir)
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
				g.GinkgoT().Logf("Approving CSR: %s", csrName)
				_, err = oc.AsAdmin().Run("adm").Args("certificate", "approve", csrName).Output()
				if err == nil {
					approvedCount++
					g.GinkgoT().Logf("Approved CSR %s (total approved: %d)", csrName, approvedCount)
				}
			}
		}

		if approvedCount < targetApprovedCount {
			g.GinkgoT().Logf("Waiting for more CSRs to approve... (approved: %d/%d, elapsed: %v)", approvedCount, targetApprovedCount, time.Since(csrStartTime))
			time.Sleep(csrPollInterval)
		}
	}

	// Verify we have approved the expected number of CSRs
	o.Expect(approvedCount).To(o.BeNumerically(">=", targetApprovedCount), fmt.Sprintf("Expected to approve at least %d CSRs, but only approved %d", targetApprovedCount, approvedCount))
	g.GinkgoT().Logf("Successfully approved %d CSRs", approvedCount)
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
			g.GinkgoT().Logf("Replacement node %s has appeared in the cluster", testConfig.TargetNodeName)

			// Wait a bit more for the node to be fully ready
			time.Sleep(30 * time.Second)

			// Verify the node is in Ready state
			nodeOutput, err := oc.AsAdmin().Run("get").Args("node", testConfig.TargetNodeName, "-o", "wide").Output()
			if err == nil {
				g.GinkgoT().Logf("[DEBUG] Node status: %s", nodeOutput)
				if strings.Contains(nodeOutput, "Ready") {
					g.GinkgoT().Logf("Node %s is now Ready", testConfig.TargetNodeName)
					return
				}
			}
		}

		g.GinkgoT().Logf("Waiting for replacement node %s to appear... (elapsed: %v)", testConfig.TargetNodeName, time.Since(startTime))
		time.Sleep(pollInterval)
	}

	// If we reach here, the timeout was exceeded
	o.Expect(false).To(o.BeTrue(), fmt.Sprintf("Replacement node %s did not appear within %v timeout", testConfig.TargetNodeName, maxWaitTime))
}

// restorePacemakerCluster restores the pacemaker cluster configuration
func restorePacemakerCluster(testConfig *TNFTestConfig, oc *exutil.CLI) {
	utils.DeleteNodeJobs(testConfig.TNFAuthJobName, testConfig.TNFAfterSetupJobName, oc)
	utils.RestoreEtcdRevision(testConfig.TargetNodeName, testConfig.TargetNodeIP, &testConfig.HypervisorConfig, oc)
	utils.CycleRemovedNode(testConfig.TargetNodeName, testConfig.TargetNodeIP, testConfig.SurvivingNodeName, testConfig.SurvivingNodeIP, &testConfig.HypervisorConfig)
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
			g.GinkgoT().Logf("Error getting cluster operators: %v", err)
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
				g.GinkgoT().Logf("Found degraded operator: %s", line)
			}
			if strings.Contains(line, coProgressingState) {
				hasProgressing = true
				allAvailable = false
				g.GinkgoT().Logf("Found progressing operator: %s", line)
			}
		}

		// Log current status
		g.GinkgoT().Logf("Cluster operators status check (elapsed: %v):", time.Since(startTime))
		g.GinkgoT().Logf("All available: %v, Has degraded: %v, Has progressing: %v", allAvailable, hasDegraded, hasProgressing)

		// If all operators are available, we're done
		if allAvailable {
			g.GinkgoT().Logf("All cluster operators are available!")
			return coOutput, nil
		}

		// Log the current operator status for debugging
		g.GinkgoT().Logf("[DEBUG] Current cluster operators status:\n%s", coOutput)

		// Wait before next check
		time.Sleep(pollInterval)
	}

	// If we reach here, the timeout was exceeded
	// Get final status for debugging
	finalCoOutput, err := oc.AsAdmin().Run("get").Args("co", "-o", "wide").Output()
	if err == nil {
		g.GinkgoT().Logf("[DEBUG] Final cluster operators status after timeout:\n%s", finalCoOutput)
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

	// Extract MAC address from BMH YAML
	macAddress, err := extractMACFromBMHYAML(bmhOutput)
	o.Expect(err).To(o.BeNil(), "Expected to find MAC address in BareMetalHost %s: %v", bmhName, err)

	g.GinkgoT().Logf("Found MAC address %s for node %s", macAddress, nodeName)
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

	g.GinkgoT().Logf("Using hypervisor configuration from test context:")
	g.GinkgoT().Logf("  Hypervisor IP: %s", testConfig.HypervisorConfig.IP)
	g.GinkgoT().Logf("  SSH User: %s", testConfig.HypervisorConfig.User)
	g.GinkgoT().Logf("  Private Key Path: %s", testConfig.HypervisorConfig.PrivateKeyPath)

	// Validate that the private key file exists
	if _, err := os.Stat(testConfig.HypervisorConfig.PrivateKeyPath); os.IsNotExist(err) {
		o.Expect(err).To(o.BeNil(), "Private key file does not exist at path: %s", testConfig.HypervisorConfig.PrivateKeyPath)
	}

	// Verify hypervisor connectivity and virsh availability
	verifyHypervisorConnectivity(&testConfig.HypervisorConfig)

	// Set target and surviving node names dynamically (random selection)
	testConfig.TargetNodeName, testConfig.SurvivingNodeName = getRandomControlPlaneNode(oc)

	// Set dynamic resource names based on target node
	setDynamicResourceNames(testConfig, oc)

	// Get IP addresses for both nodes
	testConfig.TargetNodeIP, testConfig.SurvivingNodeIP = getNodeIPs(oc, testConfig.TargetNodeName, testConfig.SurvivingNodeName)

	// Prepare known hosts file for the surviving node
	utils.PrepareSurvivingNodeKnownHostsFile(testConfig.SurvivingNodeName, testConfig.SurvivingNodeIP, &testConfig.HypervisorConfig)

	g.GinkgoT().Logf("Target node for replacement: %s (IP: %s)", testConfig.TargetNodeName, testConfig.TargetNodeIP)
	g.GinkgoT().Logf("Surviving node: %s (IP: %s)", testConfig.SurvivingNodeName, testConfig.SurvivingNodeIP)
	g.GinkgoT().Logf("Target node MAC: %s", testConfig.TargetNodeMAC)
	g.GinkgoT().Logf("Target VM for replacement: %s", testConfig.TargetVMName)
	g.GinkgoT().Logf("Target machine name: %s", testConfig.TargetMachineName)

	g.GinkgoT().Logf("Test environment setup complete. Hypervisor IP: %s", testConfig.HypervisorConfig.IP)
	g.GinkgoT().Logf("[DEBUG] setupTestEnvironment completed, testConfig.TargetVMName: %s", testConfig.TargetVMName)
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

	g.GinkgoT().Logf("Randomly selected control plane node for replacement: %s (index: %d)", selectedNode, randomIndex)
	g.GinkgoT().Logf("Surviving control plane node: %s", survivingNode)

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

	g.GinkgoT().Logf("Target node %s IP: %s", targetNodeName, targetNodeIP)
	g.GinkgoT().Logf("Surviving node %s IP: %s", survivingNodeName, survivingNodeIP)

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
	g.GinkgoT().Logf("Found machine name: %s", machineName)
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

// kubectlDeleteResource is a utility function to delete Kubernetes resources
func kubectlDeleteResource(oc *exutil.CLI, resourceType, name, namespace string) error {
	args := []string{resourceType, name}
	if namespace != "" {
		args = append(args, "-n", namespace)
	}
	_, err := oc.AsAdmin().Run("delete").Args(args...).Output()
	return err
}

// kubectlCreateResource is a utility function to create Kubernetes resources from file
func kubectlCreateResource(oc *exutil.CLI, filePath string) error {
	_, err := oc.AsAdmin().Run("create").Args("-f", filePath).Output()
	return err
}

// deleteEtcdSecrets deletes all etcd-related secrets for a node
func deleteEtcdSecrets(testConfig *TNFTestConfig, oc *exutil.CLI, nodeName string) {
	secrets := []string{
		fmt.Sprintf("%s-%s", testConfig.EtcdPeerSecretName, nodeName),
		fmt.Sprintf("%s-%s", testConfig.EtcdServingSecretName, nodeName),
		fmt.Sprintf("%s-%s", testConfig.EtcdServingMetricsSecretName, nodeName),
	}

	for _, secret := range secrets {
		err := kubectlDeleteResource(oc, secretResourceType, secret, etcdNamespace)
		o.Expect(err).To(o.BeNil(), "Expected to delete %s secret without error", secret)
	}
}

// deleteTNFJobs deletes TNF-related jobs for a node
func deleteTNFJobs(testConfig *TNFTestConfig, oc *exutil.CLI, nodeName string) {
	jobs := []string{
		fmt.Sprintf("%s-%s", testConfig.TNFAuthJobName, nodeName),
		fmt.Sprintf("%s-%s", testConfig.TNFAfterSetupJobName, nodeName),
	}

	for _, job := range jobs {
		err := kubectlDeleteResource(oc, jobResourceType, job, etcdNamespace)
		o.Expect(err).To(o.BeNil(), "Expected to delete %s job without error", job)
	}
}

// createTempFile creates a temporary file with the given content
func createTempFile(prefix, content string) (string, error) {
	tempFile, err := os.CreateTemp("", prefix)
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	_, err = tempFile.WriteString(content)
	if err != nil {
		os.Remove(tempFile.Name())
		return "", err
	}

	return tempFile.Name(), nil
}

// writeFileWithPermissions writes content to a file with specific permissions
func writeFileWithPermissions(filePath, content string, permissions os.FileMode) error {
	return os.WriteFile(filePath, []byte(content), permissions)
}

// backupEtcdSecrets backs up etcd-related secrets for the target node
func backupEtcdSecrets(testConfig *TNFTestConfig, oc *exutil.CLI, backupDir string) {
	etcdSecrets := []string{
		testConfig.EtcdPeerSecretName,
		testConfig.EtcdServingSecretName,
		testConfig.EtcdServingMetricsSecretName,
	}

	for _, secretName := range etcdSecrets {
		// Get the secret if it exists
		secretOutput, err := oc.AsAdmin().Run("get").Args("secret", secretName, "-n", etcdNamespace, "-o", "yaml").Output()
		if err != nil {
			g.GinkgoT().Logf("Warning: Could not backup etcd secret %s: %v", secretName, err)
			continue
		}

		secretFile := filepath.Join(backupDir, secretName+".yaml")
		err = os.WriteFile(secretFile, []byte(secretOutput), 0644)
		o.Expect(err).To(o.BeNil(), "Expected to write etcd secret %s backup without error", secretName)
		g.GinkgoT().Logf("Backed up etcd secret: %s", secretName)
	}
}

// recoverClusterFromBackup attempts to recover the cluster from backup if the test fails
func recoverClusterFromBackup(testConfig *TNFTestConfig, oc *exutil.CLI) {
	g.GinkgoT().Logf("Starting cluster recovery from backup directory: %s", testConfig.GlobalBackupDir)

	defer func() {
		if r := recover(); r != nil {
			g.GinkgoT().Logf("Recovery failed with panic: %v", r)
		}
		// Clean up backup directory after recovery attempt
		if testConfig.GlobalBackupDir != "" {
			os.RemoveAll(testConfig.GlobalBackupDir)
			testConfig.GlobalBackupDir = ""
		}
	}()

	// Step 1: Recreate the VM from backup
	g.GinkgoT().Logf("Step 1: Recreating VM from backup")
	if err := recoverVMFromBackup(testConfig); err != nil {
		g.GinkgoT().Logf("Failed to recover VM: %v", err)
		return
	}

	// Wait for VM to start
	g.GinkgoT().Logf("Waiting for VM to start...")
	time.Sleep(3 * time.Minute)

	// Step 2: Promote etcd learner member to prevent stalling
	g.GinkgoT().Logf("Step 2: Promoting etcd learner member to prevent stalling")
	if err := promoteEtcdLearnerMember(testConfig); err != nil {
		g.GinkgoT().Logf("Warning: Failed to promote etcd learner member: %v", err)
		// Don't return here, continue with recovery as this is not critical
	}

	// Step 3: Recreate etcd secrets from backup
	g.GinkgoT().Logf("Step 3: Recreating etcd secrets from backup")
	if err := recoverEtcdSecretsFromBackup(testConfig, oc); err != nil {
		g.GinkgoT().Logf("Failed to recover etcd secrets: %v", err)
		return
	}

	// Step 4: Recreate BMH and Machine
	g.GinkgoT().Logf("Step 4: Recreating BMH and Machine from backup")
	if err := recoverBMHAndMachineFromBackup(testConfig, oc); err != nil {
		g.GinkgoT().Logf("Failed to recover BMH and Machine: %v", err)
		return
	}

	// Step 5: Re-enable stonith on the surviving node
	g.GinkgoT().Logf("Step 5: Re-enabling stonith on the surviving node")
	if err := reenableStonith(testConfig); err != nil {
		g.GinkgoT().Logf("Warning: Failed to re-enable stonith: %v", err)
		// Don't return here, continue with recovery as this is not critical
	}

	// Step 6: Approve CSRs only if we attempted node provisioning
	if testConfig.HasAttemptedNodeProvisioning {
		g.GinkgoT().Logf("Step 6: Approving CSRs for cluster recovery (node provisioning was attempted)")
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

		g.GinkgoT().Logf("Cluster recovery initiated with CSR approval. Monitoring for 5 minutes...")
		time.Sleep(5 * time.Minute)
	} else {
		g.GinkgoT().Logf("Step 6: Skipping CSR approval (no node provisioning was attempted)")
	}

	g.GinkgoT().Logf("Cluster recovery process completed")
}

// recoverVMFromBackup recreates the VM from the backed up XML
func recoverVMFromBackup(testConfig *TNFTestConfig) error {
	// Check if the specific VM already exists
	_, err := utils.VirshVMExists(testConfig.TargetVMName, &testConfig.HypervisorConfig)
	if err == nil {
		g.GinkgoT().Logf("VM %s already exists, skipping recreation", testConfig.TargetVMName)
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

	_, err = utils.ExecuteSSHCommand(createXMLCommand, &testConfig.HypervisorConfig)
	if err != nil {
		return fmt.Errorf("failed to create XML file on hypervisor: %v", err)
	}

	// Redefine the VM using the backed up XML
	err = utils.VirshDefineVM(fmt.Sprintf("/tmp/%s.xml", testConfig.TargetVMName), &testConfig.HypervisorConfig)
	if err != nil {
		return fmt.Errorf("failed to define VM: %v", err)
	}

	// Start the VM
	err = utils.VirshStartVM(testConfig.TargetVMName, &testConfig.HypervisorConfig)
	if err != nil {
		return fmt.Errorf("failed to start VM: %v", err)
	}

	// Enable autostart
	err = utils.VirshAutostartVM(testConfig.TargetVMName, &testConfig.HypervisorConfig)
	if err != nil {
		g.GinkgoT().Logf("Warning: Failed to enable autostart for VM: %v", err)
	}

	// Clean up temporary XML file
	_, err = utils.ExecuteSSHCommand(fmt.Sprintf("rm -f /tmp/%s.xml", testConfig.TargetVMName), &testConfig.HypervisorConfig)
	if err != nil {
		g.GinkgoT().Logf("Warning: Failed to clean up temporary XML file: %v", err)
	}

	g.GinkgoT().Logf("Recreated VM: %s", testConfig.TargetVMName)
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
			g.GinkgoT().Logf("Recovery operation %s succeeded on attempt %d after %v", operationName, i+1, time.Since(startTime))
			return nil
		}

		// Check if this is an etcd learner error that we should retry
		if isEtcdLearnerError(err) {
			g.GinkgoT().Logf("Recovery operation %s failed on attempt %d due to etcd learner error (will retry): %v", operationName, i+1, err)
		} else {
			g.GinkgoT().Logf("Recovery operation %s failed on attempt %d with non-retryable error: %v", operationName, i+1, err)
			return err // Don't retry non-etcd learner errors
		}

		if i < maxRetries-1 && time.Since(startTime) < timeout {
			g.GinkgoT().Logf("Retrying recovery operation %s in %v...", operationName, retryInterval)
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
			g.GinkgoT().Logf("Operation %s succeeded after %v", operationName, time.Since(startTime))
			return nil
		}

		g.GinkgoT().Logf("Operation %s failed, retrying in %v: %v", operationName, pollInterval, err)
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
			g.GinkgoT().Logf("Warning: Backup file for etcd secret %s not found", secretName)
			continue
		}

		// Check if the secret already exists
		_, err := oc.AsAdmin().Run("get").Args("secret", secretName, "-n", etcdNamespace).Output()
		if err == nil {
			g.GinkgoT().Logf("Etcd secret %s already exists, skipping recreation", secretName)
			continue
		}

		// Retry the secret creation with etcd learner error handling
		err = retryRecoveryOperation(func() error {
			return kubectlCreateResource(oc, secretFile)
		}, fmt.Sprintf("create etcd secret %s", secretName))

		if err != nil {
			g.GinkgoT().Logf("Warning: Failed to recreate etcd secret %s after retries: %v", secretName, err)
			continue
		}
		g.GinkgoT().Logf("Recreated etcd secret: %s", secretName)
	}

	return nil
}

// recoverBMHAndMachineFromBackup recreates BMH and Machine from backup with retry logic
func recoverBMHAndMachineFromBackup(testConfig *TNFTestConfig, oc *exutil.CLI) error {
	// Recreate BMC secret with retry
	bmcSecretFile := filepath.Join(testConfig.GlobalBackupDir, testConfig.TargetBMCSecretName+".yaml")

	// Check if BMC secret already exists
	_, err := oc.AsAdmin().Run("get").Args("secret", testConfig.TargetBMCSecretName, "-n", machineAPINamespace).Output()
	if err != nil {
		// Retry BMC secret creation
		err = retryRecoveryOperation(func() error {
			return kubectlCreateResource(oc, bmcSecretFile)
		}, fmt.Sprintf("create BMC secret %s", testConfig.TargetBMCSecretName))

		if err != nil {
			return fmt.Errorf("failed to recreate BMC secret after retries: %v", err)
		}
		g.GinkgoT().Logf("Recreated BMC secret: %s", testConfig.TargetBMCSecretName)
	} else {
		g.GinkgoT().Logf("BMC secret %s already exists, skipping recreation", testConfig.TargetBMCSecretName)
	}

	// Recreate BMH with retry
	bmhFile := filepath.Join(testConfig.GlobalBackupDir, testConfig.TargetBMHName+".yaml")

	// Check if BMH already exists
	_, err = oc.AsAdmin().Run("get").Args("bmh", testConfig.TargetBMHName, "-n", machineAPINamespace).Output()
	if err != nil {
		// Retry BMH creation
		err = retryRecoveryOperation(func() error {
			return kubectlCreateResource(oc, bmhFile)
		}, fmt.Sprintf("create BareMetalHost %s", testConfig.TargetBMHName))

		if err != nil {
			return fmt.Errorf("failed to recreate BareMetalHost after retries: %v", err)
		}
		g.GinkgoT().Logf("Recreated BareMetalHost: %s", testConfig.TargetBMHName)
	} else {
		g.GinkgoT().Logf("BareMetalHost %s already exists, skipping recreation", testConfig.TargetBMHName)
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
		g.GinkgoT().Logf("Recreated Machine: %s", testConfig.TargetMachineName)
	} else {
		g.GinkgoT().Logf("Machine %s already exists, skipping recreation", testConfig.TargetMachineName)
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
		g.GinkgoT().Logf("Warning: Failed to get CSRs after retries: %v", err)
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
		g.GinkgoT().Logf("Approving CSR during recovery: %s", csrName)

		err = retryRecoveryOperation(func() error {
			_, err := oc.AsAdmin().Run("adm").Args("certificate", "approve", csrName).Output()
			return err
		}, fmt.Sprintf("approve CSR %s", csrName))

		if err == nil {
			g.GinkgoT().Logf("Approved CSR during recovery: %s", csrName)
		} else {
			g.GinkgoT().Logf("Warning: Failed to approve CSR %s after retries: %v", csrName, err)
		}
	}
}

// waitForPacemakerQuorum waits for pacemaker to restore quorum after node deletion
func waitForPacemakerQuorum() error {
	g.GinkgoT().Logf("Waiting for API server to restore quorum (checking with oc status)...")

	return retryOperationWithTimeout(func() error {
		// Check if the API server is accessible by running a simple oc command
		// If we have quorum, the API server will be working. If not, the request will timeout.
		_, err := oc.AsAdmin().Run("status").Args().Output()
		if err != nil {
			return fmt.Errorf("API server not yet accessible (no quorum): %v", err)
		}

		g.GinkgoT().Logf("[DEBUG] API server is accessible - quorum restored")
		return nil
	}, pacemakerQuorumTimeout, pacemakerQuorumPollInterval, "API server quorum restoration")
}

// waitForVMToStart waits for a VM to be running
func waitForVMToStart(vmName string, sshConfig *utils.SSHConfig) error {
	g.GinkgoT().Logf("Waiting for VM %s to start...", vmName)

	return retryOperationWithTimeout(func() error {
		// Check if VM is running using constant
		_, err := utils.VirshListAllVMs(sshConfig)
		if err != nil {
			return fmt.Errorf("VM %s not yet running: %v", vmName, err)
		}

		// Check if VM is actually running (not just defined)
		statusOutput, err := utils.VirshCommand(fmt.Sprintf("virsh domstate %s", vmName), sshConfig)
		if err != nil {
			return fmt.Errorf("failed to check VM %s state: %v", vmName, err)
		}

		if !strings.Contains(statusOutput, "running") {
			return fmt.Errorf("VM %s is not running, current state: %s", vmName, strings.TrimSpace(statusOutput))
		}

		g.GinkgoT().Logf("VM %s is now running", vmName)
		return nil
	}, vmStartTimeout, vmStartPollInterval, fmt.Sprintf("VM %s startup", vmName))
}

// promoteEtcdLearnerMember promotes the etcd learner member to voter status
func promoteEtcdLearnerMember(testConfig *TNFTestConfig) error {
	g.GinkgoT().Logf("Attempting to promote etcd learner member on surviving node: %s (IP: %s)", testConfig.SurvivingNodeName, testConfig.SurvivingNodeIP)

	return retryOperationWithTimeout(func() error {
		// First, get the list of etcd members to find the learner
		memberListCmd := `sudo podman exec -it etcd etcdctl member list`
		output, err := utils.ExecuteSSHCommand(fmt.Sprintf(`ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=%s core@%s "%s"`, utils.GetSurvivingNodeKnownHostsPath(), testConfig.SurvivingNodeIP, memberListCmd), &testConfig.HypervisorConfig)
		if err != nil {
			return fmt.Errorf("failed to get etcd member list on %s: %v", testConfig.SurvivingNodeIP, err)
		}

		g.GinkgoT().Logf("[DEBUG] Etcd member list output: %s", output)

		// Parse the output to find the learner member
		learnerMemberID, err := findLearnerMemberID(output)
		if err != nil {
			return fmt.Errorf("failed to find learner member ID: %v", err)
		}

		if learnerMemberID == "" {
			g.GinkgoT().Logf("No learner member found, all members are already voters")
			return nil // No learner to promote, this is success
		}

		g.GinkgoT().Logf("Found learner member ID: %s", learnerMemberID)

		// Promote the learner member
		promoteCmd := fmt.Sprintf(`sudo podman exec -it etcd etcdctl member promote %s`, learnerMemberID)
		promoteOutput, err := utils.ExecuteSSHCommand(fmt.Sprintf(`ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=%s core@%s "%s"`, utils.GetSurvivingNodeKnownHostsPath(), testConfig.SurvivingNodeIP, promoteCmd), &testConfig.HypervisorConfig)
		if err != nil {
			return fmt.Errorf("failed to promote etcd learner member %s on %s: %v, output: %s", learnerMemberID, testConfig.SurvivingNodeIP, err, promoteOutput)
		}

		g.GinkgoT().Logf("[DEBUG] Successfully promoted etcd learner member %s: %s", learnerMemberID, promoteOutput)
		return nil
	}, 10*time.Minute, 30*time.Second, "promote etcd learner member")
}

// findLearnerMemberID parses the etcd member list output to find the learner member ID
func findLearnerMemberID(memberListOutput string) (string, error) {
	lines := strings.Split(memberListOutput, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse line format: memberID, started, name, peerURL, clientURL, isLearner
		// Example: 40a96b5f788d2842, started, master-1, https://192.168.111.21:2380, https://192.168.111.21:2379, false
		parts := strings.Split(line, ",")
		if len(parts) < 6 {
			continue // Skip malformed lines
		}

		// Check if this member is a learner (6th field should be "true")
		isLearner := strings.TrimSpace(parts[5])
		if isLearner == "true" {
			// Also check if the member is started (2nd field should be "started")
			isStarted := strings.TrimSpace(parts[1])
			if isStarted == "started" {
				// Return the member ID (first field)
				memberID := strings.TrimSpace(parts[0])
				g.GinkgoT().Logf("Found started learner member: ID=%s, Name=%s", memberID, strings.TrimSpace(parts[2]))
				return memberID, nil
			} else {
				// Log that we found a learner but it's not started yet
				memberID := strings.TrimSpace(parts[0])
				memberName := strings.TrimSpace(parts[2])
				g.GinkgoT().Logf("Found learner member but not yet started: ID=%s, Name=%s, Status=%s", memberID, memberName, isStarted)
			}
		}
	}

	// No started learner found
	return "", nil
}

// Additional constants for pacemaker operations
const (
	//pcsResourceDebugStop = "sudo pcs resource debug-stop etcd"
	pcsResourceDebugStart = "sudo OCF_RESKEY_CRM_meta_notify_start_resource='etcd' pcs resource debug-start etcd --full"
	pcsDisableStonith = "sudo pcs property set stonith-enabled=false"
	pcsEnableStonith = "sudo pcs property set stonith-enabled=true"

	pcsClusterNodeRemove = "sudo pcs cluster node remove %s"
	pcsClusterNodeAdd = "sudo pcs cluster node add %s addr=%s --start --enable"
	pcsResourceStatus = "sudo pcs resource status etcd node=%s"
	journalctlPacemaker = "sudo journalctl -u pacemaker --no-pager | grep podman-etcd | tail -n 20"
)

// Missing functions that need to be implemented
func recreateBMCSecret(oc *exutil.CLI, backupDir string) {
	// TODO: Implement this function
	g.GinkgoT().Logf("recreateBMCSecret not yet implemented")
}

func updateAndCreateBMH(oc *exutil.CLI, backupDir string, newUUID, newMACAddress string) {
	// TODO: Implement this function
	g.GinkgoT().Logf("updateAndCreateBMH not yet implemented")
}

func waitForBMHProvisioning(oc *exutil.CLI) {
	// TODO: Implement this function
	g.GinkgoT().Logf("waitForBMHProvisioning not yet implemented")
}

func reapplyDetachedAnnotation(oc *exutil.CLI) {
	// TODO: Implement this function
	g.GinkgoT().Logf("reapplyDetachedAnnotation not yet implemented")
}

func recreateMachine(oc *exutil.CLI, backupDir string) {
	// TODO: Implement this function
	g.GinkgoT().Logf("recreateMachine not yet implemented")
}

func executeSSHScript(oc *exutil.CLI, scriptName, script string) {
	// TODO: Implement this function
	g.GinkgoT().Logf("executeSSHScript not yet implemented")
}

// Additional missing functions
func extractMACFromBMHYAML(bmhYAML string) (string, error) {
	lines := strings.Split(bmhYAML, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "bootMACAddress:") {
			// Extract MAC address from line like "bootMACAddress: 52:54:00:12:34:56"
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				// Join all parts after the first colon and trim quotes
				macAddress := strings.TrimSpace(strings.Join(parts[1:], ":"))
				macAddress = strings.Trim(macAddress, "\"'")
				return macAddress, nil
			}
		}
	}
	return "", fmt.Errorf("no bootMACAddress found in BareMetalHost YAML")
}

func verifyHypervisorConnectivity(sshConfig *utils.SSHConfig) {
	output, err := utils.VerifyConnectivity(sshConfig)
	if err != nil {
		o.Expect(err).To(o.BeNil(), "Failed to establish SSH connection to hypervisor %s@%s: %v, output: %s", sshConfig.User, sshConfig.IP, err, output)
	}
	g.GinkgoT().Logf("SSH connectivity to hypervisor verified: %s", strings.TrimSpace(output))

	// Test virsh availability and basic functionality
	output, err = utils.VerifyVirsh(sshConfig)
	if err != nil {
		o.Expect(err).To(o.BeNil(), "virsh is not available or not working on hypervisor %s@%s: %v, output: %s", sshConfig.User, sshConfig.IP, err, output)
	}
	g.GinkgoT().Logf("virsh availability verified: %s", strings.TrimSpace(output))

	// Test libvirt connection
	output, err = utils.VirshListAllVMs(sshConfig)
	if err != nil {
		o.Expect(err).To(o.BeNil(), "Failed to connect to libvirt on hypervisor %s@%s: %v, output: %s", sshConfig.User, sshConfig.IP, err, output)
	}
	g.GinkgoT().Logf("libvirt connection verified, found VMs: %s", strings.TrimSpace(output))

	g.GinkgoT().Logf("Hypervisor connectivity and virsh availability verification completed successfully")
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
	g.GinkgoT().Logf("[DEBUG] Found targetNodeMAC: %s for node: %s", testConfig.TargetNodeMAC, testConfig.TargetNodeName)

	// Find the corresponding VM name by matching MAC addresses
	var err error
	testConfig.TargetVMName, err = utils.GetVMNameByMACMatch(testConfig.TargetNodeName, testConfig.TargetNodeMAC, virshProvisioningBridge, &testConfig.HypervisorConfig)
	g.GinkgoT().Logf("[DEBUG] GetVMNameByMACMatch returned: testConfig.TargetVMName=%s, err=%v", testConfig.TargetVMName, err)
	o.Expect(err).To(o.BeNil(), "Expected to find VM name for node %s with MAC %s: %v", testConfig.TargetNodeName, testConfig.TargetNodeMAC, err)

	// Ensure we found a valid VM name
	o.Expect(testConfig.TargetVMName).ToNot(o.BeEmpty(), "Expected to find a valid VM name for node %s with MAC %s", testConfig.TargetNodeName, testConfig.TargetNodeMAC)

	// Extract and store the machine name from the BMH consumerRef
	testConfig.TargetMachineName = extractMachineNameFromBMH(oc, testConfig.TargetNodeName)
}

// waitForEtcdToStop waits for etcd to stop on the surviving node
func waitForEtcdToStop(testConfig *TNFTestConfig) error {
	g.GinkgoT().Logf("Waiting for etcd to stop on surviving node: %s", testConfig.SurvivingNodeName)

	return retryOperationWithTimeout(func() error {
		// Check etcd resource status on the surviving node
		statusCmd := fmt.Sprintf(pcsResourceStatus, testConfig.SurvivingNodeName)
		output, err := utils.ExecuteSSHCommand(fmt.Sprintf(`ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=%s core@%s "%s"`, utils.GetSurvivingNodeKnownHostsPath(), testConfig.SurvivingNodeIP, statusCmd), &testConfig.HypervisorConfig)
		if err != nil {
			return fmt.Errorf("failed to get etcd resource status on %s: %v, output: %s", testConfig.SurvivingNodeName, err, output)
		}

		g.GinkgoT().Logf("[DEBUG] Etcd resource status on %s:\n%s", testConfig.SurvivingNodeName, output)

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
		g.GinkgoT().Logf("Etcd has stopped on surviving node: %s", testConfig.SurvivingNodeName)
		return nil
	}, etcdStatusCheckTimeout, etcdStatusCheckPollInterval, fmt.Sprintf("etcd stop on %s", testConfig.SurvivingNodeName))
}

// waitForEtcdToStart waits for etcd to start on the surviving node
func waitForEtcdToStart(testConfig *TNFTestConfig) error {
	g.GinkgoT().Logf("Waiting for etcd to start on surviving node: %s", testConfig.SurvivingNodeName)

	return retryOperationWithTimeout(func() error {
		// Check etcd resource status on the surviving node
		statusCmd := fmt.Sprintf(pcsResourceStatus, testConfig.SurvivingNodeName)
		output, err := utils.ExecuteSSHCommand(fmt.Sprintf(`ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=%s core@%s "%s"`, utils.GetSurvivingNodeKnownHostsPath(), testConfig.SurvivingNodeIP, statusCmd), &testConfig.HypervisorConfig)
		if err != nil {
			return fmt.Errorf("failed to get etcd resource status on %s: %v, output: %s", testConfig.SurvivingNodeName, err, output)
		}

		g.GinkgoT().Logf("[DEBUG] Etcd resource status on %s:\n%s", testConfig.SurvivingNodeName, output)

		// Check if etcd is started on the surviving node
		// We expect to see "Started: [ master-X ]" for the survivor
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.Contains(line, "Started:") && strings.Contains(line, testConfig.SurvivingNodeName) {
				g.GinkgoT().Logf("Etcd has started on surviving node: %s", testConfig.SurvivingNodeName)
				return nil
			}
		}

		// If we get here, etcd is not started on the surviving node
		// Get pacemaker journal logs to help with debugging
		g.GinkgoT().Logf("Etcd is not started on %s, getting pacemaker journal logs for debugging", testConfig.SurvivingNodeName)
		journalOutput, journalErr := utils.ExecuteSSHCommand(fmt.Sprintf(`ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=%s core@%s "%s"`, utils.GetSurvivingNodeKnownHostsPath(), testConfig.SurvivingNodeIP, journalctlPacemaker), &testConfig.HypervisorConfig)
		if journalErr != nil {
			g.GinkgoT().Logf("Warning: Failed to get pacemaker journal logs on %s: %v", testConfig.SurvivingNodeName, journalErr)
		} else {
			g.GinkgoT().Logf("[DEBUG] Last 20 lines of pacemaker journal on %s:\n%s", testConfig.SurvivingNodeName, journalOutput)
		}

		return fmt.Errorf("etcd is not started on surviving node %s", testConfig.SurvivingNodeName)
	}, etcdStatusCheckTimeout, etcdStatusCheckPollInterval, fmt.Sprintf("etcd start on %s", testConfig.SurvivingNodeName))
}

// reenableStonith re-enables stonith on the surviving node
func reenableStonith(testConfig *TNFTestConfig) error {
	g.GinkgoT().Logf("Re-enabling stonith on surviving node: %s", testConfig.SurvivingNodeName)

	// Execute the stonith enable command on the surviving node
	enableStonithCmd := fmt.Sprintf(`ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=%s core@%s "%s"`, utils.GetSurvivingNodeKnownHostsPath(), testConfig.SurvivingNodeIP, pcsEnableStonith)

	output, err := utils.ExecuteSSHCommand(enableStonithCmd, &testConfig.HypervisorConfig)
	if err != nil {
		return fmt.Errorf("failed to re-enable stonith on %s: %v, output: %s", testConfig.SurvivingNodeName, err, output)
	}

	g.GinkgoT().Logf("Successfully re-enabled stonith on surviving node: %s", testConfig.SurvivingNodeName)
	g.GinkgoT().Logf("[DEBUG] Stonith enable output: %s", output)
	return nil
}
package machine_config

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	mcfgv1alpha1 "github.com/openshift/api/machineconfiguration/v1alpha1"
	machineclient "github.com/openshift/client-go/machine/clientset/versioned"
	machineconfigclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	worker = "worker"
	master = "master"
	custom = "infra"
)

var _ = g.Describe("[sig-mco][OCPFeatureGate:MachineConfigNodes]", func() {
	defer g.GinkgoRecover()
	var (
		MCOMachineConfigPoolBaseDir = exutil.FixturePath("testdata", "machine_config", "machineconfigpool")
		MCOMachineConfigBaseDir     = exutil.FixturePath("testdata", "machine_config", "machineconfig")
		infraMCPFixture             = filepath.Join(MCOMachineConfigPoolBaseDir, "infra-mcp.yaml")
		testFileMCFixture           = filepath.Join(MCOMachineConfigBaseDir, "0-master-mc.yaml")
		invalidWorkerMCFixture      = filepath.Join(MCOMachineConfigBaseDir, "1-worker-invalid-mc.yaml")
		invalidMasterMCFixture      = filepath.Join(MCOMachineConfigBaseDir, "1-master-invalid-mc.yaml")
		oc                          = exutil.NewCLIWithoutNamespace("machine-config")
	)

	g.It("[Serial]Should have MCN properties matching associated node properties [apigroup:machineconfiguration.openshift.io]", func() {
		if IsSingleNode(oc) { //handle SNO clusters
			ValidateMCNPropertiesSNO(oc, infraMCPFixture)
		} else { //handle standard, non-SNO, clusters
			ValidateMCNProperties(oc, infraMCPFixture)
		}
	})

	g.It("[Serial]Should properly transition through MCN conditions on node update [apigroup:machineconfiguration.openshift.io]", func() {
		ValidateMCNConditionTransitions(oc, testFileMCFixture)
	})

	g.It("[Serial][Slow]Should properly report MCN conditions on node degrade [apigroup:machineconfiguration.openshift.io]", func() {
		if IsSingleNode(oc) { //handle SNO clusters
			ValidateMCNConditionOnNodeDegrade(oc, invalidMasterMCFixture, true)
		} else { //handle standard, non-SNO, clusters
			ValidateMCNConditionOnNodeDegrade(oc, invalidWorkerMCFixture, false)
		}
	})

	g.It("[Serial][Slow]Should properly create and remove MCN on node creation and deletion [apigroup:machineconfiguration.openshift.io]", func() {
		skipOnSingleNodeTopology(oc) //skip this test for SNO
		ValidateMCNOnNodeCreationAndDeletion(oc)
	})
})

// `ValidateMCNProperties` checks that MCN properties match the corresponding node properties
// Note: This test case does not work for SNO clusters due to the cluster's one node assuming
// both the worker and master role since `GetRandomNode` selects nodes using node roles. Role
// matching is not necessarily synonymous with MCP association in edge cases, such as in SNO.
func ValidateMCNProperties(oc *exutil.CLI, fixture string) {
	// Create client set for test
	clientSet, clientErr := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(clientErr).NotTo(o.HaveOccurred())

	// Grab a random node from each default pool
	workerNode := GetRandomNode(oc, worker)
	o.Expect(workerNode.Name).NotTo(o.Equal(""), "Could not get a worker node.")
	masterNode := GetRandomNode(oc, master)
	o.Expect(masterNode.Name).NotTo(o.Equal(""), "Could not get a master node.")

	// Validate MCN for node in default `worker` pool
	framework.Logf("Validating MCN properties for node in default '%v' pool.", worker)
	mcnErr := ValidateMCNForNodeInPool(oc, clientSet, workerNode, worker)
	o.Expect(mcnErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Error validating MCN properties node in default pool '%v'.", worker))

	// Validate MCN for node in default `master` pool
	framework.Logf("Validating MCN properties for node in default '%v' pool.", master)
	mcnErr = ValidateMCNForNodeInPool(oc, clientSet, masterNode, master)
	o.Expect(mcnErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Error validating MCN properties node in default pool '%v'.", master))

	// Cleanup custom MCP on test completion or failure
	defer func() {
		// Get starting state of default worker MCP
		workerMcp, err := clientSet.MachineconfigurationV1().MachineConfigPools().Get(context.TODO(), worker, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		workerMcpReadyMachines := workerMcp.Status.ReadyMachineCount

		// Unlabel node
		framework.Logf("Removing label node-role.kubernetes.io/%v from node %v", custom, workerNode.Name)
		unlabelErr := oc.Run("label").Args(fmt.Sprintf("node/%s", workerNode.Name), fmt.Sprintf("node-role.kubernetes.io/%s-", custom)).Execute()
		o.Expect(unlabelErr).NotTo(o.HaveOccurred())

		// Wait for infra pool to report no nodes & for worker MCP to be ready
		framework.Logf("Waiting for %v MCP to be updated with %v ready machines.", custom, 0)
		WaitForMCPToBeReady(oc, clientSet, custom, 0)
		framework.Logf("Waiting for %v MCP to be updated with %v ready machines.", worker, workerMcpReadyMachines+1)
		WaitForMCPToBeReady(oc, clientSet, worker, workerMcpReadyMachines+1)

		// Delete custom MCP
		framework.Logf("Deleting MCP %v", custom)
		deleteMCPErr := oc.Run("delete").Args("mcp", custom).Execute()
		o.Expect(deleteMCPErr).NotTo(o.HaveOccurred())
	}()

	// Apply the fixture to create a custom MCP called "infra" & label the worker node accordingly
	mcpErr := oc.Run("apply").Args("-f", fixture).Execute()
	o.Expect(mcpErr).NotTo(o.HaveOccurred())
	labelErr := oc.Run("label").Args(fmt.Sprintf("node/%s", workerNode.Name), fmt.Sprintf("node-role.kubernetes.io/%s=", custom)).Execute()
	o.Expect(labelErr).NotTo(o.HaveOccurred())

	// Wait for the custom pool to be updated with the node ready
	framework.Logf("Waiting for '%v' MCP to be updated with %v ready machines.", custom, 1)
	WaitForMCPToBeReady(oc, clientSet, custom, 1)

	// Get node desired and current config versions
	customNodes, customNodeErr := GetNodesByRole(oc, custom)
	o.Expect(customNodeErr).NotTo(o.HaveOccurred())
	customNode := customNodes[0]

	// Validate MCN for node in custom pool
	framework.Logf("Validating MCN properties for node in custom '%v' pool.", custom)
	mcnErr = ValidateMCNForNodeInPool(oc, clientSet, customNode, custom)
	o.Expect(mcnErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Error validating MCN properties node in custom pool '%v'.", custom))
}

// `ValidateMCNPropertiesSNO` checks that MCN properties match the corresponding node properties
// specifically for SNO clusters. Note that this test does not include creating a custom MCP, as
// the default SNO node remains part of the master pool.
func ValidateMCNPropertiesSNO(oc *exutil.CLI, fixture string) {
	// Create client set for test
	clientSet, clientErr := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(clientErr).NotTo(o.HaveOccurred())

	// Grab the cluster's node
	node := GetRandomNode(oc, master)
	o.Expect(node.Name).NotTo(o.Equal(""), "Could not get a worker node.")

	// Validate MCN for the cluster's node
	framework.Logf("Validating MCN properties for the node in pool '%v'.", master)
	mcnErr := ValidateMCNForNodeInPool(oc, clientSet, node, master)
	o.Expect(mcnErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Error validating MCN properties for the node in pool '%v'.", master))
}

// `ValidateMCNConditionTransitions` checks that Conditions properly update on a node update
func ValidateMCNConditionTransitions(oc *exutil.CLI, fixture string) {
	// Create client set for test
	clientSet, clientErr := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(clientErr).NotTo(o.HaveOccurred())

	// Delete MC on failure or test completion
	defer func() {
		deleteMCErr := oc.Run("delete").Args("machineconfig", "90-master-testfile").Execute()
		o.Expect(deleteMCErr).NotTo(o.HaveOccurred())
	}()

	// Apply MC targeting master pool
	mcErr := oc.Run("apply").Args("-f", fixture).Execute()
	o.Expect(mcErr).NotTo(o.HaveOccurred())

	// Get an updating master node
	updatingNodes := GetCordonedNodes(oc, master)
	o.Expect(len(updatingNodes) > 0, "No ready nodes found for MCP '%v'.", master)
	masterNode := updatingNodes[0]

	// Validate transition through conditions for MCN
	// Note that some conditions are passed through quickly in a node update, so the test can
	// "miss" catching the phases. For test stability, if we fail to catch an "Unknown" status,
	// a warning will be logged instead of erroring out the test.
	framework.Logf("Waiting for Updated=False")
	err := WaitForMCNConditionStatus(clientSet, masterNode.Name, mcfgv1alpha1.MachineConfigNodeUpdated, metav1.ConditionFalse, 1*time.Minute, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred())
	framework.Logf("Waiting for UpdatePrepared=True")
	err = WaitForMCNConditionStatus(clientSet, masterNode.Name, mcfgv1alpha1.MachineConfigNodeUpdatePrepared, metav1.ConditionTrue, 1*time.Minute, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred())
	framework.Logf("Waiting for UpdateExecuted=Unknown")
	err = WaitForMCNConditionStatus(clientSet, masterNode.Name, mcfgv1alpha1.MachineConfigNodeUpdateExecuted, metav1.ConditionUnknown, 30*time.Second, 1*time.Second)
	if err != nil {
		framework.Logf("Warning, could not detect UpdateExecuted=Unknown.")
	}
	framework.Logf("Waiting for Cordoned=True")
	err = WaitForMCNConditionStatus(clientSet, masterNode.Name, mcfgv1alpha1.MachineConfigNodeUpdateCordoned, metav1.ConditionTrue, 30*time.Second, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred())
	framework.Logf("Waiting for Drained=Unknown")
	err = WaitForMCNConditionStatus(clientSet, masterNode.Name, mcfgv1alpha1.MachineConfigNodeUpdateDrained, metav1.ConditionUnknown, 15*time.Second, 1*time.Second)
	if err != nil {
		framework.Logf("Warning, could not detect Drained=Unknown.")
	}
	framework.Logf("Waiting for Drained=True")
	err = WaitForMCNConditionStatus(clientSet, masterNode.Name, mcfgv1alpha1.MachineConfigNodeUpdateDrained, metav1.ConditionTrue, 4*time.Minute, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred())
	framework.Logf("Waiting for AppliedFilesAndOS=Unknown")
	err = WaitForMCNConditionStatus(clientSet, masterNode.Name, mcfgv1alpha1.MachineConfigNodeUpdateFilesAndOS, metav1.ConditionUnknown, 30*time.Second, 1*time.Second)
	if err != nil {
		framework.Logf("Warning, could not detect AppliedFilesAndOS=Unknown.")
	}
	framework.Logf("Waiting for AppliedFilesAndOS=True")
	err = WaitForMCNConditionStatus(clientSet, masterNode.Name, mcfgv1alpha1.MachineConfigNodeUpdateFilesAndOS, metav1.ConditionTrue, 3*time.Minute, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred())
	framework.Logf("Waiting for UpdateExecuted=True")
	err = WaitForMCNConditionStatus(clientSet, masterNode.Name, mcfgv1alpha1.MachineConfigNodeUpdateExecuted, metav1.ConditionTrue, 20*time.Second, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred())
	framework.Logf("Waiting for UpdatePostActionComplete=Unknown")
	err = WaitForMCNConditionStatus(clientSet, masterNode.Name, mcfgv1alpha1.MachineConfigNodeUpdatePostActionComplete, metav1.ConditionUnknown, 30*time.Second, 1*time.Second)
	if err != nil {
		framework.Logf("Warning, could not detect UpdatePostActionComplete=Unknown.")
	}
	framework.Logf("Waiting for RebootedNode=Unknown")
	err = WaitForMCNConditionStatus(clientSet, masterNode.Name, mcfgv1alpha1.MachineConfigNodeUpdateRebooted, metav1.ConditionUnknown, 15*time.Second, 1*time.Second)
	if err != nil {
		framework.Logf("Warning, could not detect RebootedNode=Unknown.")
	}
	framework.Logf("Waiting for RebootedNode=True")
	err = WaitForMCNConditionStatus(clientSet, masterNode.Name, mcfgv1alpha1.MachineConfigNodeUpdateRebooted, metav1.ConditionTrue, 5*time.Minute, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred())
	framework.Logf("Waiting for Resumed=True")
	err = WaitForMCNConditionStatus(clientSet, masterNode.Name, mcfgv1alpha1.MachineConfigNodeResumed, metav1.ConditionTrue, 15*time.Second, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred())
	framework.Logf("Waiting for UpdateComplete=True")
	err = WaitForMCNConditionStatus(clientSet, masterNode.Name, mcfgv1alpha1.MachineConfigNodeUpdateComplete, metav1.ConditionTrue, 10*time.Second, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred())
	framework.Logf("Waiting for Uncordoned=True")
	err = WaitForMCNConditionStatus(clientSet, masterNode.Name, mcfgv1alpha1.MachineConfigNodeUpdateUncordoned, metav1.ConditionTrue, 10*time.Second, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred())
	framework.Logf("Waiting for Updated=True")
	err = WaitForMCNConditionStatus(clientSet, masterNode.Name, mcfgv1alpha1.MachineConfigNodeUpdated, metav1.ConditionTrue, 1*time.Minute, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred())

	// When an update is complete, all conditions other than `Updated` must be false
	framework.Logf("Checking all conditions other than 'Updated' are False.")
	o.Expect(ConfirmUpdatedMCNStatus(clientSet, masterNode.Name)).Should(o.BeTrue())
}

// `ValidateMCNConditionOnNodeDegrade` checks that Conditions properly update on a node failure (MCP degrade)
func ValidateMCNConditionOnNodeDegrade(oc *exutil.CLI, fixture string, isSno bool) {
	// Create client set for test
	clientSet, clientErr := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(clientErr).NotTo(o.HaveOccurred())

	// In SNO, master pool will degrade
	poolName := worker
	mcName := "91-worker-testfile-invalid"
	if isSno {
		poolName = master
		mcName = "91-master-testfile-invalid"
	}

	// Cleanup MC and fix node degradation on failure or test completion
	defer func() {
		// Delete the applied MC
		deleteMCErr := oc.Run("delete").Args("machineconfig", mcName).Execute()
		o.Expect(deleteMCErr).NotTo(o.HaveOccurred())

		// Recover the degraded MCP
		recoverErr := RecoverFromDegraded(oc, poolName)
		o.Expect(recoverErr).NotTo(o.HaveOccurred())
	}()

	// Apply invalid MC
	mcErr := oc.Run("apply").Args("-f", fixture).Execute()
	o.Expect(mcErr).NotTo(o.HaveOccurred())

	// Wait for MCP to be in a degraded state with one degraded machine
	o.Expect(WaitForMCPConditionStatus(oc, poolName, "Degraded", corev1.ConditionTrue, 8*time.Minute, 3*time.Second)).NotTo(o.HaveOccurred(), fmt.Sprintf("Error waiting for '%v' MCP to be in a degraded state.", poolName))
	mcp, err := clientSet.MachineconfigurationV1().MachineConfigPools().Get(context.TODO(), poolName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Error getting '%v' MCP.", poolName)
	o.Expect(mcp.Status.DegradedMachineCount).To(o.BeNumerically("==", 1), fmt.Sprintf("Degraded machine count is not 1. It is %v.", mcp.Status.DegradedMachineCount))

	// Get degraded node
	degradedNode, degradedNodeErr := GetDegradedNode(oc, poolName)
	o.Expect(degradedNodeErr).NotTo(o.HaveOccurred())

	// Validate MCN of degraded node
	degradedNodeMCN, degradedErr := clientSet.MachineconfigurationV1alpha1().MachineConfigNodes().Get(context.TODO(), degradedNode.Name, metav1.GetOptions{})
	o.Expect(degradedErr).NotTo(o.HaveOccurred())
	framework.Logf("Validating that `AppliedFilesAndOS` and `UpdateExecuted` conditions in '%v' MCN have a status of 'Unknown'.", degradedNodeMCN.Name)
	o.Expect(CheckMCNConditionStatus(degradedNodeMCN, mcfgv1alpha1.MachineConfigNodeUpdateFilesAndOS, metav1.ConditionUnknown))
	o.Expect(CheckMCNConditionStatus(degradedNodeMCN, mcfgv1alpha1.MachineConfigNodeUpdateExecuted, metav1.ConditionUnknown))
}

// `ValidateMCNProperties` checks that MCNs with correct properties are created on node creation
// and deleted on node deletion
func ValidateMCNOnNodeCreationAndDeletion(oc *exutil.CLI) {
	testFailed := true
	newNode := corev1.Node{}
	newMachineName := ""

	// Create machine client for test
	machineClient, machineErr := machineclient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(machineErr).NotTo(o.HaveOccurred())

	// Create client set for test
	clientSet, clientErr := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(clientErr).NotTo(o.HaveOccurred())

	// Skip test if worker nodes cannot be scaled
	canBeScaled, canScaleErr := WorkersCanBeScaled(oc, machineClient)
	o.Expect(canScaleErr).NotTo(o.HaveOccurred())
	if !canBeScaled {
		g.Skip("Worker nodes cannot be scaled using MachineSets. This test cannot be executed if workers cannot be scaled via MachineSets.")
	}

	// Get MachineSet for test
	framework.Logf("Getting MachineSet for testing.")
	machineSet := getRandomMachineSet(machineClient)
	framework.Logf("MachineSet '%s' will be used for testing", machineSet.Name)
	originalReplica := int(*machineSet.Spec.Replicas)
	originalDeletionPolicy := machineSet.Spec.DeletePolicy
	// TODO: remove post testing
	framework.Logf("originalDeletionPolicy: %v", originalDeletionPolicy)

	// Rollback replica in test MachineSet on test failure
	defer func() {
		CleanupMachineSetScale(oc, machineClient, machineSet.Name, originalReplica, originalDeletionPolicy, newNode.Name, newMachineName, testFailed)
	}()

	// Create node by scaling MachineSet
	framework.Logf("Scaling up MachineSet to create node.")
	updatedReplica := originalReplica + 1
	scaleErr := ScaleMachineSet(oc, machineSet.Name, fmt.Sprintf("%d", updatedReplica))
	o.Expect(scaleErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Error provisioning node by scaling MachineSet %v to replica value %v.", machineSet.Name, updatedReplica))
	deletePolicyErr := UpdateDeletePolicy(oc, machineSet.Name, "Newest")
	o.Expect(deletePolicyErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Error updating MachineSet %v delete policy to original value %v.", machineSet.Name, originalDeletionPolicy))

	// Get the newly created node
	framework.Logf("Getting the new machine.")
	provisioningMachine, provisioningMachineErr := GetMachinesByPhase(machineClient, machineSet.Name, "Provisioning")
	o.Expect(provisioningMachineErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Cannot find provisioning machine in MachineSet %v", machineSet.Name))
	newMachineName = provisioningMachine.Name
	framework.Logf("Waiting for new machine %v to be ready.", newMachineName)
	WaitForMachineInState(machineClient, newMachineName, "Running")
	framework.Logf("Getting new node in machine %v.", newMachineName)
	newNode, nodeErr := GetNewReadyNodeInMachine(oc, newMachineName)
	o.Expect(nodeErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Cannot find provisioning node in Machine %v", newMachineName))
	framework.Logf("Got new node: %v.", newNode.Name)

	// Validate new MCN
	validMCNErr := WaitForValidMCNProperties(clientSet, newNode)
	o.Expect(validMCNErr).NotTo(o.HaveOccurred())

	// Scale down the MachineSet to delete the created node
	framework.Logf("Scaling down MachineSet to delete node.")
	scaleErr = ScaleMachineSet(oc, machineSet.Name, fmt.Sprintf("%v", originalReplica))
	o.Expect(scaleErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Error deleting node by scaling MachineSet %v to replica value %v.", machineSet.Name, originalReplica))

	// Wait for created node to delete
	framework.Logf("Waiting for node '%v' to be deleted.", newNode.Name)
	o.Expect(WaitForNodeToBeDeleted(oc, newNode.Name), fmt.Sprintf("Error deleting node '%v'.", newNode.Name))

	// Check that corresponding MCN is removed alongside node
	o.Expect(WaitForMCNToBeDeleted(clientSet, newNode.Name), fmt.Sprintf("Error deleting MCN '%v'.", newNode.Name))

	// Return MachineSet deletion policy back to original
	deletePolicyErr = UpdateDeletePolicy(oc, machineSet.Name, originalDeletionPolicy)
	o.Expect(deletePolicyErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Error updating MachineSet %v delete policy to original value %v.", machineSet.Name, originalDeletionPolicy))

	testFailed = false
}

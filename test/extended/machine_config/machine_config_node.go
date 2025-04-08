package machine_config

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
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
		customMCFixture             = filepath.Join(MCOMachineConfigBaseDir, "0-infra-mc.yaml")
		masterMCFixture             = filepath.Join(MCOMachineConfigBaseDir, "0-master-mc.yaml")
		invalidWorkerMCFixture      = filepath.Join(MCOMachineConfigBaseDir, "1-worker-invalid-mc.yaml")
		invalidMasterMCFixture      = filepath.Join(MCOMachineConfigBaseDir, "1-master-invalid-mc.yaml")
		oc                          = exutil.NewCLIWithoutNamespace("machine-config")
	)

	g.It("[Serial]Should have MCN properties matching associated node properties [apigroup:machineconfiguration.openshift.io]", func() {
		// Skip test when there are errors connecting to the cluster
		SkipOnConnectionError(oc)

		if IsSingleNode(oc) { //handle SNO clusters
			ValidateMCNPropertiesSNO(oc, infraMCPFixture)
		} else { //handle standard, non-SNO, clusters
			ValidateMCNProperties(oc, infraMCPFixture)
		}
	})

	g.It("[Serial]Should properly transition through MCN conditions on node update [apigroup:machineconfiguration.openshift.io]", func() {
		// Skip test when there are errors connecting to the cluster
		SkipOnConnectionError(oc)

		if IsSingleNode(oc) {
			ValidateMCNConditionTransitionsSNO(oc, masterMCFixture)
		} else {
			ValidateMCNConditionTransitions(oc, customMCFixture, infraMCPFixture)
		}
	})

	g.It("[Serial][Slow]Should properly report MCN conditions on node degrade [apigroup:machineconfiguration.openshift.io]", func() {
		// Skip test when there are errors connecting to the cluster
		SkipOnConnectionError(oc)

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

	g.It("Should properly block MCN updates from a MCD that is not the associated one [apigroup:machineconfiguration.openshift.io]", func() {
		skipOnSingleNodeTopology(oc) //skip this test for SNO
		ValidateMCNScopeSadPathTest(oc)
	})

	g.It("Should properly block MCN updates by impersonation of the MCD SA [apigroup:machineconfiguration.openshift.io]", func() {
		skipOnSingleNodeTopology(oc) //skip this test for SNO
		ValidateMCNScopeImpersonationPathTest(oc)
	})

	g.It("Should properly update the MCN from the associated MCD [apigroup:machineconfiguration.openshift.io]", func() {
		skipOnSingleNodeTopology(oc) //skip this test for SNO
		ValidateMCNScopeHappyPathTest(oc)
	})
})

// `ValidateMCNProperties` checks that MCN properties match the corresponding node properties
// Note: This test case does not work for SNO clusters due to the cluster's one node assuming
// both the worker and master role since `GetRandomNode` selects nodes using node roles. Role
// matching is not necessarily synonymous with MCP association in edge cases, such as in SNO.
func ValidateMCNProperties(oc *exutil.CLI, fixture string) {
	// Create client set for test
	clientSet, clientErr := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(clientErr).NotTo(o.HaveOccurred(), "Error creating client set for test.")

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
		o.Expect(err).NotTo(o.HaveOccurred(), "Could not get worker MCP.")
		workerMcpReadyMachines := workerMcp.Status.ReadyMachineCount

		// Unlabel node
		framework.Logf("Removing label node-role.kubernetes.io/%v from node %v", custom, workerNode.Name)
		unlabelErr := oc.Run("label").Args(fmt.Sprintf("node/%s", workerNode.Name), fmt.Sprintf("node-role.kubernetes.io/%s-", custom)).Execute()
		o.Expect(unlabelErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Could not remove label 'node-role.kubernetes.io/%s' from node '%v'.", custom, workerNode.Name))

		// Wait for infra pool to report no nodes & for worker MCP to be ready
		framework.Logf("Waiting for %v MCP to be updated with %v ready machines.", custom, 0)
		WaitForMCPToBeReady(oc, clientSet, custom, 0)
		framework.Logf("Waiting for %v MCP to be updated with %v ready machines.", worker, workerMcpReadyMachines+1)
		WaitForMCPToBeReady(oc, clientSet, worker, workerMcpReadyMachines+1)

		// Delete custom MCP
		framework.Logf("Deleting MCP %v", custom)
		deleteMCPErr := oc.Run("delete").Args("mcp", custom).Execute()
		o.Expect(deleteMCPErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Error deleting MCP '%v': %v", custom, deleteMCPErr))
	}()

	// Apply the fixture to create a custom MCP called "infra" & label the worker node accordingly
	mcpErr := oc.Run("apply").Args("-f", fixture).Execute()
	o.Expect(mcpErr).NotTo(o.HaveOccurred(), "Could not create custom MCP.")
	labelErr := oc.Run("label").Args(fmt.Sprintf("node/%s", workerNode.Name), fmt.Sprintf("node-role.kubernetes.io/%s=", custom)).Execute()
	o.Expect(labelErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Could not add label 'node-role.kubernetes.io/%s' to node '%v'.", custom, workerNode.Name))

	// Wait for the custom pool to be updated with the node ready
	framework.Logf("Waiting for '%v' MCP to be updated with %v ready machines.", custom, 1)
	WaitForMCPToBeReady(oc, clientSet, custom, 1)

	// Get node in custom pool
	customNodes, customNodeErr := GetNodesByRole(oc, custom)
	o.Expect(customNodeErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Could not get node in MCP '%v'.", custom))
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
	o.Expect(clientErr).NotTo(o.HaveOccurred(), "Error creating client set for test.")

	// Grab the cluster's node
	node := GetRandomNode(oc, master)
	o.Expect(node.Name).NotTo(o.Equal(""), "Could not get a worker node.")

	// Validate MCN for the cluster's node
	framework.Logf("Validating MCN properties for the node in pool '%v'.", master)
	mcnErr := ValidateMCNForNodeInPool(oc, clientSet, node, master)
	o.Expect(mcnErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Error validating MCN properties for the node in pool '%v'.", master))
}

// `ValidateMCNConditionTransitions` checks that Conditions properly update on a node update
// Note that a custom MCP is created for this test to limit the number of upgrading nodes &
// decrease cleanup time.
func ValidateMCNConditionTransitions(oc *exutil.CLI, mcFixture string, mcpFixture string) {
	poolName := custom
	mcName := fmt.Sprintf("90-%v-testfile", poolName)

	// Create client set for test
	clientSet, clientErr := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(clientErr).NotTo(o.HaveOccurred(), "Error creating client set for test.")

	// Grab a random worker node
	workerNode := GetRandomNode(oc, worker)
	o.Expect(workerNode.Name).NotTo(o.Equal(""), "Could not get a worker node.")

	// Cleanup custom MCP and delete MC on failure or test completion
	defer func() {
		// Get starting state of default worker MCP
		workerMcp, err := clientSet.MachineconfigurationV1().MachineConfigPools().Get(context.TODO(), worker, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Could not get worker MCP.")
		workerMcpReadyMachines := workerMcp.Status.ReadyMachineCount

		// Unlabel node
		framework.Logf("Removing label node-role.kubernetes.io/%v from node %v", custom, workerNode.Name)
		unlabelErr := oc.Run("label").Args(fmt.Sprintf("node/%s", workerNode.Name), fmt.Sprintf("node-role.kubernetes.io/%s-", custom)).Execute()
		o.Expect(unlabelErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Could not remove label 'node-role.kubernetes.io/%s' from node '%v'.", custom, workerNode.Name))

		// Wait for infra MCP to report no ready nodes
		framework.Logf("Waiting for %v MCP to be updated with %v ready machines.", custom, 0)
		WaitForMCPToBeReady(oc, clientSet, custom, 0)

		// Delete applied MC
		deleteMCErr := oc.Run("delete").Args("machineconfig", mcName).Execute()
		o.Expect(deleteMCErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Could not delete MachineConfig '%v'.", mcName))

		// Wait for worker MCP to be ready
		framework.Logf("Waiting for %v MCP to be updated with %v ready machines.", worker, workerMcpReadyMachines+1)
		WaitForMCPToBeReady(oc, clientSet, worker, workerMcpReadyMachines+1)

		// Delete custom MCP
		framework.Logf("Deleting MCP %v", custom)
		deleteMCPErr := oc.Run("delete").Args("mcp", custom).Execute()
		o.Expect(deleteMCPErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Error deleting MCP '%v': %v", custom, deleteMCPErr))
	}()

	// Apply the fixture to create a custom MCP called "infra" & label the worker node accordingly
	mcpErr := oc.Run("apply").Args("-f", mcpFixture).Execute()
	o.Expect(mcpErr).NotTo(o.HaveOccurred(), "Could not create custom MCP.")
	labelErr := oc.Run("label").Args(fmt.Sprintf("node/%s", workerNode.Name), fmt.Sprintf("node-role.kubernetes.io/%s=", custom)).Execute()
	o.Expect(labelErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Could not add label 'node-role.kubernetes.io/%s' to node '%v'.", custom, workerNode.Name))

	// Apply MC targeting custom pool node
	mcErr := oc.Run("apply").Args("-f", mcFixture).Execute()
	o.Expect(mcErr).NotTo(o.HaveOccurred(), "Could not apply MachineConfig.")
	updatingNodeName := workerNode.Name

	// Validate transition through conditions for MCN
	validateTransitionThroughConditions(clientSet, updatingNodeName)

	// When an update is complete, all conditions other than `Updated` must be false
	framework.Logf("Checking all conditions other than 'Updated' are False.")
	o.Expect(ConfirmUpdatedMCNStatus(clientSet, updatingNodeName)).Should(o.BeTrue(), "Error, all conditions must be 'False' when Updated=True.")
}

// `ValidateMCNConditionTransitionsSNO` checks that Conditions properly update on a node update
// in Single Node Openshift
func ValidateMCNConditionTransitionsSNO(oc *exutil.CLI, mcFixture string) {
	poolName := master
	mcName := fmt.Sprintf("90-%v-testfile", poolName)

	// Create client set for test
	clientSet, clientErr := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(clientErr).NotTo(o.HaveOccurred(), "Error creating client set for test.")

	// Delete MC on failure or test completion
	defer func() {
		deleteMCErr := oc.Run("delete").Args("machineconfig", mcName).Execute()
		o.Expect(deleteMCErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Could not delete MachineConfig '%v'.", mcName))
	}()

	// Apply MC targeting worker node
	mcErr := oc.Run("apply").Args("-f", mcFixture).Execute()
	o.Expect(mcErr).NotTo(o.HaveOccurred(), "Could not apply MachineConfig.")

	// Get the first updating node
	updatingNodes := GetCordonedNodes(oc, poolName)
	o.Expect(len(updatingNodes) > 0).Should(o.BeTrue(), fmt.Sprintf("No ready nodes found for MCP '%v'.", poolName))
	updatingNode := updatingNodes[0]

	// Validate transition through conditions for MCN
	validateTransitionThroughConditions(clientSet, updatingNode.Name)

	// When an update is complete, all conditions other than `Updated` must be false
	framework.Logf("Checking all conditions other than 'Updated' are False.")
	o.Expect(ConfirmUpdatedMCNStatus(clientSet, updatingNode.Name)).Should(o.BeTrue(), "Error, all conditions must be 'False' when Updated=True.")
}

// `validateTransitionThroughConditions` validates the condition trasnitions in the MCN during a node update
func validateTransitionThroughConditions(clientSet *machineconfigclient.Clientset, updatingNodeName string) {
	// Note that some conditions are passed through quickly in a node update, so the test can
	// "miss" catching the phases. For test stability, if we fail to catch an "Unknown" status,
	// a warning will be logged instead of erroring out the test.
	framework.Logf("Waiting for Updated=False")
	err := WaitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1alpha1.MachineConfigNodeUpdated, metav1.ConditionFalse, 1*time.Minute, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred(), "Error, could not detect Updated=False.")
	framework.Logf("Waiting for UpdatePrepared=True")
	err = WaitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1alpha1.MachineConfigNodeUpdatePrepared, metav1.ConditionTrue, 1*time.Minute, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred(), "Error, could not detect UpdatePrepared=True.")
	framework.Logf("Waiting for UpdateExecuted=Unknown")
	err = WaitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1alpha1.MachineConfigNodeUpdateExecuted, metav1.ConditionUnknown, 30*time.Second, 1*time.Second)
	if err != nil {
		framework.Logf("Warning, could not detect UpdateExecuted=Unknown.")
	}
	framework.Logf("Waiting for Cordoned=True")
	err = WaitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1alpha1.MachineConfigNodeUpdateCordoned, metav1.ConditionTrue, 30*time.Second, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred(), "Error, could not detect Cordoned=True.")
	framework.Logf("Waiting for Drained=Unknown")
	err = WaitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1alpha1.MachineConfigNodeUpdateDrained, metav1.ConditionUnknown, 15*time.Second, 1*time.Second)
	if err != nil {
		framework.Logf("Warning, could not detect Drained=Unknown.")
	}
	framework.Logf("Waiting for Drained=True")
	err = WaitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1alpha1.MachineConfigNodeUpdateDrained, metav1.ConditionTrue, 4*time.Minute, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred(), "Error, could not detect Drained=True.")
	framework.Logf("Waiting for AppliedFilesAndOS=Unknown")
	err = WaitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1alpha1.MachineConfigNodeUpdateFilesAndOS, metav1.ConditionUnknown, 30*time.Second, 1*time.Second)
	if err != nil {
		framework.Logf("Warning, could not detect AppliedFilesAndOS=Unknown.")
	}
	framework.Logf("Waiting for AppliedFilesAndOS=True")
	err = WaitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1alpha1.MachineConfigNodeUpdateFilesAndOS, metav1.ConditionTrue, 3*time.Minute, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred(), "Error, could not detect AppliedFilesAndOS=True.")
	framework.Logf("Waiting for UpdateExecuted=True")
	err = WaitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1alpha1.MachineConfigNodeUpdateExecuted, metav1.ConditionTrue, 20*time.Second, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred(), "Error, could not detect UpdateExecuted=True.")
	framework.Logf("Waiting for UpdatePostActionComplete=Unknown")
	err = WaitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1alpha1.MachineConfigNodeUpdatePostActionComplete, metav1.ConditionUnknown, 30*time.Second, 1*time.Second)
	if err != nil {
		framework.Logf("Warning, could not detect UpdatePostActionComplete=Unknown.")
	}
	framework.Logf("Waiting for RebootedNode=Unknown")
	err = WaitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1alpha1.MachineConfigNodeUpdateRebooted, metav1.ConditionUnknown, 15*time.Second, 1*time.Second)
	if err != nil {
		framework.Logf("Warning, could not detect RebootedNode=Unknown.")
	}
	framework.Logf("Waiting for RebootedNode=True")
	err = WaitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1alpha1.MachineConfigNodeUpdateRebooted, metav1.ConditionTrue, 5*time.Minute, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred(), "Error, could not detect RebootedNode=True.")
	framework.Logf("Waiting for Resumed=True")
	err = WaitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1alpha1.MachineConfigNodeResumed, metav1.ConditionTrue, 15*time.Second, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred(), "Error, could not detect Resumed=True.")
	framework.Logf("Waiting for UpdateComplete=True")
	err = WaitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1alpha1.MachineConfigNodeUpdateComplete, metav1.ConditionTrue, 10*time.Second, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred(), "Error, could not detect UpdateComplete=True.")
	framework.Logf("Waiting for Uncordoned=True")
	err = WaitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1alpha1.MachineConfigNodeUpdateUncordoned, metav1.ConditionTrue, 10*time.Second, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred(), "Error, could not detect Uncordoned=True.")
	framework.Logf("Waiting for Updated=True")
	err = WaitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1alpha1.MachineConfigNodeUpdated, metav1.ConditionTrue, 1*time.Minute, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred(), "Error, could not detect Updated=True.")
}

// `ValidateMCNConditionOnNodeDegrade` checks that Conditions properly update on a node failure (MCP degrade)
func ValidateMCNConditionOnNodeDegrade(oc *exutil.CLI, fixture string, isSno bool) {
	// Create client set for test
	clientSet, clientErr := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(clientErr).NotTo(o.HaveOccurred(), "Error creating client set for test.")

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
		o.Expect(deleteMCErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Could not delete MachineConfig '%v'.", mcName))

		// Recover the degraded MCP
		recoverErr := RecoverFromDegraded(oc, poolName)
		o.Expect(recoverErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Could not recover MCP '%v' from degraded state.", poolName))
	}()

	// Apply invalid MC
	mcErr := oc.Run("apply").Args("-f", fixture).Execute()
	o.Expect(mcErr).NotTo(o.HaveOccurred(), "Could not apply MachineConfig.")

	// Wait for MCP to be in a degraded state with one degraded machine
	degradedErr := WaitForMCPConditionStatus(oc, poolName, "Degraded", corev1.ConditionTrue, 8*time.Minute, 3*time.Second)
	o.Expect(degradedErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Error waiting for '%v' MCP to be in a degraded state.", poolName))
	mcp, err := clientSet.MachineconfigurationV1().MachineConfigPools().Get(context.TODO(), poolName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error getting '%v' MCP.", poolName))
	o.Expect(mcp.Status.DegradedMachineCount).To(o.BeNumerically("==", 1), fmt.Sprintf("Degraded machine count is not 1. It is %v.", mcp.Status.DegradedMachineCount))

	// Get degraded node
	degradedNode, degradedNodeErr := GetDegradedNode(oc, poolName)
	o.Expect(degradedNodeErr).NotTo(o.HaveOccurred(), "Could not get degraded node.")

	// Validate MCN of degraded node
	degradedNodeMCN, degradedErr := clientSet.MachineconfigurationV1alpha1().MachineConfigNodes().Get(context.TODO(), degradedNode.Name, metav1.GetOptions{})
	o.Expect(degradedErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Error getting MCN of degraded node '%v'.", degradedNode.Name))
	framework.Logf("Validating that `AppliedFilesAndOS` and `UpdateExecuted` conditions in '%v' MCN have a status of 'Unknown'.", degradedNodeMCN.Name)
	o.Expect(CheckMCNConditionStatus(degradedNodeMCN, mcfgv1alpha1.MachineConfigNodeUpdateFilesAndOS, metav1.ConditionUnknown)).Should(o.BeTrue(), "Condition 'AppliedFilesAndOS' does not have the expected status of 'Unknown'.")
	o.Expect(CheckMCNConditionStatus(degradedNodeMCN, mcfgv1alpha1.MachineConfigNodeUpdateExecuted, metav1.ConditionUnknown)).Should(o.BeTrue(), "Condition 'UpdateExecuted' does not have the expected status of 'Unknown'.")
}

// `ValidateMCNProperties` checks that MCNs with correct properties are created on node creation
// and deleted on node deletion
func ValidateMCNOnNodeCreationAndDeletion(oc *exutil.CLI) {
	cleanupCompleted := false

	// Create machine client for test
	machineClient, machineErr := machineclient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(machineErr).NotTo(o.HaveOccurred(), "Error creating machine client for test.")

	// Create client set for test
	clientSet, clientErr := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(clientErr).NotTo(o.HaveOccurred(), "Error creating client set for test.")

	// Skip test if worker nodes cannot be scaled
	canBeScaled, canScaleErr := WorkersCanBeScaled(oc, machineClient)
	o.Expect(canScaleErr).NotTo(o.HaveOccurred(), "Error occured when determining whether worker nodes can be scaled.")
	if !canBeScaled {
		g.Skip("Worker nodes cannot be scaled using MachineSets. This test cannot be executed if workers cannot be scaled via MachineSets.")
	}

	// Get MachineSet for test
	framework.Logf("Getting MachineSet for testing.")
	machineSet := getRandomMachineSet(machineClient)
	framework.Logf("MachineSet '%s' will be used for testing", machineSet.Name)
	originalReplica := int(*machineSet.Spec.Replicas)

	// Create node by scaling MachineSet
	framework.Logf("Scaling up MachineSet to create node.")
	updatedReplica := originalReplica + 1
	scaleErr := ScaleMachineSet(oc, machineSet.Name, fmt.Sprintf("%d", updatedReplica))
	o.Expect(scaleErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Error scaling MachineSet %v to replica value %v.", machineSet.Name, updatedReplica))

	// If we fail at this point, cleanup should include scaling the MachineSet replica back down to the
	// original value, when needed (in the case where the replica value patch was successful).
	defer func() {
		cleanupErr := ScaleMachineSetDown(oc, machineSet, originalReplica, cleanupCompleted)
		o.Expect(cleanupErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Error cleaning up cluster by scaling down MachineSet '%v'.", machineSet.Name))
		cleanupCompleted = true
	}()

	// Get the new machine
	framework.Logf("Getting the new machine.")
	provisioningMachine, provisioningMachineErr := GetMachinesByPhase(machineClient, machineSet.Name, "Provisioning")
	o.Expect(provisioningMachineErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Cannot find provisioning machine in MachineSet %v", machineSet.Name))
	newMachineName := provisioningMachine.Name

	// If we fail past this point, cleanup should include scaling the MachineSet replica back down to the
	// original value and ensuring that the newly provisioned Machine is deleted.
	defer func() {
		cleanupErr := CleanupProvisionedMachine(oc, machineClient, machineSet.Name, originalReplica, newMachineName, cleanupCompleted)
		o.Expect(cleanupErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Error removing provisioned Machine '%v' by scaling down MachineSet '%v'.", newMachineName, machineSet.Name))
		cleanupCompleted = true
	}()

	// Annotate the machine so it is deleted on the MachineSet scale down
	framework.Logf("Updating delete-machine annotation on Machine '%v' to be 'true'.", newMachineName)
	deleteAnnotationErr := UpdateDeleteMachineAnnotation(oc, newMachineName)
	o.Expect(deleteAnnotationErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Error updating delete-machine annotation for machine '%v'.", newMachineName))

	// Wait for new Machine to be ready
	framework.Logf("Waiting for new machine %v to be ready.", newMachineName)
	WaitForMachineInState(machineClient, newMachineName, "Running")

	// Get the new node
	framework.Logf("Getting new node in machine %v.", newMachineName)
	newNode, nodeErr := GetNewReadyNodeInMachine(oc, newMachineName)
	o.Expect(nodeErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Cannot find provisioning node in Machine %v", newMachineName))
	framework.Logf("Got new node: %v.", newNode.Name)

	// If we fail past this point, cleanup should include scaling the MachineSet replica back down to the
	// original value and ensuring that the newly created Node is deleted.
	defer func() {
		cleanupErr := CleanupCreatedNode(oc, newMachineName, originalReplica, newNode.Name, cleanupCompleted)
		o.Expect(cleanupErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Error removing created Node '%v' by scaling down MachineSet '%v'.", newNode.Name, machineSet.Name))
		cleanupCompleted = true
	}()

	// Validate new MCN
	validMCNErr := WaitForValidMCNProperties(clientSet, newNode)
	o.Expect(validMCNErr).NotTo(o.HaveOccurred(), fmt.Sprintf("MCN for node '%v' has invalid properties.", newNode))

	// Scale down the MachineSet to delete the created node
	framework.Logf("Scaling down MachineSet to delete node.")
	scaleErr = ScaleMachineSet(oc, machineSet.Name, fmt.Sprintf("%v", originalReplica))
	o.Expect(scaleErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Error deleting node by scaling MachineSet %v to replica value %v.", machineSet.Name, originalReplica))

	// Wait for created node to delete
	framework.Logf("Waiting for node '%v' to be deleted.", newNode.Name)
	o.Expect(WaitForNodeToBeDeleted(oc, newNode.Name)).NotTo(o.HaveOccurred(), fmt.Sprintf("Error deleting node '%v'.", newNode.Name))

	// Check that corresponding MCN is removed alongside node
	o.Expect(WaitForMCNToBeDeleted(clientSet, newNode.Name)).NotTo(o.HaveOccurred(), fmt.Sprintf("Error deleting MCN '%v'.", newNode.Name))

	// If we successfully make it here, no cleanup is required
	cleanupCompleted = true
}

// `ValidateMCNScopeSadPathTest` checks that MCN updates from a MCD that is not the associated one are blocked
func ValidateMCNScopeSadPathTest(oc *exutil.CLI) {
	// Grab two random nodes from different pools, so we don't end up testing and targeting the same node.
	nodeUnderTest := GetRandomNode(oc, "worker")
	targetNode := GetRandomNode(oc, "master")

	// Attempt to patch the MCN owned by targetNode from nodeUnderTest's MCD. This should fail.
	// This oc command effectively use the service account of the nodeUnderTest's MCD pod, which should only be able to edit nodeUnderTest's MCN.
	cmdOutput, err := ExecCmdOnNodeWithError(oc, nodeUnderTest, "chroot", "/rootfs", "oc", "patch", "machineconfignodes", targetNode.Name, "--type=merge", "-p", "{\"spec\":{\"configVersion\":{\"desired\":\"rendered-worker-test\"}}}")

	o.Expect(err).To(o.HaveOccurred())
	o.Expect(cmdOutput).To(o.ContainSubstring("updates to MCN " + targetNode.Name + " can only be done from the MCN's owner node"))
}

// `ValidateMCNScopeSadPathTest` checks that MCN updates by impersonation of the MCD SA are blocked
func ValidateMCNScopeImpersonationPathTest(oc *exutil.CLI) {
	// Grab a random node from the worker pool
	nodeUnderTest := GetRandomNode(oc, "worker")

	var errb bytes.Buffer
	// Attempt to patch the MCN owned by nodeUnderTest by impersonating the MCD SA. This should fail.
	cmd := exec.Command("oc", "patch", "machineconfignodes", nodeUnderTest.Name, "--type=merge", "-p", "{\"spec\":{\"configVersion\":{\"desired\":\"rendered-worker-test\"}}}", "--as=system:serviceaccount:openshift-machine-config-operator:machine-config-daemon")
	cmd.Stderr = &errb
	err := cmd.Run()

	o.Expect(err).To(o.HaveOccurred())
	o.Expect(errb.String()).To(o.ContainSubstring("this user must have a \"authentication.kubernetes.io/node-name\" claim"))

}

// `ValidateMCNScopeSadPathTest` checks that MCN updates from the associated MCD are allowed
func ValidateMCNScopeHappyPathTest(oc *exutil.CLI) {

	// Grab a random node from the worker pool
	nodeUnderTest := GetRandomNode(oc, "worker")

	// Attempt to patch the MCN owned by nodeUnderTest from nodeUnderTest's MCD. This should succeed.
	// This oc command effectively use the service account of the nodeUnderTest's MCD pod, which should only be able to edit nodeUnderTest's MCN.
	ExecCmdOnNode(oc, nodeUnderTest, "chroot", "/rootfs", "oc", "patch", "machineconfignodes", nodeUnderTest.Name, "--type=merge", "-p", "{\"spec\":{\"configVersion\":{\"desired\":\"rendered-worker-test\"}}}")
}

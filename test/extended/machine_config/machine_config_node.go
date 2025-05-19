package machine_config

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"

	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	machineclient "github.com/openshift/client-go/machine/clientset/versioned"
	machineconfigclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
)

const (
	worker  = "worker"
	master  = "master"
	custom  = "infra"
	arbiter = "arbiter"
)

var _ = g.Describe("[sig-mco][OCPFeatureGate:MachineConfigNodes]", func() {
	defer g.GinkgoRecover()
	var (
		MCOMachineConfigPoolBaseDir    = exutil.FixturePath("testdata", "machine_config", "machineconfigpool")
		MCOMachineConfigurationBaseDir = exutil.FixturePath("testdata", "machine_config", "machineconfigurations")
		MCOMachineConfigBaseDir        = exutil.FixturePath("testdata", "machine_config", "machineconfig")
		infraMCPFixture                = filepath.Join(MCOMachineConfigPoolBaseDir, "infra-mcp.yaml")
		nodeDisruptionFixture          = filepath.Join(MCOMachineConfigurationBaseDir, "nodedisruptionpolicy-rebootless-path.yaml")
		nodeDisruptionEmptyFixture     = filepath.Join(MCOMachineConfigurationBaseDir, "managedbootimages-empty.yaml")
		customMCFixture                = filepath.Join(MCOMachineConfigBaseDir, "0-infra-mc.yaml")
		masterMCFixture                = filepath.Join(MCOMachineConfigBaseDir, "0-master-mc.yaml")
		invalidWorkerMCFixture         = filepath.Join(MCOMachineConfigBaseDir, "1-worker-invalid-mc.yaml")
		invalidMasterMCFixture         = filepath.Join(MCOMachineConfigBaseDir, "1-master-invalid-mc.yaml")
		oc                             = exutil.NewCLIWithoutNamespace("machine-config")
	)

	g.BeforeEach(func(ctx context.Context) {
		//skip these tests on hypershift platforms
		if ok, _ := exutil.IsHypershift(ctx, oc.AdminConfigClient()); ok {
			g.Skip("MachineConfigNodes is not supported on hypershift. Skipping tests.")
		}
	})

	g.It("Should have MCN properties matching associated node properties for nodes in default MCPs [apigroup:machineconfiguration.openshift.io]", func() {
		if IsSingleNode(oc) || IsTwoNode(oc) { //handle SNO & two-node clusters
			// In SNO and standard two-node openshift clusters, the nodes have both worker and master roles, but are a part
			// of the master MCP. Thus, the tests for these clusters will be limited to checking master MCP association.
			ValidateMCNPropertiesByMCPs(oc, []string{master})
		} else if IsTwoNodeArbiter(oc) { //handle two-node arbiter clusters
			// In two-node arbiter openshift clusters, there are two nodes have both worker and master roles, but are a part
			// of the master MCP. There is also a third "arbiter" node. Thus, these clusters should be tests for both master
			// and arbiter MCP association.
			ValidateMCNPropertiesByMCPs(oc, []string{master, arbiter})
		} else { //handle standard clusters
			ValidateMCNPropertiesByMCPs(oc, []string{master, worker})
		}
	})

	g.It("[Serial]Should have MCN properties matching associated node properties for nodes in custom MCPs [apigroup:machineconfiguration.openshift.io]", func() {
		skipOnSingleNodeTopology(oc) //skip this test for SNO
		skipOnTwoNodeTopology(oc)    //skip this test for two-node openshift
		ValidateMCNPropertiesCustomMCP(oc, infraMCPFixture)
	})

	g.It("[Serial]Should properly transition through MCN conditions on rebootless node update [apigroup:machineconfiguration.openshift.io]", func() {
		if IsSingleNode(oc) {
			ValidateMCNConditionTransitionsOnRebootlessUpdateSNO(oc, nodeDisruptionFixture, nodeDisruptionEmptyFixture, masterMCFixture)
		} else {
			ValidateMCNConditionTransitionsOnRebootlessUpdate(oc, nodeDisruptionFixture, nodeDisruptionEmptyFixture, customMCFixture, infraMCPFixture)
		}
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

	g.It("Should properly block MCN updates from a MCD that is not the associated one [apigroup:machineconfiguration.openshift.io]", func() {
		ValidateMCNScopeSadPathTest(oc)
	})

	g.It("Should properly block MCN updates by impersonation of the MCD SA [apigroup:machineconfiguration.openshift.io]", func() {
		ValidateMCNScopeImpersonationPathTest(oc)
	})

	g.It("Should properly update the MCN from the associated MCD [apigroup:machineconfiguration.openshift.io]", func() {
		ValidateMCNScopeHappyPathTest(oc)
	})
})

// `ValidateMCNPropertiesByMCPs` checks that MCN properties match the corresponding node properties
// for a random node in each of the desired MCPs.
func ValidateMCNPropertiesByMCPs(oc *exutil.CLI, poolNames []string) {
	framework.Logf("Validating MCN properties for node(s) in pool(s) '%v'.", poolNames)

	// Create client set for test
	clientSet, clientErr := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(clientErr).NotTo(o.HaveOccurred(), "Error creating client set for test.")

	// Validate MCN associated with node in each desired MCP
	for _, poolName := range poolNames {
		framework.Logf("Validating MCN properties for %v node.", poolName)

		// Grab a node in the desired MCP
		node := GetRandomNode(oc, poolName)
		o.Expect(node.Name).NotTo(o.Equal(""), fmt.Sprintf("Could not get a %v node.", poolName))

		// Validate MCN for the cluster's node
		framework.Logf("Validating MCN properties for the node '%v'.", node.Name)
		mcnErr := ValidateMCNForNodeInPool(oc, clientSet, node, poolName)
		o.Expect(mcnErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Error validating MCN properties for the node in pool '%v'.", poolName))
	}
}

// `ValidateMCNPropertiesCustomMCP` checks that MCN properties match the corresponding node properties
func ValidateMCNPropertiesCustomMCP(oc *exutil.CLI, fixture string) {
	// Create client set for test
	clientSet, clientErr := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(clientErr).NotTo(o.HaveOccurred(), "Error creating client set for test.")

	// Get starting state of default worker MCP, so we know what the correct number of nodes is during cleanup
	workerMcp, err := clientSet.MachineconfigurationV1().MachineConfigPools().Get(context.TODO(), worker, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Could not get worker MCP.")
	workerMcpMachines := workerMcp.Status.MachineCount

	// Grab a random node from each default pool
	workerNode := GetRandomNode(oc, worker)
	o.Expect(workerNode.Name).NotTo(o.Equal(""), "Could not get a worker node.")

	// Cleanup custom MCP on test completion or failure
	defer func() {
		// Unlabel node
		framework.Logf("Removing label node-role.kubernetes.io/%v from node %v", custom, workerNode.Name)
		unlabelErr := oc.Run("label").Args(fmt.Sprintf("node/%s", workerNode.Name), fmt.Sprintf("node-role.kubernetes.io/%s-", custom)).Execute()
		o.Expect(unlabelErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Could not remove label 'node-role.kubernetes.io/%s' from node '%v'.", custom, workerNode.Name))

		// Wait for infra pool to report no nodes & for worker MCP to be ready
		framework.Logf("Waiting for %v MCP to be updated with %v ready machines.", custom, 0)
		WaitForMCPToBeReady(oc, clientSet, custom, 0)
		framework.Logf("Waiting for %v MCP to be updated with %v ready machines.", worker, workerMcpMachines)
		WaitForMCPToBeReady(oc, clientSet, worker, workerMcpMachines)

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
	mcnErr := ValidateMCNForNodeInPool(oc, clientSet, customNode, custom)
	o.Expect(mcnErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Error validating MCN properties node in custom pool '%v'.", custom))
}

// `ValidateMCNConditionTransitions` checks that Conditions properly update on a node update
// Note that a custom MCP is created for this test to limit the number of upgrading nodes &
// decrease cleanup time.
func ValidateMCNConditionTransitionsOnRebootlessUpdate(oc *exutil.CLI, nodeDisruptionFixture string, nodeDisruptionEmptyFixture string, mcFixture string, mcpFixture string) {
	poolName := custom
	mcName := fmt.Sprintf("90-%v-testfile", poolName)

	// Create client set for test
	clientSet, clientErr := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(clientErr).NotTo(o.HaveOccurred(), "Error creating client set for test.")

	// Get starting state of default worker MCP, so we know what the correct number of nodes is during cleanup
	workerMcp, err := clientSet.MachineconfigurationV1().MachineConfigPools().Get(context.TODO(), worker, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Could not get worker MCP.")
	workerMcpMachines := workerMcp.Status.MachineCount

	// Grab a random worker node
	workerNode := GetRandomNode(oc, worker)
	o.Expect(workerNode.Name).NotTo(o.Equal(""), "Could not get a worker node.")

	// Remove node disruption policy on test completion or failure
	defer func() {
		// Apply empty MachineConfiguration fixture to remove previously set NodeDisruptionPolicy
		framework.Logf("Removing node disruption policy.")
		ApplyMachineConfigurationFixture(oc, nodeDisruptionEmptyFixture)
	}()

	// Apply a node disruption policy to allow for rebootless update
	ApplyMachineConfigurationFixture(oc, nodeDisruptionFixture)

	// Cleanup custom MCP, and delete MC on test completion or failure
	defer func() {
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
		framework.Logf("Waiting for %v MCP to be updated with %v ready machines.", worker, workerMcpMachines)
		WaitForMCPToBeReady(oc, clientSet, worker, workerMcpMachines)

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
	validateTransitionThroughConditions(clientSet, updatingNodeName, true)

	// When an update is complete, all conditions other than `Updated` must be false
	framework.Logf("Checking all conditions other than 'Updated' are False.")
	o.Expect(ConfirmUpdatedMCNStatus(clientSet, updatingNodeName)).Should(o.BeTrue(), "Error, all conditions must be 'False' when Updated=True.")
}

// `ValidateMCNConditionTransitionsSNO` checks that Conditions properly update on a node update
// in Single Node Openshift
func ValidateMCNConditionTransitionsOnRebootlessUpdateSNO(oc *exutil.CLI, nodeDisruptionFixture string, nodeDisruptionEmptyFixture string, mcFixture string) {
	poolName := master
	mcName := fmt.Sprintf("90-%v-testfile", poolName)

	// Create client set for test
	clientSet, clientErr := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(clientErr).NotTo(o.HaveOccurred(), "Error creating client set for test.")

	// Remove node disruption policy on test completion or failure
	defer func() {
		// Apply empty MachineConfiguration fixture to remove previously set NodeDisruptionPolicy
		framework.Logf("Removing node disruption policy.")
		ApplyMachineConfigurationFixture(oc, nodeDisruptionEmptyFixture)
	}()

	// Apply a node disruption policy to allow for rebootless update
	ApplyMachineConfigurationFixture(oc, nodeDisruptionFixture)

	// Delete applied MC on test completion or failure
	defer func() {
		// Delete applied MC
		framework.Logf("Deleting MC '%v'.", mcName)
		deleteMCErr := oc.Run("delete").Args("machineconfig", mcName).Execute()
		o.Expect(deleteMCErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Could not delete MachineConfig '%v'.", mcName))

		// Wait for master MCP to be ready
		time.Sleep(15 * time.Second) //wait to not catch the updated state before the deleted mc triggers an update
		framework.Logf("Waiting for %v MCP to be updated with %v ready machines.", poolName, 1)
		WaitForMCPToBeReady(oc, clientSet, poolName, 1)
	}()

	// Apply MC targeting worker node
	mcErr := oc.Run("apply").Args("-f", mcFixture).Execute()
	o.Expect(mcErr).NotTo(o.HaveOccurred(), "Could not apply MachineConfig.")

	// Get the updating node
	updatingNode := GetUpdatingNodeSNO(oc, poolName)
	framework.Logf("Node '%v' is updating.", updatingNode.Name)

	// Validate transition through conditions for MCN
	validateTransitionThroughConditions(clientSet, updatingNode.Name, true)

	// When an update is complete, all conditions other than `Updated` must be false
	framework.Logf("Checking all conditions other than 'Updated' are False.")
	o.Expect(ConfirmUpdatedMCNStatus(clientSet, updatingNode.Name)).Should(o.BeTrue(), "Error, all conditions must be 'False' when Updated=True.")
}

// `validateTransitionThroughConditions` validates the condition trasnitions in the MCN during a node update
func validateTransitionThroughConditions(clientSet *machineconfigclient.Clientset, updatingNodeName string, isRebootless bool) {
	// Note that some conditions are passed through quickly in a node update, so the test can
	// "miss" catching the phases. For test stability, if we fail to catch an "Unknown" status,
	// a warning will be logged instead of erroring out the test.
	framework.Logf("Waiting for Updated=False")
	conditionMet, err := WaitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1.MachineConfigNodeUpdated, metav1.ConditionFalse, 1*time.Minute, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occured while waiting for Updated=False: %v", err))
	o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect Updated=False.")

	framework.Logf("Waiting for UpdatePrepared=True")
	conditionMet, err = WaitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1.MachineConfigNodeUpdatePrepared, metav1.ConditionTrue, 1*time.Minute, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occured while waiting for UpdatePrepared=True: %v", err))
	o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect UpdatePrepared=True.")

	framework.Logf("Waiting for UpdateExecuted=Unknown")
	conditionMet, err = WaitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1.MachineConfigNodeUpdateExecuted, metav1.ConditionUnknown, 30*time.Second, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occured while waiting for UpdateExecuted=Unknown: %v", err))
	if !conditionMet {
		framework.Logf("Warning, could not detect UpdateExecuted=Unknown.")
	}

	// On standard, non-rebootless, update, check that node transitions through "Cordoned" and "Drained" phases
	if !isRebootless {
		framework.Logf("Waiting for Cordoned=True")
		conditionMet, err = WaitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1.MachineConfigNodeUpdateCordoned, metav1.ConditionTrue, 30*time.Second, 1*time.Second)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occured while waiting for Cordoned=True: %v", err))
		o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect Cordoned=True.")

		framework.Logf("Waiting for Drained=Unknown")
		conditionMet, err = WaitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1.MachineConfigNodeUpdateDrained, metav1.ConditionUnknown, 15*time.Second, 1*time.Second)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occured while waiting for Drained=Unknown: %v", err))
		if !conditionMet {
			framework.Logf("Warning, could not detect Drained=Unknown.")
		}

		framework.Logf("Waiting for Drained=True")
		conditionMet, err = WaitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1.MachineConfigNodeUpdateDrained, metav1.ConditionTrue, 4*time.Minute, 1*time.Second)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occured while waiting for Drained=True: %v", err))
		o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect Drained=True.")
	}

	framework.Logf("Waiting for AppliedFilesAndOS=Unknown")
	conditionMet, err = WaitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1.MachineConfigNodeUpdateFilesAndOS, metav1.ConditionUnknown, 30*time.Second, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occured while waiting for AppliedFilesAndOS=Unknown: %v", err))
	if !conditionMet {
		framework.Logf("Warning, could not detect AppliedFilesAndOS=Unknown.")
	}

	framework.Logf("Waiting for AppliedFilesAndOS=True")
	conditionMet, err = WaitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1.MachineConfigNodeUpdateFilesAndOS, metav1.ConditionTrue, 3*time.Minute, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occured while waiting for AppliedFilesAndOS=True: %v", err))
	o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect AppliedFilesAndOS=True.")

	framework.Logf("Waiting for UpdateExecuted=True")
	conditionMet, err = WaitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1.MachineConfigNodeUpdateExecuted, metav1.ConditionTrue, 20*time.Second, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occured while waiting for UpdateExecuted=True: %v", err))
	o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect UpdateExecuted=True.")

	// On rebootless update, check that node transitions through "UpdatePostActionComplete" phase
	if isRebootless {
		framework.Logf("Waiting for UpdatePostActionComplete=True")
		conditionMet, err = WaitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1.MachineConfigNodeUpdatePostActionComplete, metav1.ConditionTrue, 1*time.Minute, 1*time.Second)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occured while waiting for UpdatePostActionComplete=True: %v", err))
		o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect UpdatePostActionComplete=True.")
	} else { // On standard, non-rebootless, update, check that node transitions through "RebootedNode" phase
		framework.Logf("Waiting for RebootedNode=Unknown")
		conditionMet, err = WaitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1.MachineConfigNodeUpdateRebooted, metav1.ConditionUnknown, 15*time.Second, 1*time.Second)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occured while waiting for RebootedNode=Unknown: %v", err))
		if !conditionMet {
			framework.Logf("Warning, could not detect RebootedNode=Unknown.")
		}

		framework.Logf("Waiting for RebootedNode=True")
		conditionMet, err = WaitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1.MachineConfigNodeUpdateRebooted, metav1.ConditionTrue, 6*time.Minute, 1*time.Second)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occured while waiting for RebootedNode=True: %v", err))
		o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect RebootedNode=True.")
	}
	framework.Logf("Waiting for Resumed=True")
	conditionMet, err = WaitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1.MachineConfigNodeResumed, metav1.ConditionTrue, 15*time.Second, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occured while waiting for Resumed=True: %v", err))
	o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect Resumed=True.")

	framework.Logf("Waiting for UpdateComplete=True")
	conditionMet, err = WaitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1.MachineConfigNodeUpdateComplete, metav1.ConditionTrue, 10*time.Second, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occured while waiting for UpdateComplete=True: %v", err))
	o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect UpdateComplete=True.")

	framework.Logf("Waiting for Uncordoned=True")
	conditionMet, err = WaitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1.MachineConfigNodeUpdateUncordoned, metav1.ConditionTrue, 10*time.Second, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occured while waiting for UpdateComplete=True: %v", err))
	o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect UpdateComplete=True.")

	framework.Logf("Waiting for Updated=True")
	conditionMet, err = WaitForMCNConditionStatus(clientSet, updatingNodeName, mcfgv1.MachineConfigNodeUpdated, metav1.ConditionTrue, 1*time.Minute, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occured while waiting for Updated=True: %v", err))
	o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect Updated=True.")
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

	var degradedNodeMCN *mcfgv1.MachineConfigNode
	// Cleanup MC and fix node degradation on failure or test completion
	defer func() {
		// Delete the applied MC
		deleteMCErr := oc.Run("delete").Args("machineconfig", mcName).Execute()
		o.Expect(deleteMCErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Could not delete MachineConfig '%v'.", mcName))

		// Recover the degraded MCP
		recoverErr := RecoverFromDegraded(oc, poolName)
		o.Expect(recoverErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Could not recover MCP '%v' from degraded state.", poolName))

		// If the test reached checking the MCN ensure the NodeDegraded condition is properly restored
		if degradedNodeMCN != nil {
			conditionMet, err := WaitForMCNConditionStatus(clientSet, degradedNodeMCN.Name, mcfgv1.MachineConfigNodeNodeDegraded, metav1.ConditionFalse, 30*time.Second, 1*time.Second)
			o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error occured while waiting for NodeDegraded=False: %v", err))
			o.Expect(conditionMet).To(o.BeTrue(), "Error, could not detect NodeDegraded=False.")
		}
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
	degradedNodeMCN, degradedErr = clientSet.MachineconfigurationV1().MachineConfigNodes().Get(context.TODO(), degradedNode.Name, metav1.GetOptions{})
	o.Expect(degradedErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Error getting MCN of degraded node '%v'.", degradedNode.Name))
	framework.Logf("Validating that `AppliedFilesAndOS` and `UpdateExecuted` conditions in '%v' MCN have a status of 'Unknown'.", degradedNodeMCN.Name)
	o.Expect(CheckMCNConditionStatus(degradedNodeMCN, mcfgv1.MachineConfigNodeUpdateFilesAndOS, metav1.ConditionUnknown)).Should(o.BeTrue(), "Condition 'AppliedFilesAndOS' does not have the expected status of 'Unknown'.")
	o.Expect(CheckMCNConditionStatus(degradedNodeMCN, mcfgv1.MachineConfigNodeUpdateExecuted, metav1.ConditionUnknown)).Should(o.BeTrue(), "Condition 'UpdateExecuted' does not have the expected status of 'Unknown'.")
	nodeDegradedCondition := GetMCNCondition(degradedNodeMCN, mcfgv1.MachineConfigNodeNodeDegraded)
	o.Expect(nodeDegradedCondition).NotTo(o.BeNil())
	o.Expect(nodeDegradedCondition.Status).Should(o.Equal(metav1.ConditionTrue), "Condition 'NodeDegraded' does not have the expected status of 'True'.")
	o.Expect(nodeDegradedCondition.Message).Should(o.ContainSubstring(fmt.Sprintf("Node %s upgrade failure.", degradedNodeMCN.Name)), "Condition 'NodeDegraded' does not have the expected message.")
	o.Expect(nodeDegradedCondition.Message).Should(o.ContainSubstring("/home/core: file exists"), "Condition 'NodeDegraded' does not have the expected message details.")
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

// `ValidateMCNScopeSadPathTest` checks that MCN updates from a MCD that is not the associated one are
// blocked. This test skips on SNO clusters.
func ValidateMCNScopeSadPathTest(oc *exutil.CLI) {
	// Get all nodes from the cluster
	nodes, nodesErr := GetAllNodes(oc)
	o.Expect(nodesErr).NotTo(o.HaveOccurred(), "Error getting nodes from cluster: %v", nodesErr)
	o.Expect(len(nodes)).To(o.BeNumerically(">", 0), "Got 0 nodes from cluster.")

	// If cluster is SNO (has only one node), skip this test
	if len(nodes) == 1 {
		e2eskipper.Skipf("This test does not apply to single-node topologies")
	}

	// Grab two different nodes, so we don't end up testing and targeting the same node.
	nodeUnderTest := nodes[0]
	targetNode := nodes[1]
	framework.Logf("Testing with nodes '%v' and '%v'.", nodeUnderTest.Name, targetNode.Name)

	// Attempt to patch the MCN owned by targetNode from nodeUnderTest's MCD. This should fail.
	// This oc command effectively use the service account of the nodeUnderTest's MCD pod, which should only be able to edit nodeUnderTest's MCN.
	cmdOutput, err := ExecCmdOnNodeWithError(oc, nodeUnderTest, "chroot", "/rootfs", "oc", "patch", "machineconfignodes", targetNode.Name, "--type=merge", "-p", "{\"spec\":{\"configVersion\":{\"desired\":\"rendered-worker-test\"}}}")
	o.Expect(err).To(o.HaveOccurred())
	framework.Logf("MCN patch was successfully blocked.")
	o.Expect(cmdOutput).To(o.ContainSubstring("updates to MCN " + targetNode.Name + " can only be done from the MCN's owner node"))
	framework.Logf("Error string contains desired substring.")
}

// `ValidateMCNScopeImpersonationPathTest` checks that MCN updates by impersonation of the MCD SA are blocked
func ValidateMCNScopeImpersonationPathTest(oc *exutil.CLI) {
	// Grab a random node with a worker role
	nodeUnderTest := GetRandomNode(oc, "worker")
	o.Expect(nodeUnderTest.Name).NotTo(o.Equal(""), "Could not get a `worker` node.")
	framework.Logf("Testing with node '%v'.", nodeUnderTest.Name)

	var errb bytes.Buffer
	// Attempt to patch the MCN owned by nodeUnderTest by impersonating the MCD SA. This should fail.
	cmd := exec.Command("oc", "patch", "machineconfignodes", nodeUnderTest.Name, "--type=merge", "-p", "{\"spec\":{\"configVersion\":{\"desired\":\"rendered-worker-test\"}}}", "--as=system:serviceaccount:openshift-machine-config-operator:machine-config-daemon")
	cmd.Stderr = &errb
	err := cmd.Run()

	o.Expect(err).To(o.HaveOccurred())
	framework.Logf("MCN patch was successfully blocked.")
	o.Expect(errb.String()).To(o.ContainSubstring("this user must have a \"authentication.kubernetes.io/node-name\" claim"))
	framework.Logf("Error string contains desired substring.")
}

// `ValidateMCNScopeHappyPathTest` checks that MCN updates from the associated MCD are allowed
func ValidateMCNScopeHappyPathTest(oc *exutil.CLI) {
	// Grab a random node with a worker role
	nodeUnderTest := GetRandomNode(oc, "worker")
	o.Expect(nodeUnderTest.Name).NotTo(o.Equal(""), "Could not get a `worker` node.")
	framework.Logf("Testing with node '%v'.", nodeUnderTest.Name)

	// Get node's starting desired version
	nodeDesiredConfig := nodeUnderTest.Annotations[desiredConfigAnnotationKey]

	// Attempt to patch the MCN owned by nodeUnderTest from nodeUnderTest's MCD. This should succeed.
	// This oc command effectively use the service account of the nodeUnderTest's MCD pod, which should only be able to edit nodeUnderTest's MCN.
	ExecCmdOnNode(oc, nodeUnderTest, "chroot", "/rootfs", "oc", "patch", "machineconfignodes", nodeUnderTest.Name, "--type=merge", "-p", "{\"spec\":{\"configVersion\":{\"desired\":\"rendered-worker-test\"}}}")
	framework.Logf("MCN '%v' patched successfully.", nodeUnderTest.Name)

	// Cleanup by updating the MCN desired config back to the original value.
	framework.Logf("Cleaning up patched MCN's desired config value.")
	ExecCmdOnNode(oc, nodeUnderTest, "chroot", "/rootfs", "oc", "patch", "machineconfignodes", nodeUnderTest.Name, "--type=merge", "-p", fmt.Sprintf("{\"spec\":{\"configVersion\":{\"desired\":\"%v\"}}}", nodeDesiredConfig))
	framework.Logf("MCN successfully cleaned up.")
}

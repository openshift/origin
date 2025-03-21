package machine_config

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	machineclient "github.com/openshift/client-go/machine/clientset/versioned"
	machineconfigclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	worker = "worker"
	master = "master"
	custom = "infra"
)

var _ = g.Describe("[sig-mco][OCPFeatureGate:MachineConfigNode]", func() {
	defer g.GinkgoRecover()
	var (
		MCOMachineConfigPoolBaseDir = exutil.FixturePath("testdata", "machine_config", "machineconfigpool")
		MCOMachineConfigBaseDir     = exutil.FixturePath("testdata", "machine_config", "machineconfig")
		infraMCPFixture             = filepath.Join(MCOMachineConfigPoolBaseDir, "infra-mcp.yaml")
		testFileMCFixture           = filepath.Join(MCOMachineConfigBaseDir, "0-worker-mc.yaml")
		invalidMCFixture            = filepath.Join(MCOMachineConfigBaseDir, "1-worker-invalid-mc.yaml")
		oc                          = exutil.NewCLIWithoutNamespace("machine-config")
	)

	g.It("[Serial]Should have MCN properties matching associated node properties [apigroup:machineconfiguration.openshift.io]", func() {
		ValidateMCNProperties(oc, infraMCPFixture)
	})

	g.It("[Serial]Should properly transition through MCN conditions on node update [apigroup:machineconfiguration.openshift.io]", func() {
		ValidateMCNConditionTransitions(oc, testFileMCFixture)
	})

	g.It("[Serial]Should properly report MCN conditions on node degrade [apigroup:machineconfiguration.openshift.io]", func() {
		ValidateMCNConditionOnNodeDegrade(oc, invalidMCFixture)
	})

	g.It("[Serial][Slow]Should properly create and remove MCN on node creation and deletion [apigroup:machineconfiguration.openshift.io]", func() {
		ValidateMCNOnNodeCreationAndDeletion(oc)
	})
})

// `ValidateMCNProperties` checks that MCN properties match the corresponding node properties
func ValidateMCNProperties(oc *exutil.CLI, fixture string) {
	// Create client set for test
	clientSet, clientErr := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(clientErr).NotTo(o.HaveOccurred())

	// Grab a random node from each default pool
	workerNode := GetRandomNode(oc, worker)
	masterNode := GetRandomNode(oc, master)

	// Get node desired and current config versions
	workerCurrentConfig := workerNode.Annotations["machineconfiguration.openshift.io/currentConfig"]
	masterCurrentConfig := masterNode.Annotations["machineconfiguration.openshift.io/currentConfig"]
	workerDesiredConfig := workerNode.Annotations["machineconfiguration.openshift.io/desiredConfig"]
	masterDesiredConfig := masterNode.Annotations["machineconfiguration.openshift.io/desiredConfig"]

	// Get node MCNs
	workerNodeMCN, workerErr := clientSet.MachineconfigurationV1alpha1().MachineConfigNodes().Get(context.TODO(), workerNode.Name, metav1.GetOptions{})
	o.Expect(workerErr).NotTo(o.HaveOccurred())
	masterNodeMCN, masterErr := clientSet.MachineconfigurationV1alpha1().MachineConfigNodes().Get(context.TODO(), masterNode.Name, metav1.GetOptions{})
	o.Expect(masterErr).NotTo(o.HaveOccurred())

	// Check MCN pool name value for default MCPs
	framework.Logf("Checking pool name for default MCP nodes.")
	o.Expect(workerNodeMCN.Spec.Pool.Name).Should(o.Equal(worker))
	o.Expect(masterNodeMCN.Spec.Pool.Name).Should(o.Equal(master))

	// Check MCN name matches node name
	framework.Logf("Checking MCN name matches node name.")
	o.Expect(workerNodeMCN.Name).Should(o.Equal(workerNode.Name))
	o.Expect(masterNodeMCN.Name).Should(o.Equal(masterNode.Name))

	// Check desired config version in MCN spec matches desired config on node
	framework.Logf("Checking node desired config version matches desired config version in MCN spec.")
	o.Expect(workerNodeMCN.Spec.ConfigVersion.Desired).Should(o.Equal(workerDesiredConfig))
	o.Expect(masterNodeMCN.Spec.ConfigVersion.Desired).Should(o.Equal(masterDesiredConfig))

	// Check desired config version in MCN spec matches desired config on node
	framework.Logf("Checking node current and desired config versions match current and desired config versions in MCN status.")
	o.Expect(workerNodeMCN.Status.ConfigVersion.Current).Should(o.Equal(workerCurrentConfig))
	o.Expect(workerNodeMCN.Status.ConfigVersion.Desired).Should(o.Equal(workerDesiredConfig))
	o.Expect(masterNodeMCN.Status.ConfigVersion.Current).Should(o.Equal(masterCurrentConfig))
	o.Expect(masterNodeMCN.Status.ConfigVersion.Desired).Should(o.Equal(masterDesiredConfig))

	// Apply the fixture to create a custom MCP called "infra" & label the worker node accordingly
	mcpErr := oc.Run("apply").Args("-f", fixture).Execute()
	o.Expect(mcpErr).NotTo(o.HaveOccurred())
	labelErr := oc.Run("label").Args(fmt.Sprintf("node/%s", workerNode.Name), fmt.Sprintf("node-role.kubernetes.io/%s=", custom)).Execute()
	o.Expect(labelErr).NotTo(o.HaveOccurred())

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
		WaitForMCPToBeReady(oc, clientSet, custom, 0)
		WaitForMCPToBeReady(oc, clientSet, worker, workerMcpReadyMachines+1)

		// Delete custom MCP
		framework.Logf("Deleting MCP %v", custom)
		deleteMCPErr := oc.Run("delete").Args("mcp", custom).Execute()
		o.Expect(deleteMCPErr).NotTo(o.HaveOccurred())
	}()

	// Wait for the custom pool to be updated with the node ready
	WaitForMCPToBeReady(oc, clientSet, custom, 1)

	// Get node desired and current config versions
	customNodes, customNodeErr := GetNodesByRole(oc, custom)
	o.Expect(customNodeErr).NotTo(o.HaveOccurred())
	customNode := customNodes[0]
	customCurrentConfig := customNode.Annotations["machineconfiguration.openshift.io/currentConfig"]
	customDesiredConfig := customNode.Annotations["machineconfiguration.openshift.io/desiredConfig"]

	// Get custom node MCN
	customNodeMCN, customErr := clientSet.MachineconfigurationV1alpha1().MachineConfigNodes().Get(context.TODO(), customNode.Name, metav1.GetOptions{})
	o.Expect(customErr).NotTo(o.HaveOccurred())

	// Check MCN pool name value is correct for the custom MCP
	framework.Logf("Checking pool name for custom MCP node.")
	o.Expect(customNodeMCN.Spec.Pool.Name).Should(o.Equal(custom))

	// Check MCN name matches node name
	framework.Logf("Checking MCN name matches node name, custom MCP node.")
	o.Expect(customNodeMCN.Name).Should(o.Equal(customNode.Name))

	// Check desired config version in MCN spec matches desired config on node
	framework.Logf("Checking node desired config version matches desired config version in MCN spec, custom MCP node.")
	o.Expect(customNodeMCN.Spec.ConfigVersion.Desired).Should(o.Equal(customDesiredConfig))

	// Check desired config version in MCN spec matches desired config on node
	framework.Logf("Checking node current and desired config versions match current and desired config versions in MCN status, custom MCP node.")
	o.Expect(customNodeMCN.Status.ConfigVersion.Current).Should(o.Equal(customCurrentConfig))
	o.Expect(customNodeMCN.Status.ConfigVersion.Desired).Should(o.Equal(customDesiredConfig))
}

// `ValidateMCNConditionTransitions` checks that Conditions properly update on a node update
func ValidateMCNConditionTransitions(oc *exutil.CLI, fixture string) {
	// Create client set for test
	clientSet, clientErr := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(clientErr).NotTo(o.HaveOccurred())

	// Apply MC targeting worker pool
	mcErr := oc.Run("apply").Args("-f", fixture).Execute()
	o.Expect(mcErr).NotTo(o.HaveOccurred())

	// Delete MC on failure or test completion
	defer func() {
		deleteMCErr := oc.Run("delete").Args("machineconfig", "90-worker-testfile").Execute()
		o.Expect(deleteMCErr).NotTo(o.HaveOccurred())
	}()

	// Get an updating worker node
	updatingNodes := GetCordonedNodes(oc, worker)
	o.Expect(len(updatingNodes) > 0, "No ready nodes found for MCP %v.", worker)
	workerNode := updatingNodes[0]

	// Validate transition through conditions for MCN
	// TODO: make consts for the statuses
	framework.Logf("Checking Updated=False")
	err := waitForMCNConditionStatus(clientSet, workerNode.Name, "Updated", "False", 1*time.Minute, 5*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred())
	framework.Logf("Checking UpdatePrepared=True")
	err = waitForMCNConditionStatus(clientSet, workerNode.Name, "UpdatePrepared", "True", 1*time.Minute, 3*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred())
	framework.Logf("Checking UpdateExecuted=Unknown")
	err = waitForMCNConditionStatus(clientSet, workerNode.Name, "UpdateExecuted", "Unknown", 1*time.Minute, 3*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred())
	framework.Logf("Checking Cordoned=True")
	err = waitForMCNConditionStatus(clientSet, workerNode.Name, "Cordoned", "True", 30*time.Second, 3*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred())
	framework.Logf("Checking Drained=Unknown")
	err = waitForMCNConditionStatus(clientSet, workerNode.Name, "Drained", "Unknown", 30*time.Second, 2*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred())
	framework.Logf("Checking Drained=True")
	err = waitForMCNConditionStatus(clientSet, workerNode.Name, "Drained", "True", 7*time.Minute, 10*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred())
	framework.Logf("Checking AppliedFilesAndOS=Unknown")
	err = waitForMCNConditionStatus(clientSet, workerNode.Name, "AppliedFilesAndOS", "Unknown", 1*time.Minute, 1*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred())
	framework.Logf("Checking AppliedFilesAndOS=True")
	err = waitForMCNConditionStatus(clientSet, workerNode.Name, "AppliedFilesAndOS", "True", 3*time.Minute, 2*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred())
	framework.Logf("Checking UpdateExecuted=True")
	err = waitForMCNConditionStatus(clientSet, workerNode.Name, "UpdateExecuted", "True", 20*time.Second, 5*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred())
	framework.Logf("Checking UpdatePostActionComplete=Unknown")
	err = waitForMCNConditionStatus(clientSet, workerNode.Name, "UpdatePostActionComplete", "Unknown", 30*time.Second, 5*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred())
	framework.Logf("Checking RebootedNode=Unknown")
	err = waitForMCNConditionStatus(clientSet, workerNode.Name, "RebootedNode", "Unknown", 15*time.Second, 3*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred())
	framework.Logf("Checking RebootedNode=True")
	err = waitForMCNConditionStatus(clientSet, workerNode.Name, "RebootedNode", "True", 5*time.Minute, 5*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred())
	framework.Logf("Checking Resumed=True")
	err = waitForMCNConditionStatus(clientSet, workerNode.Name, "Resumed", "True", 15*time.Second, 5*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred())
	framework.Logf("Checking UpdateComplete=True")
	err = waitForMCNConditionStatus(clientSet, workerNode.Name, "UpdateComplete", "True", 10*time.Second, 5*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred())
	framework.Logf("Checking Uncordoned=True")
	err = waitForMCNConditionStatus(clientSet, workerNode.Name, "Uncordoned", "True", 10*time.Second, 2*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred())
	framework.Logf("Checking Updated=True")
	err = waitForMCNConditionStatus(clientSet, workerNode.Name, "Updated", "True", 1*time.Minute, 5*time.Second)
	o.Expect(err).NotTo(o.HaveOccurred())

	// When an update is complete, all conditions other than `Updated` must be false
	framework.Logf("Checking all conditions other than 'Updated' are False.")
	o.Expect(confirmUpdatedMCNStatus(clientSet, workerNode.Name)).Should(o.BeTrue())
}

// `ValidateMCNConditionOnNodeDegrade` checks that Conditions properly update on a node update
func ValidateMCNConditionOnNodeDegrade(oc *exutil.CLI, fixture string) {
	// Create client set for test
	clientSet, clientErr := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(clientErr).NotTo(o.HaveOccurred())

	// Apply invalid MC targeting worker pool
	mcErr := oc.Run("apply").Args("-f", fixture).Execute()
	o.Expect(mcErr).NotTo(o.HaveOccurred())

	// Cleanup MC and fix node degradation on failure or test completion
	defer func() {
		// Delete the applied MC
		deleteMCErr := oc.Run("delete").Args("machineconfig", "91-worker-testfile-invalid").Execute()
		o.Expect(deleteMCErr).NotTo(o.HaveOccurred())

		// Recover the degraded MCP
		recoverErr := recoverFromDegraded(oc, worker)
		o.Expect(recoverErr).NotTo(o.HaveOccurred())
	}()

	// Wait for worker MCP to be in a degraded state with one degraded machine
	// TODO consolidate into helper func that doesn't require getting MCP more than required
	o.Expect(waitForMCPConditionStatus(oc, worker, "Degraded", "True")).NotTo(o.HaveOccurred(), "Error waiting for %v MCP to be in a degraded state.")
	workerMcp, err := clientSet.MachineconfigurationV1().MachineConfigPools().Get(context.TODO(), worker, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "Error getting %v MCP.", worker)
	o.Expect(workerMcp.Status.DegradedMachineCount).To(o.BeNumerically("==", 1), "Degraded machine count is not 1.")

	// Get degraded worker node
	degradedNode, degradedNodeErr := GetDegradedNode(oc, worker)
	o.Expect(degradedNodeErr).NotTo(o.HaveOccurred(), "Error getting degraded node for %v MCP.", worker)

	// Validate MCN of degraded node
	degradedNodeMCN, degradedErr := clientSet.MachineconfigurationV1alpha1().MachineConfigNodes().Get(context.TODO(), degradedNode.Name, metav1.GetOptions{})
	o.Expect(degradedErr).NotTo(o.HaveOccurred())
	o.Expect(checkMCNConditionStatus(degradedNodeMCN, "AppliedFilesAndOS", "Unknown"))
	o.Expect(checkMCNConditionStatus(degradedNodeMCN, "UpdateExecuted", "Unknown"))
}

// `ValidateMCNProperties` checks that MCNs with correct properties are created on node creation
// and deleted on node deletion
// TODO: figure out if this needs to be a long test due to the time it takes to provision a machine & create a node
func ValidateMCNOnNodeCreationAndDeletion(oc *exutil.CLI) {
	// Create machine client for test
	machineClient, machineErr := machineclient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(machineErr).NotTo(o.HaveOccurred())

	// Create client set for test
	clientSet, clientErr := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(clientErr).NotTo(o.HaveOccurred())

	// Skip test if worker nodes cannot be scaled
	canBeScaled, canScaleErr := workersCanBeScaled(oc, machineClient)
	o.Expect(canScaleErr).NotTo(o.HaveOccurred(), "Error deciding if worker nodes can be scaled using MachineSets.")
	if !canBeScaled {
		g.Skip("Worker nodes cannot be scaled using MachineSets. This test cannot be execute if workers cannot be scaled via MachineSets.")
	}

	// Get MachineSet for test
	framework.Logf("Getting MachineSet for testing.")
	machineSet := getRandomMachineSet(machineClient)
	framework.Logf("MachineSet %s will be used for testing", machineSet.Name)
	originalReplica := int(*machineSet.Spec.Replicas)

	// Rollback replica in test MachineSet on test failure or completion
	// defer func() {
	// 	scaleErr := ScaleMachineSet(oc, machineSet.Name, fmt.Sprintf("%d", originalReplica))
	// 	o.Expect(scaleErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Error rolling back MachineSet %v to vale replica value %v.", machineSet.Name, string(originalReplica)))
	// }()

	// Create node by scaling MachineSet
	framework.Logf("Scaling up MachineSet to create node.")
	updatedReplica := originalReplica + 1
	scaleErr := ScaleMachineSet(oc, machineSet.Name, fmt.Sprintf("%d", updatedReplica))
	o.Expect(scaleErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Error provisioning node by scaling MachineSet %v to replica value %v.", machineSet.Name, string(updatedReplica)))

	// Get the newly created node
	framework.Logf("Getting the new machine.")
	provisioningMachine, provisioningMachineErr := GetMachinesByPhase(machineClient, machineSet.Name, "Provisioning")
	o.Expect(provisioningMachineErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Cannot find provisioning machine in MachineSet %v", machineSet.Name))
	newMachineName := provisioningMachine.Name
	framework.Logf("Waiting for new machine %v to be ready.", newMachineName)
	WaitForMachineInState(machineClient, newMachineName, "Running")
	framework.Logf("Getting new node in machine %v.", newMachineName)
	node, nodeErr := getNewReadyNodeInMachine(oc, newMachineName)
	o.Expect(nodeErr).NotTo(o.HaveOccurred())
	framework.Logf("Got new node: %v.", node.Name)

	// Validate new MCN
	validMCNErr := WaitForValidMCNProperties(clientSet, node)
	o.Expect(validMCNErr).NotTo(o.HaveOccurred())

	// Scale down the MachineSet to delete the created node
	framework.Logf("Scaling down MachineSet to delete node.")
	scaleErr = ScaleMachineSet(oc, machineSet.Name, fmt.Sprintf("%d", originalReplica))
	o.Expect(scaleErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Error deleting node by scaling MachineSet %v to replica value %v.", machineSet.Name, string(originalReplica)))

	// Get the deleting node
	framework.Logf("Getting the deleting machine.")
	deletingMachine, deletingMachineErr := GetMachinesByPhase(machineClient, machineSet.Name, "Deleting")
	o.Expect(deletingMachineErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Cannot find deleting machine in MachineSet %v", machineSet.Name))
	deletingMachineName := deletingMachine.Name
	framework.Logf("Machine %v is being deleted.", deletingMachineName)
	framework.Logf("Getting node in deleting machine %v.", deletingMachineName)
	deletingNode, deletingNodeErr := getNodeInMachine(oc, deletingMachineName)
	o.Expect(deletingNodeErr).NotTo(o.HaveOccurred())
	framework.Logf("Node being deleted: %v.", deletingNode.Name)

	// Check that node & MCN are removed
	o.Expect(WaitForNodeToBeDeleted(oc, deletingNode.Name))
	o.Expect(WaitForMCNToBeDeleted(clientSet, deletingNode.Name))
}

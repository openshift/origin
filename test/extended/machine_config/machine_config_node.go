package machine_config

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	mcClient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
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

// TODO: decide if this needs to be run as `Serial` or not
// This test is [Serial] because it modifies the cluster/machineconfigurations.operator.openshift.io object in each test.
var _ = g.Describe("[sig-mco][OCPFeatureGate:MachineConfigNode][Serial]", func() {
	defer g.GinkgoRecover()
	var (
		MCOMachineConfigPoolBaseDir = exutil.FixturePath("testdata", "machine_config", "machineconfigpool")
		MCOMachineConfigBaseDir     = exutil.FixturePath("testdata", "machine_config", "machineconfig")
		infraMCPFixture             = filepath.Join(MCOMachineConfigPoolBaseDir, "infra-mcp.yaml")
		testFileMCFixture           = filepath.Join(MCOMachineConfigBaseDir, "0-worker-mc.yaml")
		oc                          = exutil.NewCLIWithoutNamespace("machine-config")
	)

	// TODO: Update to properly cleanup after tests
	// g.AfterAll(func(ctx context.Context) {
	// 	// clean up the created custom MCP
	// 	CleanupCustomMCP(oc)
	// })

	g.It("Should have MCN properties matching associated node properties [apigroup:machineconfiguration.openshift.io]", func() {
		ValidateMCNProperties(oc, infraMCPFixture)
	})

	g.It("Should properly transition through MCN conditions on node update [apigroup:machineconfiguration.openshift.io]", func() {
		ValidateMCNConditionTransitions(oc, testFileMCFixture)
	})
})

// `ValidateMCNProperties` checks that MCN properties match the corresponding node properties
func ValidateMCNProperties(oc *exutil.CLI, fixture string) {
	// Create client set for test
	clientSet, clientErr := mcClient.NewForConfig(oc.KubeFramework().ClientConfig())
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

	// Wait for the custom pool to be updated with the node ready
	WaitForMCPToBeReady(oc, custom)

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

// `ValidateMCNConditionTransitions` check that Conditions properly update on a node update
func ValidateMCNConditionTransitions(oc *exutil.CLI, fixture string) {
	// Create client set for test
	clientSet, clientErr := mcClient.NewForConfig(oc.KubeFramework().ClientConfig())
	o.Expect(clientErr).NotTo(o.HaveOccurred())

	// Apply MC targeting worker pool
	mcErr := oc.Run("apply").Args("-f", fixture).Execute()
	o.Expect(mcErr).NotTo(o.HaveOccurred())

	// Delete MC on failure or test completion
	defer func() {
		deleteMCErr := oc.Run("delete").Args("machineconfig", "99-worker-testfile").Execute()
		o.Expect(deleteMCErr).NotTo(o.HaveOccurred())
	}()

	// Get an updating worker node
	updatingNodes := GetCordonedNodes(oc, worker)
	o.Expect(len(updatingNodes) > 0, "No ready nodes found for MCP %v.", worker)
	workerNode := updatingNodes[0]

	// Validate transition through conditions for MCN
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
	// Failing here!
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

// TODO: test this cleanup works when running full test
// `CleanupCustomMCP` deletes the custom MCP for the MCN tests
func CleanupCustomMCP(oc *exutil.CLI) {
	// TODO: add length check to see if any nodes are labeled with custom role
	// TODO: add check if mcp exists before trying to delete it

	// Remove custom role from nodes
	customNodes, customNodeErr := GetNodesByRole(oc, custom)
	o.Expect(customNodeErr).NotTo(o.HaveOccurred())
	for _, node := range customNodes {
		framework.Logf("Unlabeling node %v", node.Name)
		unlabelErr := oc.Run("label").Args(fmt.Sprintf("node/%s", node.Name), fmt.Sprintf("node-role.kubernetes.io/%s-", custom)).Execute()
		o.Expect(unlabelErr).NotTo(o.HaveOccurred())
	}

	// Wait for worker MCP to be updated
	// TODO: fix this since it seemes to not wait long enough to actually catch the mcp needing an update and being updated
	// TODO: maybe check the node annotations instead?
	// TODO: Maybe update WaitForMCPToBeReady to take an int again but have it be a number representing the previous number of machines in the pool? so that the updated can also chek if ready machine count is greater than the previous count.
	framework.Logf("Waiting for worker MCP to re-sync.")
	WaitForMCPToBeReady(oc, worker)

	// Delete custom MCP
	framework.Logf("Deleting MCP %v", custom)
	deleteMCPErr := oc.Run("delete").Args("mcp", custom).Execute()
	o.Expect(deleteMCPErr).NotTo(o.HaveOccurred())

	framework.Logf("Custom MCP %v has been cleaned up.", custom)
}

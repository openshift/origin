package node

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	osconfigv1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
)

var (
	// these represent the expected rendered config prefixes for worker and custom MCP nodes
	workerConfigPrefix = "rendered-worker"
	customConfigPrefix = "rendered-custom"
)

// These tests verify KubeletConfig application with various kubelet configuration features.
// The primary purpose is to test applying KubeletConfig objects to nodes and verifying that
// the kubelet configuration changes are properly applied and take effect.
var _ = g.Describe("[Suite:openshift/disruptive-longrunning][sig-node][Disruptive]", func() {
	defer g.GinkgoRecover()
	var (
		NodeMachineConfigPoolBaseDir = exutil.FixturePath("testdata", "node", "machineconfigpool")
		NodeKubeletConfigBaseDir     = exutil.FixturePath("testdata", "node", "kubeletconfig")

		customMCPFixture       = filepath.Join(NodeMachineConfigPoolBaseDir, "customMCP.yaml")
		customLoggingKCFixture = filepath.Join(NodeKubeletConfigBaseDir, "loggingKC.yaml")

		oc = exutil.NewCLIWithoutNamespace("node-kubeletconfig")
	)

	// This test is also considered `Slow` because it takes longer than 5 minutes to run.
	g.It("[Slow]should apply KubeletConfig with logging verbosity to custom pool [apigroup:machineconfiguration.openshift.io]", func() {
		// Skip this test on single node and two-node platforms since custom MCPs are not supported
		// for clusters with only a master MCP
		skipOnSingleNodeTopology(oc)
		skipOnTwoNodeTopology(oc)

		// Get the MCP and KubeletConfig fixtures needed for this test
		mcpFixture := customMCPFixture
		kcFixture := customLoggingKCFixture

		// Create kube client for test
		kubeClient, err := kubernetes.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error getting kube client: %v", err))

		// Create custom MCP
		defer deleteMCP(oc, "custom")
		err = oc.Run("apply").Args("-f", mcpFixture).Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error creating MCP `custom`: %v", err))

		// Add node to custom MCP & wait for the node to be ready in the MCP
		optedNodes, err := addWorkerNodesToCustomPool(oc, kubeClient, 1, "custom")
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error adding node to `custom` MCP: %v", err))
		defer waitTillNodeReadyWithConfig(kubeClient, optedNodes[0], workerConfigPrefix)
		defer unlabelNode(oc, optedNodes[0])
		framework.Logf("Waiting for `%v` node to be ready in `custom` MCP.", optedNodes[0])
		waitTillNodeReadyWithConfig(kubeClient, optedNodes[0], customConfigPrefix)

		// Get the current config before applying KubeletConfig
		node, err := kubeClient.CoreV1().Nodes().Get(context.TODO(), optedNodes[0], metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error getting node: %v", err))
		originalConfig := node.Annotations["machineconfiguration.openshift.io/currentConfig"]
		framework.Logf("Node '%v' has original config: %v", optedNodes[0], originalConfig)

		// Apply KubeletConfig with logging verbosity
		defer deleteKC(oc, "custom-logging-config")
		err = oc.Run("apply").Args("-f", kcFixture).Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error applying KubeletConfig: %v", err))

		// Wait for the node to reboot after applying KubeletConfig
		// KubeletConfig changes require a node reboot to take effect
		framework.Logf("Waiting for node '%v' to reboot after applying KubeletConfig", optedNodes[0])
		waitForReboot(kubeClient, optedNodes[0])

		// Verify the node has been updated with new config
		framework.Logf("Verifying node '%v' has updated config after reboot", optedNodes[0])
		node, err = kubeClient.CoreV1().Nodes().Get(context.TODO(), optedNodes[0], metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error getting node after update: %v", err))
		o.Expect(node.Annotations["machineconfiguration.openshift.io/state"]).To(o.Equal("Done"), "Node should be in Done state after reboot")
		newConfig := node.Annotations["machineconfiguration.openshift.io/currentConfig"]
		o.Expect(newConfig).NotTo(o.Equal(originalConfig), "Node config should have changed from %v to %v", originalConfig, newConfig)

		framework.Logf("Successfully applied KubeletConfig with logging verbosity to node '%v', config changed from '%v' to '%v'", optedNodes[0], originalConfig, newConfig)
	})
})

// `addWorkerNodesToCustomPool` labels the desired number of worker nodes with the MCP role
// selector so that the nodes become part of the desired custom MCP
func addWorkerNodesToCustomPool(oc *exutil.CLI, kubeClient *kubernetes.Clientset, numberOfNodes int, customMCP string) ([]string, error) {
	// Get the worker nodes
	nodes, err := kubeClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: labels.SelectorFromSet(labels.Set{"node-role.kubernetes.io/worker": ""}).String()})
	if err != nil {
		return nil, err
	}
	// Return an error if there are less worker nodes in the cluster than the desired number of nodes to add to the custom MCP
	if len(nodes.Items) < numberOfNodes {
		return nil, fmt.Errorf("Node in Worker MCP %d < Number of nodes needed in %d MCP", len(nodes.Items), numberOfNodes)
	}

	// Label the nodes with the custom MCP role selector
	var optedNodes []string
	for node_i := 0; node_i < numberOfNodes; node_i++ {
		err = oc.AsAdmin().Run("label").Args("node", nodes.Items[node_i].Name, fmt.Sprintf("node-role.kubernetes.io/%s=", customMCP)).Execute()
		if err != nil {
			return nil, err
		}
		optedNodes = append(optedNodes, nodes.Items[node_i].Name)
	}
	return optedNodes, nil
}

// `waitForReboot` waits for up to 5 minutes for the input node to start a reboot and then up to 15
// minutes for the node to complete its reboot.
func waitForReboot(kubeClient *kubernetes.Clientset, nodeName string) {
	o.Eventually(func() bool {
		node, err := kubeClient.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
		if err != nil {
			framework.Logf("Failed to grab Node '%s', error :%s", nodeName, err)
			return false
		}
		if node.Annotations["machineconfiguration.openshift.io/state"] == "Working" {
			framework.Logf("Node '%s' has entered reboot", nodeName)
			return true
		}
		return false
	}, 5*time.Minute, 10*time.Second).Should(o.BeTrue(), "Timed out waiting for Node '%s' to start reboot.", nodeName)

	o.Eventually(func() bool {
		node, err := kubeClient.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
		if err != nil {
			framework.Logf("Failed to grab Node '%s', error :%s", nodeName, err)
			return false
		}
		if node.Annotations["machineconfiguration.openshift.io/state"] == "Done" && len(node.Spec.Taints) == 0 {
			framework.Logf("Node '%s' has finished reboot", nodeName)
			return true
		}
		return false
	}, 15*time.Minute, 10*time.Second).Should(o.BeTrue(), "Timed out waiting for Node '%s' to finish reboot.", nodeName)
}

// `waitTillNodeReadyWithConfig` loops for up to 5 minutes to check whether the input node reaches
// the desired rendered config version. The config version is determined by checking if the config
// version prefix matches the stardard format of `rendered-<desired-mcp-name>`.
func waitTillNodeReadyWithConfig(kubeClient *kubernetes.Clientset, nodeName, currentConfigPrefix string) {
	o.Eventually(func() bool {
		node, err := kubeClient.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
		if err != nil {
			framework.Logf("Failed to grab Node '%s', error :%s", nodeName, err)
			return false
		}
		currentConfig := node.Annotations["machineconfiguration.openshift.io/currentConfig"]
		if strings.Contains(currentConfig, currentConfigPrefix) && node.Annotations["machineconfiguration.openshift.io/state"] == "Done" {
			framework.Logf("Node '%s' has current config `%v`", nodeName, currentConfig)
			return true
		}
		framework.Logf("Node '%s' has is not yet ready and has the current config `%v`", nodeName, currentConfig)
		return false
	}, 5*time.Minute, 10*time.Second).Should(o.BeTrue(), "Timed out waiting for Node '%s' to have rendered-worker config.", nodeName)
}

// `unlabelNode` removes the `node-role.kubernetes.io/custom` label from the node with the input
// name. This triggers the node's removal from the custom MCP named `custom`.
func unlabelNode(oc *exutil.CLI, name string) error {
	return oc.AsAdmin().Run("label").Args("node", name, "node-role.kubernetes.io/custom-").Execute()
}

// `deleteKC` deletes the KubeletConfig with the input name
func deleteKC(oc *exutil.CLI, name string) error {
	return oc.Run("delete").Args("kubeletconfig", name).Execute()
}

// `deleteMCP` deletes the MachineConfigPool with the input name
func deleteMCP(oc *exutil.CLI, name string) error {
	return oc.Run("delete").Args("mcp", name).Execute()
}

// `skipOnSingleNodeTopology` skips the test if the cluster is using single-node topology
func skipOnSingleNodeTopology(oc *exutil.CLI) {
	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	if infra.Status.ControlPlaneTopology == osconfigv1.SingleReplicaTopologyMode {
		e2eskipper.Skipf("This test does not apply to single-node topologies")
	}
}

// `skipOnTwoNodeTopology` skips the test if the cluster is using two-node topology, including
// both standard and arbiter cases.
func skipOnTwoNodeTopology(oc *exutil.CLI) {
	infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	if infra.Status.ControlPlaneTopology == osconfigv1.DualReplicaTopologyMode || infra.Status.ControlPlaneTopology == osconfigv1.HighlyAvailableArbiterMode {
		e2eskipper.Skipf("This test does not apply to two-node topologies")
	}
}

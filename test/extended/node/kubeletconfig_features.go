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
	machineconfigclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
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
		NodeKubeletConfigBaseDir = exutil.FixturePath("testdata", "node", "kubeletconfig")
		customLoggingKCFixture   = filepath.Join(NodeKubeletConfigBaseDir, "loggingKC.yaml")

		oc = exutil.NewCLIWithoutNamespace("node-kubeletconfig")
	)

	// This test is also considered `Slow` because it takes longer than 5 minutes to run.
	g.It("[Slow]should apply KubeletConfig with logging verbosity to custom pool [apigroup:machineconfiguration.openshift.io]", func(ctx context.Context) {
		// Skip this test on single node and two-node platforms since custom MCPs are not supported
		// for clusters with only a master MCP
		skipOnSingleNodeTopology(oc)
		skipOnTwoNodeTopology(oc)

		// Get the KubeletConfig fixture needed for this test
		kcFixture := customLoggingKCFixture

		// Create kube client for test
		kubeClient, err := kubernetes.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error getting kube client: %v", err))

		// Get a worker node for testing
		nodes, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{LabelSelector: labels.SelectorFromSet(labels.Set{"node-role.kubernetes.io/worker": ""}).String()})
		o.Expect(err).NotTo(o.HaveOccurred(), "Error getting worker nodes")
		o.Expect(len(nodes.Items)).To(o.BeNumerically(">", 0), "No worker nodes found")
		testNode := nodes.Items[0].Name

		// Create machine config client
		mcClient, err := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred(), "Error creating machine config client")

		// Create custom MCP for the node
		mcpConfig, err := CreateCustomMCPForNode(ctx, oc, mcClient, "custom", testNode)
		o.Expect(err).NotTo(o.HaveOccurred(), "Error creating custom MCP")

		defer func() {
			cleanupErr := CleanupCustomMCP(ctx, mcpConfig)
			if cleanupErr != nil {
				framework.Logf("Warning: cleanup had errors: %v", cleanupErr)
			}
		}()

		// Wait for the node to be ready in the custom MCP
		framework.Logf("Waiting for node %s to be ready in custom MCP", testNode)
		waitTillNodeReadyWithConfig(kubeClient, testNode, customConfigPrefix)

		// Get the current config before applying KubeletConfig
		node, err := kubeClient.CoreV1().Nodes().Get(ctx, testNode, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error getting node: %v", err))
		originalConfig := node.Annotations["machineconfiguration.openshift.io/currentConfig"]
		framework.Logf("Node %s has original config: %s", testNode, originalConfig)

		// Apply KubeletConfig with logging verbosity
		defer func() {
			if err := CleanupKubeletConfig(ctx, mcClient, "custom-logging-config", ""); err != nil {
				framework.Logf("Warning: KubeletConfig cleanup failed: %v", err)
			}
		}()
		err = oc.Run("apply").Args("-f", kcFixture).Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error applying KubeletConfig: %v", err))

		// Wait for the node to reboot after applying KubeletConfig
		// KubeletConfig changes require a node reboot to take effect
		framework.Logf("Waiting for node %s to reboot after applying KubeletConfig", testNode)
		waitForReboot(kubeClient, testNode)

		// Verify the node has been updated with new config
		framework.Logf("Verifying node %s has updated config after reboot", testNode)
		node, err = kubeClient.CoreV1().Nodes().Get(ctx, testNode, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("Error getting node after update: %v", err))
		o.Expect(node.Annotations["machineconfiguration.openshift.io/state"]).To(o.Equal("Done"), "Node should be in Done state after reboot")
		newConfig := node.Annotations["machineconfiguration.openshift.io/currentConfig"]
		o.Expect(newConfig).NotTo(o.Equal(originalConfig), "Node config should have changed from %s to %s", originalConfig, newConfig)

		framework.Logf("Successfully applied KubeletConfig with logging verbosity to node %s, config changed from %s to %s", testNode, originalConfig, newConfig)
	})
})

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

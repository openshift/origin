package node

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"

	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	machineconfigclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Suite:openshift/disruptive-longrunning][sig-node][Disruptive] Node sizing", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("node-sizing")

	g.It("should have NODE_SIZING_ENABLED=true by default and NODE_SIZING_ENABLED=false when KubeletConfig with autoSizingReserved=false is applied", func(ctx context.Context) {
		// Skip on MicroShift since it doesn't have the Machine Config Operator
		isMicroshift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroshift {
			g.Skip("Not supported on MicroShift")
		}

		mcClient, err := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred(), "Error creating MCO client")

		testMCPName := "node-sizing-test"
		testNodeMCPLabel := fmt.Sprintf("node-role.kubernetes.io/%s", testMCPName)
		kubeletConfigName := "auto-sizing-enabled"

		// Verify the default state (NODE_SIZING_ENABLED=false)
		// This feature is added in OCP 4.21
		g.By("Getting a worker node to test")
		nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{
			LabelSelector: "node-role.kubernetes.io/worker",
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to list worker nodes")
		o.Expect(len(nodes.Items)).To(o.BeNumerically(">", 0), "Should have at least one worker node")

		// Select first worker node and label it for our custom MCP
		// This approach is taken so that all the nodes do not restart at the same time for the test
		nodeName := nodes.Items[0].Name
		framework.Logf("Testing on node: %s", nodeName)

		g.By(fmt.Sprintf("Labeling node %s with %s", nodeName, testNodeMCPLabel))
		node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to get node")

		if node.Labels == nil {
			node.Labels = make(map[string]string)
		}
		node.Labels[testNodeMCPLabel] = ""
		_, err = oc.AdminKubeClient().CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to label node")

		// Create custom MCP
		g.By(fmt.Sprintf("Creating custom MachineConfigPool %s", testMCPName))
		testMCP := &mcfgv1.MachineConfigPool{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "machineconfiguration.openshift.io/v1",
				Kind:       "MachineConfigPool",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: testMCPName,
				Labels: map[string]string{
					"machineconfiguration.openshift.io/pool": testMCPName,
				},
			},
			Spec: mcfgv1.MachineConfigPoolSpec{
				MachineConfigSelector: &metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{
							Key:      "machineconfiguration.openshift.io/role",
							Operator: metav1.LabelSelectorOpIn,
							Values:   []string{"worker", testMCPName},
						},
					},
				},
				NodeSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						testNodeMCPLabel: "",
					},
				},
			},
		}

		_, err = mcClient.MachineconfigurationV1().MachineConfigPools().Create(ctx, testMCP, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to create custom MachineConfigPool")

		cleanupMCP := func() {
			g.By("Cleaning up custom MachineConfigPool")
			cleanupCtx := context.Background()
			deleteErr := mcClient.MachineconfigurationV1().MachineConfigPools().Delete(cleanupCtx, testMCPName, metav1.DeleteOptions{})
			if deleteErr != nil {
				framework.Logf("Failed to delete MachineConfigPool %s: %v", testMCPName, deleteErr)
			}
		}

		cleanupNodeLabel := func() {
			g.By(fmt.Sprintf("Removing node label %s from node %s", testNodeMCPLabel, nodeName))
			cleanupCtx := context.Background()
			node, getErr := oc.AdminKubeClient().CoreV1().Nodes().Get(cleanupCtx, nodeName, metav1.GetOptions{})
			if getErr != nil {
				framework.Logf("Failed to get node for cleanup: %v", getErr)
				return
			}

			delete(node.Labels, testNodeMCPLabel)
			_, updateErr := oc.AdminKubeClient().CoreV1().Nodes().Update(cleanupCtx, node, metav1.UpdateOptions{})
			if updateErr != nil {
				framework.Logf("Failed to remove label from node %s: %v", nodeName, updateErr)
				return
			}

			// Wait for the node to transition back to the worker pool configuration
			g.By(fmt.Sprintf("Waiting for node %s to transition back to worker pool", nodeName))
			o.Eventually(func() bool {
				currentNode, err := oc.AdminKubeClient().CoreV1().Nodes().Get(cleanupCtx, nodeName, metav1.GetOptions{})
				if err != nil {
					framework.Logf("Error getting node: %v", err)
					return false
				}
				currentConfig := currentNode.Annotations["machineconfiguration.openshift.io/currentConfig"]
				desiredConfig := currentNode.Annotations["machineconfiguration.openshift.io/desiredConfig"]

				// Check if the node is using a worker config (not node-sizing-test config)
				isWorkerConfig := currentConfig != "" && !strings.Contains(currentConfig, testMCPName) && currentConfig == desiredConfig
				if isWorkerConfig {
					framework.Logf("Node %s successfully transitioned to worker config: %s", nodeName, currentConfig)
				} else {
					framework.Logf("Node %s still transitioning: current=%s, desired=%s", nodeName, currentConfig, desiredConfig)
				}
				return isWorkerConfig
			}, 10*time.Minute, 10*time.Second).Should(o.BeTrue(), fmt.Sprintf("Node %s should transition back to worker pool", nodeName))
		}

		// Register DeferCleanup so cleanup happens even on test failure
		// DeferCleanup runs in LIFO order: MCP deleted last (registered first)
		g.DeferCleanup(cleanupMCP)
		g.DeferCleanup(cleanupNodeLabel)

		g.By("Waiting for custom MachineConfigPool to be ready")
		err = waitForMCPToBeReady(ctx, mcClient, testMCPName, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred(), "Custom MachineConfigPool should become ready")

		verifyNodeSizingEnabledFile(oc, nodeName, "true")

		// Now apply KubeletConfig and verify NODE_SIZING_ENABLED=false

		g.By("Creating KubeletConfig with autoSizingReserved=false")
		autoSizingReserved := false
		kubeletConfig := &mcfgv1.KubeletConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "machineconfiguration.openshift.io/v1",
				Kind:       "KubeletConfig",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: kubeletConfigName,
			},
			Spec: mcfgv1.KubeletConfigSpec{
				AutoSizingReserved: &autoSizingReserved,
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"machineconfiguration.openshift.io/pool": testMCPName,
					},
				},
			},
		}

		_, err = mcClient.MachineconfigurationV1().KubeletConfigs().Create(ctx, kubeletConfig, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to create KubeletConfig")

		cleanupKubeletConfig := func() {
			g.By("Cleaning up KubeletConfig")
			cleanupCtx := context.Background()
			deleteErr := mcClient.MachineconfigurationV1().KubeletConfigs().Delete(cleanupCtx, kubeletConfigName, metav1.DeleteOptions{})
			if deleteErr != nil {
				framework.Logf("Failed to delete KubeletConfig %s: %v", kubeletConfigName, deleteErr)
			}

			// Wait for custom MCP to be ready after cleanup
			g.By("Waiting for custom MCP to be ready after KubeletConfig deletion")
			waitErr := waitForMCPToBeReady(cleanupCtx, mcClient, testMCPName, 10*time.Minute)
			if waitErr != nil {
				framework.Logf("Failed to wait for custom MCP to be ready: %v", waitErr)
			}
		}
		g.DeferCleanup(cleanupKubeletConfig)

		g.By("Waiting for KubeletConfig to be created")
		var createdKC *mcfgv1.KubeletConfig
		o.Eventually(func() error {
			createdKC, err = mcClient.MachineconfigurationV1().KubeletConfigs().Get(ctx, kubeletConfigName, metav1.GetOptions{})
			return err
		}, 30*time.Second, 5*time.Second).Should(o.Succeed(), "KubeletConfig should be created")

		o.Expect(createdKC.Spec.AutoSizingReserved).NotTo(o.BeNil(), "AutoSizingReserved should not be nil")
		o.Expect(*createdKC.Spec.AutoSizingReserved).To(o.BeFalse(), "AutoSizingReserved should be false")

		g.By(fmt.Sprintf("Waiting for %s MCP to start updating", testMCPName))
		o.Eventually(func() bool {
			mcp, err := mcClient.MachineconfigurationV1().MachineConfigPools().Get(ctx, testMCPName, metav1.GetOptions{})
			if err != nil {
				framework.Logf("Error getting %s MCP: %v", testMCPName, err)
				return false
			}
			// Check if MCP is updating (has conditions indicating update in progress)
			for _, condition := range mcp.Status.Conditions {
				if condition.Type == "Updating" && condition.Status == corev1.ConditionTrue {
					return true
				}
			}
			return false
		}, 2*time.Minute, 10*time.Second).Should(o.BeTrue(), fmt.Sprintf("%s MCP should start updating", testMCPName))

		g.By(fmt.Sprintf("Waiting for %s MCP to be ready with new configuration", testMCPName))
		err = waitForMCPToBeReady(ctx, mcClient, testMCPName, 15*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("%s MCP should become ready with new configuration", testMCPName))

		verifyNodeSizingEnabledFile(oc, nodeName, "false")

		// Explicit cleanup on success; DeferCleanup ensures cleanup also runs on failure
		cleanupKubeletConfig()
		cleanupNodeLabel()
		cleanupMCP()
	})
})

// verifyNodeSizingEnabledFile verifies the NODE_SIZING_ENABLED value in the env file
func verifyNodeSizingEnabledFile(oc *exutil.CLI, nodeName, expectedValue string) {
	g.By("Verifying /etc/node-sizing-enabled.env file exists")

	output, err := ExecOnNodeWithChroot(oc, nodeName, "test", "-f", "/etc/node-sizing-enabled.env")
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("File /etc/node-sizing-enabled.env should exist on node %s. Output: %s", nodeName, output))

	g.By("Reading /etc/node-sizing-enabled.env file contents")
	output, err = ExecOnNodeWithChroot(oc, nodeName, "cat", "/etc/node-sizing-enabled.env")
	o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to read /etc/node-sizing-enabled.env")

	framework.Logf("Contents of /etc/node-sizing-enabled.env:\n%s", output)

	g.By(fmt.Sprintf("Verifying NODE_SIZING_ENABLED=%s is set in the file", expectedValue))
	o.Expect(strings.TrimSpace(output)).To(o.ContainSubstring(fmt.Sprintf("NODE_SIZING_ENABLED=%s", expectedValue)),
		fmt.Sprintf("File should contain NODE_SIZING_ENABLED=%s", expectedValue))

	framework.Logf("Successfully verified NODE_SIZING_ENABLED=%s on node %s", expectedValue, nodeName)
}

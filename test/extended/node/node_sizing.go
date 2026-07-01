package node

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"

	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	machineconfigclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Suite:openshift/disruptive-longrunning][sig-node][Disruptive] Node sizing", SkipOnMicroShift, func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("node-sizing")

	g.BeforeEach(func(ctx context.Context) {
		EnsureNodesReady(ctx, oc)
	})

	g.It("should have NODE_SIZING_ENABLED=true by default and NODE_SIZING_ENABLED=false when KubeletConfig with autoSizingReserved=false is applied", func(ctx context.Context) {

		mcClient, err := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred(), "Error creating MCO client")

		testMCPName := "node-sizing-test"
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

		// Create custom MCP for the node
		mcpConfig, err := CreateCustomMCPForNode(ctx, oc, mcClient, testMCPName, nodeName)
		o.Expect(err).NotTo(o.HaveOccurred(), "Should create custom MCP")

		cleanupMCP := func() {
			cleanupCtx := context.Background()
			err := CleanupCustomMCP(cleanupCtx, mcpConfig)
			if err != nil {
				framework.Logf("Warning: cleanup had errors: %v", err)
			}
		}
		g.DeferCleanup(cleanupMCP)

		verifyNodeSizingEnabledFile(ctx, oc, nodeName, "true")

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

		g.DeferCleanup(func() {
			cleanupCtx := context.Background()
			if err := CleanupKubeletConfig(cleanupCtx, mcClient, kubeletConfigName, testMCPName); err != nil {
				framework.Logf("Warning: KubeletConfig cleanup failed: %v", err)
			}
		})

		g.By("Applying KubeletConfig and waiting for MCP rollout")
		err = ApplyKubeletConfigAndWaitForMCP(ctx, mcClient, kubeletConfig, testMCPName, 15*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred(), "Should apply KubeletConfig and complete MCP rollout")

		// Verify KubeletConfig was created with correct spec
		createdKC, err := mcClient.MachineconfigurationV1().KubeletConfigs().Get(ctx, kubeletConfigName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to get KubeletConfig")
		o.Expect(createdKC.Spec.AutoSizingReserved).NotTo(o.BeNil(), "AutoSizingReserved should not be nil")
		o.Expect(*createdKC.Spec.AutoSizingReserved).To(o.BeFalse(), "AutoSizingReserved should be false")

		verifyNodeSizingEnabledFile(ctx, oc, nodeName, "false")

		// Explicit cleanup on success; DeferCleanup ensures cleanup also runs on failure
		CleanupKubeletConfig(ctx, mcClient, kubeletConfigName, testMCPName)
		CleanupCustomMCP(ctx, mcpConfig)
	})
})

// verifyNodeSizingEnabledFile verifies the NODE_SIZING_ENABLED value in the env file
func verifyNodeSizingEnabledFile(ctx context.Context, oc *exutil.CLI, nodeName, expectedValue string) {
	g.By("Verifying /etc/node-sizing-enabled.env file exists")

	output, err := ExecOnNodeWithChroot(ctx, oc, nodeName, "test", "-f", "/etc/node-sizing-enabled.env")
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("File /etc/node-sizing-enabled.env should exist on node %s. Output: %s", nodeName, output))

	g.By("Reading /etc/node-sizing-enabled.env file contents")
	output, err = ExecOnNodeWithChroot(ctx, oc, nodeName, "cat", "/etc/node-sizing-enabled.env")
	o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to read /etc/node-sizing-enabled.env")

	framework.Logf("Contents of /etc/node-sizing-enabled.env:\n%s", output)

	g.By(fmt.Sprintf("Verifying NODE_SIZING_ENABLED=%s is set in the file", expectedValue))
	o.Expect(strings.TrimSpace(output)).To(o.ContainSubstring(fmt.Sprintf("NODE_SIZING_ENABLED=%s", expectedValue)),
		fmt.Sprintf("File should contain NODE_SIZING_ENABLED=%s", expectedValue))

	framework.Logf("Successfully verified NODE_SIZING_ENABLED=%s on node %s", expectedValue, nodeName)
}

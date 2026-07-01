package node

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	configv1 "github.com/openshift/api/config/v1"
	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	machineconfigclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
)

// This test suite validates that the kubelet TLS configuration can be upgraded
// from TLS 1.2 to TLS 1.3 via a KubeletConfig resource applied to a custom
// MachineConfigPool containing a single worker node. Using a custom pool
// avoids rebooting all workers and makes the test significantly faster.
var _ = g.Describe("[Suite:openshift/disruptive-longrunning][sig-node][Disruptive] Kubelet TLS configuration", func() {
	var (
		oc                = exutil.NewCLIWithoutNamespace("node-kubeletconfig-tls")
		kubeletConfigName = "tls13-kubelet-config"
		testMCPName       = "kubelet-tls-test"
	)

	skipUnsupportedTopologies := func() {
		skipOnSingleNodeTopology(oc)
		skipOnTwoNodeTopology(oc)

		controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		if *controlPlaneTopology == configv1.ExternalTopologyMode {
			g.Skip("Skipping test on External (Hypershift) topology - MachineConfig API not available")
		}
	}

	g.It("should upgrade kubelet TLS from 1.2 to 1.3 on a custom pool [apigroup:machineconfiguration.openshift.io]", func(ctx context.Context) {
		skipUnsupportedTopologies()

		mcClient, err := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred(), "Error creating machine configuration client")

		g.By("Selecting a worker node for testing")
		allWorkers, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
		o.Expect(err).NotTo(o.HaveOccurred(), "Error listing worker nodes")
		workerNodes := getPureWorkerNodes(allWorkers)
		o.Expect(len(workerNodes)).To(o.BeNumerically(">", 0), "No pure worker nodes found in the cluster")

		testNode := workerNodes[0].Name
		o.Expect(isNodeInReadyState(&workerNodes[0])).To(o.BeTrue(), "Worker node %s is not in Ready state", testNode)
		framework.Logf("Selected node %s for TLS upgrade test", testNode)

		g.By("Checking default TLS configuration")
		nodeCfg, err := getKubeletConfigFromNode(ctx, oc, testNode)
		o.Expect(err).NotTo(o.HaveOccurred(), "Error reading kubelet config from node %s", testNode)
		defaultTLSVersion := nodeCfg.TLSMinVersion
		framework.Logf("Default TLS version: %q", defaultTLSVersion)

		if defaultTLSVersion == "VersionTLS13" {
			framework.Failf("Worker kubelet default is already TLS 1.3; test needs to be updated to validate a different TLS configuration change")
		}
		if defaultTLSVersion != "" && defaultTLSVersion != "VersionTLS12" {
			framework.Failf("Unexpected default TLS version %q, expected VersionTLS12 or empty (cluster default)", defaultTLSVersion)
		}

		// Create custom MCP for the node
		mcpConfig, err := CreateCustomMCPForNode(ctx, oc, mcClient, testMCPName, testNode)
		o.Expect(err).NotTo(o.HaveOccurred(), "Should create custom MCP")

		cleanupMCP := func() {
			cleanupCtx := context.Background()
			err := CleanupCustomMCP(cleanupCtx, mcpConfig)
			if err != nil {
				framework.Logf("Warning: cleanup had errors: %v", err)
			}
		}
		g.DeferCleanup(cleanupMCP)

		g.By(fmt.Sprintf("Creating KubeletConfig with Modern TLS profile (TLS 1.3) targeting pool %s", testMCPName))
		kubeletConfig := &mcfgv1.KubeletConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: kubeletConfigName,
			},
			Spec: mcfgv1.KubeletConfigSpec{
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"machineconfiguration.openshift.io/pool": testMCPName,
					},
				},
				TLSSecurityProfile: &configv1.TLSSecurityProfile{
					Type: configv1.TLSProfileModernType,
				},
			},
		}

		g.DeferCleanup(func() {
			cleanupCtx := context.Background()
			err := CleanupKubeletConfig(cleanupCtx, mcClient, kubeletConfigName, testMCPName)
			o.Expect(err).NotTo(o.HaveOccurred(), "Cleanup: failed to delete KubeletConfig %s", kubeletConfigName)
		})

		g.By("Applying KubeletConfig and waiting for MCP rollout")
		err = ApplyKubeletConfigAndWaitForMCP(ctx, mcClient, kubeletConfig, testMCPName, 15*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred(), "Should apply KubeletConfig and complete MCP rollout")

		g.By(fmt.Sprintf("Verifying node %s is Ready and Done after rollout", testNode))
		updatedNode, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, testNode, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Error getting node %s after rollout", testNode)
		o.Expect(isNodeInReadyState(updatedNode)).To(o.BeTrue(), "Node %s is not in Ready state after rollout", testNode)

		nodeState := updatedNode.Annotations["machineconfiguration.openshift.io/state"]
		o.Expect(nodeState).To(o.Equal("Done"), "Node %s is in state %q, expected 'Done'", testNode, nodeState)

		g.By("Verifying TLS version upgraded to 1.3")
		expectedTLSVersion := "VersionTLS13"
		nodeCfg, err = getKubeletConfigFromNode(ctx, oc, testNode)
		o.Expect(err).NotTo(o.HaveOccurred(), "Error reading kubelet config from node %s after rollout", testNode)
		actualTLSVersion := nodeCfg.TLSMinVersion

		o.Expect(actualTLSVersion).To(o.Equal(expectedTLSVersion),
			"TLS version should be %q, but got %q", expectedTLSVersion, actualTLSVersion)

		framework.Logf("Successfully verified kubelet TLS upgrade from %s to %s on node %s",
			defaultTLSVersion, actualTLSVersion, testNode)
	})
})

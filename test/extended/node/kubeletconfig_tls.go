package node

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	configv1 "github.com/openshift/api/config/v1"
	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	machineconfigclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"

	"k8s.io/kubernetes/test/e2e/framework"
)

// This test suite validates that the kubelet TLS configuration can be upgraded
// from TLS 1.2 to TLS 1.3 via a KubeletConfig resource applied to a custom
// MachineConfigPool containing a single worker node. Using a custom pool
// avoids rebooting all workers and makes the test significantly faster.
var _ = g.Describe("[Suite:openshift/disruptive-longrunning][sig-node][Disruptive] Kubelet TLS configuration", func() {
	defer g.GinkgoRecover()
	var (
		oc                = exutil.NewCLIWithoutNamespace("node-kubeletconfig-tls")
		kubeletConfigName = "tls13-kubelet-config"
		testMCPName       = "kubelet-tls-test"
		testNodeMCPLabel  = fmt.Sprintf("node-role.kubernetes.io/%s", testMCPName)
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

		framework.Logf("1) Selecting a worker node for testing")
		allWorkers, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
		o.Expect(err).NotTo(o.HaveOccurred(), "Error listing worker nodes")
		workerNodes := getPureWorkerNodes(allWorkers)
		o.Expect(len(workerNodes)).To(o.BeNumerically(">", 0), "No pure worker nodes found in the cluster")

		testNode := workerNodes[0].Name
		o.Expect(isNodeInReadyState(&workerNodes[0])).To(o.BeTrue(), "Worker node %s is not in Ready state", testNode)
		framework.Logf("Selected node %s for TLS upgrade test", testNode)

		framework.Logf("2) Checking default TLS configuration")
		nodeCfg, err := getKubeletConfigFromNode(ctx, oc, testNode)
		o.Expect(err).NotTo(o.HaveOccurred(), "Error reading kubelet config from node %s", testNode)
		defaultTLSVersion := nodeCfg.TLSMinVersion
		framework.Logf("Default TLS version: %q", defaultTLSVersion)

		if defaultTLSVersion == "VersionTLS13" {
			g.Skip("Worker kubelet is already on TLS 1.3; nothing to upgrade (leaked KubeletConfig or product default changed)")
		}
		if defaultTLSVersion != "" && defaultTLSVersion != "VersionTLS12" {
			framework.Failf("Unexpected default TLS version %q, expected VersionTLS12 or empty (cluster default)", defaultTLSVersion)
		}

		framework.Logf("3) Labeling node %s with %s", testNode, testNodeMCPLabel)
		patchData := []byte(fmt.Sprintf(`{"metadata":{"labels":{%q:""}}}`, testNodeMCPLabel))
		_, err = oc.AdminKubeClient().CoreV1().Nodes().Patch(ctx, testNode, types.MergePatchType, patchData, metav1.PatchOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to label node %s", testNode)

		cleanupNodeLabel := func() {
			framework.Logf("Cleanup: removing label %s from node %s", testNodeMCPLabel, testNode)
			cleanupCtx := context.Background()
			removePatch := []byte(fmt.Sprintf(`{"metadata":{"labels":{%q:null}}}`, testNodeMCPLabel))
			_, patchErr := oc.AdminKubeClient().CoreV1().Nodes().Patch(cleanupCtx, testNode, types.MergePatchType, removePatch, metav1.PatchOptions{})
			if apierrors.IsNotFound(patchErr) {
				return
			}
			if patchErr != nil {
				framework.Logf("Failed to remove label from node %s: %v", testNode, patchErr)
				return
			}

			framework.Logf("Cleanup: waiting for node %s to transition back to worker pool", testNode)
			o.Eventually(func() bool {
				currentNode, getErr := oc.AdminKubeClient().CoreV1().Nodes().Get(cleanupCtx, testNode, metav1.GetOptions{})
				if getErr != nil {
					framework.Logf("Error getting node: %v", getErr)
					return false
				}
				currentConfig := currentNode.Annotations["machineconfiguration.openshift.io/currentConfig"]
				desiredConfig := currentNode.Annotations["machineconfiguration.openshift.io/desiredConfig"]
				isWorkerConfig := currentConfig != "" && !strings.Contains(currentConfig, testMCPName) && currentConfig == desiredConfig
				if isWorkerConfig {
					framework.Logf("Node %s transitioned back to worker config: %s", testNode, currentConfig)
				} else {
					framework.Logf("Node %s still transitioning: current=%s, desired=%s", testNode, currentConfig, desiredConfig)
				}
				return isWorkerConfig
			}, 15*time.Minute, 15*time.Second).Should(o.BeTrue(),
				"Node %s should transition back to worker pool", testNode)
		}
		g.DeferCleanup(cleanupNodeLabel)

		framework.Logf("4) Creating custom MachineConfigPool %s", testMCPName)
		testMCP := &mcfgv1.MachineConfigPool{
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
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to create custom MachineConfigPool %s", testMCPName)

		cleanupMCP := func() {
			framework.Logf("Cleanup: deleting MachineConfigPool %s", testMCPName)
			cleanupCtx := context.Background()
			deleteErr := mcClient.MachineconfigurationV1().MachineConfigPools().Delete(cleanupCtx, testMCPName, metav1.DeleteOptions{})
			if apierrors.IsNotFound(deleteErr) {
				return
			}
			if deleteErr != nil {
				framework.Logf("Failed to delete MachineConfigPool %s: %v", testMCPName, deleteErr)
			}
		}
		// DeferCleanup is a safety net for failures. On success, explicit
		// cleanup calls at the end of the test run in the correct order.
		g.DeferCleanup(cleanupMCP)

		framework.Logf("Waiting for custom MachineConfigPool %s to be ready", testMCPName)
		err = waitForMCP(ctx, mcClient, testMCPName, 10*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred(), "Custom MachineConfigPool %s did not become ready", testMCPName)

		framework.Logf("5) Creating KubeletConfig with TLS 1.3 targeting pool %s", testMCPName)
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
				KubeletConfig: &runtime.RawExtension{
					Raw: []byte(`{"tlsMinVersion":"VersionTLS13"}`),
				},
			},
		}

		cleanupKubeletConfig := func() {
			framework.Logf("Cleanup: deleting KubeletConfig %s", kubeletConfigName)
			cleanupCtx := context.Background()
			deleteErr := mcClient.MachineconfigurationV1().KubeletConfigs().Delete(cleanupCtx, kubeletConfigName, metav1.DeleteOptions{})
			if apierrors.IsNotFound(deleteErr) {
				return
			}
			o.Expect(deleteErr).NotTo(o.HaveOccurred(), "Cleanup: failed to delete KubeletConfig %s", kubeletConfigName)

			framework.Logf("Cleanup: waiting for MCP %s to become ready after KubeletConfig deletion", testMCPName)
			waitErr := waitForMCP(cleanupCtx, mcClient, testMCPName, 15*time.Minute)
			if apierrors.IsNotFound(waitErr) {
				return
			}
			o.Expect(waitErr).NotTo(o.HaveOccurred(),
				"Cleanup: MCP %s did not become ready after KubeletConfig deletion", testMCPName)
		}
		g.DeferCleanup(cleanupKubeletConfig)

		_, err = mcClient.MachineconfigurationV1().KubeletConfigs().Create(ctx, kubeletConfig, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Error creating KubeletConfig with TLS 1.3")

		framework.Logf("6) Waiting for MachineConfigPool %s to begin updating", testMCPName)
		err = wait.PollUntilContextTimeout(ctx, 15*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
			mcp, getErr := mcClient.MachineconfigurationV1().MachineConfigPools().Get(ctx, testMCPName, metav1.GetOptions{})
			if getErr != nil {
				framework.Logf("Error getting MCP %s: %v", testMCPName, getErr)
				return false, nil
			}
			for _, condition := range mcp.Status.Conditions {
				if condition.Type == "Updating" && condition.Status == corev1.ConditionTrue {
					framework.Logf("MCP %s has started updating", testMCPName)
					return true, nil
				}
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(),
			"Timed out waiting for MachineConfigPool %q to start updating", testMCPName)

		framework.Logf("7) Waiting for MachineConfigPool %s to complete rollout", testMCPName)
		err = waitForMCP(ctx, mcClient, testMCPName, 30*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred(), "Error waiting for MachineConfigPool %q to become ready", testMCPName)
		framework.Logf("MachineConfigPool %s has completed rollout", testMCPName)

		framework.Logf("8) Verifying node %s is Ready and Done after rollout", testNode)
		updatedNode, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, testNode, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Error getting node %s after rollout", testNode)
		o.Expect(isNodeInReadyState(updatedNode)).To(o.BeTrue(), "Node %s is not in Ready state after rollout", testNode)

		nodeState := updatedNode.Annotations["machineconfiguration.openshift.io/state"]
		o.Expect(nodeState).To(o.Equal("Done"), "Node %s is in state %q, expected 'Done'", testNode, nodeState)

		framework.Logf("9) Verifying TLS version upgraded to 1.3")
		expectedTLSVersion := "VersionTLS13"
		nodeCfg, err = getKubeletConfigFromNode(ctx, oc, testNode)
		o.Expect(err).NotTo(o.HaveOccurred(), "Error reading kubelet config from node %s after rollout", testNode)
		actualTLSVersion := nodeCfg.TLSMinVersion

		o.Expect(actualTLSVersion).To(o.Equal(expectedTLSVersion),
			"TLS version should be %q, but got %q", expectedTLSVersion, actualTLSVersion)

		framework.Logf("Successfully verified kubelet TLS upgrade from %s to %s on node %s",
			defaultTLSVersion, actualTLSVersion, testNode)

		// Explicit cleanup on success in the correct order: KC first (revert
		// TLS on custom pool), then label (node transitions back to worker
		// pool while the custom MCP still exists), then MCP (now empty).
		// DeferCleanup remains as a safety net for failures; each function
		// is idempotent via IsNotFound checks.
		cleanupKubeletConfig()
		cleanupNodeLabel()
		cleanupMCP()
	})
})

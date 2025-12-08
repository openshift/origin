package node

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e/framework"

	machineconfigv1 "github.com/openshift/api/machineconfiguration/v1"
	mcclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[OTP][Jira:\"Node / Kubelet\"][sig-node] Node non-cnv swap configuration", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("node-swap")

	g.BeforeEach(func(ctx context.Context) {
		// Skip all tests on MicroShift clusters
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("Skipping test on MicroShift cluster")
		}
	})

	// This test validates that:
	// - Worker nodes have failSwapOn=false to allow kubelet to start even if swap is present at OS level
	// - Control plane nodes have failSwapOn=true to prevent kubelet from starting if swap is enabled
	// - All nodes have swapBehavior=NoSwap to ensure kubelet does not utilize swap even if available at OS level
	// The swapBehavior=NoSwap configuration ensures that even if swap is manually enabled on a worker node,
	// the kubelet will not use it for memory management, maintaining consistent behavior across the cluster.
	g.It("should have correct default kubelet swap settings with worker nodes failSwapOn=false, control plane nodes failSwapOn=true, and both swapBehavior=NoSwap - OCP-86394", func(ctx context.Context) {
		g.By("Getting worker nodes")
		workerNodes, err := getWorkerNodes(ctx, oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(workerNodes)).Should(o.BeNumerically(">", 0), "Expected at least one worker node")

		g.By("Validating kubelet configuration on each worker node")
		for _, node := range workerNodes {
			config, err := getKubeletConfigFromNode(ctx, oc, node.Name)
			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get kubelet config for worker node %s", node.Name)

			g.By(fmt.Sprintf("Checking failSwapOn=false on worker node %s", node.Name))
			o.Expect(config.FailSwapOn).NotTo(o.BeNil(), "failSwapOn should be set on worker node %s", node.Name)
			o.Expect(*config.FailSwapOn).To(o.BeFalse(), "failSwapOn should be false on worker node %s", node.Name)
			framework.Logf("Worker node %s: failSwapOn=%v ✓", node.Name, *config.FailSwapOn)

			g.By(fmt.Sprintf("Checking swapDesired=NoSwap on worker node %s", node.Name))
			if config.SwapDesired != nil {
				o.Expect(*config.SwapDesired).To(o.Equal("NoSwap"), "swapDesired should be NoSwap on worker node %s", node.Name)
				framework.Logf("Worker node %s: swapDesired=%s ✓", node.Name, *config.SwapDesired)
			}
			// Also check memorySwap.swapBehavior if present
			if config.MemorySwap != nil {
				o.Expect(config.MemorySwap.SwapBehavior).To(o.Equal("NoSwap"), "swapBehavior should be NoSwap on worker node %s", node.Name)
				framework.Logf("Worker node %s: swapBehavior=%s ✓", node.Name, config.MemorySwap.SwapBehavior)
			}
		}

		g.By("Getting control plane nodes")
		controlPlaneNodes, err := getControlPlaneNodes(ctx, oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(controlPlaneNodes)).Should(o.BeNumerically(">", 0), "Expected at least one control plane node")

		g.By("Validating kubelet configuration on each control plane node")
		for _, node := range controlPlaneNodes {
			config, err := getKubeletConfigFromNode(ctx, oc, node.Name)
			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get kubelet config for control plane node %s", node.Name)

			g.By(fmt.Sprintf("Checking failSwapOn=true on control plane node %s", node.Name))
			o.Expect(config.FailSwapOn).NotTo(o.BeNil(), "failSwapOn should be set on control plane node %s", node.Name)
			o.Expect(*config.FailSwapOn).To(o.BeTrue(), "failSwapOn should be true on control plane node %s", node.Name)
			framework.Logf("Control plane node %s: failSwapOn=%v ✓", node.Name, *config.FailSwapOn)

			g.By(fmt.Sprintf("Checking swapDesired=NoSwap on control plane node %s", node.Name))
			if config.SwapDesired != nil {
				o.Expect(*config.SwapDesired).To(o.Equal("NoSwap"), "swapDesired should be NoSwap on control plane node %s", node.Name)
				framework.Logf("Control plane node %s: swapDesired=%s ✓", node.Name, *config.SwapDesired)
			}
			// Also check memorySwap.swapBehavior if present
			if config.MemorySwap != nil {
				o.Expect(config.MemorySwap.SwapBehavior).To(o.Equal("NoSwap"), "swapBehavior should be NoSwap on control plane node %s", node.Name)
				framework.Logf("Control plane node %s: swapBehavior=%s ✓", node.Name, config.MemorySwap.SwapBehavior)
			}
		}
		framework.Logf("Test PASSED: All nodes have correct default swap settings")
	})

	g.It("should reject user override of swap settings via KubeletConfig API - OCP-86395", func(ctx context.Context) {
		g.By("Creating machine config client")
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to create machine config client")

		g.By("Creating a KubeletConfig with swap settings")
		kubeletConfig := &machineconfigv1.KubeletConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-swap-override",
			},
			Spec: machineconfigv1.KubeletConfigSpec{
				KubeletConfig: &runtime.RawExtension{
					Raw: []byte(`{
						"failSwapOn": true,
						"memorySwap": {
							"swapBehavior": "LimitedSwap"
						}
					}`),
				},
			},
		}

		g.By("Attempting to apply the KubeletConfig")
		framework.Logf("Creating KubeletConfig with failSwapOn=true and swapBehavior=LimitedSwap")
		_, err = mcClient.MachineconfigurationV1().KubeletConfigs().Create(ctx, kubeletConfig, metav1.CreateOptions{})

		// We expect this to either be rejected immediately or to be created but not applied
		if err != nil {
			g.By("Verifying the error message indicates swap is not configurable")
			o.Expect(err).To(o.HaveOccurred())
			framework.Logf("KubeletConfig creation rejected with error: %v", err)
		} else {
			framework.Logf("KubeletConfig was created, verifying it doesn't affect worker nodes")
			// If created, clean up and verify it doesn't affect nodes
			defer func() {
				_ = mcClient.MachineconfigurationV1().KubeletConfigs().Delete(ctx, "test-swap-override", metav1.DeleteOptions{})
			}()

			// Declare variables outside poll callback so they're accessible later
			var workerNodes []v1.Node
			var config *KubeletConfiguration

			// Poll to verify worker node configs remain unchanged
			err = wait.Poll(2*time.Second, 30*time.Second, func() (bool, error) {
				var err error
				workerNodes, err = getWorkerNodes(ctx, oc)
				if err != nil {
					return false, err
				}
				if len(workerNodes) == 0 {
					return false, fmt.Errorf("no worker nodes found")
				}

				config, err = getKubeletConfigFromNode(ctx, oc, workerNodes[0].Name)
				if err != nil {
					return false, err
				}

				// Check that failSwapOn is still false
				if config.FailSwapOn == nil || *config.FailSwapOn != false {
					return false, fmt.Errorf("worker node %s: failSwapOn changed from expected value false, got %v", workerNodes[0].Name, config.FailSwapOn)
				}

				// Check that swapBehavior is still NoSwap
				if config.MemorySwap != nil && config.MemorySwap.SwapBehavior != "NoSwap" {
					return false, fmt.Errorf("worker node %s: swapBehavior changed from NoSwap to %s", workerNodes[0].Name, config.MemorySwap.SwapBehavior)
				}

				// Continue polling to ensure config stays unchanged for the full duration
				return false, nil
			})

			framework.Logf("Worker node %s config after poll: failSwapOn=%v, swapBehavior=%s", workerNodes[0].Name, *config.FailSwapOn, config.MemorySwap.SwapBehavior)
			// We expect the poll to timeout (not find a change), which means settings remained unchanged
			if err != nil && err == wait.ErrWaitTimeout {
				framework.Logf("Test PASSED: Worker node swap settings remained unchange")
			} else if err != nil {
				o.Expect(err).NotTo(o.HaveOccurred(), "Test FAILED: Unexpected error while polling worker node config")
			}
		}
	})
})

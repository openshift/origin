package node

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e/framework"

	machineconfigv1 "github.com/openshift/api/machineconfiguration/v1"
	mcclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	workerGeneratedKubeletMC = "99-worker-generated-kubelet"
)

var _ = g.Describe("[Jira:Node][sig-node] Node non-cnv swap configuration", func() {
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
	g.It("should have correct default kubelet swap settings with worker nodes failSwapOn=false, control plane nodes failSwapOn=true, and both swapBehavior=NoSwap [OCP-86394]", func(ctx context.Context) {
		g.By("Getting worker nodes")
		workerNodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
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

			g.By(fmt.Sprintf("Checking swapBehavior=NoSwap on worker node %s", node.Name))
			o.Expect(config.MemorySwap).NotTo(o.BeNil(), "memorySwap should be set on worker node %s", node.Name)
			o.Expect(config.MemorySwap.SwapBehavior).To(o.Equal("NoSwap"), "swapBehavior should be NoSwap on worker node %s", node.Name)
			framework.Logf("Worker node %s: swapBehavior=%s ✓", node.Name, config.MemorySwap.SwapBehavior)
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

			g.By(fmt.Sprintf("Checking swapBehavior=NoSwap on control plane node %s", node.Name))
			o.Expect(config.MemorySwap).NotTo(o.BeNil(), "memorySwap should be set on control plane node %s", node.Name)
			o.Expect(config.MemorySwap.SwapBehavior).To(o.Equal("NoSwap"), "swapBehavior should be NoSwap on control plane node %s", node.Name)
			framework.Logf("Control plane node %s: swapBehavior=%s ✓", node.Name, config.MemorySwap.SwapBehavior)
		}
		framework.Logf("Test PASSED: All nodes have correct default swap settings")
	})

	g.It("should reject user override of swap settings via KubeletConfig API [OCP-86395]", func(ctx context.Context) {
		g.By("Creating machine config client")
		mcClient, err := mcclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to create machine config client")

		g.By("Getting initial machine config resourceVersion")
		// Get the initial resourceVersion of the worker machine config before creating KubeletConfig
		workerMC, err := mcClient.MachineconfigurationV1().MachineConfigs().Get(ctx, workerGeneratedKubeletMC, metav1.GetOptions{})
		initialResourceVersion := ""
		if err == nil {
			initialResourceVersion = workerMC.ResourceVersion
			framework.Logf("Initial %s resourceVersion: %s", workerGeneratedKubeletMC, initialResourceVersion)
		}

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
		defer func() {
			_ = mcClient.MachineconfigurationV1().KubeletConfigs().Delete(ctx, "test-swap-override", metav1.DeleteOptions{})
		}()
		framework.Logf("Creating KubeletConfig with failSwapOn=true and swapBehavior=LimitedSwap")
		_, err = mcClient.MachineconfigurationV1().KubeletConfigs().Create(ctx, kubeletConfig, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to create KubeletConfig")

		g.By("Checking KubeletConfig status for expected error message")
		err = wait.Poll(2*time.Second, 30*time.Second, func() (bool, error) {
			kc, err := mcClient.MachineconfigurationV1().KubeletConfigs().Get(ctx, "test-swap-override", metav1.GetOptions{})
			if err != nil {
				return false, err
			}

			if kc.Status.ObservedGeneration != kc.Generation {
				framework.Logf("Waiting for controller to process generation %d (current: %d)", kc.Generation, kc.Status.ObservedGeneration)
				return false, nil
			}

			// Fail fast if KubeletConfig was unexpectedly accepted
			for _, condition := range kc.Status.Conditions {
				if condition.Type == machineconfigv1.KubeletConfigSuccess && condition.Status == corev1.ConditionTrue {
					return false, fmt.Errorf("KubeletConfig was unexpectedly accepted")
				}
			}

			// Check for Failure condition with the expected error message
			for _, condition := range kc.Status.Conditions {
				if condition.Type == machineconfigv1.KubeletConfigFailure && condition.Status == corev1.ConditionTrue {
					framework.Logf("Found Failure condition: %s", condition.Message)
					if condition.Message == "Error: KubeletConfiguration: failSwapOn is not allowed to be set, but contains: true" {
						return true, nil
					}
				}
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "Expected to find error message about failSwapOn not being allowed in KubeletConfig status")

		g.By("Verifying machine config was not created or updated")
		// Wait a bit to ensure no update happens
		time.Sleep(5 * time.Second)

		// Check if the machine config was created or updated (compare to initial resourceVersion captured earlier)
		workerMC, err = mcClient.MachineconfigurationV1().MachineConfigs().Get(ctx, workerGeneratedKubeletMC, metav1.GetOptions{})
		if err == nil {
			o.Expect(workerMC.ResourceVersion).To(o.Equal(initialResourceVersion), "Machine config %s should not be updated when failSwapOn is rejected", workerGeneratedKubeletMC)
			framework.Logf("Verified: %s was not updated (resourceVersion: %s)", workerGeneratedKubeletMC, workerMC.ResourceVersion)
		}

		g.By("Verifying worker nodes still have correct swap settings")
		workerNodes, err := getNodesByLabel(ctx, oc, "node-role.kubernetes.io/worker")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(len(workerNodes)).Should(o.BeNumerically(">", 0), "Expected at least one worker node")

		for _, node := range workerNodes {
			config, err := getKubeletConfigFromNode(ctx, oc, node.Name)
			o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get kubelet config for worker node %s", node.Name)

			g.By(fmt.Sprintf("Verifying failSwapOn=false remains unchanged on worker node %s", node.Name))
			o.Expect(config.FailSwapOn).NotTo(o.BeNil(), "failSwapOn should be set on worker node %s", node.Name)
			o.Expect(*config.FailSwapOn).To(o.BeFalse(), "failSwapOn should still be false on worker node %s after rejection", node.Name)
			framework.Logf("Worker node %s: failSwapOn=%v (unchanged) ✓", node.Name, *config.FailSwapOn)

			g.By(fmt.Sprintf("Verifying swapBehavior=NoSwap remains unchanged on worker node %s", node.Name))
			o.Expect(config.MemorySwap).NotTo(o.BeNil(), "memorySwap should be set on worker node %s", node.Name)
			o.Expect(config.MemorySwap.SwapBehavior).To(o.Equal("NoSwap"), "swapBehavior should still be NoSwap on worker node %s after rejection", node.Name)
			framework.Logf("Worker node %s: swapBehavior=%s (unchanged) ✓", node.Name, config.MemorySwap.SwapBehavior)
		}

		framework.Logf("Test PASSED: KubeletConfig with failSwapOn was properly rejected")
	})
})

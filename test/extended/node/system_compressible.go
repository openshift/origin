package node

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	"k8s.io/kubernetes/test/e2e/framework"

	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	machineconfigclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Suite:openshift/disruptive-longrunning][sig-node][Disruptive] System Compressible CPU", g.Serial, func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("system-compressible")

	g.BeforeEach(func(ctx context.Context) {
		// Skip all tests on MicroShift clusters
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("Skipping test on MicroShift cluster")
		}
	})

	g.It("should enforce system compressible CPU limit by default", func(ctx context.Context) {
		// Select node with >= 4 CPUs
		nodeName, cpuCount, err := selectTestNode(ctx, oc, 4)
		o.Expect(err).NotTo(o.HaveOccurred(), "Should find a node with at least 4 CPUs")
		framework.Logf("Testing on node: %s with %d CPUs", nodeName, cpuCount)

		// Get kubelet config and verify system compressible is enabled
		config, err := getKubeletConfigFromNode(ctx, oc, nodeName)
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to read kubelet config")

		// Skip if reserved CPU is enabled
		if isReservedCPUEnabled(config) {
			g.Skip("Skipping: cluster uses reserved CPU feature")
		}

		// Verify system compressible is enabled
		o.Expect(isSystemCompressibleEnabled(config)).To(o.BeTrue(),
			"System compressible should be enabled by default")

		// Read SYSTEM_RESERVED_CPU from /etc/node-sizing.env
		g.By("Reading SYSTEM_RESERVED_CPU from /etc/node-sizing.env")
		nodeSizingOutput, err := ExecOnNodeWithChroot(oc, nodeName, "cat", "/etc/node-sizing.env")
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to read /etc/node-sizing.env")
		framework.Logf("/etc/node-sizing.env contents:\n%s", nodeSizingOutput)

		// Parse SYSTEM_RESERVED_CPU value (e.g., "0.5" means 500m)
		var systemReservedCPU float64
		for _, line := range strings.Split(nodeSizingOutput, "\n") {
			if strings.HasPrefix(line, "SYSTEM_RESERVED_CPU=") {
				cpuStr := strings.TrimPrefix(line, "SYSTEM_RESERVED_CPU=")
				systemReservedCPU, err = strconv.ParseFloat(cpuStr, 64)
				o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to parse SYSTEM_RESERVED_CPU value: %s", cpuStr)
				break
			}
		}
		o.Expect(systemReservedCPU).To(o.BeNumerically(">", 0), "SYSTEM_RESERVED_CPU should be set")
		framework.Logf("SYSTEM_RESERVED_CPU: %.2f (%.0f millicores)", systemReservedCPU, systemReservedCPU*1000)

		// Convert to cpuShares: cpuShares = systemReservedCPU * 1024
		cpuShares := uint64(systemReservedCPU * 1024)
		expectedWeight := getCPUWeight(&cpuShares)
		framework.Logf("Expected cpu.weight for cpuShares=%d: %d", cpuShares, expectedWeight)

		// Check cgroup cpu.weight configuration for system.slice
		g.By("Verifying system.slice cgroup CPU weight")
		actualWeight, err := readCgroupCPUWeight(oc, nodeName, "system.slice")
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to read cpu.weight for system.slice")
		framework.Logf("system.slice actual cpu.weight: %d", actualWeight)

		o.Expect(actualWeight).To(o.Equal(expectedWeight),
			"system.slice cpu.weight should be %d (cpuShares=%d, SYSTEM_RESERVED_CPU=%.2f) when system compressible is enabled",
			expectedWeight, cpuShares, systemReservedCPU)

		framework.Logf("System compressible CPU weight verified successfully")
	})

	g.It("should not enforce CPU limit when system compressible is disabled", func(ctx context.Context) {
		mcClient, err := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred(), "Error creating MCO client")

		testMCPName := "system-compressible-test"
		testNodeMCPLabel := fmt.Sprintf("node-role.kubernetes.io/%s", testMCPName)
		kubeletConfigName := "system-compressible-override"

		// Select node
		nodeName, cpuCount, err := selectTestNode(ctx, oc, 4)
		o.Expect(err).NotTo(o.HaveOccurred(), "Should find a node with at least 4 CPUs")
		framework.Logf("Testing on node: %s with %d CPUs", nodeName, cpuCount)

		// Setup cleanup functions
		cleanupNodeLabel := func() {
			g.By(fmt.Sprintf("Removing node label %s from node %s", testNodeMCPLabel, nodeName))
			cleanupCtx := context.Background()
			patchData := []byte(fmt.Sprintf(`{"metadata":{"labels":{%q:null}}}`, testNodeMCPLabel))
			_, updateErr := oc.AdminKubeClient().CoreV1().Nodes().Patch(cleanupCtx, nodeName, types.MergePatchType, patchData, metav1.PatchOptions{})
			if apierrors.IsNotFound(updateErr) {
				// Node already deleted, nothing to clean up
				return
			} else if updateErr != nil {
				framework.Failf("Failed to remove label from node %s: %v", nodeName, updateErr)
			}

			g.By(fmt.Sprintf("Waiting for node %s to transition back to worker pool", nodeName))
			o.Eventually(func() bool {
				currentNode, err := oc.AdminKubeClient().CoreV1().Nodes().Get(cleanupCtx, nodeName, metav1.GetOptions{})
				if err != nil {
					return false
				}
				currentConfig := currentNode.Annotations["machineconfiguration.openshift.io/currentConfig"]
				desiredConfig := currentNode.Annotations["machineconfiguration.openshift.io/desiredConfig"]
				isWorkerConfig := currentConfig != "" && !strings.Contains(currentConfig, testMCPName) && currentConfig == desiredConfig
				return isWorkerConfig
			}, 7*time.Minute, 10*time.Second).Should(o.BeTrue())
		}

		cleanupKubeletConfig := func() {
			g.By("Cleaning up KubeletConfig")
			cleanupCtx := context.Background()
			deleteErr := mcClient.MachineconfigurationV1().KubeletConfigs().Delete(cleanupCtx, kubeletConfigName, metav1.DeleteOptions{})
			if apierrors.IsNotFound(deleteErr) {
				// KubeletConfig already deleted, nothing to clean up
			} else if deleteErr != nil {
				framework.Failf("Failed to delete KubeletConfig %s: %v", kubeletConfigName, deleteErr)
			}
		}

		cleanupMCP := func() {
			g.By("Cleaning up custom MachineConfigPool")
			cleanupCtx := context.Background()
			deleteErr := mcClient.MachineconfigurationV1().MachineConfigPools().Delete(cleanupCtx, testMCPName, metav1.DeleteOptions{})
			if apierrors.IsNotFound(deleteErr) {
				// MachineConfigPool already deleted, nothing to clean up
			} else if deleteErr != nil {
				framework.Failf("Failed to delete MachineConfigPool %s: %v", testMCPName, deleteErr)
			}

			// Wait for worker MCP to stabilize after custom MCP deletion
			g.By("Waiting for worker MCP to stabilize after custom MCP deletion")
			waitErr := waitForMCP(cleanupCtx, mcClient, "worker", 10*time.Minute)
			if apierrors.IsNotFound(waitErr) {
				// MachineConfigPool already deleted, nothing to wait for
			} else if waitErr != nil {
				framework.Failf("Worker MCP did not stabilize after custom MCP deletion: %v", waitErr)
			}
		}

		// Register cleanups in LIFO order
		g.DeferCleanup(cleanupMCP)
		g.DeferCleanup(cleanupKubeletConfig)
		g.DeferCleanup(cleanupNodeLabel)

		// Label node
		g.By(fmt.Sprintf("Labeling node %s with %s", nodeName, testNodeMCPLabel))
		patchData := []byte(fmt.Sprintf(`{"metadata":{"labels":{%q:""}}}`, testNodeMCPLabel))
		_, err = oc.AdminKubeClient().CoreV1().Nodes().Patch(ctx, nodeName, types.MergePatchType, patchData, metav1.PatchOptions{})
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
		o.Expect(err).NotTo(o.HaveOccurred(), "Should create custom MachineConfigPool")

		// Wait for MCP ready
		g.By("Waiting for custom MachineConfigPool to be ready")
		err = waitForMCP(ctx, mcClient, testMCPName, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred(), "MCP should be ready")

		// Create KubeletConfig to disable system compressible
		g.By("Creating KubeletConfig to disable system compressible")
		kubeletConfig := &mcfgv1.KubeletConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "machineconfiguration.openshift.io/v1",
				Kind:       "KubeletConfig",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: kubeletConfigName,
			},
			Spec: mcfgv1.KubeletConfigSpec{
				KubeletConfig: &runtime.RawExtension{
					Raw: []byte(`{"systemReservedCgroup":"","enforceNodeAllocatable":["pods"]}`),
				},
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"machineconfiguration.openshift.io/pool": testMCPName,
					},
				},
			},
		}

		_, err = mcClient.MachineconfigurationV1().KubeletConfigs().Create(ctx, kubeletConfig, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Should create KubeletConfig")

		// Wait for MCP to start updating
		g.By(fmt.Sprintf("Waiting for %s MCP to start updating", testMCPName))
		o.Eventually(func() bool {
			mcp, err := mcClient.MachineconfigurationV1().MachineConfigPools().Get(ctx, testMCPName, metav1.GetOptions{})
			if err != nil {
				framework.Logf("Error getting %s MCP: %v", testMCPName, err)
				return false
			}
			for _, condition := range mcp.Status.Conditions {
				if condition.Type == "Updating" && condition.Status == corev1.ConditionTrue {
					return true
				}
			}
			return false
		}, 2*time.Minute, 10*time.Second).Should(o.BeTrue(), fmt.Sprintf("%s MCP should start updating", testMCPName))

		// Wait for MCP to apply configuration
		g.By("Waiting for MCP to update with new configuration")
		err = waitForMCP(ctx, mcClient, testMCPName, 15*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred(), "MCP should update successfully")

		// Verify system compressible is disabled
		config, err := getKubeletConfigFromNode(ctx, oc, nodeName)
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to read kubelet config")
		o.Expect(isSystemCompressibleEnabled(config)).To(o.BeFalse(),
			"System compressible should be disabled")

		// Check cgroup cpu.weight configuration for system.slice
		g.By("Verifying system.slice cgroup CPU weight when system compressible is disabled")
		actualWeight, err := readCgroupCPUWeight(oc, nodeName, "system.slice")
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to read cpu.weight for system.slice")
		framework.Logf("system.slice actual cpu.weight when disabled: %d", actualWeight)

		// When system compressible is disabled, system.slice should have the default cgroup v2 weight (100)
		defaultWeight := uint64(100)
		framework.Logf("Expected default cpu.weight: %d", defaultWeight)

		o.Expect(actualWeight).To(o.Equal(defaultWeight),
			"system.slice cpu.weight should be %d (default cgroup v2 weight) when system compressible is disabled",
			defaultWeight)

		framework.Logf("System compressible override verified successfully: cpu.weight is default value")

		// Cleanup explicitly before DeferCleanup
		cleanupKubeletConfig()
		cleanupNodeLabel()
		cleanupMCP()
	})

	g.It("should not enable system compressible when reserved CPU is configured", func(ctx context.Context) {
		mcClient, err := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred(), "Error creating MCO client")

		testMCPName := "reserved-cpu-test"
		testNodeMCPLabel := fmt.Sprintf("node-role.kubernetes.io/%s", testMCPName)
		kubeletConfigName := "reserved-cpu-config"

		// Select node
		nodeName, cpuCount, err := selectTestNode(ctx, oc, 4)
		o.Expect(err).NotTo(o.HaveOccurred(), "Should find a node with at least 4 CPUs")
		framework.Logf("Testing on node: %s with %d CPUs", nodeName, cpuCount)

		// Setup cleanup functions
		cleanupNodeLabel := func() {
			g.By(fmt.Sprintf("Removing node label %s from node %s", testNodeMCPLabel, nodeName))
			cleanupCtx := context.Background()
			patchData := []byte(fmt.Sprintf(`{"metadata":{"labels":{%q:null}}}`, testNodeMCPLabel))
			_, updateErr := oc.AdminKubeClient().CoreV1().Nodes().Patch(cleanupCtx, nodeName, types.MergePatchType, patchData, metav1.PatchOptions{})
			if apierrors.IsNotFound(updateErr) {
				// Node already deleted, nothing to clean up
				return
			} else if updateErr != nil {
				framework.Failf("Failed to remove label from node %s: %v", nodeName, updateErr)
			}

			g.By(fmt.Sprintf("Waiting for node %s to transition back to worker pool", nodeName))
			o.Eventually(func() bool {
				currentNode, err := oc.AdminKubeClient().CoreV1().Nodes().Get(cleanupCtx, nodeName, metav1.GetOptions{})
				if err != nil {
					return false
				}
				currentConfig := currentNode.Annotations["machineconfiguration.openshift.io/currentConfig"]
				desiredConfig := currentNode.Annotations["machineconfiguration.openshift.io/desiredConfig"]
				isWorkerConfig := currentConfig != "" && !strings.Contains(currentConfig, testMCPName) && currentConfig == desiredConfig
				return isWorkerConfig
			}, 7*time.Minute, 10*time.Second).Should(o.BeTrue())
		}

		cleanupKubeletConfig := func() {
			g.By("Cleaning up KubeletConfig")
			cleanupCtx := context.Background()
			deleteErr := mcClient.MachineconfigurationV1().KubeletConfigs().Delete(cleanupCtx, kubeletConfigName, metav1.DeleteOptions{})
			if apierrors.IsNotFound(deleteErr) {
				// KubeletConfig already deleted, nothing to clean up
			} else if deleteErr != nil {
				framework.Failf("Failed to delete KubeletConfig %s: %v", kubeletConfigName, deleteErr)
			}
		}

		cleanupMCP := func() {
			g.By("Cleaning up custom MachineConfigPool")
			cleanupCtx := context.Background()
			deleteErr := mcClient.MachineconfigurationV1().MachineConfigPools().Delete(cleanupCtx, testMCPName, metav1.DeleteOptions{})
			if apierrors.IsNotFound(deleteErr) {
				// MachineConfigPool already deleted, nothing to clean up
			} else if deleteErr != nil {
				framework.Failf("Failed to delete MachineConfigPool %s: %v", testMCPName, deleteErr)
			}

			// Wait for worker MCP to stabilize after custom MCP deletion
			g.By("Waiting for worker MCP to stabilize after custom MCP deletion")
			waitErr := waitForMCP(cleanupCtx, mcClient, "worker", 10*time.Minute)
			if apierrors.IsNotFound(waitErr) {
				// MachineConfigPool already deleted, nothing to wait for
			} else if waitErr != nil {
				framework.Failf("Worker MCP did not stabilize after custom MCP deletion: %v", waitErr)
			}
		}

		// Register cleanups in LIFO order
		g.DeferCleanup(cleanupMCP)
		g.DeferCleanup(cleanupKubeletConfig)
		g.DeferCleanup(cleanupNodeLabel)

		// Label node
		g.By(fmt.Sprintf("Labeling node %s with %s", nodeName, testNodeMCPLabel))
		patchData := []byte(fmt.Sprintf(`{"metadata":{"labels":{%q:""}}}`, testNodeMCPLabel))
		_, err = oc.AdminKubeClient().CoreV1().Nodes().Patch(ctx, nodeName, types.MergePatchType, patchData, metav1.PatchOptions{})
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
		o.Expect(err).NotTo(o.HaveOccurred(), "Should create custom MachineConfigPool")

		// Wait for MCP ready
		g.By("Waiting for custom MachineConfigPool to be ready")
		err = waitForMCP(ctx, mcClient, testMCPName, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred(), "MCP should be ready")

		// Configure static CPU manager with reserved CPUs
		g.By("Creating KubeletConfig with reserved CPU")
		kubeletConfig := &mcfgv1.KubeletConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "machineconfiguration.openshift.io/v1",
				Kind:       "KubeletConfig",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: kubeletConfigName,
			},
			Spec: mcfgv1.KubeletConfigSpec{
				KubeletConfig: &runtime.RawExtension{
					Raw: []byte(`{"cpuManagerPolicy":"static","reservedSystemCPUs":"0-1"}`),
				},
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"machineconfiguration.openshift.io/pool": testMCPName,
					},
				},
			},
		}

		_, err = mcClient.MachineconfigurationV1().KubeletConfigs().Create(ctx, kubeletConfig, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Should create KubeletConfig")

		// Wait for MCP to start updating
		g.By(fmt.Sprintf("Waiting for %s MCP to start updating", testMCPName))
		o.Eventually(func() bool {
			mcp, err := mcClient.MachineconfigurationV1().MachineConfigPools().Get(ctx, testMCPName, metav1.GetOptions{})
			if err != nil {
				framework.Logf("Error getting %s MCP: %v", testMCPName, err)
				return false
			}
			for _, condition := range mcp.Status.Conditions {
				if condition.Type == "Updating" && condition.Status == corev1.ConditionTrue {
					return true
				}
			}
			return false
		}, 2*time.Minute, 10*time.Second).Should(o.BeTrue(), fmt.Sprintf("%s MCP should start updating", testMCPName))

		// Wait for configuration
		g.By("Waiting for MCP to update with reserved CPU configuration")
		err = waitForMCP(ctx, mcClient, testMCPName, 15*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred(), "MCP should update successfully")

		// Verify reserved CPU is enabled
		config, err := getKubeletConfigFromNode(ctx, oc, nodeName)
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to read kubelet config")
		o.Expect(isReservedCPUEnabled(config)).To(o.BeTrue(),
			"Reserved CPU should be enabled")

		// Verify system compressible is NOT enabled
		o.Expect(isSystemCompressibleEnabled(config)).To(o.BeFalse(),
			"System compressible should not be enabled when reserved CPU is configured")

		framework.Logf("Reserved CPU takes precedence over system compressible")

		// Cleanup explicitly before DeferCleanup
		cleanupKubeletConfig()
		cleanupNodeLabel()
		cleanupMCP()
	})
})

// Helper Functions

// getCPUWeight converts cpuShares to CPU weight
// This matches the kernel's conversion formula
func getCPUWeight(cpuShares *uint64) uint64 {
	if cpuShares == nil {
		return 0
	}
	if *cpuShares >= 262144 {
		return 10000
	}
	return 1 + ((*cpuShares-2)*9999)/262142
}

// isSystemCompressibleEnabled checks if system-reserved-compressible is in EnforceNodeAllocatable
func isSystemCompressibleEnabled(config *kubeletconfigv1beta1.KubeletConfiguration) bool {
	if config.EnforceNodeAllocatable == nil {
		return false
	}

	for _, val := range config.EnforceNodeAllocatable {
		if val == "system-reserved-compressible" {
			return true
		}
	}
	return false
}

// isReservedCPUEnabled checks if reserved CPU feature is enabled
func isReservedCPUEnabled(config *kubeletconfigv1beta1.KubeletConfiguration) bool {
	// Check for static CPU manager policy
	if config.CPUManagerPolicy == "static" {
		return true
	}

	// Check for reserved system CPUs
	if config.ReservedSystemCPUs != "" {
		return true
	}

	return false
}

// selectTestNode selects a worker node with at least minCPUs CPU cores
// Returns node name and actual CPU count
func selectTestNode(ctx context.Context, oc *exutil.CLI, minCPUs int) (string, int, error) {
	nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: "node-role.kubernetes.io/worker",
	})
	if err != nil {
		return "", 0, err
	}

	for _, node := range nodes.Items {
		// Skip unschedulable nodes
		if node.Spec.Unschedulable {
			framework.Logf("Skipping node %s: unschedulable", node.Name)
			continue
		}

		// Check if node is Ready
		isReady := false
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
				isReady = true
				break
			}
		}
		if !isReady {
			framework.Logf("Skipping node %s: not Ready", node.Name)
			continue
		}

		// Check MCP stability - current config should match desired config
		currentConfig := node.Annotations["machineconfiguration.openshift.io/currentConfig"]
		desiredConfig := node.Annotations["machineconfiguration.openshift.io/desiredConfig"]
		if currentConfig == "" || desiredConfig == "" || currentConfig != desiredConfig {
			framework.Logf("Skipping node %s: MCP not stable (current=%s, desired=%s)", node.Name, currentConfig, desiredConfig)
			continue
		}

		// Check CPU count
		cpuQuantity := node.Status.Capacity[corev1.ResourceCPU]
		cpuCount := int(cpuQuantity.Value())
		if cpuCount >= minCPUs {
			framework.Logf("Selected node %s with %d CPUs (capacity: %s)", node.Name, cpuCount, cpuQuantity.String())
			return node.Name, cpuCount, nil
		}
	}
	return "", 0, fmt.Errorf("no suitable worker node found with at least %d CPUs (schedulable, ready, MCP stable)", minCPUs)
}

// readCgroupCPUWeight reads cpu.weight file for a cgroup slice
func readCgroupCPUWeight(oc *exutil.CLI, nodeName, slicePath string) (uint64, error) {
	weightPath := fmt.Sprintf("/sys/fs/cgroup/%s/cpu.weight", slicePath)

	output, err := ExecOnNodeWithChroot(oc, nodeName, "cat", weightPath)
	if err != nil {
		return 0, fmt.Errorf("failed to read %s: %w", weightPath, err)
	}

	weight, err := strconv.ParseUint(strings.TrimSpace(output), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse cpu.weight: %w", err)
	}

	return weight, nil
}

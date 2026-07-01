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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubeletconfigv1beta1 "k8s.io/kubelet/config/v1beta1"
	"k8s.io/kubernetes/test/e2e/framework"
	"sigs.k8s.io/yaml"

	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	machineconfigclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Suite:openshift/disruptive-longrunning][sig-node][Disruptive] System Compressible CPU", g.Serial, SkipOnMicroShift, func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("system-compressible")

	g.BeforeEach(func(ctx context.Context) {
		EnsureNodesReady(ctx, oc)
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

		g.By("Reading systemReserved.cpu from /etc/openshift/kubelet.conf.d/20-auto-sizing.conf")
		autoSizingOutput, err := ExecOnNodeWithChroot(ctx, oc, nodeName, "cat", "/etc/openshift/kubelet.conf.d/20-auto-sizing.conf")
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to read /etc/openshift/kubelet.conf.d/20-auto-sizing.conf")
		framework.Logf("/etc/openshift/kubelet.conf.d/20-auto-sizing.conf contents:\n%s", autoSizingOutput)

		var autoSizingConfig kubeletconfigv1beta1.KubeletConfiguration
		err = yaml.Unmarshal([]byte(autoSizingOutput), &autoSizingConfig)
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to parse auto-sizing config")

		cpuQuantity, ok := autoSizingConfig.SystemReserved["cpu"]
		o.Expect(ok).To(o.BeTrue(), "systemReserved.cpu should be set")
		cpuResource, err := resource.ParseQuantity(cpuQuantity)
		o.Expect(err).NotTo(o.HaveOccurred(), "systemReserved.cpu must be a valid resource quantity")
		systemReservedCPU := float64(cpuResource.MilliValue()) / 1000.0
		o.Expect(systemReservedCPU).To(o.BeNumerically(">", 0), "systemReserved.cpu should be greater than 0")
		framework.Logf("systemReserved.cpu: %.2f (%.0f millicores)", systemReservedCPU, systemReservedCPU*1000)

		// Convert to cpuShares: cpuShares = systemReservedCPU * 1024
		cpuShares := uint64(systemReservedCPU * 1024)
		expectedWeight := getCPUWeight(&cpuShares)
		framework.Logf("Expected cpu.weight for cpuShares=%d: %d", cpuShares, expectedWeight)

		// Check cgroup cpu.weight configuration for system.slice
		g.By("Verifying system.slice cgroup CPU weight")
		actualWeight, err := readCgroupCPUWeight(ctx, oc, nodeName, "system.slice")
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
		kubeletConfigName := "system-compressible-override"

		// Select node
		nodeName, cpuCount, err := selectTestNode(ctx, oc, 4)
		o.Expect(err).NotTo(o.HaveOccurred(), "Should find a node with at least 4 CPUs")
		framework.Logf("Testing on node: %s with %d CPUs", nodeName, cpuCount)

		// Create custom MCP for the node
		mcpConfig, err := CreateCustomMCPForNode(ctx, oc, mcClient, testMCPName, nodeName)
		o.Expect(err).NotTo(o.HaveOccurred(), "Should create custom MCP")

		// Register cleanups in LIFO order
		g.DeferCleanup(func() {
			cleanupCtx := context.Background()
			if err := CleanupCustomMCP(cleanupCtx, mcpConfig); err != nil {
				framework.Logf("Warning: MCP cleanup had errors: %v", err)
			}
		})
		g.DeferCleanup(func() {
			cleanupCtx := context.Background()
			if err := CleanupKubeletConfig(cleanupCtx, mcClient, kubeletConfigName, ""); err != nil {
				framework.Logf("Warning: KubeletConfig cleanup failed: %v", err)
			}
		})

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

		g.By("Applying KubeletConfig and waiting for MCP rollout")
		err = ApplyKubeletConfigAndWaitForMCP(ctx, mcClient, kubeletConfig, testMCPName, 15*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred(), "Should apply KubeletConfig and complete MCP rollout")

		// Verify system compressible is disabled
		config, err := getKubeletConfigFromNode(ctx, oc, nodeName)
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to read kubelet config")
		o.Expect(isSystemCompressibleEnabled(config)).To(o.BeFalse(),
			"System compressible should be disabled")

		// Check cgroup cpu.weight configuration for system.slice
		g.By("Verifying system.slice cgroup CPU weight when system compressible is disabled")
		actualWeight, err := readCgroupCPUWeight(ctx, oc, nodeName, "system.slice")
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
		CleanupKubeletConfig(ctx, mcClient, kubeletConfigName, "")
		CleanupCustomMCP(ctx, mcpConfig)
	})

	g.It("should not enable system compressible when reserved CPU is configured", func(ctx context.Context) {
		mcClient, err := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred(), "Error creating MCO client")

		testMCPName := "reserved-cpu-test"
		kubeletConfigName := "reserved-cpu-config"

		// Select node
		nodeName, cpuCount, err := selectTestNode(ctx, oc, 4)
		o.Expect(err).NotTo(o.HaveOccurred(), "Should find a node with at least 4 CPUs")
		framework.Logf("Testing on node: %s with %d CPUs", nodeName, cpuCount)

		// Create custom MCP for the node
		mcpConfig, err := CreateCustomMCPForNode(ctx, oc, mcClient, testMCPName, nodeName)
		o.Expect(err).NotTo(o.HaveOccurred(), "Should create custom MCP")

		// Register cleanups in LIFO order
		g.DeferCleanup(func() {
			cleanupCtx := context.Background()
			if err := CleanupCustomMCP(cleanupCtx, mcpConfig); err != nil {
				framework.Logf("Warning: MCP cleanup had errors: %v", err)
			}
		})
		g.DeferCleanup(func() {
			cleanupCtx := context.Background()
			if err := CleanupKubeletConfig(cleanupCtx, mcClient, kubeletConfigName, ""); err != nil {
				framework.Logf("Warning: KubeletConfig cleanup failed: %v", err)
			}
		})

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

		g.By("Applying KubeletConfig and waiting for MCP rollout")
		err = ApplyKubeletConfigAndWaitForMCP(ctx, mcClient, kubeletConfig, testMCPName, 15*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred(), "Should apply KubeletConfig and complete MCP rollout")

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
		CleanupKubeletConfig(ctx, mcClient, kubeletConfigName, "")
		CleanupCustomMCP(ctx, mcpConfig)
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
func readCgroupCPUWeight(ctx context.Context, oc *exutil.CLI, nodeName, slicePath string) (uint64, error) {
	weightPath := fmt.Sprintf("/sys/fs/cgroup/%s/cpu.weight", slicePath)

	output, err := ExecOnNodeWithChroot(ctx, oc, nodeName, "cat", weightPath)
	if err != nil {
		return 0, fmt.Errorf("failed to read %s: %w", weightPath, err)
	}

	weight, err := strconv.ParseUint(strings.TrimSpace(output), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse cpu.weight: %w", err)
	}

	return weight, nil
}

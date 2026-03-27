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

var _ = g.Describe("[Suite:openshift/disruptive-longrunning][sig-node][Disruptive] System Compressible CPU", func() {
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

		// Calculate expected CPU limit based on actual CPU count
		// See https://docs.google.com/spreadsheets/d/1FbSBnG4NGk9te3xiWZ2D9BF9DG_F_GPVp5mTVE6PsDg/edit?usp=sharing
		expectedLimit := getSystemSliceCPULimit(cpuCount)
		framework.Logf("Expected system.slice CPU limit for %d CPUs: %.2f millicores", cpuCount, expectedLimit)

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

		// Check cgroup cpu.weight configuration for system.slice
		g.By("Verifying system.slice cgroup CPU configuration")
		cpuWeight, err := readCgroupCPUWeight(oc, nodeName, "system.slice")
		if err == nil {
			framework.Logf("system.slice cpu.weight: %d", cpuWeight)
		} else {
			framework.Logf("Could not read cpu.weight: %v", err)
		}

		// Scale load processes based on CPU count
		// Use all CPUs for kubepods.slice to create contention
		kubepodsProcesses := cpuCount
		// Use 75% of CPUs for system.slice to attempt exceeding the limit
		systemProcesses := (cpuCount * 3) / 4
		if systemProcesses < 3 {
			systemProcesses = 3 // Minimum 3 processes
		}

		// Create CPU load: Start kubepods.slice FIRST to establish contention
		// This prevents system.slice from spiking before limits are enforced
		g.By(fmt.Sprintf("Creating CPU load in kubepods.slice first (%d processes)", kubepodsProcesses))
		kubepodsUnits, err := createCPULoadInSlice(oc, nodeName, "kubepods.slice", kubepodsProcesses)
		o.Expect(err).NotTo(o.HaveOccurred(), "Should create CPU load in kubepods.slice")
		defer stopCPULoad(oc, nodeName, kubepodsUnits)

		// Wait for kubepods.slice to fully consume CPU
		framework.Logf("Waiting 10 seconds for kubepods.slice to establish CPU contention")
		time.Sleep(10 * time.Second)

		g.By(fmt.Sprintf("Creating CPU load in system.slice (%d processes)", systemProcesses))
		systemUnits, err := createCPULoadInSlice(oc, nodeName, "system.slice", systemProcesses)
		o.Expect(err).NotTo(o.HaveOccurred(), "Should create CPU load in system.slice")
		defer stopCPULoad(oc, nodeName, systemUnits)

		// Monitor system.slice CPU usage for 60 seconds
		g.By("Monitoring system.slice CPU usage")
		samples, err := monitorSliceCPUUsage(ctx, oc, nodeName, "system.slice",
			60*time.Second, 2*time.Second)
		o.Expect(err).NotTo(o.HaveOccurred(), "Should collect CPU usage samples")

		// Verify CPU stays within expected limit (allow 3 samples to exceed for transient spikes)
		err = verifyCPULimit(samples, expectedLimit, 3)
		o.Expect(err).NotTo(o.HaveOccurred(),
			fmt.Sprintf("System.slice CPU should be limited to ~%.2fm", expectedLimit))

		framework.Logf("System compressible CPU limit enforced successfully")
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
			if updateErr != nil && !apierrors.IsNotFound(updateErr) {
				framework.Logf("Failed to remove label from node %s: %v", nodeName, updateErr)
				return
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
			if deleteErr != nil && !apierrors.IsNotFound(deleteErr) {
				framework.Logf("Failed to delete KubeletConfig %s: %v", kubeletConfigName, deleteErr)
			}
		}

		cleanupMCP := func() {
			g.By("Cleaning up custom MachineConfigPool")
			cleanupCtx := context.Background()
			deleteErr := mcClient.MachineconfigurationV1().MachineConfigPools().Delete(cleanupCtx, testMCPName, metav1.DeleteOptions{})
			if deleteErr != nil && !apierrors.IsNotFound(deleteErr) {
				framework.Logf("Failed to delete MachineConfigPool %s: %v", testMCPName, deleteErr)
				return
			}

			// Wait for worker MCP to stabilize after custom MCP deletion
			g.By("Waiting for worker MCP to stabilize after custom MCP deletion")
			waitErr := waitForMCP(cleanupCtx, mcClient, "worker", 10*time.Minute)
			if waitErr != nil && !apierrors.IsNotFound(waitErr) {
				framework.Logf("Warning: worker MCP did not stabilize after custom MCP deletion: %v", waitErr)
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

		// Scale load processes based on CPU count
		kubepodsProcesses := cpuCount
		systemProcesses := (cpuCount * 3) / 4
		if systemProcesses < 3 {
			systemProcesses = 3
		}

		// Expected minimum CPU usage when limit is disabled
		// Should be able to use at least 2x the normal limit
		expectedLimit := getSystemSliceCPULimit(cpuCount)
		minimumExpectedCPU := expectedLimit * 1.5

		// Create CPU load and verify NO limit is enforced
		// Start kubepods.slice first to establish contention
		g.By(fmt.Sprintf("Creating CPU load in kubepods.slice first (%d processes)", kubepodsProcesses))
		kubepodsUnits, err := createCPULoadInSlice(oc, nodeName, "kubepods.slice", kubepodsProcesses)
		o.Expect(err).NotTo(o.HaveOccurred(), "Should create CPU load in kubepods.slice")
		defer stopCPULoad(oc, nodeName, kubepodsUnits)

		// Wait for kubepods.slice to fully consume CPU
		framework.Logf("Waiting 10 seconds for kubepods.slice to establish CPU contention")
		time.Sleep(10 * time.Second)

		g.By(fmt.Sprintf("Creating CPU load in system.slice (%d processes)", systemProcesses))
		systemUnits, err := createCPULoadInSlice(oc, nodeName, "system.slice", systemProcesses)
		o.Expect(err).NotTo(o.HaveOccurred(), "Should create CPU load in system.slice")
		defer stopCPULoad(oc, nodeName, systemUnits)

		// Monitor and verify system.slice CAN exceed the normal limit
		g.By("Monitoring system.slice CPU usage")
		samples, err := monitorSliceCPUUsage(ctx, oc, nodeName, "system.slice",
			60*time.Second, 2*time.Second)
		o.Expect(err).NotTo(o.HaveOccurred(), "Should collect CPU usage samples")

		// Find max sample
		maxCPU := 0.0
		for _, sample := range samples {
			if sample > maxCPU {
				maxCPU = sample
			}
		}

		framework.Logf("Max system.slice CPU usage: %.2f millicores (expected limit when enabled: %.2f millicores)", maxCPU, expectedLimit)
		o.Expect(maxCPU).To(o.BeNumerically(">", minimumExpectedCPU),
			fmt.Sprintf("System.slice should be able to use more than %.2fm when limit is disabled", minimumExpectedCPU))

		framework.Logf("System compressible override verified successfully")

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
			if updateErr != nil && !apierrors.IsNotFound(updateErr) {
				framework.Logf("Failed to remove label from node %s: %v", nodeName, updateErr)
				return
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
			if deleteErr != nil && !apierrors.IsNotFound(deleteErr) {
				framework.Logf("Failed to delete KubeletConfig %s: %v", kubeletConfigName, deleteErr)
			}
		}

		cleanupMCP := func() {
			g.By("Cleaning up custom MachineConfigPool")
			cleanupCtx := context.Background()
			deleteErr := mcClient.MachineconfigurationV1().MachineConfigPools().Delete(cleanupCtx, testMCPName, metav1.DeleteOptions{})
			if deleteErr != nil && !apierrors.IsNotFound(deleteErr) {
				framework.Logf("Failed to delete MachineConfigPool %s: %v", testMCPName, deleteErr)
				return
			}

			// Wait for worker MCP to stabilize after custom MCP deletion
			g.By("Waiting for worker MCP to stabilize after custom MCP deletion")
			waitErr := waitForMCP(cleanupCtx, mcClient, "worker", 10*time.Minute)
			if waitErr != nil && !apierrors.IsNotFound(waitErr) {
				framework.Logf("Warning: worker MCP did not stabilize after custom MCP deletion: %v", waitErr)
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

// cpuUsageSample represents a CPU usage measurement
type cpuUsageSample struct {
	timestamp time.Time
	usageUsec uint64
}

// getSystemSliceCPULimit calculates the expected CPU limit for system.slice based on CPU count.
// The limit is calculated based on cgroup weights as documented in:
// https://docs.google.com/spreadsheets/d/1FbSBnG4NGk9te3xiWZ2D9BF9DG_F_GPVp5mTVE6PsDg/edit?usp=sharing
//
// CPU cores to limit mapping (millicores):
// 4: 517.53, 6: 519.6, 8: 520.6, 16: 522.1, 32: 522.9,
// 64: 843, 96: 1223, 128: 1603.1, 224: 2763.1
//
// The test will fail if CPU count exceeds 224 (maximum tested value).
func getSystemSliceCPULimit(cpuCount int) float64 {
	// Lookup table based on actual calculations
	limits := map[int]float64{
		4:   517.53,
		6:   519.6,
		8:   520.6,
		16:  522.1,
		32:  522.9,
		64:  843.0,
		96:  1223.0,
		128: 1603.1,
		224: 2763.1,
	}

	// Fail if CPU count exceeds maximum tested value
	if cpuCount > 224 {
		framework.Failf("CPU count %d exceeds maximum tested value of 224. Please update getSystemSliceCPULimit with new limits.", cpuCount)
	}

	// If below minimum, use 4-CPU value
	if cpuCount < 4 {
		return 517.53
	}

	// Return exact value if available
	if limit, found := limits[cpuCount]; found {
		return limit
	}

	// For values between known points, use linear interpolation
	sortedKeys := []int{4, 6, 8, 16, 32, 64, 96, 128, 224}

	// Find the two closest points for interpolation
	for i := 0; i < len(sortedKeys)-1; i++ {
		lower := sortedKeys[i]
		upper := sortedKeys[i+1]

		if cpuCount >= lower && cpuCount <= upper {
			// Linear interpolation
			lowerLimit := limits[lower]
			upperLimit := limits[upper]
			ratio := float64(cpuCount-lower) / float64(upper-lower)
			return lowerLimit + ratio*(upperLimit-lowerLimit)
		}
	}

	// Should never reach here due to the checks above
	framework.Failf("Unexpected CPU count %d in getSystemSliceCPULimit", cpuCount)
	return 0
}

// getNodeCPUCount returns the number of CPUs on a node
func getNodeCPUCount(ctx context.Context, oc *exutil.CLI, nodeName string) (int, error) {
	node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return 0, err
	}
	cpuQuantity := node.Status.Capacity[corev1.ResourceCPU]
	return int(cpuQuantity.Value()), nil
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
		cpuQuantity := node.Status.Capacity[corev1.ResourceCPU]
		cpuCount := int(cpuQuantity.Value())
		if cpuCount >= minCPUs {
			framework.Logf("Selected node %s with %d CPUs (capacity: %s)", node.Name, cpuCount, cpuQuantity.String())
			return node.Name, cpuCount, nil
		}
	}
	return "", 0, fmt.Errorf("no worker node found with at least %d CPUs", minCPUs)
}

// createCPULoadInSlice creates numProcesses CPU load processes in the specified cgroup slice
func createCPULoadInSlice(oc *exutil.CLI, nodeName, sliceName string, numProcesses int) ([]string, error) {
	unitNames := make([]string, 0, numProcesses)

	for i := 0; i < numProcesses; i++ {
		unitName := fmt.Sprintf("cpu-load-%s-%d", strings.ReplaceAll(sliceName, ".", "-"), i)

		cmd := fmt.Sprintf(
			"systemd-run --unit=%s --slice=%s bash -c 'while true; do :; done'",
			unitName, sliceName,
		)

		output, err := ExecOnNodeWithChroot(oc, nodeName, "bash", "-c", cmd)
		if err != nil {
			framework.Logf("Failed to create CPU load unit %s: %v, output: %s", unitName, err, output)
			stopCPULoad(oc, nodeName, unitNames)
			return nil, err
		}

		framework.Logf("Created CPU load unit: %s in slice %s", unitName, sliceName)
		unitNames = append(unitNames, unitName)
	}

	time.Sleep(5 * time.Second)

	return unitNames, nil
}

// stopCPULoad stops all CPU load systemd units
func stopCPULoad(oc *exutil.CLI, nodeName string, unitNames []string) error {
	for _, unitName := range unitNames {
		cmd := fmt.Sprintf("systemctl stop %s", unitName)
		output, err := ExecOnNodeWithChroot(oc, nodeName, "bash", "-c", cmd)
		if err != nil {
			framework.Logf("Warning: failed to stop unit %s: %v, output: %s", unitName, err, output)
		} else {
			framework.Logf("Stopped CPU load unit: %s", unitName)
		}
	}
	return nil
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

// readCgroupCPUStat reads cpu.stat file and extracts usage_usec
func readCgroupCPUStat(oc *exutil.CLI, nodeName, slicePath string) (uint64, error) {
	statPath := fmt.Sprintf("/sys/fs/cgroup/%s/cpu.stat", slicePath)

	output, err := ExecOnNodeWithChroot(oc, nodeName, "cat", statPath)
	if err != nil {
		return 0, fmt.Errorf("failed to read %s: %w", statPath, err)
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "usage_usec" {
			usageUsec, err := strconv.ParseUint(fields[1], 10, 64)
			if err != nil {
				return 0, fmt.Errorf("failed to parse usage_usec: %w", err)
			}
			return usageUsec, nil
		}
	}

	return 0, fmt.Errorf("usage_usec not found in cpu.stat")
}

// calculateMillicores calculates CPU usage in millicores from two samples
func calculateMillicores(sample1, sample2 cpuUsageSample) float64 {
	usecDelta := float64(sample2.usageUsec - sample1.usageUsec)
	timeDelta := sample2.timestamp.Sub(sample1.timestamp).Microseconds()

	if timeDelta == 0 {
		return 0
	}

	return (usecDelta / float64(timeDelta)) * 1000.0
}

// monitorSliceCPUUsage monitors CPU usage of a cgroup slice for the specified duration
func monitorSliceCPUUsage(ctx context.Context, oc *exutil.CLI, nodeName, sliceName string, duration time.Duration, sampleInterval time.Duration) ([]float64, error) {
	samples := make([]float64, 0)

	prevUsage, err := readCgroupCPUStat(oc, nodeName, sliceName)
	if err != nil {
		return nil, err
	}
	prevSample := cpuUsageSample{
		timestamp: time.Now(),
		usageUsec: prevUsage,
	}

	ticker := time.NewTicker(sampleInterval)
	defer ticker.Stop()

	deadline := time.Now().Add(duration)

	for time.Now().Before(deadline) {
		select {
		case <-ticker.C:
			currUsage, err := readCgroupCPUStat(oc, nodeName, sliceName)
			if err != nil {
				framework.Logf("Warning: failed to read CPU usage: %v", err)
				continue
			}

			currSample := cpuUsageSample{
				timestamp: time.Now(),
				usageUsec: currUsage,
			}

			millicores := calculateMillicores(prevSample, currSample)
			samples = append(samples, millicores)
			framework.Logf("CPU usage for %s: %.2f millicores", sliceName, millicores)

			prevSample = currSample

		case <-ctx.Done():
			return samples, ctx.Err()
		}
	}

	return samples, nil
}

// verifyCPULimit checks that all CPU usage samples are within the expected limit
func verifyCPULimit(samples []float64, limitMillicores float64, allowExceedCount int) error {
	if len(samples) == 0 {
		return fmt.Errorf("no CPU samples collected")
	}

	exceedCount := 0
	for i, sample := range samples {
		if sample > limitMillicores {
			exceedCount++
			framework.Logf("Sample %d exceeded limit: %.2f > %.2f millicores", i, sample, limitMillicores)
		}
	}

	if exceedCount > allowExceedCount {
		return fmt.Errorf("CPU limit exceeded in %d/%d samples (allowed: %d)",
			exceedCount, len(samples), allowExceedCount)
	}

	return nil
}

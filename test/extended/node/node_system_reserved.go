package node

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"
	"k8s.io/utils/ptr"

	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	machineconfigclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

var _ = g.Describe("[Suite:openshift/conformance/serial][Serial][sig-node] System reserved", func() {
	defer g.GinkgoRecover()

	f := framework.NewDefaultFramework("system-reserved")
	f.NamespacePodSecurityLevel = admissionapi.LevelPrivileged

	oc := exutil.NewCLI("system-reserved")

	g.It("should enable system-reserved-compressible and NODE_SIZING_ENABLED when KubeletConfig with system-reserved-compressible is applied", func(ctx context.Context) {
		// Skip on MicroShift since it doesn't have the Machine Config Operator
		isMicroshift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroshift {
			g.Skip("Not supported on MicroShift")
		}

		mcClient, err := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred(), "Error creating machine config client")

		// First, verify the default state (NODE_SIZING_ENABLED=false)
		g.By("Getting a worker node to test")
		nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{
			LabelSelector: "node-role.kubernetes.io/worker",
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to list worker nodes")
		o.Expect(len(nodes.Items)).To(o.BeNumerically(">", 0), "Should have at least one worker node")

		nodeName := nodes.Items[0].Name
		framework.Logf("Testing on node: %s", nodeName)

		// Label the first worker node for our custom MCP
		// This approach is taken so that all the nodes do not restart at the same time for the test
		testMCPLabel := "system-reserved-test"
		g.By(fmt.Sprintf("Labeling node %s with %s=true", nodeName, testMCPLabel))
		node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to get node")

		if node.Labels == nil {
			node.Labels = make(map[string]string)
		}
		node.Labels[testMCPLabel] = "true"
		_, err = oc.AdminKubeClient().CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to label node")

		// Clean up node label on test completion (only if test fails before explicit cleanup)
		defer func() {
			g.By("Cleaning up node label (defer)")
			node, getErr := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
			if getErr != nil {
				framework.Logf("Failed to get node for cleanup: %v", getErr)
				return
			}
			// Only remove the label if it still exists (i.e., test failed before explicit cleanup)
			if _, exists := node.Labels[testMCPLabel]; exists {
				framework.Logf("Node label still exists, removing it")
				delete(node.Labels, testMCPLabel)
				_, updateErr := oc.AdminKubeClient().CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
				if updateErr != nil {
					framework.Logf("Failed to remove label from node %s: %v", nodeName, updateErr)
				}
			}
		}()

		// Create custom MachineConfigPool
		testMCPName := "system-reserved-test"
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
						testMCPLabel: "true",
					},
				},
			},
		}

		_, err = mcClient.MachineconfigurationV1().MachineConfigPools().Create(ctx, testMCP, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to create custom MachineConfigPool")

		// Clean up MachineConfigPool on test completion
		defer func() {
			g.By("Cleaning up custom MachineConfigPool")
			deleteErr := mcClient.MachineconfigurationV1().MachineConfigPools().Delete(ctx, testMCPName, metav1.DeleteOptions{})
			if deleteErr != nil {
				framework.Logf("Failed to delete MachineConfigPool %s: %v", testMCPName, deleteErr)
			}
		}()

		g.By("Waiting for custom MachineConfigPool to be ready")
		err = waitForMCPToBeReady(ctx, mcClient, testMCPName, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred(), "Custom MachineConfigPool should become ready")

		namespace := oc.Namespace()

		g.By("Setting privileged pod security labels on namespace")
		err = oc.AsAdmin().Run("label").Args("namespace", namespace, "pod-security.kubernetes.io/enforce=privileged", "pod-security.kubernetes.io/audit=privileged", "--overwrite").Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to label namespace with privileged pod security")

		g.By("Creating a privileged pod with /etc and /sys/fs/cgroup mounted to verify default state")
		podName := "system-reserved-test-before"
		pod := createPrivilegedPodWithHostPaths(podName, namespace, nodeName)

		_, err = oc.AdminKubeClient().CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to create privileged pod")

		g.By("Waiting for pod to be running")
		waitForPodRunning(ctx, oc, podName, namespace)

		verifyNodeSizingEnabledFile(ctx, oc, podName, namespace, nodeName, "false")

		verifyCpuWeightNotSet(ctx, oc, podName, namespace, nodeName)

		g.By("Deleting the test pod before applying KubeletConfig")
		err = oc.AdminKubeClient().CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to delete test pod")

		// Now apply KubeletConfig and verify system-reserved-compressible is enabled
		kubeletConfigName := "system-compressible-enabled"

		// Clean up KubeletConfig on test completion
		defer func() {
			g.By("Cleaning up KubeletConfig")
			deleteErr := mcClient.MachineconfigurationV1().KubeletConfigs().Delete(ctx, kubeletConfigName, metav1.DeleteOptions{})
			if deleteErr != nil {
				framework.Logf("Failed to delete KubeletConfig %s: %v", kubeletConfigName, deleteErr)
			}

			// Wait for custom MCP to be ready after cleanup
			g.By("Waiting for custom MCP to be ready after cleanup")
			waitErr := waitForMCPToBeReady(ctx, mcClient, testMCPName, 10*time.Minute)
			if waitErr != nil {
				framework.Logf("Failed to wait for custom MCP to be ready: %v", waitErr)
			}
		}()

		g.By("Creating KubeletConfig with system-reserved-compressible enabled")
		autoSizingReserved := true

		// Create the kubelet configuration as a map
		kubeletConfigData := map[string]interface{}{
			"systemReservedCgroup": "/system.slice",
			"enforceNodeAllocatable": []string{
				"pods",
				"system-reserved-compressible",
			},
		}

		// Marshal to JSON
		kubeletConfigJSON, err := json.Marshal(kubeletConfigData)
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to marshal kubelet config to JSON")

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
				KubeletConfig: &runtime.RawExtension{
					Raw: kubeletConfigJSON,
				},
				MachineConfigPoolSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"machineconfiguration.openshift.io/pool": testMCPName,
					},
				},
			},
		}

		_, err = mcClient.MachineconfigurationV1().KubeletConfigs().Create(ctx, kubeletConfig, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to create KubeletConfig")

		g.By("Waiting for KubeletConfig to be created")
		var createdKC *mcfgv1.KubeletConfig
		o.Eventually(func() error {
			createdKC, err = mcClient.MachineconfigurationV1().KubeletConfigs().Get(ctx, kubeletConfigName, metav1.GetOptions{})
			return err
		}, 30*time.Second, 5*time.Second).Should(o.Succeed(), "KubeletConfig should be created")

		o.Expect(createdKC.Spec.AutoSizingReserved).NotTo(o.BeNil(), "AutoSizingReserved should not be nil")
		o.Expect(*createdKC.Spec.AutoSizingReserved).To(o.BeTrue(), "AutoSizingReserved should be true")

		// Verify the kubelet config contains the expected fields
		var verifyKubeletConfig map[string]interface{}
		err = json.Unmarshal(createdKC.Spec.KubeletConfig.Raw, &verifyKubeletConfig)
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to unmarshal kubelet config")
		o.Expect(verifyKubeletConfig["systemReservedCgroup"]).To(o.Equal("/system.slice"), "SystemReservedCgroup should be /system.slice")
		enforceNodeAllocatable, ok := verifyKubeletConfig["enforceNodeAllocatable"].([]interface{})
		o.Expect(ok).To(o.BeTrue(), "enforceNodeAllocatable should be an array")
		o.Expect(enforceNodeAllocatable).To(o.ContainElement("system-reserved-compressible"), "EnforceNodeAllocatable should contain system-reserved-compressible")

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

		g.By("Creating a second privileged pod with /etc and /sys/fs/cgroup mounted to verify KubeletConfig was applied")
		podName = "system-reserved-test-after"
		pod = createPrivilegedPodWithHostPaths(podName, namespace, nodeName)

		// Clean up pod on test completion
		defer func() {
			g.By("Cleaning up test pod")
			deleteErr := oc.AdminKubeClient().CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
			if deleteErr != nil {
				framework.Logf("Failed to delete pod %s: %v", podName, deleteErr)
			}
		}()

		_, err = oc.AdminKubeClient().CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to create privileged pod")

		g.By("Waiting for pod to be running")
		waitForPodRunning(ctx, oc, podName, namespace)

		verifyNodeSizingEnabledFile(ctx, oc, podName, namespace, nodeName, "true")

		verifyCpuWeightSet(ctx, oc, podName, namespace, nodeName)

		// This must happen before the MCP is deleted to avoid leaving the node in a degraded state
		g.By(fmt.Sprintf("Removing node label %s from node %s to transition back to worker pool", testMCPLabel, nodeName))
		node, err = oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to get node for cleanup")
		delete(node.Labels, testMCPLabel)
		_, err = oc.AdminKubeClient().CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to remove label from node")

		// Wait for the node to transition back to the worker pool configuration
		// Without this the other tests fail
		g.By(fmt.Sprintf("Waiting for node %s to transition back to worker pool", nodeName))
		o.Eventually(func() bool {
			currentNode, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
			if err != nil {
				framework.Logf("Error getting node: %v", err)
				return false
			}
			currentConfig := currentNode.Annotations["machineconfiguration.openshift.io/currentConfig"]
			desiredConfig := currentNode.Annotations["machineconfiguration.openshift.io/desiredConfig"]

			// Check if the node is using a worker config (not system-reserved-test config)
			isWorkerConfig := currentConfig != "" && !strings.Contains(currentConfig, "system-reserved-test") && currentConfig == desiredConfig
			if isWorkerConfig {
				framework.Logf("Node %s successfully transitioned to worker config: %s", nodeName, currentConfig)
			} else {
				framework.Logf("Node %s still transitioning: current=%s, desired=%s", nodeName, currentConfig, desiredConfig)
			}
			return isWorkerConfig
		}, 10*time.Minute, 10*time.Second).Should(o.BeTrue(), fmt.Sprintf("Node %s should transition back to worker pool", nodeName))
	})
})

// createPrivilegedPodWithHostPaths creates a privileged pod with /etc and /sys/fs/cgroup mounted
func createPrivilegedPodWithHostPaths(podName, namespace, nodeName string) *corev1.Pod {
	return &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			NodeName:      nodeName,
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:    "test-container",
					Image:   image.LocationFor("registry.k8s.io/e2e-test-images/agnhost:2.53"),
					Command: []string{"/bin/sh", "-c", "sleep 300"},
					SecurityContext: &corev1.SecurityContext{
						Privileged: ptr.To(true),
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "host-etc",
							MountPath: "/host/etc",
							ReadOnly:  true,
						},
						{
							Name:      "host-cgroup",
							MountPath: "/host/sys/fs/cgroup",
							ReadOnly:  true,
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "host-etc",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/etc",
						},
					},
				},
				{
					Name: "host-cgroup",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/sys/fs/cgroup",
						},
					},
				},
			},
		},
	}
}

// waitForPodRunning waits for a pod to reach running state
func waitForPodRunning(ctx context.Context, oc *exutil.CLI, podName, namespace string) {
	o.Eventually(func() bool {
		p, err := oc.AdminKubeClient().CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return false
		}
		return p.Status.Phase == corev1.PodRunning
	}, "2m", "5s").Should(o.BeTrue(), "Pod should be running")
}

// verifyNodeSizingEnabledFile verifies the NODE_SIZING_ENABLED value in the env file
func verifyNodeSizingEnabledFile(ctx context.Context, oc *exutil.CLI, podName, namespace, nodeName, expectedValue string) {
	g.By("Verifying /etc/node-sizing-enabled.env file exists")
	output, err := oc.AsAdmin().Run("exec").Args(podName, "-n", namespace, "--", "test", "-f", "/host/etc/node-sizing-enabled.env").Output()
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("File /etc/node-sizing-enabled.env should exist on node %s. Output: %s", nodeName, output))

	g.By("Reading /etc/node-sizing-enabled.env file contents")
	output, err = oc.AsAdmin().Run("exec").Args(podName, "-n", namespace, "--", "cat", "/host/etc/node-sizing-enabled.env").Output()
	o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to read /etc/node-sizing-enabled.env")

	framework.Logf("Contents of /etc/node-sizing-enabled.env:\n%s", output)

	g.By(fmt.Sprintf("Verifying NODE_SIZING_ENABLED=%s is set in the file", expectedValue))
	o.Expect(strings.TrimSpace(output)).To(o.ContainSubstring(fmt.Sprintf("NODE_SIZING_ENABLED=%s", expectedValue)),
		fmt.Sprintf("File should contain NODE_SIZING_ENABLED=%s", expectedValue))

	framework.Logf("Successfully verified NODE_SIZING_ENABLED=%s on node %s", expectedValue, nodeName)
}

// verifyCpuWeightNotSet verifies that cpu.weight is not set in /sys/fs/cgroup/system.slice
func verifyCpuWeightNotSet(ctx context.Context, oc *exutil.CLI, podName, namespace, nodeName string) {
	g.By("Verifying /sys/fs/cgroup/system.slice/cpu.weight is not set")

	// First check if the file exists
	_, err := oc.AsAdmin().Run("exec").Args(podName, "-n", namespace, "--", "test", "-f", "/host/sys/fs/cgroup/system.slice/cpu.weight").Output()

	if err != nil {
		// File doesn't exist, which is expected in default state
		framework.Logf("cpu.weight file does not exist on node %s (expected before KubeletConfig)", nodeName)
		return
	}

	// If file exists, read its contents
	output, err := oc.AsAdmin().Run("exec").Args(podName, "-n", namespace, "--", "cat", "/host/sys/fs/cgroup/system.slice/cpu.weight").Output()
	if err != nil {
		framework.Logf("Could not read cpu.weight file on node %s (this is expected): %v", nodeName, err)
		return
	}

	framework.Logf("Contents of /sys/fs/cgroup/system.slice/cpu.weight before KubeletConfig:\n%s", output)

	// In the default state, the file might exist but should be empty or have a default value
	// We just log it for informational purposes
	framework.Logf("cpu.weight before applying KubeletConfig: %s", strings.TrimSpace(output))
}

// verifyCpuWeightSet verifies that cpu.weight is set to a non-zero value in /sys/fs/cgroup/system.slice
func verifyCpuWeightSet(ctx context.Context, oc *exutil.CLI, podName, namespace, nodeName string) {
	g.By("Verifying /sys/fs/cgroup/system.slice/cpu.weight is set")

	output, err := oc.AsAdmin().Run("exec").Args(podName, "-n", namespace, "--", "test", "-f", "/host/sys/fs/cgroup/system.slice/cpu.weight").Output()
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("File /sys/fs/cgroup/system.slice/cpu.weight should exist on node %s. Output: %s", nodeName, output))

	g.By("Reading /sys/fs/cgroup/system.slice/cpu.weight file contents")
	output, err = oc.AsAdmin().Run("exec").Args(podName, "-n", namespace, "--", "cat", "/host/sys/fs/cgroup/system.slice/cpu.weight").Output()
	o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to read /sys/fs/cgroup/system.slice/cpu.weight")

	framework.Logf("Contents of /sys/fs/cgroup/system.slice/cpu.weight:\n%s", output)

	cpuWeight := strings.TrimSpace(output)
	g.By(fmt.Sprintf("Verifying cpu.weight is set to a non-empty value (found: %s)", cpuWeight))
	o.Expect(cpuWeight).NotTo(o.BeEmpty(), "cpu.weight should be set to a non-empty value")
	o.Expect(cpuWeight).NotTo(o.Equal("0"), "cpu.weight should not be 0")

	framework.Logf("Successfully verified cpu.weight=%s is set on node %s", cpuWeight, nodeName)
}

// waitForMCPToBeReady waits for a MachineConfigPool to be ready
func waitForMCPToBeReady(ctx context.Context, mcClient *machineconfigclient.Clientset, poolName string, timeout time.Duration) error {
	return wait.PollImmediate(10*time.Second, timeout, func() (bool, error) {
		mcp, err := mcClient.MachineconfigurationV1().MachineConfigPools().Get(ctx, poolName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		// Check if all conditions are met for a ready state
		updating := false
		degraded := false
		ready := false

		for _, condition := range mcp.Status.Conditions {
			switch condition.Type {
			case "Updating":
				if condition.Status == corev1.ConditionTrue {
					updating = true
				}
			case "Degraded":
				if condition.Status == corev1.ConditionTrue {
					degraded = true
				}
			case "Updated":
				if condition.Status == corev1.ConditionTrue {
					ready = true
				}
			}
		}

		if degraded {
			return false, fmt.Errorf("MachineConfigPool %s is degraded", poolName)
		}

		// Ready when not updating and updated condition is true
		isReady := !updating && ready && mcp.Status.ReadyMachineCount == mcp.Status.MachineCount

		if isReady {
			framework.Logf("MachineConfigPool %s is ready: %d/%d machines ready",
				poolName, mcp.Status.ReadyMachineCount, mcp.Status.MachineCount)
		} else {
			framework.Logf("MachineConfigPool %s not ready yet: updating=%v, ready=%v, machines=%d/%d",
				poolName, updating, ready, mcp.Status.ReadyMachineCount, mcp.Status.MachineCount)
		}

		return isReady, nil
	})
}

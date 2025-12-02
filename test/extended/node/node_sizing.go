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
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"
	"k8s.io/utils/ptr"

	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	machineconfigclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

var _ = g.Describe("[Suite:openshift/machine-config-operator/disruptive][Suite:openshift/conformance/serial][Serial][sig-node] Node sizing", func() {
	defer g.GinkgoRecover()

	f := framework.NewDefaultFramework("node-sizing")
	f.NamespacePodSecurityLevel = admissionapi.LevelPrivileged

	oc := exutil.NewCLI("node-sizing")

	g.It("should have NODE_SIZING_ENABLED=true by default and NODE_SIZING_ENABLED=false when KubeletConfig with autoSizingReserved=false is applied", func(ctx context.Context) {
		// Skip on MicroShift since it doesn't have the Machine Config Operator
		isMicroshift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroshift {
			g.Skip("Not supported on MicroShift")
		}

		mcClient, err := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred(), "Error creating machine config client")

		testMCPName := "node-sizing-test"
		testMCPLabel := fmt.Sprintf("node-role.kubernetes.io/%s", testMCPName)
		kubeletConfigName := "auto-sizing-enabled"

		// First, verify the default state (NODE_SIZING_ENABLED=false)
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

		g.By(fmt.Sprintf("Labeling node %s with %s", nodeName, testMCPLabel))
		node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to get node")

		if node.Labels == nil {
			node.Labels = make(map[string]string)
		}
		node.Labels[testMCPLabel] = ""
		_, err = oc.AdminKubeClient().CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to label node")

		// Create custom MachineConfigPool
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
						testMCPLabel: "",
					},
				},
			},
		}

		_, err = mcClient.MachineconfigurationV1().MachineConfigPools().Create(ctx, testMCP, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to create custom MachineConfigPool")

		// Add cleanup for MachineConfigPool - added first so it runs LAST (after node label cleanup)
		// This ensures the node transitions back to worker pool before we delete the MCP
		g.DeferCleanup(func() {
			g.By("Cleaning up custom MachineConfigPool")
			deleteErr := mcClient.MachineconfigurationV1().MachineConfigPools().Delete(ctx, testMCPName, metav1.DeleteOptions{})
			if deleteErr != nil {
				framework.Logf("Failed to delete MachineConfigPool %s: %v", testMCPName, deleteErr)
			}
		})

		// Add cleanup for node label - added second so it runs BEFORE MCP deletion
		// This ensures the node transitions back to worker pool before we delete the MCP
		g.DeferCleanup(func() {
			g.By(fmt.Sprintf("Removing node label %s from node %s", testMCPLabel, nodeName))
			node, getErr := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
			if getErr != nil {
				framework.Logf("Failed to get node for cleanup: %v", getErr)
				return
			}

			delete(node.Labels, testMCPLabel)
			_, updateErr := oc.AdminKubeClient().CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
			if updateErr != nil {
				framework.Logf("Failed to remove label from node %s: %v", nodeName, updateErr)
				return
			}

			// Wait for the node to transition back to the worker pool configuration
			g.By(fmt.Sprintf("Waiting for node %s to transition back to worker pool", nodeName))
			o.Eventually(func() bool {
				currentNode, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
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
		})

		g.By("Waiting for custom MachineConfigPool to be ready")
		err = waitForMCPToBeReady(ctx, mcClient, testMCPName, 5*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred(), "Custom MachineConfigPool should become ready")

		namespace := oc.Namespace()

		g.By("Setting privileged pod security labels on namespace")
		err = oc.AsAdmin().Run("label").Args("namespace", namespace, "pod-security.kubernetes.io/enforce=privileged", "pod-security.kubernetes.io/audit=privileged", "--overwrite").Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to label namespace with privileged pod security")

		g.By("Creating a privileged pod with /etc mounted to verify default state")
		podName := "node-sizing-test"
		pod := createPrivilegedPodWithHostEtc(podName, namespace, nodeName)

		_, err = oc.AdminKubeClient().CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to create privileged pod")

		g.By("Waiting for pod to be running")
		waitForPodRunning(ctx, oc, podName, namespace)

		verifyNodeSizingEnabledFile(ctx, oc, podName, namespace, nodeName, "true")

		g.By("Deleting the test pod before applying KubeletConfig")
		err = oc.AdminKubeClient().CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to delete test pod")

		// Now apply KubeletConfig and verify NODE_SIZING_ENABLED=true

		g.By("Creating KubeletConfig with autoSizingReserved=true")
		autoSizingReserved := true
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

		// Add cleanup for KubeletConfig
		g.DeferCleanup(func() {
			g.By("Cleaning up KubeletConfig")
			deleteErr := mcClient.MachineconfigurationV1().KubeletConfigs().Delete(ctx, kubeletConfigName, metav1.DeleteOptions{})
			if deleteErr != nil {
				framework.Logf("Failed to delete KubeletConfig %s: %v", kubeletConfigName, deleteErr)
			}

			// Wait for custom MCP to be ready after cleanup
			g.By("Waiting for custom MCP to be ready after KubeletConfig deletion")
			waitErr := waitForMCPToBeReady(ctx, mcClient, testMCPName, 10*time.Minute)
			if waitErr != nil {
				framework.Logf("Failed to wait for custom MCP to be ready: %v", waitErr)
			}
		})

		g.By("Waiting for KubeletConfig to be created")
		var createdKC *mcfgv1.KubeletConfig
		o.Eventually(func() error {
			createdKC, err = mcClient.MachineconfigurationV1().KubeletConfigs().Get(ctx, kubeletConfigName, metav1.GetOptions{})
			return err
		}, 30*time.Second, 5*time.Second).Should(o.Succeed(), "KubeletConfig should be created")

		o.Expect(createdKC.Spec.AutoSizingReserved).NotTo(o.BeNil(), "AutoSizingReserved should not be nil")
		o.Expect(*createdKC.Spec.AutoSizingReserved).To(o.BeTrue(), "AutoSizingReserved should be true")

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

		g.By("Creating a second privileged pod with /etc mounted to verify KubeletConfig was applied")
		podName = "node-sizing-autosizing-test"
		pod = createPrivilegedPodWithHostEtc(podName, namespace, nodeName)

		_, err = oc.AdminKubeClient().CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to create privileged pod")

		// Add cleanup for test pod
		g.DeferCleanup(func() {
			g.By("Cleaning up test pod")
			deleteErr := oc.AdminKubeClient().CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
			if deleteErr != nil {
				framework.Logf("Failed to delete pod %s: %v", podName, deleteErr)
			}
		})

		g.By("Waiting for pod to be running")
		waitForPodRunning(ctx, oc, podName, namespace)

		verifyNodeSizingEnabledFile(ctx, oc, podName, namespace, nodeName, "false")

		// Cleanup will be handled automatically by DeferCleanup in reverse order (LIFO):
		// 1. Delete test pod (added last, runs first)
		// 2. Delete KubeletConfig and wait for MCP to reconcile (added third, runs second)
		// 3. Remove node label and wait for node to transition back to worker pool (added second, runs third)
		// 4. Delete custom MachineConfigPool (added first, runs last)
	})
})

// createPrivilegedPodWithHostEtc creates a privileged pod with /etc mounted
func createPrivilegedPodWithHostEtc(podName, namespace, nodeName string) *corev1.Pod {
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
					Image:   image.LocationFor("registry.k8s.io/e2e-test-images/agnhost:2.56"),
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

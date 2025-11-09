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

	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	machineconfigclient "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Suite:openshift/conformance/serial][Serial][sig-node] Node sizing", func() {
	defer g.GinkgoRecover()

	f := framework.NewDefaultFramework("node-sizing")
	f.NamespacePodSecurityLevel = admissionapi.LevelPrivileged

	oc := exutil.NewCLI("node-sizing")

	g.It("should have NODE_SIZING_ENABLED=false by default and NODE_SIZING_ENABLED=true when KubeletConfig with autoSizingReserved=true is applied", func(ctx context.Context) {
		// Skip on MicroShift since it doesn't have the Machine Config Operator
		isMicroshift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroshift {
			g.Skip("Not supported on MicroShift")
		}

		// Skip test on hypershift platforms
		if ok, _ := exutil.IsHypershift(ctx, oc.AdminConfigClient()); ok {
			g.Skip("KubeletConfig is not supported on hypershift. Skipping test.")
		}

		// First, verify the default state (NODE_SIZING_ENABLED=false)
		g.By("Getting a worker node to test")
		nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{
			LabelSelector: "node-role.kubernetes.io/worker",
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to list worker nodes")
		o.Expect(len(nodes.Items)).To(o.BeNumerically(">", 0), "Should have at least one worker node")

		nodeName := nodes.Items[0].Name
		framework.Logf("Testing on node: %s", nodeName)

		namespace := oc.Namespace()

		g.By("Setting privileged pod security labels on namespace")
		err = oc.AsAdmin().Run("label").Args("namespace", namespace, "pod-security.kubernetes.io/enforce=privileged", "pod-security.kubernetes.io/audit=privileged", "--overwrite").Execute()
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to label namespace with privileged pod security")

		g.By("Creating a privileged pod with /etc mounted to verify default state")
		podName := "node-sizing-test"

		pod := &corev1.Pod{
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
						Image:   "registry.k8s.io/e2e-test-images/agnhost:2.53",
						Command: []string{"/bin/sh", "-c", "sleep 300"},
						SecurityContext: &corev1.SecurityContext{
							Privileged: func() *bool { b := true; return &b }(),
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

		_, err = oc.AdminKubeClient().CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to create privileged pod")

		g.By("Waiting for pod to be running")
		o.Eventually(func() bool {
			p, err := oc.AdminKubeClient().CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
			if err != nil {
				return false
			}
			return p.Status.Phase == corev1.PodRunning
		}, "2m", "5s").Should(o.BeTrue(), "Pod should be running")

		g.By("Verifying /etc/node-sizing-enabled.env file exists")
		output, err := oc.AsAdmin().Run("exec").Args(podName, "-n", namespace, "--", "test", "-f", "/host/etc/node-sizing-enabled.env").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("File /etc/node-sizing-enabled.env should exist on node %s. Output: %s", nodeName, output))

		g.By("Reading /etc/node-sizing-enabled.env file contents")
		output, err = oc.AsAdmin().Run("exec").Args(podName, "-n", namespace, "--", "cat", "/host/etc/node-sizing-enabled.env").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to read /etc/node-sizing-enabled.env")

		framework.Logf("Contents of /etc/node-sizing-enabled.env:\n%s", output)

		g.By("Verifying NODE_SIZING_ENABLED=false is set in the file by default")
		o.Expect(strings.TrimSpace(output)).To(o.ContainSubstring("NODE_SIZING_ENABLED=false"),
			"File should contain NODE_SIZING_ENABLED=false by default")

		framework.Logf("Successfully verified NODE_SIZING_ENABLED=false on node %s", nodeName)

		g.By("Deleting the test pod before applying KubeletConfig")
		err = oc.AdminKubeClient().CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to delete test pod")

		// Now apply KubeletConfig and verify NODE_SIZING_ENABLED=true

		// Create machine config client
		mcClient, err := machineconfigclient.NewForConfig(oc.KubeFramework().ClientConfig())
		o.Expect(err).NotTo(o.HaveOccurred(), "Error creating machine config client")

		kubeletConfigName := "auto-sizing-enabled"

		// Clean up KubeletConfig on test completion
		defer func() {
			g.By("Cleaning up KubeletConfig")
			deleteErr := mcClient.MachineconfigurationV1().KubeletConfigs().Delete(ctx, kubeletConfigName, metav1.DeleteOptions{})
			if deleteErr != nil {
				framework.Logf("Failed to delete KubeletConfig %s: %v", kubeletConfigName, deleteErr)
			}

			// Wait for worker MCP to be ready after cleanup
			g.By("Waiting for worker MCP to be ready after cleanup")
			waitErr := waitForMCPToBeReady(ctx, mcClient, "worker", 10*time.Minute)
			if waitErr != nil {
				framework.Logf("Failed to wait for worker MCP to be ready: %v", waitErr)
			}
		}()

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
						"pools.operator.machineconfiguration.openshift.io/worker": "",
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

		g.By("Waiting for worker MCP to start updating")
		o.Eventually(func() bool {
			mcp, err := mcClient.MachineconfigurationV1().MachineConfigPools().Get(ctx, "worker", metav1.GetOptions{})
			if err != nil {
				framework.Logf("Error getting worker MCP: %v", err)
				return false
			}
			// Check if MCP is updating (has conditions indicating update in progress)
			for _, condition := range mcp.Status.Conditions {
				if condition.Type == "Updating" && condition.Status == corev1.ConditionTrue {
					return true
				}
			}
			return false
		}, 2*time.Minute, 10*time.Second).Should(o.BeTrue(), "Worker MCP should start updating")

		g.By("Waiting for worker MCP to be ready with new configuration")
		err = waitForMCPToBeReady(ctx, mcClient, "worker", 15*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred(), "Worker MCP should become ready with new configuration")

		g.By("Getting a worker node to test after KubeletConfig is applied")
		nodes, err = oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{
			LabelSelector: "node-role.kubernetes.io/worker",
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to list worker nodes")
		o.Expect(len(nodes.Items)).To(o.BeNumerically(">", 0), "Should have at least one worker node")

		nodeName = nodes.Items[0].Name
		framework.Logf("Testing on node: %s", nodeName)

		g.By("Creating a second privileged pod with /etc mounted to verify KubeletConfig was applied")
		podName = "node-sizing-autosizing-test"

		pod = &corev1.Pod{
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
						Image:   "registry.k8s.io/e2e-test-images/agnhost:2.53",
						Command: []string{"/bin/sh", "-c", "sleep 300"},
						SecurityContext: &corev1.SecurityContext{
							Privileged: func() *bool { b := true; return &b }(),
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
		o.Eventually(func() bool {
			p, err := oc.AdminKubeClient().CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
			if err != nil {
				return false
			}
			return p.Status.Phase == corev1.PodRunning
		}, "2m", "5s").Should(o.BeTrue(), "Pod should be running")

		g.By("Verifying /etc/node-sizing-enabled.env file exists after KubeletConfig is applied")
		output, err = oc.AsAdmin().Run("exec").Args(podName, "-n", namespace, "--", "test", "-f", "/host/etc/node-sizing-enabled.env").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("File /etc/node-sizing-enabled.env should exist on node %s. Output: %s", nodeName, output))

		g.By("Reading /etc/node-sizing-enabled.env file contents after KubeletConfig is applied")
		output, err = oc.AsAdmin().Run("exec").Args(podName, "-n", namespace, "--", "cat", "/host/etc/node-sizing-enabled.env").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to read /etc/node-sizing-enabled.env")

		framework.Logf("Contents of /etc/node-sizing-enabled.env after applying KubeletConfig:\n%s", output)

		g.By("Verifying NODE_SIZING_ENABLED=true is set in the file")
		o.Expect(strings.TrimSpace(output)).To(o.ContainSubstring("NODE_SIZING_ENABLED=true"),
			"File should contain NODE_SIZING_ENABLED=true")

		framework.Logf("Successfully verified NODE_SIZING_ENABLED=true on node %s after applying KubeletConfig with autoSizingReserved=true", nodeName)
	})
})

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

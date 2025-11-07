package node

import (
	"context"
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-node] Node sizing", func() {
	defer g.GinkgoRecover()

	f := framework.NewDefaultFramework("node-sizing")
	f.NamespacePodSecurityLevel = admissionapi.LevelPrivileged

	oc := exutil.NewCLI("node-sizing")

	g.It("should have NODE_SIZING_ENABLED=true in /etc/node-sizing-enabled.env", func(ctx context.Context) {
		// Skip on MicroShift since it doesn't have the Machine Config Operator
		isMicroshift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroshift {
			g.Skip("Not supported on MicroShift")
		}

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

		g.By("Creating a privileged pod with /etc mounted")
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

		g.By("Verifying /etc/node-sizing-enabled.env file exists")
		output, err := oc.AsAdmin().Run("exec").Args(podName, "-n", namespace, "--", "test", "-f", "/host/etc/node-sizing-enabled.env").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("File /etc/node-sizing-enabled.env should exist on node %s. Output: %s", nodeName, output))

		g.By("Reading /etc/node-sizing-enabled.env file contents")
		output, err = oc.AsAdmin().Run("exec").Args(podName, "-n", namespace, "--", "cat", "/host/etc/node-sizing-enabled.env").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to read /etc/node-sizing-enabled.env")

		framework.Logf("Contents of /etc/node-sizing-enabled.env:\n%s", output)

		g.By("Verifying NODE_SIZING_ENABLED=true is set in the file")
		o.Expect(strings.TrimSpace(output)).To(o.ContainSubstring("NODE_SIZING_ENABLED=true"),
			"File should contain NODE_SIZING_ENABLED=true")

		framework.Logf("Successfully verified NODE_SIZING_ENABLED=true on node %s", nodeName)
	})
})

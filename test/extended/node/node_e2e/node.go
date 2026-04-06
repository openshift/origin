package node

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"

	nodeutils "github.com/openshift/origin/test/extended/node"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-node] [Jira:Node/Kubelet] Kubelet, CRI-O, CPU manager", func() {
	var (
		oc             = exutil.NewCLIWithoutNamespace("node")
		nodeE2EBaseDir = exutil.FixturePath("testdata", "node", "node_e2e")
		podDevFuseYAML = filepath.Join(nodeE2EBaseDir, "pod-dev-fuse.yaml")
	)

	// Skip all tests on MicroShift clusters as MachineConfig resources are not available
	g.BeforeEach(func() {
		var isMicroShift bool
		var err error

		// Retry check for robustness - OpenShift should eventually respond
		pollErr := wait.Poll(2*time.Second, 30*time.Second, func() (bool, error) {
			isMicroShift, err = exutil.IsMicroShiftCluster(oc.AdminKubeClient())
			if err != nil {
				e2e.Logf("Failed to check if cluster is MicroShift: %v, retrying...", err)
				return false, nil
			}
			return true, nil
		})

		if pollErr != nil {
			e2e.Logf("Setup failed: unable to determine if cluster is MicroShift after retries: %v", err)
			g.Fail("Setup failed: unable to determine cluster type - this is an infrastructure/connectivity issue, not a test failure")
		}

		if isMicroShift {
			g.Skip("Skipping test on MicroShift cluster - MachineConfig resources are not available")
		}
	})

	//author: asahay@redhat.com
	g.It("[OTP] validate KUBELET_LOG_LEVEL", func() {
		var kubeservice string
		var kubelet string
		var err error

		g.By("Polling to check kubelet log level on ready nodes")
		waitErr := wait.Poll(10*time.Second, 1*time.Minute, func() (bool, error) {
			g.By("Getting all node names in the cluster")
			nodeName, nodeErr := oc.AsAdmin().Run("get").Args("nodes", "-o=jsonpath={.items[*].metadata.name}").Output()
			o.Expect(nodeErr).NotTo(o.HaveOccurred())
			e2e.Logf("\nNode Names are %v", nodeName)
			nodes := strings.Fields(nodeName)

			for _, node := range nodes {
				g.By("Checking if node " + node + " is Ready")
				nodeStatus, statusErr := oc.AsAdmin().Run("get").Args("nodes", node, "-o=jsonpath={.status.conditions[?(@.type=='Ready')].status}").Output()
				o.Expect(statusErr).NotTo(o.HaveOccurred())
				e2e.Logf("\nNode %s Status is %s\n", node, nodeStatus)

				if nodeStatus == "True" {
					g.By("Checking KUBELET_LOG_LEVEL in kubelet.service on node " + node)
					kubeservice, err = nodeutils.ExecOnNodeWithChroot(oc, node, "/bin/bash", "-c", "systemctl show kubelet.service | grep KUBELET_LOG_LEVEL")
					o.Expect(err).NotTo(o.HaveOccurred())

					g.By("Checking kubelet process for --v=2 flag on node " + node)
					kubelet, err = nodeutils.ExecOnNodeWithChroot(oc, node, "/bin/bash", "-c", "ps aux | grep [k]ubelet")
					o.Expect(err).NotTo(o.HaveOccurred())

					g.By("Verifying KUBELET_LOG_LEVEL is set and kubelet is running with --v=2")
					if strings.Contains(kubeservice, "KUBELET_LOG_LEVEL") && strings.Contains(kubelet, "--v=2") {
						e2e.Logf("KUBELET_LOG_LEVEL is 2.\n")
						return true, nil
					} else {
						e2e.Logf("KUBELET_LOG_LEVEL is not 2.\n")
						return false, nil
					}
				} else {
					e2e.Logf("\nNode %s is not Ready, Skipping\n", node)
				}
			}
			return false, nil
		})

		if waitErr != nil {
			e2e.Logf("Kubelet Log level is:\n %v\n", kubeservice)
			e2e.Logf("Running Process of kubelet are:\n %v\n", kubelet)
		}
		o.Expect(waitErr).NotTo(o.HaveOccurred(), "KUBELET_LOG_LEVEL is not expected, timed out")
	})

	//author: cmaurya@redhat.com
	g.It("[OTP] validate cgroupv2 is default [OCP-80983]", func() {
		g.By("Check cgroup version on all Ready worker nodes")
		nodeNames, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "-l", "node-role.kubernetes.io/worker", "-o=jsonpath={.items[*].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		workers := strings.Fields(nodeNames)
		o.Expect(workers).NotTo(o.BeEmpty(), "No worker nodes found")

		for _, worker := range workers {
			nodeStatus, err := oc.AsAdmin().Run("get").Args("nodes", worker, "-o=jsonpath={.status.conditions[?(@.type=='Ready')].status}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if nodeStatus != "True" {
				e2e.Logf("Skipping worker node %s (not Ready)", worker)
				continue
			}
			cgroupV, err := nodeutils.ExecOnNodeWithChroot(oc, worker, "/bin/bash", "-c", "stat -c %T -f /sys/fs/cgroup")
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("cgroup version on node %s: [%v]", worker, cgroupV)
			o.Expect(cgroupV).To(o.ContainSubstring("cgroup2fs"), "Node %s does not have cgroupv2", worker)
		}

		g.By("Changing cgroup from v2 to v1 should result in error")
		output, err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("nodes.config.openshift.io", "cluster", "-p", `{"spec": {"cgroupMode": "v1"}}`, "--type=merge").Output()
		o.Expect(err).Should(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("spec.cgroupMode: Unsupported value: \"v1\": supported values: \"v2\", \"\""))
	})

	//author: cmaurya@redhat.com
	g.It("[OTP] Allow dev fuse by default in CRI-O [OCP-70987]", func() {
		podName := "pod-devfuse"
		ns := "devfuse-test"

		g.By("Create a test namespace")
		err := oc.AsAdmin().WithoutNamespace().Run("create").Args("namespace", ns).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("namespace", ns, "--ignore-not-found").Execute()

		g.By("Create a pod with dev fuse annotation")
		err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", podDevFuseYAML, "-n", ns).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Wait for pod to be ready")
		err = wait.Poll(5*time.Second, 1*time.Minute, func() (bool, error) {
			status, pollErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", podName, "-n", ns, "-o=jsonpath={.status.conditions[?(@.type=='Ready')].status}").Output()
			if pollErr != nil {
				e2e.Logf("Error polling pod status: %v", pollErr)
				return false, nil
			}
			return status == "True", nil
		})
		if err != nil {
			podStatus, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", podName, "-n", ns, "-o=jsonpath={.status}").Output()
			e2e.Logf("Pod status on timeout: %s", podStatus)
		}
		o.Expect(err).NotTo(o.HaveOccurred(), "pod did not become ready")

		g.By("Check /dev/fuse is mounted inside the pod")
		output, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args(podName, "-n", ns, "--", "stat", "/dev/fuse").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("/dev/fuse mount output: %s", output)
		o.Expect(output).To(o.ContainSubstring("fuse"), "dev fuse is not mounted inside pod")
	})
})

var _ = g.Describe("[sig-node] [Jira:Node/Kubelet] NODE initContainer policy,volume,readiness,quota", func() {
	defer g.GinkgoRecover()

	var (
		oc = exutil.NewCLI("node-initcontainer")
	)

	// Skip all tests on MicroShift clusters as MachineConfig resources are not available
	g.BeforeEach(func() {
		var isMicroShift bool
		var err error

		// Retry check for robustness - OpenShift should eventually respond
		pollErr := wait.Poll(2*time.Second, 30*time.Second, func() (bool, error) {
			isMicroShift, err = exutil.IsMicroShiftCluster(oc.AdminKubeClient())
			if err != nil {
				e2e.Logf("Failed to check if cluster is MicroShift: %v, retrying...", err)
				return false, nil
			}
			return true, nil
		})

		if pollErr != nil {
			e2e.Logf("Setup failed: unable to determine if cluster is MicroShift after retries: %v", err)
			g.Fail("Setup failed: unable to determine cluster type - this is an infrastructure/connectivity issue, not a test failure")
		}

		if isMicroShift {
			g.Skip("Skipping test on MicroShift cluster - MachineConfig resources are not available")
		}
	})

	//author: bgudi@redhat.com
	g.It("[OTP] Init containers should not restart when the exited init container is removed from node [OCP-38271]", func() {
		g.By("Test for case OCP-38271")
		oc.SetupProject()

		podName := "initcon-pod"
		namespace := oc.Namespace()
		ctx := context.Background()

		g.By("Create a pod with init container")
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: namespace,
			},
			Spec: corev1.PodSpec{
				InitContainers: []corev1.Container{
					{
						Name:    "inittest",
						Image:   "image-registry.openshift-image-registry.svc:5000/openshift/tools:latest",
						Command: []string{"/bin/sh", "-ec", "echo running >> /mnt/data/test"},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "data",
								MountPath: "/mnt/data",
							},
						},
					},
				},
				Containers: []corev1.Container{
					{
						Name:    "hello-test",
						Image:   "image-registry.openshift-image-registry.svc:5000/openshift/tools:latest",
						Command: []string{"/bin/sh", "-c", "sleep 3600"},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "data",
								MountPath: "/mnt/data",
							},
						},
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: "data",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				},
				RestartPolicy: corev1.RestartPolicyNever,
			},
		}

		_, err := oc.KubeClient().CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			oc.KubeClient().CoreV1().Pods(namespace).Delete(ctx, podName, metav1.DeleteOptions{})
		}()

		g.By("Check pod status")
		err = e2epod.WaitForPodRunningInNamespace(ctx, oc.KubeClient(), pod)
		o.Expect(err).NotTo(o.HaveOccurred(), "pod is not running")

		g.By("Check init container exit normally")
		err = wait.Poll(5*time.Second, 1*time.Minute, func() (bool, error) {
			pod, err := oc.KubeClient().CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			for _, status := range pod.Status.InitContainerStatuses {
				if status.Name == "inittest" {
					if status.State.Terminated != nil && status.State.Terminated.ExitCode == 0 {
						e2e.Logf("Init container exited with code 0")
						return true, nil
					}
				}
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "container not exit normally")

		g.By("Get node where pod is running")
		pod, err = oc.KubeClient().CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		nodeName := pod.Spec.NodeName
		o.Expect(nodeName).NotTo(o.BeEmpty(), "pod node name is empty")

		g.By("Get init container ID from pod status")
		var containerID string
		for _, status := range pod.Status.InitContainerStatuses {
			if status.Name == "inittest" {
				containerID = status.ContainerID
				break
			}
		}
		o.Expect(containerID).NotTo(o.BeEmpty(), "init container ID is empty")

		// Extract the actual container ID (remove prefix like "cri-o://")
		containerIDPattern := regexp.MustCompile(`^[^/]+://(.+)$`)
		matches := containerIDPattern.FindStringSubmatch(containerID)
		o.Expect(matches).To(o.HaveLen(2), "failed to parse container ID")
		actualContainerID := matches[1]

		g.By("Delete init container from node")
		output, err := nodeutils.ExecOnNodeWithChroot(oc, nodeName, "crictl", "rm", actualContainerID)
		o.Expect(err).NotTo(o.HaveOccurred(), "fail to delete container")
		e2e.Logf("Container deletion output: %s", output)

		g.By("Check init container not restart again")
		err = wait.Poll(5*time.Second, 1*time.Minute, func() (bool, error) {
			pod, err := oc.KubeClient().CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			for _, status := range pod.Status.InitContainerStatuses {
				if status.Name == "inittest" {
					if status.RestartCount > 0 {
						e2e.Logf("Init container restarted, restart count: %d", status.RestartCount)
						return true, fmt.Errorf("init container restarted")
					}
				}
			}
			e2e.Logf("Init container has not restarted")
			return false, nil
		})
		o.Expect(err).To(o.Equal(wait.ErrWaitTimeout), "expected timeout while waiting confirms init container did not restart")
	})
})

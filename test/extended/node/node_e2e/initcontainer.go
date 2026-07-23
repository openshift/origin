package node

import (
	"context"
	"fmt"
	"regexp"
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

var _ = g.Describe("[sig-node] [Jira:Node/Kubelet] NODE initContainer policy,volume,readiness,quota", func() {
	defer g.GinkgoRecover()

	var (
		oc = exutil.NewCLI("node-initcontainer")
	)

	// Skip all tests on MicroShift clusters as MachineConfig resources are not available
	g.BeforeEach(func() {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
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

		g.By("Get pod and verify init container exited normally")
		pod, err = oc.KubeClient().CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		o.Expect(pod.Status.InitContainerStatuses).To(o.ContainElement(o.SatisfyAll(
			o.HaveField("Name", "inittest"),
			o.HaveField("State.Terminated.ExitCode", o.Equal(int32(0))),
		)), "init container 'inittest' should have terminated with exit code 0")

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

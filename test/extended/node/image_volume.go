package node

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/kubelet/kuberuntime"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-node] [FeatureGate:ImageVolume] ImageVolume", func() {
	defer g.GinkgoRecover()

	f := framework.NewDefaultFramework("image-volume")
	f.NamespacePodSecurityLevel = admissionapi.LevelPrivileged

	var (
		oc      = exutil.NewCLI("image-volume")
		podName = "image-volume-test"
		image   = "image-registry.openshift-image-registry.svc:5000/openshift/cli:latest"
	)

	g.BeforeEach(func() {
		// Microshift doesn't inherit OCP feature gates, and ImageVolume won't work either
		isMicroshift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroshift {
			g.Skip("Not supported on Microshift")
		}
	})

	g.It("should succeed with pod and pull policy of Always", func(ctx context.Context) {
		pod := buildPodWithImageVolume(f.Namespace.Name, "", podName, image)
		createPodAndWaitForRunning(ctx, oc, pod)
		verifyVolumeMounted(f, pod, "ls", "/mnt/image/bin/oc")
	})

	g.It("should handle multiple image volumes", func(ctx context.Context) {
		pod := buildPodWithMultipleImageVolumes(f.Namespace.Name, "", podName, image, image)
		createPodAndWaitForRunning(ctx, oc, pod)
		verifyVolumeMounted(f, pod, "ls", "/mnt/image/bin/oc")
		verifyVolumeMounted(f, pod, "ls", "/mnt/image2/bin/oc")
	})

	g.It("should fail when image does not exist", func(ctx context.Context) {
		pod := buildPodWithImageVolume(f.Namespace.Name, "", podName, "nonexistent:latest")

		g.By("Creating a pod with non-existent image volume")
		_, err := oc.AdminKubeClient().CoreV1().Pods(f.Namespace.Name).Create(ctx, pod, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for pod to be ErrImagePull or ImagePullBackOff")
		// wait for 5 mins so that the pod in metal-ovn-two-node-arbiter-ipv6-techpreview can become ImagePullBackOff.
		err = e2epod.WaitForPodCondition(ctx, oc.AdminKubeClient(), pod.Namespace, pod.Name, "ImagePullBackOff", 5*time.Minute, func(pod *v1.Pod) (bool, error) {
			return len(pod.Status.ContainerStatuses) > 0 &&
					pod.Status.ContainerStatuses[0].State.Waiting != nil &&
					(pod.Status.ContainerStatuses[0].State.Waiting.Reason == "ImagePullBackOff" || pod.Status.ContainerStatuses[0].State.Waiting.Reason == "ErrImagePull"),
				nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("should succeed if image volume is not existing but unused", func(ctx context.Context) {
		pod := buildPodWithImageVolume(f.Namespace.Name, "", podName, "nonexistent:latest")
		pod.Spec.Containers[0].VolumeMounts = []v1.VolumeMount{}
		createPodAndWaitForRunning(ctx, oc, pod)
		// The container has no image volume mount, so just checking running is enough
	})

	g.It("should succeed with multiple pods and same image on the same node", func(ctx context.Context) {
		pod1 := buildPodWithImageVolume(f.Namespace.Name, "", podName, image)
		pod1 = createPodAndWaitForRunning(ctx, oc, pod1)

		pod2 := buildPodWithImageVolume(f.Namespace.Name, pod1.Spec.NodeName, podName+"-2", image)
		pod2 = createPodAndWaitForRunning(ctx, oc, pod2)

		verifyVolumeMounted(f, pod1, "ls", "/mnt/image/bin/oc")
		verifyVolumeMounted(f, pod2, "ls", "/mnt/image/bin/oc")
	})

	g.It("should report kubelet image volume metrics correctly [OCP-84149]", func(ctx context.Context) {
		const (
			podName   = "image-volume-metrics-test"
			imageRef  = "quay.io/crio/artifact:v1"
			mountPath = "/mnt/image"
		)

		// Step 1: Create a pod with OCI image as volume source
		g.By("Creating a pod with OCI image as volume source")
		pod := buildPodWithImageVolume(f.Namespace.Name, "", podName, imageRef)
		pod = createPodAndWaitForRunning(ctx, oc, pod)

		// Step 2: Verify the image is mounted successfully and read-only
		g.By("Verifying image volume is mounted into the container")
		verifyImageVolumeMounted(f, pod, mountPath)

		g.By("Verifying the mounted volume is read-only")
		verifyVolumeReadOnly(f, pod, mountPath)

		// Step 3: Check kubelet metrics about image volume
		g.By("Checking kubelet metrics for image volume")
		metrics, err := getKubeletMetrics(ctx, oc, pod.Spec.NodeName)
		o.Expect(err).NotTo(o.HaveOccurred(), "Failed to get kubelet metrics")

		g.By("Verifying kubelet_image_volume_requested_total metric")
		requestedTotal, found := parseMetricValue(metrics, "kubelet_image_volume_requested_total")
		o.Expect(found).To(o.BeTrue(), "kubelet_image_volume_requested_total metric should exist")
		o.Expect(requestedTotal).To(o.BeNumerically(">=", 1),
			"kubelet_image_volume_requested_total should be at least 1")

		g.By("Verifying kubelet_image_volume_mounted_succeed_total metric")
		succeededTotal, found := parseMetricValue(metrics, "kubelet_image_volume_mounted_succeed_total")
		o.Expect(found).To(o.BeTrue(), "kubelet_image_volume_mounted_succeed_total metric should exist")
		o.Expect(succeededTotal).To(o.BeNumerically(">=", 1),
			"kubelet_image_volume_mounted_succeed_total should be at least 1")

		g.By("Verifying kubelet_image_volume_mounted_errors_total metric")
		errorsTotal, found := parseMetricValue(metrics, "kubelet_image_volume_mounted_errors_total")
		o.Expect(found).To(o.BeTrue(), "kubelet_image_volume_mounted_errors_total metric should exist")
		o.Expect(errorsTotal).To(o.Equal(0),
			"kubelet_image_volume_mounted_errors_total should be 0")
	})

	g.Context("when subPath is used", func() {
		g.It("should handle image volume with subPath", func(ctx context.Context) {
			pod := buildPodWithImageVolumeSubPath(f.Namespace.Name, "", podName, image, "bin")
			createPodAndWaitForRunning(ctx, oc, pod)
			verifyVolumeMounted(f, pod, "ls", "/mnt/image/oc")
		})

		g.It("should fail to mount image volume with invalid subPath", func(ctx context.Context) {
			pod := buildPodWithImageVolumeSubPath(f.Namespace.Name, "", podName, image, "noexist")
			g.By("Creating a pod with image volume and subPath")
			_, err := oc.AdminKubeClient().CoreV1().Pods(f.Namespace.Name).Create(ctx, pod, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Waiting for a pod to fail")
			err = e2epod.WaitForPodContainerToFail(ctx, oc.AdminKubeClient(), pod.Namespace, pod.Name, 0, kuberuntime.ErrCreateContainer.Error(), 5*time.Minute)
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})
})

func createPodAndWaitForRunning(ctx context.Context, oc *exutil.CLI, pod *v1.Pod) *v1.Pod {
	g.By("Creating a pod")
	_, err := oc.AdminKubeClient().CoreV1().Pods(pod.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("Waiting for pod to be running")
	err = e2epod.WaitForPodRunningInNamespace(ctx, oc.AdminKubeClient(), pod)
	o.Expect(err).NotTo(o.HaveOccurred())

	created, err := oc.AdminKubeClient().CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	return created
}

func verifyVolumeMounted(f *framework.Framework, pod *v1.Pod, commands ...string) {
	g.By("Verifying image volume in pod is mounted")
	stdout := e2epod.ExecCommandInContainer(f, pod.Name, pod.Spec.Containers[0].Name, commands...)
	o.Expect(stdout).NotTo(o.BeEmpty())
}

func buildPodWithImageVolume(namespace, nodeName, podName, image string) *v1.Pod {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
		},
		Spec: v1.PodSpec{
			NodeName: nodeName,
			Containers: []v1.Container{
				{
					Name:    "test-container",
					Image:   "image-registry.openshift-image-registry.svc:5000/openshift/tools:latest",
					Command: []string{"sh", "-c", "trap 'exit 0' TERM INT; sleep infinity & wait"},
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      "image-vol",
							MountPath: "/mnt/image",
						},
					},
				},
			},
			Volumes: []v1.Volume{
				{
					Name: "image-vol",
					VolumeSource: v1.VolumeSource{
						Image: &v1.ImageVolumeSource{
							Reference: image,
						},
					},
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
		},
	}
	return pod
}

func buildPodWithImageVolumeSubPath(namespace, nodeName, podName, image, subPath string) *v1.Pod {
	pod := buildPodWithImageVolume(namespace, nodeName, podName, image)
	pod.Spec.Containers[0].VolumeMounts[0].SubPath = subPath
	return pod
}

func buildPodWithMultipleImageVolumes(namespace, nodeName, podName, image1, image2 string) *v1.Pod {
	pod := buildPodWithImageVolume(namespace, nodeName, podName, image1)
	pod.Spec.Volumes = append(pod.Spec.Volumes, v1.Volume{
		Name: "image-vol-2",
		VolumeSource: v1.VolumeSource{
			Image: &v1.ImageVolumeSource{
				Reference: image2,
			},
		},
	})
	pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, v1.VolumeMount{
		Name:      "image-vol-2",
		MountPath: "/mnt/image2",
	})
	return pod
}

// verifyImageVolumeMounted verifies that the image volume is mounted and accessible
func verifyImageVolumeMounted(f *framework.Framework, pod *v1.Pod, mountPath string) {
	g.By(fmt.Sprintf("Checking if volume is mounted at %s", mountPath))

	// Verify the content of the expected file
	stdout := e2epod.ExecCommandInContainer(f, pod.Name, pod.Spec.Containers[0].Name,
		"cat", mountPath+"/file")
	o.Expect(stdout).To(o.Equal("2"), "File content should be '2'")
}

// verifyVolumeReadOnly verifies that the mounted volume is read-only
func verifyVolumeReadOnly(f *framework.Framework, pod *v1.Pod, mountPath string) {
	g.By("Verifying the volume is mounted as read-only")

	// Check mount options
	stdout := e2epod.ExecCommandInContainer(f, pod.Name, pod.Spec.Containers[0].Name,
		"mount")
	o.Expect(stdout).To(o.ContainSubstring(mountPath), "Mount point should be listed")

	// Verify read-only in mount output
	mountLines := strings.Split(stdout, "\n")
	for _, line := range mountLines {
		if strings.Contains(line, mountPath) {
			o.Expect(line).To(o.MatchRegexp(`\bro\b`),
				"Volume should be mounted with 'ro' (read-only) option")
			framework.Logf("Mount info: %s", line)
			break
		}
	}

	// Try to write to the volume (should fail)
	g.By("Attempting to write to the read-only volume (should fail)")
	_, _, err := e2epod.ExecCommandInContainerWithFullOutput(f, pod.Name, pod.Spec.Containers[0].Name,
		"touch", mountPath+"/testfile")
	o.Expect(err).To(o.HaveOccurred(), "Writing to read-only volume should fail")
}

// getKubeletMetrics fetches kubelet metrics from a specific node
func getKubeletMetrics(ctx context.Context, oc *exutil.CLI, nodeName string) (string, error) {
	metricsPath := fmt.Sprintf("/api/v1/nodes/%s/proxy/metrics", nodeName)

	data, err := oc.AdminKubeClient().CoreV1().RESTClient().Get().
		AbsPath(metricsPath).
		DoRaw(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get metrics from node %s: %w", nodeName, err)
	}

	return string(data), nil
}

// parseMetricValue parses a Prometheus metric value from metrics output
func parseMetricValue(metrics, metricName string) (int, bool) {
	// Look for lines like: kubelet_image_volume_requested_total 1
	// Skip HELP and TYPE lines
	re := regexp.MustCompile(fmt.Sprintf(`^%s\s+(\d+)`, metricName))

	lines := strings.Split(metrics, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue // Skip comment lines
		}

		matches := re.FindStringSubmatch(line)
		if len(matches) == 2 {
			value, err := strconv.Atoi(matches[1])
			if err == nil {
				return value, true
			}
		}
	}

	framework.Logf("Metric %s not found in output", metricName)
	return 0, false
}

package node

import (
	"context"
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
		// Skip if ImageVolume feature is not enabled
		if !exutil.IsTechPreviewNoUpgrade(context.TODO(), oc.AdminConfigClient()) {
			g.Skip("skipping, this feature is only supported on TechPreviewNoUpgrade clusters")
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

package node

import (
	"context"
	"path"
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
	"github.com/openshift/origin/test/extended/util/image"
)

// imageVolumeTestConfig parameterizes the shared image/artifact volume test suite.
type imageVolumeTestConfig struct {
	// describeLabel is appended to "[sig-node] [FeatureGate:ImageVolume] "
	describeLabel string
	// frameworkName is used for framework.NewDefaultFramework and exutil.NewCLI
	frameworkName string
	// getVolumeRef returns the reference for the image/artifact volume.
	// Called at the start of each test that needs a valid, pullable reference.
	getVolumeRef func(ctx context.Context, oc *exutil.CLI, namespace string) string
	// pathsToVerify are paths relative to the image root that must exist after mounting.
	// The directory of the first path is used as the subPath in subPath test.
	pathsToVerify []string
}

// Register image volume tests
var _ = describeImageVolumeTests(imageVolumeTestConfig{
	describeLabel: "ArtifactVolume",
	frameworkName: "artifact-volume",
	getVolumeRef: func(_ context.Context, _ *exutil.CLI, _ string) string {
		return image.LocationFor("quay.io/crio/artifact:subpath")
	},
	pathsToVerify: []string{"subpath/2", "subpath/3"},
})

var _ = describeImageVolumeTests(imageVolumeTestConfig{
	describeLabel: "ImageVolume",
	frameworkName: "image-volume",
	getVolumeRef: func(_ context.Context, _ *exutil.CLI, _ string) string {
		return "image-registry.openshift-image-registry.svc:5000/openshift/cli:latest"
	},
	pathsToVerify: []string{"bin/oc"},
})

// describeImageVolumeTests generates a full test suite for image or artifact volumes.
func describeImageVolumeTests(config imageVolumeTestConfig) bool {
	return g.Describe("[sig-node] [FeatureGate:ImageVolume] "+config.describeLabel, func() {
		defer g.GinkgoRecover()

		f := framework.NewDefaultFramework(config.frameworkName)
		f.NamespacePodSecurityLevel = admissionapi.LevelPrivileged

		var (
			oc      = exutil.NewCLI(config.frameworkName)
			podName = config.frameworkName + "-test"
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
			ref := config.getVolumeRef(ctx, oc, f.Namespace.Name)
			pod := buildPodWithImageVolume(f.Namespace.Name, "", podName, ref)
			createPodAndWaitForRunning(ctx, oc, pod)
			verifyPathsExist(f, pod, "/mnt/image", config.pathsToVerify)
		})

		g.It("should handle multiple image volumes", func(ctx context.Context) {
			ref := config.getVolumeRef(ctx, oc, f.Namespace.Name)
			pod := buildPodWithMultipleImageVolumes(f.Namespace.Name, "", podName, ref, ref)
			createPodAndWaitForRunning(ctx, oc, pod)
			verifyPathsExist(f, pod, "/mnt/image", config.pathsToVerify)
			verifyPathsExist(f, pod, "/mnt/image2", config.pathsToVerify)
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
			ref := config.getVolumeRef(ctx, oc, f.Namespace.Name)

			pod1 := buildPodWithImageVolume(f.Namespace.Name, "", podName, ref)
			pod1 = createPodAndWaitForRunning(ctx, oc, pod1)

			pod2 := buildPodWithImageVolume(f.Namespace.Name, pod1.Spec.NodeName, podName+"-2", ref)
			pod2 = createPodAndWaitForRunning(ctx, oc, pod2)

			verifyPathsExist(f, pod1, "/mnt/image", config.pathsToVerify)
			verifyPathsExist(f, pod2, "/mnt/image", config.pathsToVerify)
		})

		g.Context("when subPath is used", func() {
			g.It("should handle image volume with subPath", func(ctx context.Context) {
				ref := config.getVolumeRef(ctx, oc, f.Namespace.Name)
				// Use the top-level directory of the first path as the subPath
				subPath := strings.Split(config.pathsToVerify[0], "/")[0]
				pod := buildPodWithImageVolumeSubPath(f.Namespace.Name, "", podName, ref, subPath)
				createPodAndWaitForRunning(ctx, oc, pod)
				verifyPathsExist(f, pod, "/mnt/image", trimSubPath(config.pathsToVerify, subPath))
			})

			g.It("should fail to mount image volume with invalid subPath", func(ctx context.Context) {
				ref := config.getVolumeRef(ctx, oc, f.Namespace.Name)
				pod := buildPodWithImageVolumeSubPath(f.Namespace.Name, "", podName, ref, "noexist")
				g.By("Creating a pod with image volume and subPath")
				_, err := oc.AdminKubeClient().CoreV1().Pods(f.Namespace.Name).Create(ctx, pod, metav1.CreateOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("Waiting for a pod to fail")
				err = e2epod.WaitForPodContainerToFail(ctx, oc.AdminKubeClient(), pod.Namespace, pod.Name, 0, kuberuntime.ErrCreateContainer.Error(), 5*time.Minute)
				o.Expect(err).NotTo(o.HaveOccurred())
			})
		})
	})
}

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

func verifyPathsExist(f *framework.Framework, pod *v1.Pod, mountPoint string, paths []string) {
	args := []string{"ls"}
	for _, p := range paths {
		args = append(args, path.Join(mountPoint, p))
	}
	g.By("Verifying paths exist in pod")
	stdout := e2epod.ExecCommandInContainer(f, pod.Name, pod.Spec.Containers[0].Name, args...)
	o.Expect(stdout).NotTo(o.BeEmpty())
}

// trimSubPath returns paths relative to subPath.
// Panics via gomega if any path does not start with subPath as its first component.
func trimSubPath(paths []string, subPath string) []string {
	result := make([]string, len(paths))
	for i, p := range paths {
		parts := strings.Split(p, "/")
		o.ExpectWithOffset(1, parts[0]).To(o.Equal(subPath), "path %q does not start with subPath %q", p, subPath)
		result[i] = path.Join(parts[1:]...)
	}
	return result
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

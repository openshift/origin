package cli

import (
	"context"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8simage "k8s.io/kubernetes/test/utils/image"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-cli] oc rsh", func() {
	defer g.GinkgoRecover()

	var (
		oc        = exutil.NewCLIWithPodSecurityLevel("oc-rsh", admissionapi.LevelBaseline)
		podsLabel = exutil.ParseLabelsOrDie("name=hello-busybox")
	)

	g.Describe("specific flags", func() {
		g.It("should work well when access to a remote shell", g.Label("Size:M"), func() {
			namespace := oc.Namespace()
			g.By("Creating pods with multi containers")

			_, err := oc.KubeClient().CoreV1().Pods(namespace).Create(context.Background(), newPodWithTwoContainers(), metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("expecting the pod to be running")
			pods, err := exutil.WaitForPods(oc.KubeClient().CoreV1().Pods(namespace), podsLabel, exutil.CheckPodIsRunning, 1, 4*time.Minute)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("running the rsh command without specify container name")
			out, err := oc.Run("rsh").Args(pods[0], "mkdir", "/tmp/test1").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.MatchRegexp(`Default.*container.*hello-busybox`))

			g.By("running the rsh command with specify container name and shell")
			_, err = oc.Run("rsh").Args("--container=hello-busybox-2", "--shell=/bin/sh", pods[0], "mkdir", "/tmp/test3").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})
})

func newPodWithTwoContainers() *corev1.Pod {
	return &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "doublecontainers",
			Labels: map[string]string{
				"name": "hello-busybox",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "hello-busybox",
					Image:   k8simage.GetE2EImage(k8simage.BusyBox),
					Command: []string{"/bin/sleep", "1h"},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("256Mi"),
						},
					},
					TerminationMessagePath: "/dev/termination-log-1",
					ImagePullPolicy:        corev1.PullIfNotPresent,
				},
				{
					Name:    "hello-busybox-2",
					Image:   k8simage.GetE2EImage(k8simage.BusyBox),
					Command: []string{"/bin/sleep", "1h"},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("256Mi"),
						},
					},
					TerminationMessagePath: "/dev/termination-log-2",
					ImagePullPolicy:        corev1.PullIfNotPresent,
				},
			},
			RestartPolicy: corev1.RestartPolicyAlways,
			DNSPolicy:     corev1.DNSClusterFirst,
		},
	}
}

package pods

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

var _ = Describe("[sig-node] Pod lifecycle", func() {
	oc := exutil.NewCLI("pod-lifecycle")

	It(fmt.Sprintf("should create pod %s and verify it starts", "test-pod-"+oc.Namespace()), func() {
		podName := "test-pod-" + oc.Namespace()
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: podName,
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:    "test",
						Image:   image.ShellImage(),
						Command: []string{"/bin/sh", "-c", "echo hello && sleep 10"},
					},
				},
				RestartPolicy: corev1.RestartPolicyNever,
			},
		}

		_, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Create(context.Background(), pod, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() corev1.PodPhase {
			p, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Get(context.Background(), podName, metav1.GetOptions{})
			if err != nil {
				return corev1.PodUnknown
			}
			return p.Status.Phase
		}, "60s", "5s").Should(Equal(corev1.PodSucceeded))
	})
})

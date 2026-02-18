package node

import (
	"context"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

var _ = g.Describe("[sig-node] zstd:chunked Image", func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLI("zstd-chunked-image")
	)

	g.It("should successfully run date command", func(ctx context.Context) {
		namespace := oc.Namespace()

		// Define a pod that runs the date command using a prebuilt zstd:chunked image
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "zstd-chunked-pod",
				Namespace: namespace,
			},
			Spec: corev1.PodSpec{
				RestartPolicy: corev1.RestartPolicyNever,
				Containers: []corev1.Container{
					{
						Name:    "zstd-chunked-container",
						Image:   image.LocationFor("quay.io/crio/zstd-chunked:1"),
						Command: []string{"date"},
					},
				},
			},
		}

		g.By("Creating a pod with prebuilt zstd:chunked image")
		var err error
		pod, err = oc.KubeClient().CoreV1().Pods(namespace).Create(context.Background(), pod, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for pod to complete")
		err = e2epod.WaitForPodSuccessInNamespaceTimeout(ctx, oc.KubeClient(), pod.Name, namespace, 1*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Verifying pod completed successfully")
		pod, err = oc.KubeClient().CoreV1().Pods(namespace).Get(context.Background(), pod.Name, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(pod.Status.Phase).To(o.Equal(corev1.PodSucceeded))
	})
})

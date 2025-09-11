package node

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-builds][sig-node][Feature:Builds][apigroup:build.openshift.io] zstd:chunked Image", func() {
	defer g.GinkgoRecover()
	var (
		oc                 = exutil.NewCLI("zstd-chunked-image")
		customBuildAdd     = exutil.FixturePath("testdata", "node", "zstd-chunked")
		customBuildFixture = exutil.FixturePath("testdata", "node", "zstd-chunked", "test-custom-build.yaml")
	)

	g.It("should successfully run date command", func(ctx context.Context) {
		namespace := oc.Namespace()

		g.By("creating custom builder image")
		// Build with buildah with --compression-format zstd:chunked to ensure the image is compressed with zstd:chunked.
		// https://docs.redhat.com/en/documentation/openshift_container_platform/4.18/html/builds_using_buildconfig/custom-builds-buildah#builds-build-custom-builder-image_custom-builds-buildah
		err := oc.Run("new-build").Args("--binary", "--strategy=docker", "--name=custom-builder-image").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		br, _ := exutil.StartBuildAndWait(oc, "custom-builder-image", fmt.Sprintf("--from-dir=%s", customBuildAdd))
		br.AssertSuccess()
		g.By("start custom build and build should complete")
		err = oc.AsAdmin().Run("create").Args("-f", customBuildFixture).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = oc.AsAdmin().Run("start-build").Args("sample-custom-build").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitForABuild(oc.BuildClient().BuildV1().Builds(oc.Namespace()), "sample-custom-build-1", nil, nil, nil)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Define a pod that runs the date command using the zstd-chunked image
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
						Image:   fmt.Sprintf("image-registry.openshift-image-registry.svc:5000/%s/sample-custom:latest", namespace),
						Command: []string{"date"},
					},
				},
			},
		}

		g.By("Creating a pod")
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

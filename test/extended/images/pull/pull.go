package pull

import (
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"

	kapiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	provenanceTestPod = "provenance"
)

var _ = g.Describe("[Feature:ImagePull] Image Pull", func() {
	defer g.GinkgoRecover()

	var (
		oc = exutil.NewCLI("image-provenance", exutil.KubeConfigPath())
		f  = oc.KubeFramework()
	)

	// This test will pull an unqualified image reference
	// "openshift3/image-provenance" and verify that the output
	// from the running pod does not contain the string
	// 'docker.io'.
	//
	// To make this work the same image-by-name is pushed to two
	// registries.
	//
	// The image pushed to the docker.io registry should contain:
	//
	//  FROM busybox
	//  CMD exec /bin/sh -c 'trap : TERM INT; echo "This image came from docker.io"; tail -f /dev/null & wait'
	//
	// and the image pushed to "registry.access.redhat.com" should contain:
	//
	//  FROM busybox
	//  CMD exec /bin/sh -c 'trap : TERM INT; echo "This image came from openshift.io"; tail -f /dev/null & wait'
	//
	// When we pull the unqualified image reference
	// "openshift3/image-provenance" in this test we should get
	// "This image came from OpenShift.io" as the ImagePull logic
	// no longer uses the result of calling through
	// ParseNormalizedNamed(); calls to that function add
	// "docker.io" which we don't want to happen as we rely on the
	// registry search order in the docker daemon to find and pull
	// the image based on its --add-registry search configuration.

	g.Describe("[Conformance] Validate pull image provenance of unqualified image references", func() {
		g.It("It should not pull an image from docker.io", func() {
			g.By("creating a pod from an unqualified image reference")
			image := "frobware/image-provenance" // add :good to make test pass
			pod := testPOD(image, provenanceTestPod)
			_, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Create(pod)
			o.Expect(err).NotTo(o.HaveOccurred())
			defer f.ClientSet.CoreV1().Pods(f.Namespace.Name).Delete(pod.Name, nil)

			g.By("waiting for the pod to become running")
			err = f.WaitForPodRunning(pod.Name)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("inspecting pod output")
			podOutput, logErr := oc.Run("logs").Args(provenanceTestPod).Output()
			o.Expect(logErr).NotTo(o.HaveOccurred())
			if strings.Contains(podOutput, "docker.io") {
				e2e.Failf("Pod image was pulled from docker.io; %q\n", strings.Trim(podOutput, "\n"))
			}
		})
	})
})

func testPOD(image, name string) *kapiv1.Pod {
	return &kapiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: kapiv1.PodSpec{
			Containers: []kapiv1.Container{
				{
					Name:            name,
					Image:           image,
					ImagePullPolicy: kapiv1.PullAlways,
				},
			},
		},
	}
}

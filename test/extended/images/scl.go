package images

import (
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
	kapi "k8s.io/kubernetes/pkg/api"
)

func getPodNameForTest(image string, t *tc) string {
	return fmt.Sprintf("%s-%s-%s", image, t.Version, t.BaseOS)
}

var _ = g.Describe("images: Usage and SCL enablement of the S2I images", func() {
	defer g.GinkgoRecover()
	var oc = exutil.NewCLI("s2i-usage", exutil.KubeConfigPath())
	resolveDockerImageReferences()

	for imageName, tcs := range S2IAllImages() {
		for _, t := range tcs {
			g.Describe(fmt.Sprintf("s2i usage of %q", getPodNameForTest(imageName, t)), func() {
				g.It("should provide usage", func() {
					g.By("creating a sample pod that executes the binary directly")
					pod := exutil.GetPodForContainer(kapi.Container{
						Name:  "test",
						Image: t.DockerImageReference,
					})
					oc.KubeFramework().TestContainerOutput(getPodNameForTest(imageName, t), pod, 0, []string{"Sample invocation"})
				})
			})

			g.Describe(fmt.Sprintf("rhscl enablement of %q", getPodNameForTest(imageName, t)), func() {
				g.It("should allow various invocations of binary", func() {
					g.By(fmt.Sprintf("creating a sample pod for %q that executes the binary directly", t.DockerImageReference))
					pod := exutil.GetPodForContainer(kapi.Container{
						Image:   t.DockerImageReference,
						Name:    "test",
						Command: []string{"/bin/bash", "-c", t.Cmd},
					})

					oc.KubeFramework().TestContainerOutput(getPodNameForTest(imageName, t), pod, 0, []string{t.Expected})

					g.By(fmt.Sprintf("creating a sample pod for %q", t.DockerImageReference))
					pod = exutil.GetPodForContainer(kapi.Container{
						Image:   t.DockerImageReference,
						Name:    "test",
						Command: []string{"/usr/bin/sleep", "infinity"},
					})
					_, err := oc.KubeREST().Pods(oc.Namespace()).Create(pod)
					o.Expect(err).NotTo(o.HaveOccurred())

					err = oc.KubeFramework().WaitForPodRunning(pod.Name)
					o.Expect(err).NotTo(o.HaveOccurred())

					g.By("calling the binary using 'oc exec /bin/bash -c'")
					out, err := oc.Run("exec").Args("-p", pod.Name, "--", "/bin/bash", "-c", t.Cmd).Output()
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Ω(out).Should(o.ContainSubstring(t.Expected))

					g.By("calling the binary using 'oc exec /bin/sh -ic'")
					out, err = oc.Run("exec").Args("-p", pod.Name, "--", "/bin/sh", "-ic", t.Cmd).Output()
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Ω(out).Should(o.ContainSubstring(t.Expected))
				})
			})
		}
	}
})

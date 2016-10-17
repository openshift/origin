package image_ecosystem

import (
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
	kapi "k8s.io/kubernetes/pkg/api"
)

func getPodNameForTest(image string, t tc) string {
	return fmt.Sprintf("%s-%s-%s", image, t.Version, t.BaseOS)
}

var _ = g.Describe("[image_ecosystem][Slow] openshift images should be SCL enabled", func() {
	defer g.GinkgoRecover()
	var oc = exutil.NewCLI("s2i-usage", exutil.KubeConfigPath())

	g.JustBeforeEach(func() {
		g.By("waiting for builder service account")
		err := exutil.WaitForBuilderAccount(oc.KubeREST().ServiceAccounts(oc.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	for image, tcs := range GetTestCaseForImages(AllImages) {
		for _, t := range tcs {
			g.Describe("returning s2i usage when running the image", func() {
				g.It(fmt.Sprintf("%q should print the usage", t.DockerImageReference), func() {
					g.By(fmt.Sprintf("creating a sample pod for %q", t.DockerImageReference))
					pod := exutil.GetPodForContainer(kapi.Container{
						Name:  "test",
						Image: t.DockerImageReference,
					})
					oc.KubeFramework().TestContainerOutput(getPodNameForTest(image, t), pod, 0, []string{"Sample invocation"})
				})
			})

			g.Describe("using the SCL in s2i images", func() {
				g.It(fmt.Sprintf("%q should be SCL enabled", t.DockerImageReference), func() {
					g.By(fmt.Sprintf("creating a sample pod for %q with /bin/bash -c command", t.DockerImageReference))
					pod := exutil.GetPodForContainer(kapi.Container{
						Image:   t.DockerImageReference,
						Name:    "test",
						Command: []string{"/bin/bash", "-c", t.Cmd},
					})

					oc.KubeFramework().TestContainerOutput(getPodNameForTest(image, t), pod, 0, []string{t.Expected})

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
					o.Expect(out).Should(o.ContainSubstring(t.Expected))

					g.By("calling the binary using 'oc exec /bin/sh -ic'")
					out, err = oc.Run("exec").Args("-p", pod.Name, "--", "/bin/sh", "-ic", t.Cmd).Output()
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(out).Should(o.ContainSubstring(t.Expected))
				})
			})
		}
	}
})

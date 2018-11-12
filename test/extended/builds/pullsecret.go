package builds

import (
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:Builds][Slow] using pull secrets in a build", func() {
	defer g.GinkgoRecover()
	var (
		exampleBuild = exutil.FixturePath("testdata", "builds", "test-docker-app")
		oc           = exutil.NewCLI("cli-pullsecret-build", exutil.KubeConfigPath())
	)

	g.Context("", func() {
		g.BeforeEach(func() {
			exutil.DumpDockerInfo()
		})

		g.JustBeforeEach(func() {
			g.By("waiting for builder service account")
			err := exutil.WaitForServiceAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()), "builder")
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.Context("start-build test context", func() {
			g.AfterEach(func() {
				if g.CurrentGinkgoTestDescription().Failed {
					exutil.DumpPodStates(oc)
					exutil.DumpPodLogsStartingWith("", oc)
				}
			})

			g.Describe("binary builds", func() {
				g.It("should be able to run a build that is implicitly pulling from the internal registry", func() {
					g.By("creating a build")
					err := oc.Run("new-build").Args("--binary", "--strategy=docker", "--name=docker").Execute()
					o.Expect(err).NotTo(o.HaveOccurred())
					br, err := exutil.StartBuildAndWait(oc, "docker", fmt.Sprintf("--from-dir=%s", exampleBuild))
					br.AssertSuccess()
				})
			})
		})
	})
})

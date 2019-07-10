package builds

import (
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:Builds][Conformance] custom build with buildah", func() {
	defer g.GinkgoRecover()
	var (
		oc                 = exutil.NewCLI("custom-build", exutil.KubeConfigPath())
		customBuildAdd     = exutil.FixturePath("testdata", "builds", "custom-build")
		customBuildFixture = exutil.FixturePath("testdata", "builds", "test-custom-build.yaml")
	)

	g.Context("", func() {

		g.BeforeEach(func() {
			exutil.PreTestDump()
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpConfigMapStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.Describe("being created from new-build", func() {
			g.It("should complete build with custom builder image", func() {
				g.By("create custom builder image")
				err := oc.Run("new-build").Args("--binary", "--strategy=docker", "--name=custom-builder-image").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				br, _ := exutil.StartBuildAndWait(oc, "custom-builder-image", fmt.Sprintf("--from-dir=%s", customBuildAdd))
				br.AssertSuccess()
				g.By("start custom build and build should complete")
				err = oc.AsAdmin().Run("create").Args("-f", customBuildFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = oc.AsAdmin().Run("start-build").Args("sample-custom-build").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = exutil.WaitForABuild(oc.BuildClient().BuildV1().Builds(oc.Namespace()), "sample-custom-build"+"-1", nil, nil, nil)
				o.Expect(err).NotTo(o.HaveOccurred())
			})

		})
	})
})

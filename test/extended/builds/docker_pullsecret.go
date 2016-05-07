package builds

import (
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[builds][pullsecret][Conformance] docker build using a pull secret", func() {
	defer g.GinkgoRecover()
	const (
		buildTestPod     = "build-test-pod"
		buildTestService = "build-test-svc"
	)

	var (
		buildFixture = exutil.FixturePath("fixtures", "test-docker-build-pullsecret.json")
		oc           = exutil.NewCLI("docker-build-pullsecret", exutil.KubeConfigPath())
	)

	g.JustBeforeEach(func() {
		g.By("waiting for builder service account")
		err := exutil.WaitForBuilderAccount(oc.AdminKubeREST().ServiceAccounts(oc.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.Describe("Building from a template", func() {
		g.It("should create a docker build that pulls using a secret run it", func() {
			oc.SetOutputDir(exutil.TestContext.OutputDir)

			g.By(fmt.Sprintf("calling oc create -f %q", buildFixture))
			err := oc.Run("create").Args("-f", buildFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting a test build")
			_, err = oc.Run("start-build").Args("docker-build").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("expecting the build succeeds")
			err = exutil.WaitForABuild(oc.REST().Builds(oc.Namespace()), "docker-build-1", exutil.CheckBuildSuccessFn, exutil.CheckBuildFailedFn)
			if err != nil {
				exutil.DumpBuildLogs("docker-build", oc)
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting a second build that pulls the image from the first build")
			_, err = oc.Run("start-build").Args("docker-build-pull").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("expecting the build succeeds")
			err = exutil.WaitForABuild(oc.REST().Builds(oc.Namespace()), "docker-build-pull-1", exutil.CheckBuildSuccessFn, exutil.CheckBuildFailedFn)
			if err != nil {
				exutil.DumpBuildLogs("docker-build-pull", oc)
			}
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})
})

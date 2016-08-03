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
		buildFixture = exutil.FixturePath("testdata", "test-docker-build-pullsecret.json")
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

			g.By("starting a build")
			br, err := exutil.StartBuildAndWait(oc, "docker-build")
			o.Expect(err).NotTo(o.HaveOccurred())
			br.AssertSuccess()

			g.By("starting a second build that pulls the image from the first build")
			br, err = exutil.StartBuildAndWait(oc, "docker-build-pull")
			o.Expect(err).NotTo(o.HaveOccurred())
			br.AssertSuccess()

		})
	})
})

package builds

import (
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[builds] build with empty source", func() {
	defer g.GinkgoRecover()
	var (
		buildFixture = exutil.FixturePath("testdata", "test-nosrc-build.json")
		oc           = exutil.NewCLI("cli-build-nosrc", exutil.KubeConfigPath())
		exampleBuild = exutil.FixturePath("testdata", "test-build-app")
	)

	g.JustBeforeEach(func() {
		g.By("waiting for builder service account")
		err := exutil.WaitForBuilderAccount(oc.KubeREST().ServiceAccounts(oc.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())
		oc.Run("create").Args("-f", buildFixture).Execute()
	})

	g.Describe("started build", func() {
		g.It("should build even with an empty source in build config", func() {
			g.By("starting the empty source build")
			br, err := exutil.StartBuildAndWait(oc, "nosrc-build", fmt.Sprintf("--from-dir=%s", exampleBuild))
			br.AssertSuccess()

			g.By(fmt.Sprintf("verifying the status of %q", br.BuildPath))
			build, err := oc.REST().Builds(oc.Namespace()).Get(br.Build.Name)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(build.Spec.Source.Dockerfile).To(o.BeNil())
			o.Expect(build.Spec.Source.Git).To(o.BeNil())
			o.Expect(build.Spec.Source.Images).To(o.BeNil())
			o.Expect(build.Spec.Source.Binary).NotTo(o.BeNil())
		})
	})
})

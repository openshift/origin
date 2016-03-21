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
		buildFixture = exutil.FixturePath("..", "extended", "fixtures", "test-nosrc-build.json")
		oc           = exutil.NewCLI("cli-build-nosrc", exutil.KubeConfigPath())
		exampleBuild = exutil.FixturePath("..", "extended", "fixtures", "test-build-app")
	)

	g.JustBeforeEach(func() {
		g.By("waiting for builder service account")
		err := exutil.WaitForBuilderAccount(oc.KubeREST().ServiceAccounts(oc.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())
		oc.Run("create").Args("-f", buildFixture).Execute()
	})

	g.Describe("started build", func() {
		g.It("should build even with an empty source in build config", func() {
			g.By("starting the build with --wait flag")
			out, err := oc.Run("start-build").Args("nosrc-build", "--wait", fmt.Sprintf("--from-dir=%s", exampleBuild)).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying build success")
			err = exutil.WaitForABuild(oc.REST().Builds(oc.Namespace()), "nosrc-build-1", exutil.CheckBuildSuccessFn, exutil.CheckBuildFailedFn)

			g.By(fmt.Sprintf("verifying the build %q status", out))
			build, err := oc.REST().Builds(oc.Namespace()).Get("nosrc-build-1")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(build.Spec.Source.Dockerfile).To(o.BeNil())
			o.Expect(build.Spec.Source.Git).To(o.BeNil())
			o.Expect(build.Spec.Source.Images).To(o.BeNil())
			o.Expect(build.Spec.Source.Binary).NotTo(o.BeNil())
		})
	})
})

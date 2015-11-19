package builds

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	buildapi "github.com/openshift/origin/pkg/build/api"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("builds: parallel: s2i build with proxy", func() {
	defer g.GinkgoRecover()
	var (
		buildFixture = exutil.FixturePath("..", "extended", "fixtures", "test-build-proxy.json")
		oc           = exutil.NewCLI("build-proxy", exutil.KubeConfigPath())
	)

	g.JustBeforeEach(func() {
		g.By("waiting for builder service account")
		err := exutil.WaitForBuilderAccount(oc.KubeREST().ServiceAccounts(oc.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())
		oc.Run("create").Args("-f", buildFixture).Execute()
	})

	g.Describe("start build with broken proxy", func() {
		g.It("should start a build and wait for the build to to fail", func() {
			g.By("starting the build with --wait and --follow flags")
			out, err := oc.Run("start-build").Args("sample-build", "--follow", "--wait").Output()
			o.Expect(err).To(o.HaveOccurred())
			g.By("verifying the build sample-app-1 output")
			// The git ls-remote check should exit the build when the remote
			// repository is not accessible. It should never get to the clone.
			o.Expect(out).NotTo(o.ContainSubstring("clone"))
			o.Expect(out).To(o.ContainSubstring(`unable to access 'https://github.com/openshift/ruby-hello-world.git/': Failed connect to 127.0.0.1:3128`))
			g.By("verifying the build sample-build-1 status")
			build, err := oc.REST().Builds(oc.Namespace()).Get("sample-build-1")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(build.Status.Phase).Should(o.BeEquivalentTo(buildapi.BuildPhaseFailed))
		})

	})

})

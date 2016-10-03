package builds

import (
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	buildapi "github.com/openshift/origin/pkg/build/api"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[builds][Slow] the s2i build should support proxies", func() {
	defer g.GinkgoRecover()
	var (
		buildFixture = exutil.FixturePath("testdata", "test-build-proxy.json")
		oc           = exutil.NewCLI("build-proxy", exutil.KubeConfigPath())
	)

	g.JustBeforeEach(func() {
		g.By("waiting for builder service account")
		err := exutil.WaitForBuilderAccount(oc.KubeREST().ServiceAccounts(oc.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())
		oc.Run("create").Args("-f", buildFixture).Execute()
	})

	g.Describe("start build with broken proxy", func() {
		g.It("should start a build and wait for the build to fail", func() {
			g.By("starting the build")

			br, _ := exutil.StartBuildAndWait(oc, "sample-build")
			br.AssertFailure()

			g.By("verifying the build sample-build-1 output")
			// The git ls-remote check should exit the build when the remote
			// repository is not accessible. It should never get to the clone.
			buildLog, err := br.Logs()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(buildLog).NotTo(o.ContainSubstring("clone"))
			if !strings.Contains(buildLog, `unable to access 'https://github.com/openshift/ruby-hello-world.git/': Failed connect to 127.0.0.1:3128`) {
				fmt.Fprintf(g.GinkgoWriter, "\nbuild log:\n%s\n", buildLog)
			}
			o.Expect(buildLog).To(o.ContainSubstring(`unable to access 'https://github.com/openshift/ruby-hello-world.git/': Failed connect to 127.0.0.1:3128`))

			g.By("verifying the build sample-build-1 status")
			o.Expect(br.Build.Status.Phase).Should(o.BeEquivalentTo(buildapi.BuildPhaseFailed))
		})
	})

	g.Describe("start build with broken proxy and a no_proxy override", func() {
		g.It("should start a build and wait for the build to succeed", func() {
			g.By("starting the build")
			br, _ := exutil.StartBuildAndWait(oc, "sample-build-noproxy")
			br.AssertSuccess()
		})

	})

})

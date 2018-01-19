package builds

import (
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:Builds][Slow] the s2i build should support proxies", func() {
	defer g.GinkgoRecover()
	var (
		buildFixture = exutil.FixturePath("testdata", "builds", "test-build-proxy.yaml")
		oc           = exutil.NewCLI("build-proxy", exutil.KubeConfigPath())
	)

	g.Context("", func() {

		g.BeforeEach(func() {
			exutil.DumpDockerInfo()
		})

		g.JustBeforeEach(func() {
			g.By("waiting for builder service account")
			err := exutil.WaitForBuilderAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()))
			o.Expect(err).NotTo(o.HaveOccurred())
			oc.Run("create").Args("-f", buildFixture).Execute()
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.Describe("start build with broken proxy", func() {
			g.It("should start a build and wait for the build to fail", func() {
				g.By("starting the build")

				br, _ := exutil.StartBuildAndWait(oc, "sample-build", "--build-loglevel=5")
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
			g.It("should start an s2i build and wait for the build to succeed", func() {
				g.By("starting the build")
				br, _ := exutil.StartBuildAndWait(oc, "sample-s2i-build-noproxy", "--build-loglevel=5")
				br.AssertSuccess()
				buildLog, err := br.Logs()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(buildLog).NotTo(o.ContainSubstring("gituser:password"), "build log should not include proxy credentials")
				o.Expect(buildLog).NotTo(o.ContainSubstring("envuser:password"), "build log should not include proxy credentials")
				o.Expect(buildLog).To(o.ContainSubstring("proxy1"), "build log should include proxy host")
				o.Expect(buildLog).To(o.ContainSubstring("proxy2"), "build log should include proxy host")
				o.Expect(buildLog).To(o.ContainSubstring("proxy3"), "build log should include proxy host")
				o.Expect(buildLog).To(o.ContainSubstring("proxy4"), "build log should include proxy host")
			})
			g.It("should start a docker build and wait for the build to succeed", func() {
				g.By("starting the build")
				br, _ := exutil.StartBuildAndWait(oc, "sample-docker-build-noproxy", "--build-loglevel=5")
				br.AssertSuccess()
				buildLog, err := br.Logs()
				o.Expect(err).NotTo(o.HaveOccurred())
				// envuser:password will appear in the log because the full/unstripped env HTTP_PROXY variable is injected
				// into the dockerfile and displayed by docker build.
				o.Expect(buildLog).NotTo(o.ContainSubstring("gituser:password"), "build log should not include proxy credentials")
				o.Expect(buildLog).To(o.ContainSubstring("proxy1"), "build log should include proxy host")
				o.Expect(buildLog).To(o.ContainSubstring("proxy2"), "build log should include proxy host")
				o.Expect(buildLog).To(o.ContainSubstring("proxy3"), "build log should include proxy host")
				o.Expect(buildLog).To(o.ContainSubstring("proxy4"), "build log should include proxy host")
			})
		})
	})
})

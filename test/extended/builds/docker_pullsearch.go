package builds

import (
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	"k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-builds][Feature:Builds][pullsearch] docker build where the registry is not specified", func() {
	defer g.GinkgoRecover()
	var (
		buildFixture = exutil.FixturePath("testdata", "builds", "test-build-search-registries.yaml")
		oc           = exutil.NewCLIWithPodSecurityLevel("docker-build-pullsearch", api.LevelPrivileged)
	)

	g.Context("", func() {

		g.BeforeEach(func() {
			exutil.PreTestDump()
		})

		g.AfterEach(func() {
			if g.CurrentSpecReport().Failed() {
				exutil.DumpPodStates(oc)
				exutil.DumpConfigMapStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.Describe("Building from a Dockerfile whose FROM image ref does not specify the image registry", func() {
			testDockerBuild := func(logsMustMatchRegexp string, startBuildAddArgs ...string) {
				g.By("creating a BuildConfig whose base image does not have a fully qualified registry name")
				err := oc.Run("create").Args("-f", buildFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("starting a build")
				br, err := exutil.StartBuildAndWait(oc, append([]string{"ubi"}, startBuildAddArgs...)...)
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertSuccess()

				if logsMustMatchRegexp != "" {
					g.By(fmt.Sprintf("verify that the build log included a message that matched %q", logsMustMatchRegexp))
					buildPodLogs, err := br.Logs()
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(buildPodLogs).To(o.MatchRegexp(logsMustMatchRegexp))
				}
			}
			g.It("should create a docker build that can search from our predefined list of image registries and succeed [apigroup:build.openshift.io]", func() {
				testDockerBuild(buildInDefaultUserNSRegexp)
			})

			g.It("should create an unprivileged docker build that can still search our predefined list of image registries and shortnames and succeed [apigroup:build.openshift.io]", func() {
				testDockerBuild(buildInUserNSRegexp, "--env", buildInUserNSEnvVar)
			})
		})
	})
})

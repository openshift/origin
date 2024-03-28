package builds

import (
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	// failedToReadNodeCredsRegexp is the message that we expect the
	// builder to log when it can't read the node's pull secrets
	failedToReadNodeCredsRegexp = `proceeding without node credentials: open /var/lib/kubelet/config.json: permission denied`
	// failedToPullWithoutCredsRegexp is the message that we expect the
	// builder to log when it can't pull an image because it doesn't have
	// the right credentials to access the registry
	failedToPullWithoutCredsRegexp = `Error pulling image ".*": initializing source .*: unable to retrieve auth token: invalid username/password: unauthorized:`
)

var _ = g.Describe("[sig-builds][Feature:Builds] custom build with buildah", func() {
	defer g.GinkgoRecover()
	var (
		oc                 = exutil.NewCLIWithPodSecurityLevel("custom-build", admissionapi.LevelBaseline)
		customBuildAdd     = exutil.FixturePath("testdata", "builds", "custom-build")
		customBuildAddAnon = exutil.FixturePath("testdata", "builds", "custom-build-anon")
		customBuildFixture = exutil.FixturePath("testdata", "builds", "test-custom-build.yaml")
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

		g.Describe("being created from new-build", func() {
			testCustomBuild := func(customBuilderImageDir, logsMustMatchRegexp, dockerPatch, customPatch string) {
				g.By("create custom builder image")
				err := oc.Run("new-build").Args("--binary", "--strategy=docker", "--name=custom-builder-image").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				if dockerPatch != "" {
					g.By("applying patch to build config for creating custom builder image")
					err := oc.Run("patch").Args("bc/custom-builder-image", "-p", dockerPatch).Execute()
					o.Expect(err).NotTo(o.HaveOccurred())
				}

				br, err := exutil.StartBuildAndWait(oc, "custom-builder-image", fmt.Sprintf("--from-dir=%s", customBuilderImageDir))
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertSuccess()
				if logsMustMatchRegexp != "" {
					g.By(fmt.Sprintf("verify that the build log for creating the custom builder image included a message that matched %q", logsMustMatchRegexp))
					buildPodLogs, err := br.Logs()
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(buildPodLogs).To(o.MatchRegexp(logsMustMatchRegexp))
				}

				g.By("create build config using custom builder image")
				err = oc.AsAdmin().Run("create").Args("-f", customBuildFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				if customPatch != "" {
					g.By("applying patch to build config using custom builder image")
					err = oc.AsAdmin().Run("patch").Args("bc/sample-custom-build", "-p", customPatch).Execute()
					o.Expect(err).NotTo(o.HaveOccurred())
				}

				g.By("start custom build and build should complete")
				br, err = exutil.StartBuildAndWait(oc.AsAdmin(), "sample-custom-build")

				o.Expect(err).NotTo(o.HaveOccurred())

				if logsMustMatchRegexp != "" {
					g.By(fmt.Sprintf("verify that the build log included a message that matched %q", logsMustMatchRegexp))
					buildPodLogs, err := br.Logs()
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(buildPodLogs).To(o.MatchRegexp(logsMustMatchRegexp))
				}
			}

			g.It("should complete build with custom builder image built from base image pulled using node secrets [apigroup:build.openshift.io]", func() {
				testCustomBuild(customBuildAdd, buildInDefaultUserNSRegexp, buildInDefaultUserNSPatch("dockerStrategy", 2), buildInDefaultUserNSPatch("customStrategy", 2))
			})
			g.It("should complete unprivileged build with custom builder image built from base image pulled anonymously [apigroup:build.openshift.io]", func() {
				testCustomBuild(customBuildAddAnon, buildInUserNSRegexp, buildInUserNSPatch("dockerStrategy", 2), buildInUserNSPatch("customStrategy", 2))
			})
			g.It("should fail to build the custom builder image without node credentials [apigroup:build.openshift.io]", func() {
				g.By("create custom builder image")
				err := oc.Run("new-build").Args("--binary", "--strategy=docker", "--name=custom-builder-image", "--env", buildInUserNSEnvVar, "--env", "BUILD_LOGLEVEL=2").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				br, err := exutil.StartBuildAndWait(oc, "custom-builder-image", fmt.Sprintf("--from-dir=%s", customBuildAdd))
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertFailure()
				logsMustMatchRegexps := []string{
					buildInUserNSRegexp,
					failedToReadNodeCredsRegexp,
					failedToPullWithoutCredsRegexp,
				}
				for _, logsMustMatchRegexp := range logsMustMatchRegexps {
					g.By(fmt.Sprintf("verify that the build log for failing to create the custom builder image included a message that matched %q", logsMustMatchRegexp))
					buildPodLogs, err := br.Logs()
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(buildPodLogs).To(o.MatchRegexp(logsMustMatchRegexp))
				}
			})
		})
	})
})

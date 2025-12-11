package builds

import (
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
			g.It("should create a docker build that has buildah search from our predefined list of image registries and succeed [apigroup:build.openshift.io]", g.Label("Size:L"), func() {

				g.By("creating a BuildConfig whose base image does not have a fully qualified registry name")
				err := oc.Run("create").Args("-f", buildFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("starting a build")
				br, err := exutil.StartBuildAndWait(oc, "ubi")
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertSuccess()

			})
		})
	})
})

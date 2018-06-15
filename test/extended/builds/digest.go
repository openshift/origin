package builds

import (
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("[Feature:Builds][Slow] completed builds should have digest of the image in their status", func() {
	defer g.GinkgoRecover()
	var (
		imageStreamFixture = exutil.FixturePath("..", "integration", "testdata", "test-image-stream.json")
		stiBuildFixture    = exutil.FixturePath("testdata", "builds", "test-s2i-build.json")
		dockerBuildFixture = exutil.FixturePath("testdata", "builds", "test-docker-build.json")
		oc                 = exutil.NewCLI("build-sti-labels", exutil.KubeConfigPath())
	)

	g.Context("", func() {

		g.BeforeEach(func() {
			exutil.DumpDockerInfo()
			g.By("waiting for builder service account")
			err := exutil.WaitForBuilderAccount(oc.AdminKubeClient().Core().ServiceAccounts(oc.Namespace()))
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("creating test imagestream")
			err = oc.Run("create").Args("-f", imageStreamFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.Describe("S2I build", func() {
			g.Describe("started with normal log level", func() {
				testBuildDigest(oc, stiBuildFixture, 0)
			})

			g.Describe("started with log level >5", func() {
				testBuildDigest(oc, stiBuildFixture, 7)
			})
		})

		g.Describe("Docker build", func() {
			g.Describe("started with normal log level", func() {
				testBuildDigest(oc, dockerBuildFixture, 0)
			})

			g.Describe("started with log level >5", func() {
				testBuildDigest(oc, dockerBuildFixture, 7)
			})
		})
	})
})

func testBuildDigest(oc *exutil.CLI, buildFixture string, buildLogLevel uint) {
	g.It(fmt.Sprintf("should save the image digest when finished"), func() {
		g.By("creating test build")
		err := oc.Run("create").Args("-f", buildFixture).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		logLevelArg := fmt.Sprintf("--build-loglevel=%d", buildLogLevel)
		g.By("starting a test build")
		br, err := exutil.StartBuildAndWait(oc, "test", logLevelArg)
		o.Expect(err).NotTo(o.HaveOccurred())
		br.AssertSuccess()

		g.By("checking that the image digest has been saved to the build status")
		o.Expect(br.Build.Status.Output.To).NotTo(o.BeNil())

		ist, err := oc.ImageClient().Image().ImageStreamTags(oc.Namespace()).Get("test:latest", v1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(br.Build.Status.Output.To.ImageDigest).To(o.Equal(ist.Image.Name))
	})
}

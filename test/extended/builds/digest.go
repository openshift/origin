package builds

import (
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[builds][Slow] completed builds should have digest of the image in their status", func() {
	defer g.GinkgoRecover()
	var (
		imageStreamFixture = exutil.FixturePath("..", "integration", "testdata", "test-image-stream.json")
		stiBuildFixture    = exutil.FixturePath("testdata", "test-s2i-build.json")
		dockerBuildFixture = exutil.FixturePath("testdata", "test-docker-build.json")
		oc                 = exutil.NewCLI("build-sti-labels", exutil.KubeConfigPath())
	)

	g.BeforeEach(func() {
		g.By("waiting for builder service account")
		err := exutil.WaitForBuilderAccount(oc.AdminKubeClient().Core().ServiceAccounts(oc.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("creating test imagestream")
		err = oc.Run("create").Args("-f", imageStreamFixture).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
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

func testBuildDigest(oc *exutil.CLI, buildFixture string, buildLogLevel uint) {
	g.It(fmt.Sprintf("should save the image digest when finished"), func() {
		g.By("creating test build")
		err := oc.Run("create").Args("-f", buildFixture).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		logLevelArg := fmt.Sprintf("--build-loglevel=%d", buildLogLevel)
		g.By("starting a test build")
		br, err := exutil.StartBuildAndWait(oc, "test", logLevelArg)

		g.By("checking that the image digest has been saved to the build status")
		o.Expect(br.Build.Status.Output.To).NotTo(o.BeNil())

		ist, err := oc.Client().ImageStreamTags(oc.Namespace()).Get("test", "latest")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(br.Build.Status.Output.To.ImageDigest).To(o.Equal(ist.Image.Name))
	})
}

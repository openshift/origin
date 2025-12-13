package builds

import (
	"context"
	"fmt"

	manifestschema2 "github.com/distribution/distribution/v3/manifest/schema2"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-builds][Feature:Builds][Slow] completed builds should have digest of the image in their status", func() {
	defer g.GinkgoRecover()
	var (
		imageStreamFixture = exutil.FixturePath("testdata", "builds", "test-image-stream.json")
		stiBuildFixture    = exutil.FixturePath("testdata", "builds", "test-s2i-build.json")
		dockerBuildFixture = exutil.FixturePath("testdata", "builds", "test-docker-build.json")
		oc                 = exutil.NewCLI("build-sti-labels")
	)

	g.Context("", func() {

		g.BeforeEach(func() {
			exutil.PreTestDump()

			g.By("creating test imagestream")
			err := oc.Run("create").Args("-f", imageStreamFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.AfterEach(func() {
			if g.CurrentSpecReport().Failed() {
				exutil.DumpPodStates(oc)
				exutil.DumpConfigMapStates(oc)
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
	g.It(fmt.Sprintf("should save the image digest when finished [apigroup:build.openshift.io][apigroup:image.openshift.io]"), g.Label("Size:L"), func() {
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

		ist, err := oc.ImageClient().ImageV1().ImageStreamTags(oc.Namespace()).Get(context.Background(), "test:latest", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(br.Build.Status.Output.To.ImageDigest).To(o.Equal(ist.Image.Name))

		g.By("checking that the image layers have valid docker v2schema2 MIME types")
		image, err := oc.AdminImageClient().ImageV1().Images().Get(context.Background(), br.Build.Status.Output.To.ImageDigest, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		framework.Logf("media type for image %s: %s", image.Name, image.DockerImageManifestMediaType)
		for _, layer := range image.DockerImageLayers {
			framework.Logf("checking MIME type for layer %s", layer.Name)
			o.Expect(layer.MediaType).To(o.Equal(manifestschema2.MediaTypeLayer))
		}
	})
}

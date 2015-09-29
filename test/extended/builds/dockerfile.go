package builds

import (
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("builds: parallel: Dockerfile input", func() {
	defer g.GinkgoRecover()
	var (
		imageStreamFixture = exutil.FixturePath("..", "integration", "fixtures", "test-image-stream.json")
		oc                 = exutil.NewCLI("build-dockerfile-env", exutil.KubeConfigPath())
		testDockerfile     = `
FROM openshift/origin-base
USER 1001
`
	)

	g.JustBeforeEach(func() {
		g.By("waiting for builder service account")
		err := exutil.WaitForBuilderAccount(oc.KubeREST().ServiceAccounts(oc.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.Describe("being created from new-app", func() {
		g.It("should create a image via new-build", func() {
			oc.SetOutputDir(exutil.TestContext.OutputDir)

			g.By(fmt.Sprintf("calling oc create -f %q", imageStreamFixture))
			err := oc.Run("create").Args("-f", imageStreamFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("calling oc new-app with Dockerfile"))
			err = oc.Run("new-build").Args("-D", "-").InputString(testDockerfile).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting a test build")
			bc, err := oc.REST().BuildConfigs(oc.Namespace()).Get("origin-base")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(bc.Spec.Source.Git).To(o.BeNil())
			o.Expect(bc.Spec.Source.Dockerfile).NotTo(o.BeNil())
			o.Expect(*bc.Spec.Source.Dockerfile).To(o.Equal(testDockerfile))
			buildName := "origin-base-1"

			g.By("expecting the Dockerfile build is in Complete phase")
			err = exutil.WaitForABuild(oc.REST().Builds(oc.Namespace()), buildName, exutil.CheckBuildSuccessFunc, exutil.CheckBuildFailedFunc)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("getting the Docker image reference from ImageStream")
			image, err := oc.REST().ImageStreamTags(oc.Namespace()).Get("test", "latest")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(image.Image.DockerImageMetadata.Config.User).To(o.Equal("1001"))
		})
	})
})

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
		oc             = exutil.NewCLI("build-dockerfile-env", exutil.KubeConfigPath())
		testDockerfile = `
FROM openshift/origin-base
USER 1001
`
		testDockerfile2 = `
FROM centos:7
RUN yum install -y httpd
USER 1001
`
	)

	g.JustBeforeEach(func() {
		g.By("waiting for builder service account")
		err := exutil.WaitForBuilderAccount(oc.KubeREST().ServiceAccounts(oc.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())
		oc.SetOutputDir(exutil.TestContext.OutputDir)
	})

	g.Describe("being created from new-build", func() {
		g.It("should create a image via new-build", func() {
			g.By(fmt.Sprintf("calling oc new-build with Dockerfile"))
			err := oc.Run("new-build").Args("-D", "-").InputString(testDockerfile).Execute()
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

			g.By("getting the build Docker image reference from ImageStream")
			image, err := oc.REST().ImageStreamTags(oc.Namespace()).Get("origin-base", "latest")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(image.Image.DockerImageMetadata.Config.User).To(o.Equal("1001"))
		})

		g.It("should create a image via new-build and infer the origin tag", func() {
			g.By(fmt.Sprintf("calling oc new-build with Dockerfile that uses the same tag as the output"))
			err := oc.Run("new-build").Args("-D", "-").InputString(testDockerfile2).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting a test build")
			bc, err := oc.REST().BuildConfigs(oc.Namespace()).Get("centos")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(bc.Spec.Source.Git).To(o.BeNil())
			o.Expect(bc.Spec.Source.Dockerfile).NotTo(o.BeNil())
			o.Expect(*bc.Spec.Source.Dockerfile).To(o.Equal(testDockerfile2))
			o.Expect(bc.Spec.Output.To).ToNot(o.BeNil())
			o.Expect(bc.Spec.Output.To.Name).To(o.Equal("centos:latest"))

			buildName := "centos-1"
			g.By("expecting the Dockerfile build is in Complete phase")
			err = exutil.WaitForABuild(oc.REST().Builds(oc.Namespace()), buildName, exutil.CheckBuildSuccessFunc, exutil.CheckBuildFailedFunc)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("getting the built Docker image reference from ImageStream")
			image, err := oc.REST().ImageStreamTags(oc.Namespace()).Get("centos", "latest")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(image.Image.DockerImageMetadata.Config.User).To(o.Equal("1001"))

			g.By("checking for the imported tag")
			_, err = oc.REST().ImageStreamTags(oc.Namespace()).Get("centos", "7")
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})
})

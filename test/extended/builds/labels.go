package builds

import (
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	eximages "github.com/openshift/origin/test/extended/images"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("default: Check S2I and Docker build image for proper Docker labels", func() {
	defer g.GinkgoRecover()
	var (
		imageStreamFixture = exutil.FixturePath("..", "integration", "fixtures", "test-image-stream.json")
		stiBuildFixture    = exutil.FixturePath("fixtures", "test-sti-build.json")
		dockerBuildFixture = exutil.FixturePath("fixtures", "test-docker-build.json")
		oc                 = exutil.NewCLI("build-sti-env", exutil.KubeConfigPath())
	)

	g.JustBeforeEach(func() {
		g.By("waiting for builder service account")
		err := exutil.WaitForBuilderAccount(oc.KubeREST().ServiceAccounts(oc.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.Describe("S2I build from a template", func() {
		g.It(fmt.Sprintf("should create a image from %q template with proper Docker labels", stiBuildFixture), func() {
			oc.SetOutputDir(exutil.TestContext.OutputDir)

			g.By(fmt.Sprintf("calling oc create -f %q", imageStreamFixture))
			err := oc.Run("create").Args("-f", imageStreamFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("calling oc create -f %q", stiBuildFixture))
			err = oc.Run("create").Args("-f", stiBuildFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting a test build")
			buildName, err := oc.Run("start-build").Args("test").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("o.Expecting the S2I build is in Complete phase")
			err = exutil.WaitForABuild(oc.REST().Builds(oc.Namespace()), buildName, exutil.CheckBuildSuccessFunc, exutil.CheckBuildFailedFunc)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("getting the Docker image reference from ImageStream")
			imageRef, err := exutil.GetDockerImageReference(oc.REST().ImageStreams(oc.Namespace()), "test", "latest")
			o.Expect(err).NotTo(o.HaveOccurred())

			imageLabels, err := eximages.GetImageLabels(oc.REST().ImageStreamImages(oc.Namespace()), "test", imageRef)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("inspecting the new image for proper Docker labels")
			err = ExpectOpenShiftLabels(imageLabels)
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})

	g.Describe("Docker build from a template", func() {
		g.It(fmt.Sprintf("should create a image from %q template with proper Docker labels", dockerBuildFixture), func() {
			oc.SetOutputDir(exutil.TestContext.OutputDir)

			g.By(fmt.Sprintf("calling oc create -f %q", imageStreamFixture))
			err := oc.Run("create").Args("-f", imageStreamFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("calling oc create -f %q", dockerBuildFixture))
			err = oc.Run("create").Args("-f", dockerBuildFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting a test build")
			buildName, err := oc.Run("start-build").Args("test").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("o.Expecting the Docker build is in Complete phase")
			err = exutil.WaitForABuild(oc.REST().Builds(oc.Namespace()), buildName, exutil.CheckBuildSuccessFunc, exutil.CheckBuildFailedFunc)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("getting the Docker image reference from ImageStream")
			imageRef, err := exutil.GetDockerImageReference(oc.REST().ImageStreams(oc.Namespace()), "test", "latest")
			o.Expect(err).NotTo(o.HaveOccurred())

			imageLabels, err := eximages.GetImageLabels(oc.REST().ImageStreamImages(oc.Namespace()), "test", imageRef)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("inspecting the new image for proper Docker labels")
			err = ExpectOpenShiftLabels(imageLabels)
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})
})

// ExpectOpenShiftLabels tests if builded Docker image contains appropriate
// labels.
func ExpectOpenShiftLabels(labels map[string]string) error {
	ExpectedLabels := []string{
		"io.openshift.build.commit.author",
		"io.openshift.build.commit.date",
		"io.openshift.build.commit.id",
		"io.openshift.build.commit.ref",
		"io.openshift.build.commit.message",
		"io.openshift.build.source-location",
		"io.openshift.build.source-context-dir",
	}

	for _, label := range ExpectedLabels {
		if labels[label] == "" {
			return fmt.Errorf("Builded image doesn't contain proper Docker image labels. Missing %q label", label)
		}
	}

	return nil
}

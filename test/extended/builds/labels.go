package builds

import (
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	eximages "github.com/openshift/origin/test/extended/images"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:Builds][Slow] result image should have proper labels set", func() {
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
		})

		g.JustBeforeEach(func() {
			g.By("waiting for builder service account")
			err := exutil.WaitForBuilderAccount(oc.AdminKubeClient().Core().ServiceAccounts(oc.Namespace()))
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
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
				br, err := exutil.StartBuildAndWait(oc, "test")
				br.AssertSuccess()

				g.By("getting the Docker image reference from ImageStream")
				imageRef, err := exutil.GetDockerImageReference(oc.ImageClient().Image().ImageStreams(oc.Namespace()), "test", "latest")
				o.Expect(err).NotTo(o.HaveOccurred())

				imageLabels, err := eximages.GetImageLabels(oc.ImageClient().Image().ImageStreamImages(oc.Namespace()), "test", imageRef)
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
				br, err := exutil.StartBuildAndWait(oc, "test")
				br.AssertSuccess()

				g.By("getting the Docker image reference from ImageStream")
				imageRef, err := exutil.GetDockerImageReference(oc.ImageClient().Image().ImageStreams(oc.Namespace()), "test", "latest")
				o.Expect(err).NotTo(o.HaveOccurred())

				imageLabels, err := eximages.GetImageLabels(oc.ImageClient().Image().ImageStreamImages(oc.Namespace()), "test", imageRef)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("inspecting the new image for proper Docker labels")
				err = ExpectOpenShiftLabels(imageLabels)
				o.Expect(err).NotTo(o.HaveOccurred())
			})
		})
	})
})

// ExpectOpenShiftLabels tests if built Docker image contains appropriate
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
		"user-specified-label",
	}

	for _, label := range ExpectedLabels {
		if labels[label] == "" {
			return fmt.Errorf("Built image doesn't contain proper Docker image labels. Missing %q label", label)
		}
	}
	if labels["io.k8s.display-name"] != "overridden" {
		return fmt.Errorf("Existing label was not overridden with user specified value: %s=%s", labels["io.k8s.display-name"], labels["overridden"])
	}
	if labels["io.openshift.builder-version"] != "overridden2" {
		return fmt.Errorf("System generated label was not overridden with user specified value: %s=%s", labels["io.openshift.builder-version"], labels["overridden2"])
	}
	return nil
}

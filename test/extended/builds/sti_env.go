package builds

import (
	"fmt"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "k8s.io/kubernetes/test/e2e"

	buildapi "github.com/openshift/origin/pkg/build/api"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = Describe("default: STI build with .sti/environment file", func() {
	defer GinkgoRecover()
	var (
		imageStreamFixture = exutil.FixturePath("..", "integration", "fixtures", "test-image-stream.json")
		stiEnvBuildFixture = exutil.FixturePath("fixtures", "test-env-build.json")
		oc                 = exutil.NewCLI("build-sti-env", exutil.KubeConfigPath())
	)

	Describe("Building from a template", func() {
		It(fmt.Sprintf("should create a image from %q template and run it in a pod", stiEnvBuildFixture), func() {
			oc.SetOutputDir(exutil.TestContext.OutputDir)

			By(fmt.Sprintf("calling oc create -f %q", imageStreamFixture))
			err := oc.Run("create").Args("-f", imageStreamFixture).Execute()
			Expect(err).NotTo(HaveOccurred())

			By(fmt.Sprintf("calling oc create -f %q", stiEnvBuildFixture))
			err = oc.Run("create").Args("-f", stiEnvBuildFixture).Execute()
			Expect(err).NotTo(HaveOccurred())

			By("starting a test build")
			buildName, err := oc.Run("start-build").Args("test").Output()
			Expect(err).NotTo(HaveOccurred())

			By("expecting the build is in Complete phase")
			err = exutil.WaitForABuild(oc.REST().Builds(oc.Namespace()), buildName,
				// The build passed
				func(b *buildapi.Build) bool {
					return b.Name == buildName && b.Status.Phase == buildapi.BuildPhaseComplete
				},
				// The build failed
				func(b *buildapi.Build) bool {
					if b.Name != buildName {
						return false
					}
					return b.Status.Phase == buildapi.BuildPhaseFailed || b.Status.Phase == buildapi.BuildPhaseError
				},
			)
			Expect(err).NotTo(HaveOccurred())

			By("getting the Docker image reference from ImageStream")
			imageName, err := exutil.GetDockerImageReference(oc.REST().ImageStreams(oc.Namespace()), "test", "latest")
			Expect(err).NotTo(HaveOccurred())

			By("writing the pod defintion to a file")
			outputPath := filepath.Join(exutil.TestContext.OutputDir, oc.Namespace()+"-sample-pod.json")
			pod := exutil.CreatePodForImage(imageName)
			err = exutil.WriteObjectToFile(pod, outputPath)
			Expect(err).NotTo(HaveOccurred())

			By(fmt.Sprintf("calling oc create -f %q", outputPath))
			err = oc.Run("create").Args("-f", outputPath).Execute()
			Expect(err).NotTo(HaveOccurred())

			By("expecting the pod to be running")
			err = oc.KubeFramework().WaitForPodRunning(pod.Name)
			Expect(err).NotTo(HaveOccurred())

			By("expecting the pod container has TEST_ENV variable set")
			out, err := oc.Run("exec").Args("-p", pod.Name, "--", "curl", "http://0.0.0.0:8080").Output()
			Expect(err).NotTo(HaveOccurred())

			if !strings.Contains(out, "success") {
				Failf("Pod %q contains does not contain expected variable: %q", pod.Name, out)
			}
		})
	})
})

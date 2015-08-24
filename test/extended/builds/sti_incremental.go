package builds

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"k8s.io/kubernetes/test/e2e"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("default: S2I incremental build with push and pull to authenticated registry", func() {
	defer g.GinkgoRecover()
	var (
		templateFixture = exutil.FixturePath("fixtures", "incremental-auth-build.json")
		oc              = exutil.NewCLI("build-sti-env", exutil.KubeConfigPath())
	)

	g.JustBeforeEach(func() {
		g.By("waiting for builder service account")
		err := exutil.WaitForBuilderAccount(oc.KubeREST().ServiceAccounts(oc.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.Describe("Building from a template", func() {
		g.It(fmt.Sprintf("should create a build from %q template and run it", templateFixture), func() {
			oc.SetOutputDir(exutil.TestContext.OutputDir)

			g.By(fmt.Sprintf("calling oc new-app -f %q", templateFixture))
			err := oc.Run("new-app").Args("-f", templateFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting a test build")
			buildName, err := oc.Run("start-build").Args("initial-build").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("expecting the build is in Complete phase")
			err = exutil.WaitForABuild(oc.REST().Builds(oc.Namespace()), buildName, exutil.CheckBuildSuccessFunc, exutil.CheckBuildFailedFunc)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting a test build using the image produced by the last build")
			buildName, err = oc.Run("start-build").Args("internal-build").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("expecting the build is in Complete phase")
			err = exutil.WaitForABuild(oc.REST().Builds(oc.Namespace()), buildName, exutil.CheckBuildSuccessFunc, exutil.CheckBuildFailedFunc)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("getting the Docker image reference from ImageStream")
			imageName, err := exutil.GetDockerImageReference(oc.REST().ImageStreams(oc.Namespace()), "internal-image", "latest")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("writing the pod definition to a file")
			outputPath := filepath.Join(exutil.TestContext.OutputDir, oc.Namespace()+"-sample-pod.json")
			pod := exutil.CreatePodForImage(imageName)
			err = exutil.WriteObjectToFile(pod, outputPath)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("calling oc create -f %q", outputPath))
			err = oc.Run("create").Args("-f", outputPath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("expecting the pod to be running")
			err = oc.KubeFramework().WaitForPodRunning(pod.Name)
			o.Expect(err).NotTo(o.HaveOccurred())

			// even though the pod is running, the app isn't always started
			// so wait until webrick output is complete before curling.
			logs := ""
			count := 0
			for strings.Contains(logs, "8080") && count < 10 {
				logs, _ = oc.Run("logs").Args(pod.Name).Output()
				time.Sleep(time.Second)
				count++
			}

			g.By("expecting the pod container has saved artifacts")
			out, err := oc.Run("exec").Args("-p", pod.Name, "--", "curl", "http://0.0.0.0:8080").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			if !strings.Contains(out, "artifacts exist") {
				logs, _ = oc.Run("logs").Args(pod.Name).Output()
				e2e.Failf("Pod %q does not contain expected artifacts: %q\n%q", pod.Name, out, logs)
			}
		})
	})
})

package builds

import (
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[builds][Slow] incremental s2i build", func() {
	defer g.GinkgoRecover()

	const (
		buildTestPod     = "build-test-pod"
		buildTestService = "build-test-svc"
	)

	var (
		templateFixture      = exutil.FixturePath("testdata", "incremental-auth-build.json")
		podAndServiceFixture = exutil.FixturePath("testdata", "test-build-podsvc.json")
		oc                   = exutil.NewCLI("build-sti-inc", exutil.KubeConfigPath())
	)

	g.JustBeforeEach(func() {
		g.By("waiting for builder service account")
		err := exutil.WaitForBuilderAccount(oc.AdminKubeClient().Core().ServiceAccounts(oc.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.Describe("Building from a template", func() {
		g.It(fmt.Sprintf("should create a build from %q template and run it", templateFixture), func() {
			oc.SetOutputDir(exutil.TestContext.OutputDir)

			g.By(fmt.Sprintf("calling oc new-app -f %q", templateFixture))
			err := oc.Run("new-app").Args("-f", templateFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting a test build")
			br, _ := exutil.StartBuildAndWait(oc, "initial-build")
			br.AssertSuccess()

			g.By("starting a test build using the image produced by the last build")
			br2, _ := exutil.StartBuildAndWait(oc, "internal-build")
			br2.AssertSuccess()

			g.By("getting the Docker image reference from ImageStream")
			imageName, err := exutil.GetDockerImageReference(oc.Client().ImageStreams(oc.Namespace()), "internal-image", "latest")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("instantiating a pod and service with the new image")
			err = oc.Run("new-app").Args("-f", podAndServiceFixture, "-p", "IMAGE_NAME="+imageName).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for the service to become available")
			err = e2e.WaitForEndpoint(oc.KubeFramework().ClientSet, oc.Namespace(), buildTestService)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("expecting the pod container has saved artifacts")
			out, err := oc.Run("exec").Args("-p", buildTestPod, "--", "curl", "http://0.0.0.0:8080").Output()
			if err != nil {
				logs, _ := oc.Run("logs").Args(buildTestPod).Output()
				e2e.Failf("Failed to curl in application container: \n%q, pod logs: \n%q", out, logs)
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			if !strings.Contains(out, "artifacts exist") {
				logs, _ := oc.Run("logs").Args(buildTestPod).Output()
				e2e.Failf("Pod %q does not contain expected artifacts: %q\n%q", buildTestPod, out, logs)
			}
		})
	})
})

package builds

import (
	"fmt"
	"strings"

	e2e "k8s.io/kubernetes/test/e2e/framework"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:Builds][Slow] s2i build with environment file in sources", func() {
	defer g.GinkgoRecover()
	const (
		buildTestPod     = "build-test-pod"
		buildTestService = "build-test-svc"
	)

	var (
		imageStreamFixture   = exutil.FixturePath("..", "integration", "testdata", "test-image-stream.json")
		stiEnvBuildFixture   = exutil.FixturePath("testdata", "builds", "test-env-build.json")
		podAndServiceFixture = exutil.FixturePath("testdata", "builds", "test-build-podsvc.json")
		oc                   = exutil.NewCLI("build-sti-env", exutil.KubeConfigPath())
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

		g.Describe("Building from a template", func() {
			g.It(fmt.Sprintf("should create a image from %q template and run it in a pod", stiEnvBuildFixture), func() {
				oc.SetOutputDir(exutil.TestContext.OutputDir)

				g.By(fmt.Sprintf("calling oc create -f %q", imageStreamFixture))
				err := oc.Run("create").Args("-f", imageStreamFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By(fmt.Sprintf("calling oc create -f %q", stiEnvBuildFixture))
				err = oc.Run("create").Args("-f", stiEnvBuildFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("starting a test build")
				path := exutil.FixturePath("testdata", "builds", "sti-environment-build-app")
				br, _ := exutil.StartBuildAndWait(oc, "test", "--from-dir", path)
				br.AssertSuccess()

				g.By("getting the Docker image reference from ImageStream")
				imageName, err := exutil.GetDockerImageReference(oc.ImageClient().Image().ImageStreams(oc.Namespace()), "test", "latest")
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("instantiating a pod and service with the new image")
				err = oc.Run("new-app").Args("-f", podAndServiceFixture, "-p", "IMAGE_NAME="+imageName).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("waiting for the pod to be running")
				err = e2e.WaitForPodNameRunningInNamespace(oc.KubeFramework().ClientSet, "build-test-pod", oc.Namespace())
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("waiting for the service to become available")
				err = e2e.WaitForEndpoint(oc.KubeFramework().ClientSet, oc.Namespace(), buildTestService)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("expecting the pod container has TEST_ENV variable set")
				out, err := oc.Run("exec").Args("-p", buildTestPod, "--", "curl", "http://0.0.0.0:8080").Output()
				o.Expect(err).NotTo(o.HaveOccurred())

				if !strings.Contains(out, "success") {
					e2e.Failf("expected 'success' response when executing curl in %q, got %q", buildTestPod, out)
				}
			})
		})
	})
})

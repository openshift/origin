package builds

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	admissionapi "k8s.io/pod-security-admission/api"

	e2e "k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/pod"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-builds][Feature:Builds][Slow] incremental s2i build", func() {
	defer g.GinkgoRecover()

	const (
		buildTestPod     = "build-test-pod"
		buildTestService = "build-test-svc"
	)

	var (
		templateFixture      = exutil.FixturePath("testdata", "builds", "incremental-auth-build.json")
		podAndServiceFixture = exutil.FixturePath("testdata", "builds", "test-build-podsvc.json")
		oc                   = exutil.NewCLIWithPodSecurityLevel("build-sti-inc", admissionapi.LevelBaseline)
	)

	g.Context("", func() {
		g.BeforeEach(func() {
			exutil.PreTestDump()
		})

		g.AfterEach(func() {
			if g.CurrentSpecReport().Failed() {
				exutil.DumpPodStates(oc)
				exutil.DumpConfigMapStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.Describe("Building from a template", func() {
			g.It(fmt.Sprintf("should create a build from %q template and run it [apigroup:build.openshift.io][apigroup:image.openshift.io]", filepath.Base(templateFixture)), g.Label("Size:L"), func() {

				g.By(fmt.Sprintf("calling oc new-app -f %q", templateFixture))
				err := oc.Run("new-app").Args("-f", templateFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("starting a test build")
				br, _ := exutil.StartBuildAndWait(oc, "incremental-build")
				br.AssertSuccess()

				g.By("starting a test build using the image produced by the last build")
				br2, _ := exutil.StartBuildAndWait(oc, "incremental-build")
				br2.AssertSuccess()

				g.By("getting the container image reference from ImageStream")
				imageName, err := exutil.GetDockerImageReference(oc.ImageClient().ImageV1().ImageStreams(oc.Namespace()), "incremental-image", "latest")
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("instantiating a pod and service with the new image")
				err = oc.Run("new-app").Args("-f", podAndServiceFixture, "-p", "IMAGE_NAME="+imageName).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("waiting for the pod to be running")
				err = pod.WaitForPodNameRunningInNamespace(context.TODO(), oc.KubeFramework().ClientSet, "build-test-pod", oc.Namespace())
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("waiting for the service to become available")
				err = exutil.WaitForEndpoint(oc.KubeFramework().ClientSet, oc.Namespace(), buildTestService)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("expecting the pod container has saved artifacts")
				out, err := oc.Run("exec").Args(buildTestPod, "--", "curl", "http://0.0.0.0:8080").Output()
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
})

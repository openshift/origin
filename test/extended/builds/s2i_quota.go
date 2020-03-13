package builds

import (
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	buildv1 "github.com/openshift/api/build/v1"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-devex][Feature:Builds] s2i build with a quota", func() {
	defer g.GinkgoRecover()
	const (
		buildTestPod     = "build-test-pod"
		buildTestService = "build-test-svc"
	)

	var (
		buildFixture = exutil.FixturePath("testdata", "builds", "test-s2i-build-quota.json")
		oc           = exutil.NewCLI("s2i-build-quota", exutil.KubeConfigPath())
	)

	g.Context("", func() {
		g.BeforeEach(func() {
			exutil.PreTestDump()
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpConfigMapStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.Describe("Building from a template", func() {
			g.It("should create an s2i build with a quota and run it", func() {
				g.By(fmt.Sprintf("calling oc create -f %q", buildFixture))
				err := oc.Run("create").Args("-f", buildFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("starting a test build")
				br, _ := exutil.StartBuildAndWait(oc, "s2i-build-quota", "--from-dir", exutil.FixturePath("testdata", "builds", "build-quota"))
				br.AssertSuccess()
				o.Expect(br.Build.Status.StartTimestamp).NotTo(o.BeNil(), "Build start timestamp should be set")
				o.Expect(br.Build.Status.CompletionTimestamp).NotTo(o.BeNil(), "Build completion timestamp should be set")
				o.Expect(br.Build.Status.Duration).Should(o.BeNumerically(">", 0), "Build duration should be greater than zero")
				duration := br.Build.Status.CompletionTimestamp.Rfc3339Copy().Time.Sub(br.Build.Status.StartTimestamp.Rfc3339Copy().Time)
				o.Expect(br.Build.Status.Duration).To(o.Equal(duration), "Build duration should be computed correctly")

				g.By("expecting the build logs to contain the correct cgroups values")
				buildLog, err := br.LogsNoTimestamp()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(buildLog).To(o.ContainSubstring("MEMORY=419430400"))
				// TODO: re-enable this check when https://github.com/containers/buildah/issues/1213 is resolved.
				//o.Expect(buildLog).To(o.ContainSubstring("MEMORYSWAP=419430400"))

				testScheme := runtime.NewScheme()
				utilruntime.Must(buildv1.Install(testScheme))

				events, err := oc.KubeClient().CoreV1().Events(oc.Namespace()).Search(testScheme, br.Build)
				o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to get events from the build")
				o.Expect(events).NotTo(o.BeNil(), "Build event list should not be nil")

				exutil.CheckForBuildEvent(oc.KubeClient().CoreV1(), br.Build, BuildStartedEventReason, BuildStartedEventMessage)
				exutil.CheckForBuildEvent(oc.KubeClient().CoreV1(), br.Build, BuildCompletedEventReason, BuildCompletedEventMessage)

			})
		})
	})
})

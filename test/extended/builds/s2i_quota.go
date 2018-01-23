package builds

import (
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:Builds][Conformance] s2i build with a quota", func() {
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
			g.It("should create an s2i build with a quota and run it", func() {
				oc.SetOutputDir(exutil.TestContext.OutputDir)

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
				buildLog, err := br.Logs()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(buildLog).To(o.ContainSubstring("MEMORY=209715200"))
				o.Expect(buildLog).To(o.ContainSubstring("MEMORYSWAP=209715200"))

				events, err := oc.KubeClient().Core().Events(oc.Namespace()).Search(legacyscheme.Scheme, br.Build)
				o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to get events from the build")
				o.Expect(events).NotTo(o.BeNil(), "Build event list should not be nil")

				exutil.CheckForBuildEvent(oc.KubeClient().Core(), br.Build, buildapi.BuildStartedEventReason, buildapi.BuildStartedEventMessage)
				exutil.CheckForBuildEvent(oc.KubeClient().Core(), br.Build, buildapi.BuildCompletedEventReason, buildapi.BuildCompletedEventMessage)

			})
		})
	})
})

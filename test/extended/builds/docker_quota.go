package builds

import (
	"fmt"

	"k8s.io/pod-security-admission/api"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	buildv1 "github.com/openshift/api/build/v1"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-builds][Feature:Builds] docker build with a quota", func() {
	defer g.GinkgoRecover()

	var (
		buildFixture = exutil.FixturePath("testdata", "builds", "test-docker-build-quota.json")
		oc           = exutil.NewCLIWithPodSecurityLevel("docker-build-quota", api.LevelPrivileged)
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
			g.It("should create a docker build with a quota and run it [apigroup:build.openshift.io]", func() {
				g.By(fmt.Sprintf("calling oc create -f %q", buildFixture))
				err := oc.Run("create").Args("-f", buildFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("starting a test build")
				br, err := exutil.StartBuildAndWait(oc, "docker-build-quota", "--from-dir", exutil.FixturePath("testdata", "builds", "build-quota"))
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertFailure()
				g.By("checking status of the test build")
				o.Expect(br.Build.Status.StartTimestamp).NotTo(o.BeNil(), "Build start timestamp should be set")
				o.Expect(br.Build.Status.CompletionTimestamp).NotTo(o.BeNil(), "Build completion timestamp should be set")
				o.Expect(br.Build.Status.Duration).Should(o.BeNumerically(">", 0), "Build duration should be greater than zero")
				duration := br.Build.Status.CompletionTimestamp.Rfc3339Copy().Time.Sub(br.Build.Status.StartTimestamp.Rfc3339Copy().Time)
				o.Expect(br.Build.Status.Duration).To(o.Equal(duration), "Build duration should be computed correctly")

				g.By("expecting the build logs to contain the correct cgroups values")
				buildLog, err := br.LogsNoTimestamp()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(buildLog).To(o.ContainSubstring("MEMORY=419430400"))
				o.Expect(buildLog).To(o.ContainSubstring("MEMORYSWAP=419430400"))
				o.Expect(buildLog).To(o.ContainSubstring("QUOTA=98000"))
				o.Expect(buildLog).To(o.ContainSubstring("PERIOD=100000"))
				// 1003 is roughly 98000/100000 of 1024 (set in cgroups v1)
				// 998 is ((((((((1003 - 2) * 9999) / 262142) + 1) - 1) * 262142) / 9999) + 2) (cgroups v2, one round trip)
				// 972 is ((((((((998 - 2) * 9999) / 262142) + 1) - 1) * 262142) / 9999) + 2) (cgroups v2, two round trips)
				o.Expect(buildLog).To(o.Or(o.ContainSubstring("SHARES=1003"), o.ContainSubstring("SHARES=998"), o.ContainSubstring("SHARES=972")))

				testScheme := runtime.NewScheme()
				utilruntime.Must(buildv1.Install(testScheme))

				events, err := oc.KubeClient().CoreV1().Events(oc.Namespace()).Search(testScheme, br.Build)
				o.Expect(err).NotTo(o.HaveOccurred(), "Should be able to get events from the build")
				o.Expect(events).NotTo(o.BeNil(), "Build event list should not be nil")

				exutil.CheckForBuildEvent(oc.KubeClient().CoreV1(), br.Build, BuildStartedEventReason, BuildStartedEventMessage)
			})
		})
	})
})

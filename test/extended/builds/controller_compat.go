package builds

import (
	g "github.com/onsi/ginkgo"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-builds][Feature:Builds][Slow] build controller", func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLI("build-controllers", exutil.KubeConfigPath())
	)

	g.Context("", func() {

		g.BeforeEach(func() {
			exutil.PreTestDump()
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.Describe("RunBuildCompletePodDeleteTest", func() {
			g.It("should succeed", func() {
				RunBuildCompletePodDeleteTest(g.GinkgoT(), oc.BuildClient().BuildV1(), oc.AdminKubeClient(), oc.Namespace())
			})
		})
		g.Describe("RunBuildDeleteTest", func() {
			g.It("should succeed", func() {
				RunBuildDeleteTest(g.GinkgoT(), oc.AdminBuildClient().BuildV1(), oc.AdminKubeClient(), oc.Namespace())
			})
		})
		g.Describe("RunBuildRunningPodDeleteTest", func() {
			g.It("should succeed", func() {
				g.Skip("skipping until devex team figures this out in the new split API setup, see https://bugzilla.redhat.com/show_bug.cgi?id=164118")
				RunBuildRunningPodDeleteTest(g.GinkgoT(), oc.AdminBuildClient().BuildV1(), oc.AdminKubeClient(), oc.Namespace())
			})
		})
	})
})

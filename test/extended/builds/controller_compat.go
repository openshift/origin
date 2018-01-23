package builds

import (
	"os"

	g "github.com/onsi/ginkgo"

	"github.com/openshift/origin/test/common/build"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[bldcompat][Slow][Compatibility] build controller", func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLI("compat-build-controllers", exutil.KubeConfigPath())
	)

	g.Context("", func() {

		g.BeforeEach(func() {
			exutil.DumpDockerInfo()
		})

		g.JustBeforeEach(func() {
			os.Setenv("OS_TEST_NAMESPACE", oc.Namespace())
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.Describe("RunBuildControllerTest", func() {
			g.It("should succeed", func() {
				build.RunBuildControllerTest(g.GinkgoT(), oc.BuildClient().Build(), oc.InternalAdminKubeClient())
			})
		})
		g.Describe("RunBuildControllerPodSyncTest", func() {
			g.It("should succeed", func() {
				build.RunBuildControllerPodSyncTest(g.GinkgoT(), oc.BuildClient().Build(), oc.InternalAdminKubeClient())
			})
		})
		g.Describe("RunImageChangeTriggerTest [SkipPrevControllers]", func() {
			g.It("should succeed", func() {
				build.RunImageChangeTriggerTest(g.GinkgoT(), oc.AdminBuildClient().Build(), oc.AdminImageClient().Image())
			})
		})
		g.Describe("RunBuildDeleteTest", func() {
			g.It("should succeed", func() {
				build.RunBuildDeleteTest(g.GinkgoT(), oc.AdminBuildClient().Build(), oc.InternalAdminKubeClient())
			})
		})
		g.Describe("RunBuildRunningPodDeleteTest", func() {
			g.It("should succeed", func() {
				build.RunBuildRunningPodDeleteTest(g.GinkgoT(), oc.AdminBuildClient().Build(), oc.InternalAdminKubeClient())
			})
		})
		g.Describe("RunBuildConfigChangeControllerTest", func() {
			g.It("should succeed", func() {
				build.RunBuildConfigChangeControllerTest(g.GinkgoT(), oc.AdminBuildClient().Build())
			})
		})
	})
})

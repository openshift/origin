package builds

import (
	"fmt"
	"os"
	"path/filepath"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:Builds][quota][Slow] docker build with a quota", func() {
	defer g.GinkgoRecover()
	const (
		buildTestPod     = "build-test-pod"
		buildTestService = "build-test-svc"
	)

	var (
		buildFixture = exutil.FixturePath("testdata", "builds", "test-docker-build-quota.json")
		oc           = exutil.NewCLI("docker-build-quota", exutil.KubeConfigPath())
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
			g.It("should create a docker build with a quota and run it", func() {
				oc.SetOutputDir(exutil.TestContext.OutputDir)

				g.By(fmt.Sprintf("calling oc create -f %q", buildFixture))
				err := oc.Run("create").Args("-f", buildFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("starting a test build")
				path := exutil.FixturePath("testdata", "builds", "build-quota")
				o.Expect(os.Chmod(filepath.Join(path, ".s2i", "bin", "assemble"), 0755)).NotTo(o.HaveOccurred())
				br, err := exutil.StartBuildAndWait(oc, "docker-build-quota", "--from-dir", path)

				g.By("expecting the build is in Failed phase")
				br.AssertFailure()

				g.By("expecting the build logs to contain the correct cgroups values")
				out, err := br.Logs()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(out).To(o.ContainSubstring("MEMORY=209715200"))
				o.Expect(out).To(o.ContainSubstring("MEMORYSWAP=209715200"))
			})
		})
	})
})

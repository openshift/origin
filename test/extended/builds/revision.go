package builds

import (
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:Builds] build have source revision metadata", func() {
	defer g.GinkgoRecover()
	var (
		buildFixture = exutil.FixturePath("testdata", "builds", "test-build-revision.json")
		oc           = exutil.NewCLI("cli-build-revision", exutil.KubeConfigPath())
	)

	g.Context("", func() {
		g.BeforeEach(func() {
			exutil.DumpDockerInfo()
		})

		g.JustBeforeEach(func() {
			g.By("waiting for builder service account")
			err := exutil.WaitForBuilderAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()))
			o.Expect(err).NotTo(o.HaveOccurred())
			oc.Run("create").Args("-f", buildFixture).Execute()
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.Describe("started build", func() {
			g.It("should contain source revision information", func() {
				g.By("starting the build")
				br, _ := exutil.StartBuildAndWait(oc, "sample-build")
				br.AssertSuccess()

				g.By(fmt.Sprintf("verifying the status of %q", br.BuildPath))
				build, err := oc.BuildClient().Build().Builds(oc.Namespace()).Get(br.Build.Name, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(build.Spec.Revision).NotTo(o.BeNil())
				o.Expect(build.Spec.Revision.Git).NotTo(o.BeNil())
				o.Expect(build.Spec.Revision.Git.Commit).NotTo(o.BeEmpty())
				o.Expect(build.Spec.Revision.Git.Author.Name).NotTo(o.BeEmpty())
				o.Expect(build.Spec.Revision.Git.Committer.Name).NotTo(o.BeEmpty())
				o.Expect(build.Spec.Revision.Git.Message).NotTo(o.BeEmpty())
			})
		})
	})
})

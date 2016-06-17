package builds

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	buildapi "github.com/openshift/origin/pkg/build/api"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[builds][Slow] Capabilities should be dropped for s2i builders", func() {
	defer g.GinkgoRecover()
	var (
		s2ibuilderFixture      = exutil.FixturePath("..", "extended", "testdata", "s2i-dropcaps", "rootable-ruby")
		rootAccessBuildFixture = exutil.FixturePath("..", "extended", "testdata", "s2i-dropcaps", "root-access-build.yaml")
		oc                     = exutil.NewCLI("build-s2i-dropcaps", exutil.KubeConfigPath())
	)

	g.JustBeforeEach(func() {
		g.By("waiting for builder service account")
		err := exutil.WaitForBuilderAccount(oc.KubeREST().ServiceAccounts(oc.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.Describe("s2i build with a rootable builder", func() {
		g.It("should not be able to switch to root with an assemble script", func() {

			g.By("calling oc new-build for rootable-builder")
			err := oc.Run("new-build").Args("--binary", "--name=rootable-ruby").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting the rootable-ruby build with --wait flag")
			err = oc.Run("start-build").Args("rootable-ruby", fmt.Sprintf("--from-dir=%s", s2ibuilderFixture),
				"--wait").Execute()
			// debug for failures on jenkins
			if err != nil {
				exutil.DumpBuildLogs("rootable-ruby", oc)
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("creating a build that tries to gain root access via su")
			err = oc.Run("create").Args("-f", rootAccessBuildFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("start the root-access-build with the --wait flag")
			err = oc.Run("start-build").Args("root-access-build", "--wait").Execute()
			// debug for failures on jenkins
			if err == nil {
				exutil.DumpBuildLogs("root-access-build", oc)
			}
			o.Expect(err).To(o.HaveOccurred())

			g.By("verifying the build status")
			builds, err := oc.REST().Builds(oc.Namespace()).List(kapi.ListOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(builds.Items).ToNot(o.BeEmpty())

			// Find the build
			var build *buildapi.Build
			for i := range builds.Items {
				if builds.Items[i].Name == "root-access-build-1" {
					build = &builds.Items[i]
					break
				}
			}
			o.Expect(build).NotTo(o.BeNil())
			o.Expect(build.Status.Phase).Should(o.BeEquivalentTo(buildapi.BuildPhaseFailed))
		})
	})

})

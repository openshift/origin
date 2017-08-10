package builds

import (
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	"k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
	s2istatus "github.com/openshift/source-to-image/pkg/util/status"
)

var _ = g.Describe("[builds][Conformance] s2i build with a root user image", func() {
	defer g.GinkgoRecover()

	var (
		buildFixture = exutil.FixturePath("testdata", "s2i-build-root.yaml")
		oc           = exutil.NewCLI("s2i-build-root", exutil.KubeConfigPath())
	)

	g.JustBeforeEach(func() {
		g.By("waiting for builder service account")
		err := exutil.WaitForBuilderAccount(oc.AdminKubeClient().Core().ServiceAccounts(oc.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.Describe("Building using an image with a root default user", func() {
		g.It("should fail the build immediately", func() {
			framework.SkipIfProviderIs("gce")
			oc.SetOutputDir(exutil.TestContext.OutputDir)

			g.By(fmt.Sprintf("calling oc create -f %q", buildFixture))
			err := oc.Run("create").Args("-f", buildFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting a test build")
			// this uses the build-quota dir as the binary input source on purpose - we don't really care what we upload
			// to the build since it will fail before we ever consume the inputs.
			br, _ := exutil.StartBuildAndWait(oc, "s2i-build-root", "--from-dir", exutil.FixturePath("testdata", "build-quota"))
			br.AssertFailure()
			o.Expect(string(br.Build.Status.Reason)).To(o.Equal(string(s2istatus.ReasonPullBuilderImageFailed)))
			o.Expect(string(br.Build.Status.Message)).To(o.Equal(string(s2istatus.ReasonMessagePullBuilderImageFailed)))

		})
	})
})

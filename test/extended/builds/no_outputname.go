package builds

import (
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-builds][Feature:Builds] build without output image", func() {
	defer g.GinkgoRecover()
	var (
		dockerImageFixture = exutil.FixturePath("testdata", "builds", "test-docker-no-outputname.json")
		s2iImageFixture    = exutil.FixturePath("testdata", "builds", "test-s2i-no-outputname.json")
		oc                 = exutil.NewCLIWithPodSecurityLevel("build-no-outputname", admissionapi.LevelBaseline)
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

		g.Describe("building from templates", func() {
			g.It(fmt.Sprintf("should create an image from a docker template without an output image reference defined [apigroup:build.openshift.io]"), g.Label("Size:L"), func() {
				err := oc.Run("create").Args("-f", dockerImageFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("expecting build to pass without an output image reference specified")
				br, err := exutil.StartBuildAndWait(oc, "test-docker")
				br.AssertSuccess()

				g.By("verifying the build test-docker-1 output")
				buildLog, err := br.Logs()
				fmt.Fprintf(g.GinkgoWriter, "\nBuild log:\n%s\n", buildLog)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(buildLog).Should(o.ContainSubstring(`Build complete, no image push requested`))
			})

			g.It(fmt.Sprintf("should create an image from a S2i template without an output image reference defined [apigroup:build.openshift.io]"), g.Label("Size:L"), func() {
				err := oc.Run("create").Args("-f", s2iImageFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("expecting build to pass without an output image reference specified")
				br, err := exutil.StartBuildAndWait(oc, "test-sti")
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertSuccess()

				g.By("verifying the build test-sti-1 output")
				buildLog, err := br.Logs()
				fmt.Fprintf(g.GinkgoWriter, "\nBuild log:\n%s\n", buildLog)
				o.Expect(err).NotTo(o.HaveOccurred())

				o.Expect(buildLog).Should(o.ContainSubstring(`Build complete, no image push requested`))
			})
		})
	})
})

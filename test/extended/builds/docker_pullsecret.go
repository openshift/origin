package builds

import (
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-builds][Feature:Builds][pullsecret] docker build using a pull secret", func() {
	defer g.GinkgoRecover()
	const (
		buildTestPod     = "build-test-pod"
		buildTestService = "build-test-svc"
	)

	var (
		buildFixture = exutil.FixturePath("testdata", "builds", "test-docker-build-pullsecret.json")
		oc           = exutil.NewCLIWithPodSecurityLevel("docker-build-pullsecret", admissionapi.LevelBaseline)
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
			g.It("should create a docker build that pulls using a secret run it [apigroup:build.openshift.io]", g.Label("Size:L"), func() {

				g.By(fmt.Sprintf("calling oc create -f %q", buildFixture))
				err := oc.Run("create").Args("-f", buildFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("starting a build")
				br, err := exutil.StartBuildAndWait(oc, "docker-build")
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertSuccess()

				g.By("starting a second build that pulls the image from the first build")
				br, err = exutil.StartBuildAndWait(oc, "docker-build-pull")
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertSuccess()

			})
		})
	})
})

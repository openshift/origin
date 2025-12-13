package builds

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-builds][Feature:Builds] build with empty source", func() {
	defer g.GinkgoRecover()
	var (
		buildFixture = exutil.FixturePath("testdata", "builds", "test-nosrc-build.json")
		oc           = exutil.NewCLIWithPodSecurityLevel("cli-build-nosrc", admissionapi.LevelBaseline)
		exampleBuild = exutil.FixturePath("testdata", "builds", "test-build-app")
	)

	g.Context("", func() {

		g.BeforeEach(func() {
			exutil.PreTestDump()
		})

		g.JustBeforeEach(func() {
			oc.Run("create").Args("-f", buildFixture).Execute()
		})

		g.AfterEach(func() {
			if g.CurrentSpecReport().Failed() {
				exutil.DumpPodStates(oc)
				exutil.DumpConfigMapStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.Describe("started build", func() {
			g.It("should build even with an empty source in build config [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
				g.By("starting the empty source build")
				br, err := exutil.StartBuildAndWait(oc, "nosrc-build", fmt.Sprintf("--from-dir=%s", exampleBuild))
				br.AssertSuccess()

				g.By(fmt.Sprintf("verifying the status of %q", br.BuildPath))
				build, err := oc.BuildClient().BuildV1().Builds(oc.Namespace()).Get(context.Background(), br.Build.Name, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(build.Spec.Source.Dockerfile).To(o.BeNil())
				o.Expect(build.Spec.Source.Git).To(o.BeNil())
				o.Expect(build.Spec.Source.Images).To(o.BeNil())
				o.Expect(build.Spec.Source.Binary).NotTo(o.BeNil())
			})
		})
	})
})

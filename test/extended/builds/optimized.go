package builds

import (
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:Builds] Optimized image builds", func() {
	defer g.GinkgoRecover()
	var (
		oc             = exutil.NewCLI("build-dockerfile-env", exutil.KubeConfigPath())
		skipLayers     = buildapi.ImageOptimizationSkipLayers
		testDockerfile = `
FROM centos:7
RUN yum list installed
USER 1001
`
	)

	g.Context("", func() {

		g.BeforeEach(func() {
			exutil.DumpDockerInfo()
		})

		g.JustBeforeEach(func() {
			g.By("waiting for builder service account")
			err := exutil.WaitForBuilderAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()))
			o.Expect(err).NotTo(o.HaveOccurred())
			oc.SetOutputDir(exutil.TestContext.OutputDir)
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.It("should succeed [Conformance]", func() {
			g.By("creating a build directly")
			build, err := oc.AdminBuildClient().Build().Builds(oc.Namespace()).Create(&buildapi.Build{
				ObjectMeta: metav1.ObjectMeta{
					Name: "optimized",
				},
				Spec: buildapi.BuildSpec{
					CommonSpec: buildapi.CommonSpec{
						Source: buildapi.BuildSource{
							Dockerfile: &testDockerfile,
						},
						Strategy: buildapi.BuildStrategy{
							DockerStrategy: &buildapi.DockerBuildStrategy{
								ImageOptimizationPolicy: &skipLayers,
							},
						},
					},
				},
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(build.Spec.Strategy.DockerStrategy.ImageOptimizationPolicy).ToNot(o.BeNil())
			result := exutil.NewBuildResult(oc, build)
			err = exutil.WaitForBuildResult(oc.AdminBuildClient().Build().Builds(oc.Namespace()), result)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(result.BuildSuccess).To(o.BeTrue(), "Build did not succeed: %v", result)

			pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Get(build.Name+"-build", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			if strings.HasSuffix(pod.Spec.Containers[0].Image, ":v3.6.0-alpha.0") {
				g.Skip(fmt.Sprintf("The currently selected builder image does not yet support optimized image builds: %s", pod.Spec.Containers[0].Image))
			}

			s, err := result.Logs()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(s).To(o.ContainSubstring("--> RUN yum list installed"))
			o.Expect(s).To(o.ContainSubstring(fmt.Sprintf("\"OPENSHIFT_BUILD_NAMESPACE\"=\"%s\"", oc.Namespace())))
			o.Expect(s).To(o.ContainSubstring("--> Committing changes to "))
			o.Expect(s).To(o.ContainSubstring("Build complete, no image push requested"))
			e2e.Logf("Build logs:\n%s", result)
		})
	})
})

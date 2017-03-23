package builds

import (
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrs "k8s.io/kubernetes/pkg/api/errors"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	buildapi "github.com/openshift/origin/pkg/build/api"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[builds] Optimized image builds", func() {
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

	g.JustBeforeEach(func() {
		g.By("waiting for builder service account")
		err := exutil.WaitForBuilderAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())
		oc.SetOutputDir(exutil.TestContext.OutputDir)
	})

	g.It("should succeed as an admin [Conformance]", func() {
		g.By("creating a build directly")
		build, err := oc.AdminClient().Builds(oc.Namespace()).Create(&buildapi.Build{
			ObjectMeta: kapi.ObjectMeta{
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
		err = exutil.WaitForBuildResult(oc.AdminClient().Builds(oc.Namespace()), result)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result.BuildSuccess).To(o.BeTrue(), "Build did not succeed: %v", result)

		pod, err := oc.KubeClient().Pods(oc.Namespace()).Get(build.Name + "-build")
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

	g.It("should fail as a normal user [Conformance]", func() {
		g.By("creating a build directly")
		_, err := oc.Client().Builds(oc.Namespace()).Create(&buildapi.Build{
			ObjectMeta: kapi.ObjectMeta{
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
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(kapierrs.IsForbidden(err)).To(o.BeTrue(), "Unexpected error: %v", err)
	})
})

package builds

import (
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	core "k8s.io/kubernetes/pkg/apis/core"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:Builds] Multi-stage image builds", func() {
	defer g.GinkgoRecover()
	var (
		oc             = exutil.NewCLI("build-multistage", exutil.KubeConfigPath())
		testDockerfile = `
FROM scratch as test
USER 1001
FROM centos:7
COPY --from=test /usr/bin/curl /test/
`
	)

	g.Context("", func() {

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
			build, err := oc.BuildClient().Build().Builds(oc.Namespace()).Create(&buildapi.Build{
				ObjectMeta: metav1.ObjectMeta{
					Name: "multi-stage",
				},
				Spec: buildapi.BuildSpec{
					CommonSpec: buildapi.CommonSpec{
						Source: buildapi.BuildSource{
							Dockerfile: &testDockerfile,
							Images: []buildapi.ImageSource{
								{From: core.ObjectReference{Kind: "DockerImage", Name: "centos:7"}, As: []string{"scratch"}},
							},
						},
						Strategy: buildapi.BuildStrategy{
							DockerStrategy: &buildapi.DockerBuildStrategy{},
						},
						Output: buildapi.BuildOutput{
							To: &core.ObjectReference{
								Kind: "DockerImage",
								Name: fmt.Sprintf("docker-registry.default.svc:5000/%s/multi-stage:v1", oc.Namespace()),
							},
						},
					},
				},
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			result := exutil.NewBuildResult(oc, build)
			err = exutil.WaitForBuildResult(oc.AdminBuildClient().Build().Builds(oc.Namespace()), result)
			o.Expect(err).NotTo(o.HaveOccurred())

			pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Get(build.Name+"-build", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			if !result.BuildSuccess && strings.HasSuffix(pod.Spec.Containers[0].Image, ":v3.10.0-alpha.0") {
				g.Skip(fmt.Sprintf("The currently selected builder image does not yet support optimized image builds: %s", pod.Spec.Containers[0].Image))
			}

			o.Expect(result.BuildSuccess).To(o.BeTrue(), "Build did not succeed: %#v", result)

			s, err := result.Logs()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(s).ToNot(o.ContainSubstring("--> FROM scratch"))
			o.Expect(s).To(o.ContainSubstring("--> COPY --from"))
			o.Expect(s).To(o.ContainSubstring(fmt.Sprintf("\"OPENSHIFT_BUILD_NAMESPACE\"=\"%s\"", oc.Namespace())))
			o.Expect(s).To(o.ContainSubstring("--> Committing changes to "))
			e2e.Logf("Build logs:\n%s", result)

			c := oc.KubeFramework().PodClient()
			pod = c.Create(&corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:    "run",
							Image:   fmt.Sprintf("docker-registry.default.svc:5000/%s/multi-stage:v1", oc.Namespace()),
							Command: []string{"/test/curl", "-k", "https://kubernetes.default.svc"},
						},
					},
				},
			})
			c.WaitForSuccess(pod.Name, e2e.PodStartTimeout)
			data, err := c.GetLogs(pod.Name, &corev1.PodLogOptions{}).DoRaw()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("Pod logs:\n%s", string(data))
		})
	})
})

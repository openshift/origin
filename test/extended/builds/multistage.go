package builds

import (
	"context"
	"fmt"
	"regexp"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"

	buildv1 "github.com/openshift/api/build/v1"

	eximages "github.com/openshift/origin/test/extended/images"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

var _ = g.Describe("[sig-builds][Feature:Builds] Multi-stage image builds", func() {
	defer g.GinkgoRecover()
	var (
		oc             = exutil.NewCLIWithPodSecurityLevel("build-multistage", admissionapi.LevelBaseline)
		testDockerfile = fmt.Sprintf(`
FROM scratch as test
USER 1001
FROM %[1]s as other
COPY --from=test /usr/bin/curl /test/
COPY --from=%[2]s /bin/echo /test/
COPY --from=%[2]s /bin/wget /test/
`, image.LimitedShellImage(), image.MustGatherImage())
	)

	g.AfterEach(func() {
		if g.CurrentSpecReport().Failed() {
			exutil.DumpPodStates(oc)
			exutil.DumpConfigMapStates(oc)
			exutil.DumpPodLogsStartingWith("", oc)
		}
	})

	g.It("should succeed [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
		g.By("creating a build directly")
		registryURL, err := eximages.GetDockerRegistryURL(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		build, err := oc.BuildClient().BuildV1().Builds(oc.Namespace()).Create(context.Background(), &buildv1.Build{
			ObjectMeta: metav1.ObjectMeta{
				Name: "multi-stage",
			},
			Spec: buildv1.BuildSpec{
				CommonSpec: buildv1.CommonSpec{
					Source: buildv1.BuildSource{
						Dockerfile: &testDockerfile,
						Images: []buildv1.ImageSource{
							{From: corev1.ObjectReference{Kind: "DockerImage", Name: image.LimitedShellImage()}, As: []string{"scratch"}},
						},
					},
					Strategy: buildv1.BuildStrategy{
						DockerStrategy: &buildv1.DockerBuildStrategy{
							Env: []corev1.EnvVar{
								{
									Name:  "BUILD_LOGLEVEL",
									Value: "2",
								},
							},
						},
					},
					Output: buildv1.BuildOutput{
						To: &corev1.ObjectReference{
							Kind: "DockerImage",
							Name: fmt.Sprintf("%s/%s/multi-stage:v1", registryURL, oc.Namespace()),
						},
					},
				},
			},
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		result := exutil.NewBuildResult(oc, build)
		err = exutil.WaitForBuildResult(oc.AdminBuildClient().BuildV1().Builds(oc.Namespace()), result)
		o.Expect(err).NotTo(o.HaveOccurred())

		pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Get(context.Background(), build.Name+"-build", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(result.BuildSuccess).To(o.BeTrue(), "Build did not succeed: %#v", result)

		s, err := result.Logs()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(s).ToNot(o.ContainSubstring("--> FROM scratch"))
		o.Expect(s).ToNot(o.ContainSubstring("FROM busybox"))
		o.Expect(s).To(o.MatchRegexp("(\\[1/2\\] STEP 1/3|STEP 1/2|STEP 1): FROM %s AS test", regexp.QuoteMeta(image.LimitedShellImage())))
		o.Expect(s).To(o.ContainSubstring("COPY --from"))
		o.Expect(s).To(o.ContainSubstring("\"OPENSHIFT_BUILD_NAMESPACE\"=\"%s\"", oc.Namespace()))
		e2e.Logf("Build logs:\n%v", result)

		c := e2epod.PodClientNS(oc.KubeFramework(), oc.Namespace())
		pod = c.Create(context.TODO(), &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: corev1.PodSpec{
				RestartPolicy: corev1.RestartPolicyNever,
				Containers: []corev1.Container{
					{
						Name:    "run",
						Image:   fmt.Sprintf("%s/%s/multi-stage:v1", registryURL, oc.Namespace()),
						Command: []string{"/test/curl", "-k", "https://kubernetes.default.svc"},
					},
					{
						Name:    "check",
						Image:   fmt.Sprintf("%s/%s/multi-stage:v1", registryURL, oc.Namespace()),
						Command: []string{"ls", "/test/"},
					},
				},
			},
		})
		c.WaitForSuccess(context.TODO(), pod.Name, e2e.PodStartTimeout)
		data, err := oc.Run("logs").Args("-f", "test", "-c", "run").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		m, err := oc.Run("logs").Args("-f", "test", "-c", "check").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(m).To(o.ContainSubstring("echo"))
		o.Expect(m).To(o.ContainSubstring("wget"))
		e2e.Logf("Pod logs:\n%s\n%s", string(data), string(m))
	})
})

package builds

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	buildv1 "github.com/openshift/api/build/v1"
	"github.com/openshift/library-go/pkg/build/naming"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-builds][Feature:Builds][Slow] builds should have deadlines", func() {
	defer g.GinkgoRecover()
	var (
		sourceFixture = exutil.FixturePath("testdata", "builds", "test-cds-sourcebuild.json")
		dockerFixture = exutil.FixturePath("testdata", "builds", "test-cds-dockerbuild.json")
		oc            = exutil.NewCLI("cli-start-build")
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

		g.Describe("oc start-build source-build --wait", func() {
			g.It("Source: should start a build and wait for the build failed and build pod being killed by kubelet [apigroup:build.openshift.io]", g.Label("Size:L"), func() {

				g.By("calling oc create source-build")
				err := oc.Run("create").Args("-f", sourceFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("starting the source build with --wait flag and short timeout")
				br, err := exutil.StartBuildAndWait(oc, "source-build", "--wait")
				o.Expect(br.StartBuildErr).To(o.HaveOccurred()) // start-build should detect the build error

				g.By("verifying the build status")
				o.Expect(br.BuildAttempt).To(o.BeTrue())                                           // the build should have been attempted
				o.Expect(br.Build.Status.Phase).Should(o.BeEquivalentTo(buildv1.BuildPhaseFailed)) // the build should have failed

				g.By("verifying the build pod status")
				pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Get(context.Background(), GetBuildPodName(br.Build), metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(pod.Status.Phase).Should(o.BeEquivalentTo(corev1.PodFailed))
				o.Expect(pod.Status.Reason).Should(o.ContainSubstring("DeadlineExceeded"))

			})
		})

		g.Describe("oc start-build docker-build --wait", func() {
			g.It("Docker: should start a build and wait for the build failed and build pod being killed by kubelet [apigroup:build.openshift.io]", g.Label("Size:L"), func() {

				g.By("calling oc create docker-build")
				err := oc.Run("create").Args("-f", dockerFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("starting the docker build with --wait flag and short timeout")
				br, err := exutil.StartBuildAndWait(oc, "docker-build", "--wait")
				o.Expect(br.StartBuildErr).To(o.HaveOccurred()) // start-build should detect the build error

				g.By("verifying the build status")
				o.Expect(br.BuildAttempt).To(o.BeTrue())                                           // the build should have been attempted
				o.Expect(br.Build.Status.Phase).Should(o.BeEquivalentTo(buildv1.BuildPhaseFailed)) // the build should have failed

				g.By("verifying the build pod status")
				pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Get(context.Background(), GetBuildPodName(br.Build), metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(pod.Status.Phase).Should(o.BeEquivalentTo(corev1.PodFailed))
				o.Expect(pod.Status.Reason).Should(o.ContainSubstring("DeadlineExceeded"))

			})
		})

	})
})

var buildPodSuffix = "build"

// GetBuildPodName returns name of the build pod.
func GetBuildPodName(build *buildv1.Build) string {
	return naming.GetPodName(build.Name, buildPodSuffix)
}

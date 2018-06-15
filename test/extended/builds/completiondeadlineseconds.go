package builds

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:Builds][Slow] builds should have deadlines", func() {
	defer g.GinkgoRecover()
	var (
		sourceFixture = exutil.FixturePath("testdata", "builds", "test-cds-sourcebuild.json")
		dockerFixture = exutil.FixturePath("testdata", "builds", "test-cds-dockerbuild.json")
		oc            = exutil.NewCLI("cli-start-build", exutil.KubeConfigPath())
	)

	g.Context("", func() {
		g.BeforeEach(func() {
			exutil.DumpDockerInfo()
		})

		g.JustBeforeEach(func() {
			g.By("waiting for builder service account")
			err := exutil.WaitForBuilderAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()))
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.Describe("oc start-build source-build --wait", func() {
			g.It("Source: should start a build and wait for the build failed and build pod being killed by kubelet", func() {

				g.By("calling oc create source-build")
				err := oc.Run("create").Args("-f", sourceFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("starting the source build with --wait flag and short timeout")
				br, err := exutil.StartBuildAndWait(oc, "source-build", "--wait")
				o.Expect(br.StartBuildErr).To(o.HaveOccurred()) // start-build should detect the build error

				g.By("verifying the build status")
				o.Expect(br.BuildAttempt).To(o.BeTrue())                                            // the build should have been attempted
				o.Expect(br.Build.Status.Phase).Should(o.BeEquivalentTo(buildapi.BuildPhaseFailed)) // the build should have failed

				g.By("verifying the build pod status")
				pod, err := oc.KubeClient().Core().Pods(oc.Namespace()).Get(buildapi.GetBuildPodName(br.Build), metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(pod.Status.Phase).Should(o.BeEquivalentTo(kapi.PodFailed))
				o.Expect(pod.Status.Reason).Should(o.ContainSubstring("DeadlineExceeded"))

			})
		})

		g.Describe("oc start-build docker-build --wait", func() {
			g.It("Docker: should start a build and wait for the build failed and build pod being killed by kubelet", func() {

				g.By("calling oc create docker-build")
				err := oc.Run("create").Args("-f", dockerFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("starting the docker build with --wait flag and short timeout")
				br, err := exutil.StartBuildAndWait(oc, "docker-build", "--wait")
				o.Expect(br.StartBuildErr).To(o.HaveOccurred()) // start-build should detect the build error

				g.By("verifying the build status")
				o.Expect(br.BuildAttempt).To(o.BeTrue())                                            // the build should have been attempted
				o.Expect(br.Build.Status.Phase).Should(o.BeEquivalentTo(buildapi.BuildPhaseFailed)) // the build should have failed

				g.By("verifying the build pod status")
				pod, err := oc.KubeClient().Core().Pods(oc.Namespace()).Get(buildapi.GetBuildPodName(br.Build), metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(pod.Status.Phase).Should(o.BeEquivalentTo(kapi.PodFailed))
				o.Expect(pod.Status.Reason).Should(o.ContainSubstring("DeadlineExceeded"))

			})
		})

	})
})

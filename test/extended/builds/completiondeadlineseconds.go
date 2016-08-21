package builds

import (
	kapi "k8s.io/kubernetes/pkg/api"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	buildapi "github.com/openshift/origin/pkg/build/api"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[builds][Slow] builds should have deadlines", func() {
	defer g.GinkgoRecover()
	var (
		sourceFixture = exutil.FixturePath("testdata", "test-cds-sourcebuild.json")
		dockerFixture = exutil.FixturePath("testdata", "test-cds-dockerbuild.json")
		oc            = exutil.NewCLI("cli-start-build", exutil.KubeConfigPath())
	)

	g.JustBeforeEach(func() {
		g.By("waiting for builder service account")
		err := exutil.WaitForBuilderAccount(oc.KubeREST().ServiceAccounts(oc.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())
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
			pod, err := oc.KubeREST().Pods(oc.Namespace()).Get(buildapi.GetBuildPodName(br.Build))
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
			pod, err := oc.KubeREST().Pods(oc.Namespace()).Get(buildapi.GetBuildPodName(br.Build))
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(pod.Status.Phase).Should(o.BeEquivalentTo(kapi.PodFailed))
			o.Expect(pod.Status.Reason).Should(o.ContainSubstring("DeadlineExceeded"))

		})
	})

})

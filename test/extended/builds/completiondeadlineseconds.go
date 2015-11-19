package builds

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	buildapi "github.com/openshift/origin/pkg/build/api"
	buildutil "github.com/openshift/origin/pkg/build/util"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("builds: build with CompletionDeadlineSeconds", func() {
	defer g.GinkgoRecover()
	var (
		sourceFixture = exutil.FixturePath("..", "extended", "fixtures", "test-cds-sourcebuild.json")
		dockerFixture = exutil.FixturePath("..", "extended", "fixtures", "test-cds-dockerbuild.json")
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

			g.By("starting the build with --wait flag")
			_, err = oc.Run("start-build").Args("source-build", "--wait").Output()
			o.Expect(err).To(o.HaveOccurred())

			g.By("verifying the build status")
			builds, err := oc.REST().Builds(oc.Namespace()).List(labels.Everything(), fields.Everything())
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(builds.Items).To(o.HaveLen(1))

			build := builds.Items[0]
			o.Expect(build.Status.Phase).Should(o.BeEquivalentTo(buildapi.BuildPhaseFailed))

			g.By("verifying the build pod status")
			pod, err := oc.KubeREST().Pods(oc.Namespace()).Get(buildutil.GetBuildPodName(&build))
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

			g.By("starting the build with --wait flag")
			_, err = oc.Run("start-build").Args("docker-build", "--wait").Output()
			o.Expect(err).To(o.HaveOccurred())

			g.By("verifying the build status")
			builds, err := oc.REST().Builds(oc.Namespace()).List(labels.Everything(), fields.Everything())
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(builds.Items).To(o.HaveLen(1))

			build := builds.Items[0]
			o.Expect(build.Status.Phase).Should(o.BeEquivalentTo(buildapi.BuildPhaseFailed))

			g.By("verifying the build pod status")
			pod, err := oc.KubeREST().Pods(oc.Namespace()).Get(buildutil.GetBuildPodName(&build))
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(pod.Status.Phase).Should(o.BeEquivalentTo(kapi.PodFailed))
			o.Expect(pod.Status.Reason).Should(o.ContainSubstring("DeadlineExceeded"))
		})
	})

})

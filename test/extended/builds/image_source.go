package builds

import (
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[builds][Slow] build can have Docker image source", func() {
	defer g.GinkgoRecover()
	var (
		buildFixture     = exutil.FixturePath("testdata", "test-imagesource-build.yaml")
		oc               = exutil.NewCLI("build-image-source", exutil.KubeConfigPath())
		imageSourceLabel = exutil.ParseLabelsOrDie("app=imagesourceapp")
		imageDockerLabel = exutil.ParseLabelsOrDie("app=imagedockerapp")
	)

	g.JustBeforeEach(func() {
		g.By("waiting for builder service account")
		err := exutil.WaitForBuilderAccount(oc.KubeREST().ServiceAccounts(oc.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("waiting for imagestreams to be imported")
		err = exutil.WaitForAnImageStream(oc.AdminREST().ImageStreams("openshift"), "jenkins", exutil.CheckImageStreamLatestTagPopulatedFn, exutil.CheckImageStreamTagNotFoundFn)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.Describe("build with image source", func() {
		g.It("should complete successfully and contain the expected file", func() {
			g.By("Creating build configs for source build")
			err := oc.Run("create").Args("-f", buildFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting the source strategy build")
			br, err := exutil.StartBuildAndWait(oc, "imagesourcebuild")
			br.AssertSuccess()

			g.By("expecting the pod to deploy successfully")
			pods, err := exutil.WaitForPods(oc.KubeREST().Pods(oc.Namespace()), imageSourceLabel, exutil.CheckPodIsRunningFn, 1, 2*time.Minute)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(len(pods)).To(o.Equal(1))
			pod, err := oc.KubeREST().Pods(oc.Namespace()).Get(pods[0])
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("expecting the pod to contain the file from the input image")
			out, err := oc.Run("exec").Args(pod.Name, "-c", pod.Spec.Containers[0].Name, "--", "ls", "injected/dir").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("jenkins.war"))
		})
	})
	g.Describe("build with image docker", func() {
		g.It("should complete successfully and contain the expected file", func() {
			g.By("Creating build configs for docker build")
			err := oc.Run("create").Args("-f", buildFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting the docker strategy build")
			br, err := exutil.StartBuildAndWait(oc, "imagedockerbuild")
			br.AssertSuccess()

			g.By("expect the pod to deploy successfully")
			pods, err := exutil.WaitForPods(oc.KubeREST().Pods(oc.Namespace()), imageDockerLabel, exutil.CheckPodIsRunningFn, 1, 2*time.Minute)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(len(pods)).To(o.Equal(1))
			pod, err := oc.KubeREST().Pods(oc.Namespace()).Get(pods[0])
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("expecting the pod to contain the file from the input image")
			out, err := oc.Run("exec").Args(pod.Name, "-c", pod.Spec.Containers[0].Name, "--", "ls", "injected/dir").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("jenkins.war"))
		})

	})

})

package builds

import (
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	kapiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:Builds][Slow] build can have Docker image source", func() {
	defer g.GinkgoRecover()
	var (
		buildConfigFixture = exutil.FixturePath("testdata", "builds", "test-imagesource-buildconfig.yaml")
		s2iBuildFixture    = exutil.FixturePath("testdata", "builds", "test-imageresolution-s2i-build.yaml")
		dockerBuildFixture = exutil.FixturePath("testdata", "builds", "test-imageresolution-docker-build.yaml")
		customBuildFixture = exutil.FixturePath("testdata", "builds", "test-imageresolution-custom-build.yaml")
		oc                 = exutil.NewCLI("build-image-source", exutil.KubeConfigPath())
		imageSourceLabel   = exutil.ParseLabelsOrDie("app=imagesourceapp")
		imageDockerLabel   = exutil.ParseLabelsOrDie("app=imagedockerapp")
		sourceBuildLabel   = exutil.ParseLabelsOrDie("openshift.io/build.name=imagesourcebuild")
		dockerBuildLabel   = exutil.ParseLabelsOrDie("openshift.io/build.name=imagedockerbuild")
		customBuildLabel   = exutil.ParseLabelsOrDie("openshift.io/build.name=imagecustombuild")
	)

	g.Context("", func() {
		g.BeforeEach(func() {
			exutil.DumpDockerInfo()
		})

		g.JustBeforeEach(func() {
			g.By("waiting for builder service account")
			err := exutil.WaitForBuilderAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()))
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for imagestreams to be imported")
			err = exutil.WaitForAnImageStream(oc.AdminImageClient().Image().ImageStreams("openshift"), "ruby", exutil.CheckImageStreamLatestTagPopulatedFn, exutil.CheckImageStreamTagNotFoundFn)
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.Describe("buildconfig with input source image and s2i strategy", func() {
			g.It("should complete successfully and contain the expected file", func() {
				g.By("Creating build configs for source build")
				err := oc.Run("create").Args("-f", buildConfigFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("starting building the private input image")
				br, err := exutil.StartBuildAndWait(oc, "inputimage")
				br.AssertSuccess()

				g.By("starting the source strategy build")
				br, err = exutil.StartBuildAndWait(oc, "imagesourcebuildconfig")
				br.AssertSuccess()

				g.By("expecting the pod to deploy successfully")
				pods, err := exutil.WaitForPods(oc.KubeClient().Core().Pods(oc.Namespace()), imageSourceLabel, exutil.CheckPodIsRunningFn, 1, 2*time.Minute)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(len(pods)).To(o.Equal(1))
				pod, err := oc.KubeClient().Core().Pods(oc.Namespace()).Get(pods[0], metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("expecting the pod to contain the file from the input image")
				out, err := oc.Run("exec").Args(pod.Name, "-c", pod.Spec.Containers[0].Name, "--", "ls", "injected/dir").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(out).To(o.ContainSubstring("ruby"))
			})
		})
		g.Describe("buildconfig with input source image and docker strategy", func() {
			g.It("should complete successfully and contain the expected file", func() {
				g.By("Creating build configs for docker build")
				err := oc.Run("create").Args("-f", buildConfigFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("starting building the private input image")
				br, err := exutil.StartBuildAndWait(oc, "inputimage")
				br.AssertSuccess()

				g.By("starting the docker strategy build")
				br, err = exutil.StartBuildAndWait(oc, "imagedockerbuildconfig")
				br.AssertSuccess()

				g.By("expect the pod to deploy successfully")
				pods, err := exutil.WaitForPods(oc.KubeClient().Core().Pods(oc.Namespace()), imageDockerLabel, exutil.CheckPodIsRunningFn, 1, 2*time.Minute)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(len(pods)).To(o.Equal(1))
				pod, err := oc.KubeClient().Core().Pods(oc.Namespace()).Get(pods[0], metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("expecting the pod to contain the file from the input image")
				out, err := oc.Run("exec").Args(pod.Name, "-c", pod.Spec.Containers[0].Name, "--", "ls", "injected/dir").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(out).To(o.ContainSubstring("ruby"))
			})
		})
		g.Describe("creating a build with an input source image and s2i strategy", func() {
			g.It("should resolve the imagestream references and secrets", func() {
				g.By("Creating build configs for input image")
				err := oc.Run("create").Args("-f", buildConfigFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("starting building the private input image")
				br, err := exutil.StartBuildAndWait(oc, "inputimage")
				br.AssertSuccess()

				g.By("Creating a build for source build")
				err = oc.Run("create").Args("-f", s2iBuildFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("expecting the build pod to start running")
				pods, err := exutil.WaitForPods(oc.KubeClient().Core().Pods(oc.Namespace()), sourceBuildLabel, exutil.CheckPodIsRunningFn, 1, 2*time.Minute)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(len(pods)).To(o.Equal(1))
				pod, err := oc.KubeClient().Core().Pods(oc.Namespace()).Get(pods[0], metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				foundEnv := false
				for _, env := range pod.Spec.Containers[0].Env {
					if env.Name == "BUILD" {
						foundEnv = true

						obj, _, err := legacyscheme.Codecs.UniversalDecoder().Decode([]byte(env.Value), nil, nil)
						o.Expect(err).NotTo(o.HaveOccurred())
						ok := false
						build, ok := obj.(*buildapi.Build)
						o.Expect(ok).To(o.BeTrue(), "could not convert build env\n %s\n to a build object", env.Value)
						o.Expect(build.Spec.Strategy.SourceStrategy.From.Kind).To(o.Equal("DockerImage"))
						o.Expect(build.Spec.Strategy.SourceStrategy.From.Name).To(o.ContainSubstring("@sha256:"))
						o.Expect(build.Spec.Strategy.SourceStrategy.PullSecret).NotTo(o.BeNil())

						o.Expect(build.Spec.Source.Images[0].From.Kind).To(o.Equal("DockerImage"))
						o.Expect(build.Spec.Source.Images[0].From.Name).To(o.ContainSubstring("@sha256:"))
						o.Expect(build.Spec.Source.Images[0].PullSecret).NotTo(o.BeNil())

						o.Expect(build.Spec.Output.To.Kind).To(o.Equal("DockerImage"))
						o.Expect(build.Spec.Output.PushSecret).NotTo(o.BeNil())
					}
				}
				o.Expect(foundEnv).To(o.BeTrue(), "did not find BUILD env in build pod %#v", pod)
			})
		})
		g.Describe("creating a build with an input source image and docker strategy", func() {
			g.It("should resolve the imagestream references and secrets", func() {
				g.By("Creating build configs for input image")
				err := oc.Run("create").Args("-f", buildConfigFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("starting building the private input image")
				br, err := exutil.StartBuildAndWait(oc, "inputimage")
				br.AssertSuccess()

				g.By("Creating a build for docker build")
				err = oc.Run("create").Args("-f", dockerBuildFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("expecting the build pod to start running")
				pods, err := exutil.WaitForPods(oc.KubeClient().Core().Pods(oc.Namespace()), dockerBuildLabel, exutil.CheckPodIsRunningFn, 1, 2*time.Minute)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(len(pods)).To(o.Equal(1))
				pod, err := oc.KubeClient().Core().Pods(oc.Namespace()).Get(pods[0], metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				foundEnv := false
				for _, env := range pod.Spec.Containers[0].Env {
					if env.Name == "BUILD" {
						foundEnv = true

						obj, _, err := legacyscheme.Codecs.UniversalDecoder().Decode([]byte(env.Value), nil, nil)
						o.Expect(err).NotTo(o.HaveOccurred())
						ok := false
						build, ok := obj.(*buildapi.Build)
						o.Expect(ok).To(o.BeTrue(), "could not convert build env\n %s\n to a build object", env.Value)
						o.Expect(build.Spec.Strategy.DockerStrategy.From.Kind).To(o.Equal("DockerImage"))
						o.Expect(build.Spec.Strategy.DockerStrategy.From.Name).To(o.ContainSubstring("@sha256:"))
						o.Expect(build.Spec.Strategy.DockerStrategy.PullSecret).NotTo(o.BeNil())

						o.Expect(build.Spec.Source.Images[0].From.Kind).To(o.Equal("DockerImage"))
						o.Expect(build.Spec.Source.Images[0].From.Name).To(o.ContainSubstring("@sha256:"))
						o.Expect(build.Spec.Source.Images[0].PullSecret).NotTo(o.BeNil())

						o.Expect(build.Spec.Output.To.Kind).To(o.Equal("DockerImage"))
						o.Expect(build.Spec.Output.PushSecret).NotTo(o.BeNil())
					}
				}
				o.Expect(foundEnv).To(o.BeTrue(), "did not find BUILD env in build pod %#v", pod)
			})
		})
		g.Describe("creating a build with an input source image and custom strategy", func() {
			g.It("should resolve the imagestream references and secrets", func() {
				g.By("Creating build configs for input image")
				err := oc.Run("create").Args("-f", buildConfigFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("starting building the private input image")
				br, err := exutil.StartBuildAndWait(oc, "inputimage")
				br.AssertSuccess()

				g.By("Creating a build for custom build")
				err = oc.AsAdmin().Run("create").Args("-f", customBuildFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("expecting the build pod to exist")
				pods, err := exutil.WaitForPods(oc.KubeClient().Core().Pods(oc.Namespace()), customBuildLabel, func(kapiv1.Pod) bool { return true }, 1, 2*time.Minute)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(len(pods)).To(o.Equal(1))
				pod, err := oc.KubeClient().Core().Pods(oc.Namespace()).Get(pods[0], metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				foundBuildEnv := false
				foundCustomEnv := false
				for _, env := range pod.Spec.Containers[0].Env {
					if env.Name == "BUILD" {
						foundBuildEnv = true

						obj, _, err := legacyscheme.Codecs.UniversalDecoder().Decode([]byte(env.Value), nil, nil)
						o.Expect(err).NotTo(o.HaveOccurred())
						ok := false
						build, ok := obj.(*buildapi.Build)
						o.Expect(ok).To(o.BeTrue(), "could not convert build env\n %s\n to a build object", env.Value)
						o.Expect(build.Spec.Strategy.CustomStrategy.From.Kind).To(o.Equal("DockerImage"))
						o.Expect(build.Spec.Strategy.CustomStrategy.From.Name).To(o.ContainSubstring("@sha256:"))
						o.Expect(build.Spec.Strategy.CustomStrategy.PullSecret).NotTo(o.BeNil())

						o.Expect(build.Spec.Source.Images[0].From.Kind).To(o.Equal("DockerImage"))
						o.Expect(build.Spec.Source.Images[0].From.Name).To(o.ContainSubstring("@sha256:"))
						o.Expect(build.Spec.Source.Images[0].PullSecret).NotTo(o.BeNil())

						o.Expect(build.Spec.Output.To.Kind).To(o.Equal("DockerImage"))
						o.Expect(build.Spec.Output.PushSecret).NotTo(o.BeNil())
					}
					if env.Name == buildapi.CustomBuildStrategyBaseImageKey {
						foundCustomEnv = true
						o.Expect(env.Value).To(o.ContainSubstring("@sha256:"))
					}
				}
				o.Expect(foundBuildEnv).To(o.BeTrue(), "did not find BUILD env in build pod %#v", pod)
				o.Expect(foundCustomEnv).To(o.BeTrue(), "did not find Custom base image env in build pod %#v", pod)
			})
		})
	})
})

package builds

import (
	"context"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	admissionapi "k8s.io/pod-security-admission/api"

	buildv1 "github.com/openshift/api/build/v1"
	exutil "github.com/openshift/origin/test/extended/util"
)

var (
	// Decoder understands groupified and non-groupfied.  It deals in internals for now, but will be updated later
	Decoder runtime.Decoder

	// EncoderScheme can identify types for serialization. We use this for the event recorder and other things that need to
	// identify external kinds.
	EncoderScheme = runtime.NewScheme()
	// Encoder always encodes to groupfied.
	Encoder runtime.Encoder
)

func init() {
	annotationDecodingScheme := runtime.NewScheme()
	utilruntime.Must(buildv1.Install(annotationDecodingScheme))
	utilruntime.Must(buildv1.DeprecatedInstallWithoutGroup(annotationDecodingScheme))
	annotationDecoderCodecFactory := serializer.NewCodecFactory(annotationDecodingScheme)
	Decoder = annotationDecoderCodecFactory.UniversalDecoder(buildv1.GroupVersion)

	utilruntime.Must(buildv1.Install(EncoderScheme))
	annotationEncoderCodecFactory := serializer.NewCodecFactory(EncoderScheme)
	Encoder = annotationEncoderCodecFactory.LegacyCodec(buildv1.GroupVersion)
}

var _ = g.Describe("[sig-builds][Feature:Builds][Slow] build can have container image source", func() {
	defer g.GinkgoRecover()
	var (
		buildConfigFixture = exutil.FixturePath("testdata", "builds", "test-imagesource-buildconfig.yaml")
		s2iBuildFixture    = exutil.FixturePath("testdata", "builds", "test-imageresolution-s2i-build.yaml")
		dockerBuildFixture = exutil.FixturePath("testdata", "builds", "test-imageresolution-docker-build.yaml")
		customBuildFixture = exutil.FixturePath("testdata", "builds", "test-imageresolution-custom-build.yaml")
		oc                 = exutil.NewCLIWithPodSecurityLevel("build-image-source", admissionapi.LevelRestricted)
		imageSourceLabel   = exutil.ParseLabelsOrDie("app=imagesourceapp")
		imageDockerLabel   = exutil.ParseLabelsOrDie("app=imagedockerapp")
		sourceBuildLabel   = exutil.ParseLabelsOrDie("openshift.io/build.name=imagesourcebuild")
		dockerBuildLabel   = exutil.ParseLabelsOrDie("openshift.io/build.name=imagedockerbuild")
		customBuildLabel   = exutil.ParseLabelsOrDie("openshift.io/build.name=imagecustombuild")
	)

	g.Context("[apigroup:image.openshift.io]", func() {
		g.BeforeEach(func() {
			exutil.PreTestDump()
		})

		g.JustBeforeEach(func() {
			g.By("waiting for imagestreams to be imported")
			err := exutil.WaitForAnImageStream(oc.AdminImageClient().ImageV1().ImageStreams("openshift"), "ruby", exutil.CheckImageStreamLatestTagPopulated, exutil.CheckImageStreamTagNotFound)
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.AfterEach(func() {
			if g.CurrentSpecReport().Failed() {
				exutil.DumpPodStates(oc)
				exutil.DumpConfigMapStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.Describe("buildconfig with input source image and s2i strategy", func() {
			g.It("should complete successfully and contain the expected file [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
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
				pods, err := exutil.WaitForPods(oc.KubeClient().CoreV1().Pods(oc.Namespace()), imageSourceLabel, exutil.CheckPodIsRunning, 1, 4*time.Minute)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(len(pods)).To(o.Equal(1))
				pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Get(context.Background(), pods[0], metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("expecting the pod to contain the file from the input image")
				out, err := oc.Run("exec").Args(pod.Name, "-c", pod.Spec.Containers[0].Name, "--", "ls", "-R", "-l", "injected/opt/app-root").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(out).To(o.ContainSubstring("bin -> ../../rh/6/root/usr/bin"))
			})
		})
		g.Describe("buildconfig with input source image and docker strategy", func() {
			g.It("should complete successfully and contain the expected file [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
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
				pods, err := exutil.WaitForPods(oc.KubeClient().CoreV1().Pods(oc.Namespace()), imageDockerLabel, exutil.CheckPodIsRunning, 1, 4*time.Minute)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(len(pods)).To(o.Equal(1))
				pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Get(context.Background(), pods[0], metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("expecting the pod to contain the file from the input image")
				out, err := oc.Run("exec").Args(pod.Name, "-c", pod.Spec.Containers[0].Name, "--", "ls", "-R", "-l", "injected/opt/app-root").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(out).To(o.ContainSubstring("bin -> ../../rh/6/root/usr/bin"))
			})
		})
		g.Describe("creating a build with an input source image and s2i strategy", func() {
			g.It("should resolve the imagestream references and secrets [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
				g.By("Creating build configs for input image")
				err := oc.Run("create").Args("-f", buildConfigFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("starting building the private input image")
				br, err := exutil.StartBuildAndWait(oc, "inputimage")
				br.AssertSuccess()

				g.By("Creating a build for source build")
				err = oc.Run("create").Args("-f", s2iBuildFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("expecting the build pod to exist")
				pods, err := exutil.WaitForPods(oc.KubeClient().CoreV1().Pods(oc.Namespace()), sourceBuildLabel, exutil.CheckPodNoOp, 1, 4*time.Minute)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(len(pods)).To(o.Equal(1))
				pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Get(context.Background(), pods[0], metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				foundEnv := false
				for _, env := range pod.Spec.Containers[0].Env {
					if env.Name == "BUILD" {
						foundEnv = true

						obj, err := runtime.Decode(Decoder, []byte(env.Value))
						o.Expect(err).NotTo(o.HaveOccurred())
						build, ok := obj.(*buildv1.Build)
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
			g.It("should resolve the imagestream references and secrets [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
				g.By("Creating build configs for input image")
				err := oc.Run("create").Args("-f", buildConfigFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("starting building the private input image")
				br, err := exutil.StartBuildAndWait(oc, "inputimage")
				br.AssertSuccess()

				g.By("Creating a build for docker build")
				err = oc.Run("create").Args("-f", dockerBuildFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("expecting the build pod to exist")
				pods, err := exutil.WaitForPods(oc.KubeClient().CoreV1().Pods(oc.Namespace()), dockerBuildLabel, exutil.CheckPodNoOp, 1, 4*time.Minute)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(len(pods)).To(o.Equal(1))
				pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Get(context.Background(), pods[0], metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				foundEnv := false
				for _, env := range pod.Spec.Containers[0].Env {
					if env.Name == "BUILD" {
						foundEnv = true

						obj, err := runtime.Decode(Decoder, []byte(env.Value))
						o.Expect(err).NotTo(o.HaveOccurred())
						build, ok := obj.(*buildv1.Build)
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
			g.It("should resolve the imagestream references and secrets [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
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
				pods, err := exutil.WaitForPods(oc.KubeClient().CoreV1().Pods(oc.Namespace()), customBuildLabel, exutil.CheckPodNoOp, 1, 4*time.Minute)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(len(pods)).To(o.Equal(1))
				pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Get(context.Background(), pods[0], metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				foundBuildEnv := false
				foundCustomEnv := false
				for _, env := range pod.Spec.Containers[0].Env {
					if env.Name == "BUILD" {
						foundBuildEnv = true

						obj, err := runtime.Decode(Decoder, []byte(env.Value))
						o.Expect(err).NotTo(o.HaveOccurred())
						build, ok := obj.(*buildv1.Build)
						o.Expect(ok).To(o.BeTrue(), "could not convert build env\n %s\n to a build object (%+v)", env.Value, obj)
						o.Expect(build.Spec.Strategy.CustomStrategy.From.Kind).To(o.Equal("DockerImage"))
						o.Expect(build.Spec.Strategy.CustomStrategy.From.Name).To(o.ContainSubstring("@sha256:"))
						o.Expect(build.Spec.Strategy.CustomStrategy.PullSecret).NotTo(o.BeNil())

						o.Expect(build.Spec.Source.Images[0].From.Kind).To(o.Equal("DockerImage"))
						o.Expect(build.Spec.Source.Images[0].From.Name).To(o.ContainSubstring("@sha256:"))
						o.Expect(build.Spec.Source.Images[0].PullSecret).NotTo(o.BeNil())

						o.Expect(build.Spec.Output.To.Kind).To(o.Equal("DockerImage"))
						o.Expect(build.Spec.Output.PushSecret).NotTo(o.BeNil())
					}
					if env.Name == buildv1.CustomBuildStrategyBaseImageKey {
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

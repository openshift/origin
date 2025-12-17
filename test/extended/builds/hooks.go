package builds

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
	admissionapi "k8s.io/pod-security-admission/api"
)

var _ = g.Describe("[sig-builds][Feature:Builds][Slow] testing build configuration hooks", func() {
	defer g.GinkgoRecover()
	var (
		dockerBuildFixture = exutil.FixturePath("testdata", "builds", "build-postcommit", "docker.yaml")
		s2iBuildFixture    = exutil.FixturePath("testdata", "builds", "build-postcommit", "sti.yaml")
		imagestreamFixture = exutil.FixturePath("testdata", "builds", "build-postcommit", "imagestreams.yaml")
		oc                 = exutil.NewCLIWithPodSecurityLevel("cli-test-hooks", admissionapi.LevelBaseline)
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

		g.Describe("testing postCommit hook", func() {

			g.It("should run s2i postCommit hooks [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
				oc.Run("create").Args("-f", imagestreamFixture).Execute()
				oc.Run("create").Args("-f", s2iBuildFixture).Execute()

				g.By("successfully running a script with args")
				err := oc.Run("patch").Args("bc/mys2itest", "-p", `{"spec":{"postCommit":{"script":"echo hello $1","args":["world"],"command":null}}}`).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				br, _ := exutil.StartBuildAndWait(oc, "mys2itest")
				br.AssertSuccess()
				o.Expect(br.Logs()).To(o.ContainSubstring("hello world"))

				g.By("successfuly running an explicit command")
				err = oc.Run("patch").Args("bc/mys2itest", "-p", `{"spec":{"postCommit":{"command":["sh","-c"],"args":["echo explicit command"],"script":""}}}`).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				br, _ = exutil.StartBuildAndWait(oc, "mys2itest")
				br.AssertSuccess()
				o.Expect(br.Logs()).To(o.ContainSubstring("explicit command"))

				g.By("successfuly modifying the default entrypoint")
				err = oc.Run("patch").Args("bc/mys2itest", "-p", `{"spec":{"postCommit":{"args":["echo","default entrypoint"],"command":null,"script":""}}}`).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				br, _ = exutil.StartBuildAndWait(oc, "mys2itest")
				br.AssertSuccess()
				o.Expect(br.Logs()).To(o.ContainSubstring("default entrypoint"))

				g.By("running a failing script")
				err = oc.Run("patch").Args("bc/mys2itest", "-p", `{"spec":{"postCommit":{"script":"echo about to fail && false","args":null,"command":null}}}`).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				br, _ = exutil.StartBuildAndWait(oc, "mys2itest")
				br.AssertFailure()
				o.Expect(br.Logs()).To(o.ContainSubstring("about to fail"))

				g.By("running a failing explicit command")
				err = oc.Run("patch").Args("bc/mys2itest", "-p", `{"spec":{"postCommit":{"command":["sh","-c"],"args":["echo about to fail && false"],"script":""}}}`).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				br, _ = exutil.StartBuildAndWait(oc, "mys2itest")
				br.AssertFailure()
				o.Expect(br.Logs()).To(o.ContainSubstring("about to fail"))

				g.By("failing default entrypoint")
				err = oc.Run("patch").Args("bc/mys2itest", "-p", `{"spec":{"postCommit":{"args":["sh","-c","echo about to fail && false"],"command":null,"script":""}}}`).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				br, _ = exutil.StartBuildAndWait(oc, "mys2itest")
				br.AssertFailure()
				o.Expect(br.Logs()).To(o.ContainSubstring("about to fail"))

				g.By("not modifying the final image")
				g.By("patching the build config")
				err = oc.Run("patch").Args("bc/mys2itest", "-p", `{"spec":{"postCommit":{"script":"","args":["/tmp/postCommit"],"command":["touch"]}}}`).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = oc.Run("patch").Args("bc/mys2itest", "-p", `{"spec":{"output":{"to":{"kind":"ImageStreamTag","name":"mys2itest:latest"}}}}`).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("starting a build")
				br, _ = exutil.StartBuildAndWait(oc, "mys2itest")
				br.AssertSuccess()

				g.By("expecting the pod to deploy successfully")
				deploymentConfigLabel := exutil.ParseLabelsOrDie("app=mys2itest")
				pods, err := exutil.WaitForPods(oc.KubeClient().CoreV1().Pods(oc.Namespace()), deploymentConfigLabel, exutil.CheckPodIsRunning, 1, 2*time.Minute)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(len(pods)).To(o.Equal(1))

				g.By("getting the pod information")
				pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Get(context.Background(), pods[0], metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("verifying the postCommit hook did not modify the final image")
				out, err := oc.Run("exec").Args(pod.Name, "-c", pod.Spec.Containers[0].Name, "--", "ls", "/tmp/postCommit").Output()
				o.Expect(err).To(o.HaveOccurred())
				o.Expect(out).To(o.ContainSubstring("No such file or directory"))

			})

			g.It("should run docker postCommit hooks [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
				oc.Run("create").Args("-f", imagestreamFixture).Execute()
				oc.Run("create").Args("-f", dockerBuildFixture).Execute()

				g.By("successfully running a script with args")
				err := oc.Run("patch").Args("bc/mydockertest", "-p", `{"spec":{"postCommit":{"script":"echo hello $1","args":["world"],"command":null}}}`).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				br, _ := exutil.StartBuildAndWait(oc, "mydockertest")
				br.AssertSuccess()
				o.Expect(br.Logs()).To(o.ContainSubstring("hello world"))

				g.By("successfuly running an explicit command")
				err = oc.Run("patch").Args("bc/mydockertest", "-p", `{"spec":{"postCommit":{"command":["sh","-c"],"args":["echo explicit command"],"script":""}}}`).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				br, _ = exutil.StartBuildAndWait(oc, "mydockertest")
				br.AssertSuccess()
				o.Expect(br.Logs()).To(o.ContainSubstring("explicit command"))

				g.By("successfuly modifying the default entrypoint")
				err = oc.Run("patch").Args("bc/mydockertest", "-p", `{"spec":{"postCommit":{"args":["echo","default entrypoint"],"command":null,"script":""}}}`).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				br, _ = exutil.StartBuildAndWait(oc, "mydockertest")
				br.AssertSuccess()
				o.Expect(br.Logs()).To(o.ContainSubstring("default entrypoint"))

				g.By("running a failing script")
				err = oc.Run("patch").Args("bc/mydockertest", "-p", `{"spec":{"postCommit":{"script":"echo about to fail && false","args":null,"command":null}}}`).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				br, _ = exutil.StartBuildAndWait(oc, "mydockertest")
				br.AssertFailure()
				o.Expect(br.Logs()).To(o.ContainSubstring("about to fail"))

				g.By("running a failing explicit command")
				err = oc.Run("patch").Args("bc/mydockertest", "-p", `{"spec":{"postCommit":{"command":["sh","-c"],"args":["echo about to fail && false"],"script":""}}}`).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				br, _ = exutil.StartBuildAndWait(oc, "mydockertest")
				br.AssertFailure()
				o.Expect(br.Logs()).To(o.ContainSubstring("about to fail"))

				g.By("failing default entrypoint")
				err = oc.Run("patch").Args("bc/mydockertest", "-p", `{"spec":{"postCommit":{"args":["sh","-c","echo about to fail && false"],"command":null,"script":""}}}`).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				br, _ = exutil.StartBuildAndWait(oc, "mydockertest")
				br.AssertFailure()
				o.Expect(br.Logs()).To(o.ContainSubstring("about to fail"))

				g.By("not modifying the final image")
				g.By("patching the build config")
				err = oc.Run("patch").Args("bc/mydockertest", "-p", `{"spec":{"postCommit":{"script":"","args":["/tmp/postCommit"],"command":["touch"]}}}`).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = oc.Run("patch").Args("bc/mydockertest", "-p", `{"spec":{"output":{"to":{"kind":"ImageStreamTag","name":"mydockertest:latest"}}}}`).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = oc.Run("patch").Args("bc/mydockertest", "-p", fmt.Sprintf(`{"spec":{"source":{"dockerfile":"FROM %s \n ENTRYPOINT /bin/sleep 600 \n"}}}`, image.ShellImage())).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("starting a build")
				br, _ = exutil.StartBuildAndWait(oc, "mydockertest")
				br.AssertSuccess()

				g.By("expecting the pod to deploy successfully")
				deploymentConfigLabel := exutil.ParseLabelsOrDie("app=mydockertest")
				pods, err := exutil.WaitForPods(oc.KubeClient().CoreV1().Pods(oc.Namespace()), deploymentConfigLabel, exutil.CheckPodIsRunning, 1, 2*time.Minute)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(len(pods)).To(o.Equal(1))

				g.By("getting the pod information")
				pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Get(context.Background(), pods[0], metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("verifying the postCommit hook did not modify the final image")
				out, err := oc.Run("exec").Args(pod.Name, "-c", pod.Spec.Containers[0].Name, "--", "ls", "/tmp/postCommit").Output()
				o.Expect(err).To(o.HaveOccurred())
				o.Expect(out).To(o.ContainSubstring("No such file or directory"))

			})
		})
	})
})

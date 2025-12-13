package builds

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	imageeco "github.com/openshift/origin/test/extended/image_ecosystem"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	admissionapi "k8s.io/pod-security-admission/api"
)

var _ = g.Describe("[sig-builds][Feature:Builds][Slow] builds with a context directory", func() {
	defer g.GinkgoRecover()
	var (
		appFixture            = exutil.FixturePath("testdata", "builds", "test-context-build.json")
		oc                    = exutil.NewCLIWithPodSecurityLevel("contextdir", admissionapi.LevelBaseline)
		s2iBuildConfigName    = "s2icontext"
		s2iBuildName          = "s2icontext-1"
		dcName                = "frontend"
		deploymentName        = "frontend-1"
		dcLabel               = exutil.ParseLabelsOrDie(fmt.Sprintf("deployment=%s", deploymentName))
		serviceName           = "frontend"
		dockerBuildConfigName = "dockercontext"
		dockerBuildName       = "dockercontext-1"
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

		g.Describe("s2i context directory build", func() {
			g.It(fmt.Sprintf("should s2i build an application using a context directory [apigroup:build.openshift.io]"), g.Label("Size:L"), func() {

				exutil.WaitForImageStreamImport(oc)
				exutil.WaitForOpenShiftNamespaceImageStreams(oc)

				g.By(fmt.Sprintf("calling oc create -f %q", appFixture))
				err := oc.Run("create").Args("-f", appFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("starting a build")
				err = oc.Run("start-build").Args(s2iBuildConfigName).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("waiting for build to finish")
				err = exutil.WaitForABuild(oc.BuildClient().BuildV1().Builds(oc.Namespace()), s2iBuildName, exutil.CheckBuildSuccess, exutil.CheckBuildFailed, nil)
				if err != nil {
					exutil.DumpBuildLogs("s2icontext", oc)
				}
				o.Expect(err).NotTo(o.HaveOccurred())

				// oc.KubeFramework().WaitForAnEndpoint currently will wait forever;  for now, prefacing with our WaitForADeploymentToComplete,
				// which does have a timeout, since in most cases a failure in the service coming up stems from a failed deployment
				g.By("waiting for a deployment")
				err = exutil.WaitForDeploymentConfig(oc.KubeClient(), oc.AppsClient().AppsV1(), oc.Namespace(), dcName, 1, true, oc)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("waiting for endpoint")
				err = exutil.WaitForEndpoint(oc.KubeFramework().ClientSet, oc.Namespace(), serviceName)
				o.Expect(err).NotTo(o.HaveOccurred())

				assertPageContent := func(content string) {
					_, err := exutil.WaitForPods(oc.KubeClient().CoreV1().Pods(oc.Namespace()), dcLabel, exutil.CheckPodIsRunning, 1, 4*time.Minute)
					o.Expect(err).NotTo(o.HaveOccurred())

					result, err := imageeco.CheckPageContains(oc, "frontend", "", content)
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(result).To(o.BeTrue())
				}

				g.By("testing application content")
				assertPageContent("Hello world!")

				g.By("checking the pod count")
				pods, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).List(context.Background(), metav1.ListOptions{LabelSelector: dcLabel.String()})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(len(pods.Items)).To(o.Equal(1))

				g.By("expecting the pod not to contain two copies of the source")
				pod := pods.Items[0]
				out, err := oc.Run("exec").Args(pod.Name, "-c", pod.Spec.Containers[0].Name, "--", "ls", "/opt/app-root/src").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(out).NotTo(o.ContainSubstring("2.3"))
			})
		})

		g.Describe("docker context directory build", func() {
			g.It(fmt.Sprintf("should docker build an application using a context directory [apigroup:build.openshift.io]"), g.Label("Size:L"), func() {
				g.By("initializing local repo")
				repo, err := exutil.NewGitRepo("contextdir")
				o.Expect(err).NotTo(o.HaveOccurred())
				defer repo.Remove()
				err = repo.AddAndCommit("2.3/Dockerfile", fmt.Sprintf("FROM %s", image.ShellImage()))
				o.Expect(err).NotTo(o.HaveOccurred())

				exutil.WaitForOpenShiftNamespaceImageStreams(oc)
				g.By(fmt.Sprintf("calling oc create -f %q", appFixture))
				err = oc.Run("create").Args("-f", appFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("starting a build")
				err = oc.Run("start-build").Args(dockerBuildConfigName, "--from-repo", repo.RepoPath).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				// build will fail if we don't use the right context dir because there won't be a dockerfile present.
				g.By("waiting for build to finish")
				err = exutil.WaitForABuild(oc.BuildClient().BuildV1().Builds(oc.Namespace()), dockerBuildName, exutil.CheckBuildSuccess, exutil.CheckBuildFailed, nil)
				if err != nil {
					exutil.DumpBuildLogs("dockercontext", oc)
				}
				o.Expect(err).NotTo(o.HaveOccurred())
			})
		})
	})
})

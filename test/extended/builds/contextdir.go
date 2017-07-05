package builds

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	imageeco "github.com/openshift/origin/test/extended/image_ecosystem"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[builds][Slow] builds with a context directory", func() {
	defer g.GinkgoRecover()
	var (
		appFixture            = exutil.FixturePath("testdata", "test-context-build.json")
		oc                    = exutil.NewCLI("contextdir", exutil.KubeConfigPath())
		s2iBuildConfigName    = "s2icontext"
		s2iBuildName          = "s2icontext-1"
		dcName                = "frontend"
		deploymentName        = "frontend-1"
		dcLabel               = exutil.ParseLabelsOrDie(fmt.Sprintf("deployment=%s", deploymentName))
		serviceName           = "frontend"
		dockerBuildConfigName = "dockercontext"
		dockerBuildName       = "dockercontext-1"
	)
	g.Describe("s2i context directory build", func() {
		g.It(fmt.Sprintf("should s2i build an application using a context directory"), func() {
			oc.SetOutputDir(exutil.TestContext.OutputDir)

			exutil.CheckOpenShiftNamespaceImageStreams(oc)
			g.By(fmt.Sprintf("calling oc create -f %q", appFixture))
			err := oc.Run("create").Args("-f", appFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting a build")
			err = oc.Run("start-build").Args(s2iBuildConfigName).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for build to finish")
			err = exutil.WaitForABuild(oc.Client().Builds(oc.Namespace()), s2iBuildName, exutil.CheckBuildSuccessFn, exutil.CheckBuildFailedFn, nil)
			if err != nil {
				exutil.DumpBuildLogs("s2icontext", oc)
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			// oc.KubeFramework().WaitForAnEndpoint currently will wait forever;  for now, prefacing with our WaitForADeploymentToComplete,
			// which does have a timeout, since in most cases a failure in the service coming up stems from a failed deployment
			g.By("waiting for a deployment")
			err = exutil.WaitForDeploymentConfig(oc.KubeClient(), oc.Client(), oc.Namespace(), dcName, 1, oc)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for endpoint")
			err = e2e.WaitForEndpoint(oc.KubeFramework().ClientSet, oc.Namespace(), serviceName)
			o.Expect(err).NotTo(o.HaveOccurred())

			assertPageContent := func(content string) {
				_, err := exutil.WaitForPods(oc.KubeClient().Core().Pods(oc.Namespace()), dcLabel, exutil.CheckPodIsRunningFn, 1, 2*time.Minute)
				o.Expect(err).NotTo(o.HaveOccurred())

				result, err := imageeco.CheckPageContains(oc, "frontend", "", content)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(result).To(o.BeTrue())
			}

			g.By("testing application content")
			assertPageContent("Hello world!")

			g.By("checking the pod count")
			pods, err := oc.KubeClient().Core().Pods(oc.Namespace()).List(metav1.ListOptions{LabelSelector: dcLabel.String()})
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
		g.It(fmt.Sprintf("should docker build an application using a context directory"), func() {
			oc.SetOutputDir(exutil.TestContext.OutputDir)

			exutil.CheckOpenShiftNamespaceImageStreams(oc)
			g.By(fmt.Sprintf("calling oc create -f %q", appFixture))
			err := oc.Run("create").Args("-f", appFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting a build")
			err = oc.Run("start-build").Args(dockerBuildConfigName).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			// build will fail if we don't use the right context dir because there won't be a dockerfile present.
			g.By("waiting for build to finish")
			err = exutil.WaitForABuild(oc.Client().Builds(oc.Namespace()), dockerBuildName, exutil.CheckBuildSuccessFn, exutil.CheckBuildFailedFn, nil)
			if err != nil {
				exutil.DumpBuildLogs("dockercontext", oc)
			}
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})
})

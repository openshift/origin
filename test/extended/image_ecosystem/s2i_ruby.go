package image_ecosystem

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-devex][Feature:ImageEcosystem][ruby][Slow] hot deploy for openshift ruby image", func() {
	defer g.GinkgoRecover()
	var (
		railsTemplate  = "rails-postgresql-example"
		oc             = exutil.NewCLI("s2i-ruby")
		modifyCommand  = []string{"sed", "-ie", `s%render :file => 'public/index.html'%%`, "app/controllers/welcome_controller.rb"}
		removeCommand  = []string{"rm", "-f", "public/index.html"}
		deploymentName = "rails-postgresql-example"
		podLabel       = exutil.ParseLabelsOrDie(fmt.Sprintf("name=%s", deploymentName))
	)

	g.Context("", func() {
		g.JustBeforeEach(func() {
			exutil.PreTestDump()
		})

		g.AfterEach(func() {
			if g.CurrentSpecReport().Failed() {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.Describe("Rails example", func() {
			g.It(fmt.Sprintf("should work with hot deploy [apigroup:image.openshift.io][apigroup:operator.openshift.io][apigroup:config.openshift.io][apigroup:build.openshift.io]"), g.Label("Size:L"), func() {
				exutil.WaitForOpenShiftNamespaceImageStreams(oc)
				g.By(fmt.Sprintf("calling oc new-app %q", railsTemplate))
				err := oc.Run("new-app").Args(railsTemplate).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("waiting for build to finish")
				// This example seems to be taking quite some time, so let's use custom timeouts
				err = exutil.WaitForABuildWithTimeout(oc.BuildClient().BuildV1().Builds(oc.Namespace()), deploymentName+"-1", 5*time.Minute, 15*time.Minute, nil, nil, nil)
				if err != nil {
					exutil.DumpBuildLogs(deploymentName, oc)
				}

				o.Expect(err).NotTo(o.HaveOccurred())

				err = exutil.WaitForDeploymentReadyWithTimeout(oc, deploymentName, oc.Namespace(), -1, 15*time.Minute)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("waiting for endpoint")
				err = exutil.WaitForEndpoint(oc.KubeFramework().ClientSet, oc.Namespace(), deploymentName)
				o.Expect(err).NotTo(o.HaveOccurred())

				assertPageContent := func(content string) {
					_, err := exutil.WaitForPods(oc.KubeClient().CoreV1().Pods(oc.Namespace()), podLabel, exutil.CheckPodIsRunning, 1, 4*time.Minute)
					o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())

					result, err := CheckPageContains(oc, deploymentName, "", content)
					o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())
					o.ExpectWithOffset(1, result).To(o.BeTrue())
				}

				// with hot deploy disabled, making a change to
				// welcome_controller.rb should not affect the app
				g.By("testing application content")
				assertPageContent("Welcome to your Rails application on OpenShift")
				g.By("modifying the source code with disabled hot deploy")
				err = RunInPodContainer(oc, podLabel, modifyCommand)
				o.Expect(err).NotTo(o.HaveOccurred())
				g.By("testing application content source modification")
				assertPageContent("Welcome to your Rails application on OpenShift")

				g.By("turning on hot-deploy")
				err = oc.Run("set", "env").Args("deployment", deploymentName, "RAILS_ENV=development").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = exutil.WaitForDeploymentReady(oc, deploymentName, oc.Namespace(), -1)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("waiting for a new endpoint")
				err = exutil.WaitForEndpoint(oc.KubeFramework().ClientSet, oc.Namespace(), deploymentName)
				o.Expect(err).NotTo(o.HaveOccurred())

				// NOTE: The code below was here when the test was based on DeploymentConfig. The deployments seem to exhibit a different behavior
				// where the deployment is not ready until the pods and endpoints have transitioned to the new replica set. Therefore, I'm commenting this
				// out. If we ever encounter spurious errors here, we can learn from how the situation was handled with the DeploymentConfigs.
				//
				// // Ran into an issue where we'd try to hit the endpoint before it was updated, resulting in
				// // request timeouts against the previous pod's ip.  So make sure the endpoint is pointing to the
				// // new pod before hitting it.
				// err = wait.Poll(1*time.Second, 1*time.Minute, func() (bool, error) {
				// 	newEndpoint, err := oc.KubeFramework().ClientSet.CoreV1().Endpoints(oc.Namespace()).Get(context.Background(), dcName, metav1.GetOptions{})
				// 	if err != nil {
				// 		return false, err
				// 	}
				// 	if !strings.Contains(newEndpoint.Subsets[0].Addresses[0].TargetRef.Name, rcNameTwo) {
				// 		e2e.Logf("waiting on endpoint address ref %s to contain %s", newEndpoint.Subsets[0].Addresses[0].TargetRef.Name, rcNameTwo)
				// 		return false, nil
				// 	}
				// 	e2e.Logf("old endpoint was %#v, new endpoint is %#v", oldEndpoint, newEndpoint)
				// 	return true, nil
				// })
				// o.Expect(err).NotTo(o.HaveOccurred())

				// now hot deploy is enabled, a change to welcome_controller.rb
				// should affect the app
				g.By("modifying the source code with enabled hot deploy")
				assertPageContent("Welcome to your Rails application on OpenShift")
				err = RunInPodContainer(oc, podLabel, modifyCommand)
				o.Expect(err).NotTo(o.HaveOccurred())
				err = RunInPodContainer(oc, podLabel, removeCommand)
				o.Expect(err).NotTo(o.HaveOccurred())
				assertPageContent("Hello, Rails!")
			})
		})
	})
})

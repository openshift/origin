package image_ecosystem

import (
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[image_ecosystem][ruby][Slow] hot deploy for openshift ruby image", func() {
	defer g.GinkgoRecover()
	var (
		railsTemplate = "https://raw.githubusercontent.com/openshift/rails-ex/master/openshift/templates/rails-postgresql.json"
		oc            = exutil.NewCLI("s2i-ruby", exutil.KubeConfigPath())
		modifyCommand = []string{"sed", "-ie", `s%render :file => 'public/index.html'%%`, "app/controllers/welcome_controller.rb"}
		removeCommand = []string{"rm", "-f", "public/index.html"}
		dcName        = "rails-postgresql-example"
		rcNameOne     = fmt.Sprintf("%s-1", dcName)
		rcNameTwo     = fmt.Sprintf("%s-2", dcName)
		dcLabelOne    = exutil.ParseLabelsOrDie(fmt.Sprintf("deployment=%s", rcNameOne))
		dcLabelTwo    = exutil.ParseLabelsOrDie(fmt.Sprintf("deployment=%s", rcNameTwo))
	)

	g.Context("", func() {
		g.BeforeEach(func() {
			exutil.DumpDockerInfo()
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.Describe("Rails example", func() {
			g.It(fmt.Sprintf("should work with hot deploy"), func() {
				oc.SetOutputDir(exutil.TestContext.OutputDir)

				exutil.CheckOpenShiftNamespaceImageStreams(oc)
				g.By(fmt.Sprintf("calling oc new-app -f %q", railsTemplate))
				err := oc.Run("new-app").Args("-f", railsTemplate).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("waiting for build to finish")
				err = exutil.WaitForABuild(oc.BuildClient().Build().Builds(oc.Namespace()), rcNameOne, nil, nil, nil)
				if err != nil {
					exutil.DumpBuildLogs(dcName, oc)
				}
				o.Expect(err).NotTo(o.HaveOccurred())

				err = exutil.WaitForDeploymentConfig(oc.KubeClient(), oc.AppsClient().Apps(), oc.Namespace(), dcName, 1, true, oc)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("waiting for endpoint")
				err = e2e.WaitForEndpoint(oc.KubeFramework().ClientSet, oc.Namespace(), dcName)
				o.Expect(err).NotTo(o.HaveOccurred())
				oldEndpoint, err := oc.KubeFramework().ClientSet.Core().Endpoints(oc.Namespace()).Get(dcName, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				assertPageContent := func(content string, dcLabel labels.Selector) {
					_, err := exutil.WaitForPods(oc.KubeClient().Core().Pods(oc.Namespace()), dcLabel, exutil.CheckPodIsRunning, 1, 4*time.Minute)
					o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())

					result, err := CheckPageContains(oc, dcName, "", content)
					o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())
					o.ExpectWithOffset(1, result).To(o.BeTrue())
				}

				// with hot deploy disabled, making a change to
				// welcome_controller.rb should not affect the app
				g.By("testing application content")
				assertPageContent("Welcome to your Rails application on OpenShift", dcLabelOne)
				g.By("modifying the source code with disabled hot deploy")
				err = RunInPodContainer(oc, dcLabelOne, modifyCommand)
				o.Expect(err).NotTo(o.HaveOccurred())
				g.By("testing application content source modification")
				assertPageContent("Welcome to your Rails application on OpenShift", dcLabelOne)

				pods, err := oc.KubeClient().Core().Pods(oc.Namespace()).List(metav1.ListOptions{LabelSelector: dcLabelOne.String()})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(len(pods.Items)).To(o.Equal(1))

				g.By("turning on hot-deploy")
				err = oc.Run("env").Args("dc", dcName, "RAILS_ENV=development").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = exutil.WaitForDeploymentConfig(oc.KubeClient(), oc.AppsClient().Apps(), oc.Namespace(), dcName, 2, true, oc)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("waiting for a new endpoint")
				err = e2e.WaitForEndpoint(oc.KubeFramework().ClientSet, oc.Namespace(), dcName)
				o.Expect(err).NotTo(o.HaveOccurred())

				// Ran into an issue where we'd try to hit the endpoint before it was updated, resulting in
				// request timeouts against the previous pod's ip.  So make sure the endpoint is pointing to the
				// new pod before hitting it.
				err = wait.Poll(1*time.Second, 1*time.Minute, func() (bool, error) {
					newEndpoint, err := oc.KubeFramework().ClientSet.Core().Endpoints(oc.Namespace()).Get(dcName, metav1.GetOptions{})
					if err != nil {
						return false, err
					}
					if !strings.Contains(newEndpoint.Subsets[0].Addresses[0].TargetRef.Name, rcNameTwo) {
						e2e.Logf("waiting on endpoint address ref %s to contain %s", newEndpoint.Subsets[0].Addresses[0].TargetRef.Name, rcNameTwo)
						return false, nil
					}
					e2e.Logf("old endpoint was %#v, new endpoint is %#v", oldEndpoint, newEndpoint)
					return true, nil
				})
				o.Expect(err).NotTo(o.HaveOccurred())

				// now hot deploy is enabled, a change to welcome_controller.rb
				// should affect the app
				g.By("modifying the source code with enabled hot deploy")
				assertPageContent("Welcome to your Rails application on OpenShift", dcLabelTwo)
				err = RunInPodContainer(oc, dcLabelTwo, modifyCommand)
				o.Expect(err).NotTo(o.HaveOccurred())
				err = RunInPodContainer(oc, dcLabelTwo, removeCommand)
				o.Expect(err).NotTo(o.HaveOccurred())
				assertPageContent("Hello, Rails!", dcLabelTwo)
			})
		})
	})
})

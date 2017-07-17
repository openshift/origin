package image_ecosystem

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
	g.Describe("Rails example", func() {
		g.It(fmt.Sprintf("should work with hot deploy"), func() {
			oc.SetOutputDir(exutil.TestContext.OutputDir)

			exutil.CheckOpenShiftNamespaceImageStreams(oc)
			g.By(fmt.Sprintf("calling oc new-app -f %q", railsTemplate))
			err := oc.Run("new-app").Args("-f", railsTemplate).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for build to finish")
			err = exutil.WaitForABuild(oc.Client().Builds(oc.Namespace()), rcNameOne, nil, nil, nil)
			if err != nil {
				exutil.DumpBuildLogs(dcName, oc)
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			// oc.KubeFramework().WaitForAnEndpoint currently will wait forever;  for now, prefacing with our WaitForADeploymentToComplete,
			// which does have a timeout, since in most cases a failure in the service coming up stems from a failed deployment
			err = exutil.WaitForDeploymentConfig(oc.KubeClient(), oc.Client(), oc.Namespace(), dcName, 1, oc)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for endpoint")
			err = e2e.WaitForEndpoint(oc.KubeFramework().ClientSet, oc.Namespace(), dcName)
			o.Expect(err).NotTo(o.HaveOccurred())

			assertPageContent := func(content string, dcLabel labels.Selector) {
				_, err := exutil.WaitForPods(oc.KubeClient().Core().Pods(oc.Namespace()), dcLabel, exutil.CheckPodIsRunningFn, 1, 2*time.Minute)
				o.Expect(err).NotTo(o.HaveOccurred())

				result, err := CheckPageContains(oc, dcName, "", content)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(result).To(o.BeTrue())
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
			err = exutil.WaitForDeploymentConfig(oc.KubeClient(), oc.Client(), oc.Namespace(), dcName, 2, oc)
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

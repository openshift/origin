package images

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("images: s2i: ruby", func() {
	defer g.GinkgoRecover()
	var (
		railsTemplate = "https://raw.githubusercontent.com/openshift/rails-ex/master/openshift/templates/rails-postgresql.json"
		oc            = exutil.NewCLI("s2i-ruby", exutil.KubeConfigPath())
		modifyCommand = []string{"sed", "-ie", `s%render :file => 'public/index.html'%%`, "app/controllers/welcome_controller.rb"}
		removeCommand = []string{"rm", "-f", "public/index.html"}
		dcName        = "rails-postgresql-example-1"
		dcLabel       = exutil.ParseLabelsOrDie(fmt.Sprintf("deployment=%s", dcName))
	)
	g.Describe("Rails example", func() {
		g.It(fmt.Sprintf("should work with hot deploy"), func() {
			oc.SetOutputDir(exutil.TestContext.OutputDir)

			g.By(fmt.Sprintf("calling oc new-app -f %q", railsTemplate))
			err := oc.Run("new-app").Args("-f", railsTemplate).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for build to finish")
			err = exutil.WaitForABuild(oc.REST().Builds(oc.Namespace()), dcName, exutil.CheckBuildSuccessFunc, exutil.CheckBuildFailedFunc)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for endpoint")
			err = oc.KubeFramework().WaitForAnEndpoint("rails-postgresql-example")
			o.Expect(err).NotTo(o.HaveOccurred())

			assertPageContent := func(content string) {
				_, err := exutil.WaitForPods(oc.KubeREST().Pods(oc.Namespace()), dcLabel, exutil.CheckPodIsRunningFunc, 1, 120*time.Second)
				o.Expect(err).NotTo(o.HaveOccurred())

				result, err := CheckPageContains(oc, "rails-postgresql-example", "", content)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(result).To(o.BeTrue())
			}

			g.By("testing application content")
			assertPageContent("Welcome to your Rails application on OpenShift")
			g.By("modifying the source code with disabled hot deploy")
			RunInPodContainer(oc, dcLabel, modifyCommand)
			RunInPodContainer(oc, dcLabel, removeCommand)
			g.By("testing application content source modification")
			assertPageContent("Welcome to your Rails application on OpenShift")

			pods, err := oc.KubeREST().Pods(oc.Namespace()).List(dcLabel, nil)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(len(pods.Items)).To(o.Equal(1))

			g.By("turning on hot-deploy")
			err = oc.Run("env").Args("rc", dcName, "RAILS_ENV=development").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			err = oc.Run("scale").Args("rc", dcName, "--replicas=0").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			err = exutil.WaitUntilPodIsGone(oc.KubeREST().Pods(oc.Namespace()), pods.Items[0].Name, 60*time.Second)
			o.Expect(err).NotTo(o.HaveOccurred())
			err = oc.Run("scale").Args("rc", dcName, "--replicas=1").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("modifying the source code with enabled hot deploy")
			RunInPodContainer(oc, dcLabel, modifyCommand)
			RunInPodContainer(oc, dcLabel, removeCommand)
			assertPageContent("Hello, Rails!")
		})
	})
})

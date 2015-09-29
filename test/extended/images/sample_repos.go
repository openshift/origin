package images

import (
	"fmt"
	"time"

	"k8s.io/kubernetes/test/e2e"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

type SampleRepoConfig struct {
	repoName             string
	templateURL          string
	buildConfigName      string
	serviceName          string
	deploymentConfigName string
	expectedString       string
	appPath              string
}

// NewSampleRepoTest creates a function for a new ginkgo test case that will instantiate a template
// from a url, kick off the buildconfig defined in that template, wait for the build/deploy,
// and then confirm the application is serving an expected string value.
func NewSampleRepoTest(c SampleRepoConfig) func() {
	return func() {
		defer g.GinkgoRecover()
		var oc = exutil.NewCLI(c.repoName+"-repo-test", exutil.KubeConfigPath())

		g.JustBeforeEach(func() {
			g.By("Waiting for builder service account")
			err := exutil.WaitForBuilderAccount(oc.KubeREST().ServiceAccounts(oc.Namespace()))
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.Describe("Building "+c.repoName+" app from new-app", func() {
			g.It(fmt.Sprintf("should build a "+c.repoName+" image and run it in a pod"), func() {
				oc.SetOutputDir(exutil.TestContext.OutputDir)

				g.By(fmt.Sprintf("calling oc new-app with the " + c.repoName + " example template"))
				err := oc.Run("new-app").Args("-f", c.templateURL).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("starting a build")
				buildName, err := oc.Run("start-build").Args(c.buildConfigName).Output()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("expecting the build is in the Complete phase")
				err = exutil.WaitForABuild(oc.REST().Builds(oc.Namespace()), buildName, exutil.CheckBuildSuccessFunc, exutil.CheckBuildFailedFunc)
				if err != nil {
					logs, _ := oc.Run("build-logs").Args(buildName).Output()
					e2e.Failf("build failed: %s", logs)
				}
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("expecting the deployment to be complete")
				err = exutil.WaitForADeployment(oc.KubeREST().ReplicationControllers(oc.Namespace()), c.deploymentConfigName, exutil.CheckDeploymentCompletedFunc, exutil.CheckDeploymentFailedFunc)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("expecting the service is available")
				serviceIP, err := oc.Run("get").Args("service", c.serviceName).Template("{{ .spec.clusterIP }}").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(serviceIP).ShouldNot(o.Equal(""))

				g.By("expecting an endpoint is available")
				err = oc.KubeFramework().WaitForAnEndpoint(c.serviceName)
				o.Expect(err).NotTo(o.HaveOccurred())

				response, err := exutil.FetchURL("http://"+serviceIP+":8080"+c.appPath, time.Duration(10*time.Second))
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(response).Should(o.ContainSubstring(c.expectedString))
			})
		})
	}
}

var _ = g.Describe("samplerepo: test the sample application repositories", func() {

	g.Describe("samplerepo: images: parallel: test ruby images with rails-ex repo", NewSampleRepoTest(
		SampleRepoConfig{
			"rails",
			"https://raw.githubusercontent.com/openshift/rails-ex/master/openshift/templates/rails-postgresql.json",
			"rails-postgresql-example",
			"rails-postgresql-example",
			"rails-postgresql-example",
			"Listing articles",
			"/articles",
		},
	))

	g.Describe("samplerepo: images: parallel: test python images with django-ex repo", NewSampleRepoTest(
		SampleRepoConfig{
			"django",
			"https://raw.githubusercontent.com/openshift/django-ex/master/openshift/templates/django-postgresql.json",
			"django-psql-example",
			"django-psql-example",
			"django-psql-example",
			"Page views: 1",
			"",
		},
	))

	g.Describe("samplerepo: images: parallel: test nodejs images with nodejs-ex repo", NewSampleRepoTest(
		SampleRepoConfig{
			"nodejs",
			"https://raw.githubusercontent.com/openshift/nodejs-ex/master/openshift/templates/nodejs-mongodb.json",
			"nodejs-mongodb-example",
			"nodejs-mongodb-example",
			"nodejs-mongodb-example",
			"<span class=\"code\" id=\"count-value\">1</span>",
			"",
		},
	))

	var _ = g.Describe("samplerepo: images: parallel: test php images with cakephp-ex repo", NewSampleRepoTest(
		SampleRepoConfig{
			"cakephp",
			"https://raw.githubusercontent.com/openshift/cakephp-ex/master/openshift/templates/cakephp-mysql.json",
			"cakephp-mysql-example",
			"cakephp-mysql-example",
			"cakephp-mysql-example",
			"<span class=\"code\" id=\"count-value\">1</span>",
			"",
		},
	))

	var _ = g.Describe("samplerepo: images: parallel: test perl images with dancer-ex repo", NewSampleRepoTest(
		SampleRepoConfig{
			"dancer",
			"https://raw.githubusercontent.com/openshift/dancer-ex/master/openshift/templates/dancer-mysql.json",
			"dancer-mysql-example",
			"dancer-mysql-example",
			"dancer-mysql-example",
			"<span class=\"code\" id=\"count-value\">1</span>",
			"",
		},
	))

})

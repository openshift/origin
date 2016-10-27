package image_ecosystem

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

type SampleRepoConfig struct {
	repoName               string
	templateURL            string
	buildConfigName        string
	serviceName            string
	deploymentConfigName   string
	expectedString         string
	appPath                string
	dbDeploymentConfigName string
	dbServiceName          string
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

				err := exutil.WaitForOpenShiftNamespaceImageStreams(oc)
				o.Expect(err).NotTo(o.HaveOccurred())
				g.By(fmt.Sprintf("calling oc new-app with the " + c.repoName + " example template"))
				err = oc.Run("new-app").Args("-f", c.templateURL).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				// all the templates automatically start a build.
				buildName := c.buildConfigName + "-1"

				g.By("expecting the build is in the Complete phase")
				err = exutil.WaitForABuild(oc.REST().Builds(oc.Namespace()), buildName, exutil.CheckBuildSuccessFn, exutil.CheckBuildFailedFn)
				if err != nil {
					exutil.DumpBuildLogs(c.buildConfigName, oc)
				}
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("expecting the app deployment to be complete")
				err = exutil.WaitForADeploymentToComplete(oc.KubeREST().ReplicationControllers(oc.Namespace()), c.deploymentConfigName, oc)
				o.Expect(err).NotTo(o.HaveOccurred())

				if len(c.dbDeploymentConfigName) > 0 {
					g.By("expecting the db deployment to be complete")
					err = exutil.WaitForADeploymentToComplete(oc.KubeREST().ReplicationControllers(oc.Namespace()), c.dbDeploymentConfigName, oc)
					o.Expect(err).NotTo(o.HaveOccurred())

					g.By("expecting the db service is available")
					serviceIP, err := oc.Run("get").Args("service", c.dbServiceName).Template("{{ .spec.clusterIP }}").Output()
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(serviceIP).ShouldNot(o.Equal(""))

					g.By("expecting a db endpoint is available")
					err = oc.KubeFramework().WaitForAnEndpoint(c.dbServiceName)
					o.Expect(err).NotTo(o.HaveOccurred())
				}

				g.By("expecting the app service is available")
				serviceIP, err := oc.Run("get").Args("service", c.serviceName).Template("{{ .spec.clusterIP }}").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(serviceIP).ShouldNot(o.Equal(""))

				g.By("expecting an app endpoint is available")
				err = oc.KubeFramework().WaitForAnEndpoint(c.serviceName)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("verifying string from app request")
				response, err := exutil.FetchURL("http://"+serviceIP+":8080"+c.appPath, time.Duration(30*time.Second))
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(response).Should(o.ContainSubstring(c.expectedString))
			})
		})
	}
}

var _ = g.Describe("[image_ecosystem][Slow] openshift sample application repositories", func() {

	g.Describe("[image_ecosystem][ruby] test ruby images with rails-ex db repo", NewSampleRepoTest(
		SampleRepoConfig{
			"rails-postgresql",
			"https://raw.githubusercontent.com/openshift/rails-ex/master/openshift/templates/rails-postgresql.json",
			"rails-postgresql-example",
			"rails-postgresql-example",
			"rails-postgresql-example",
			"Listing articles",
			"/articles",
			"postgresql",
			"postgresql",
		},
	))

	g.Describe("[image_ecosystem][python] test python images with django-ex db repo", NewSampleRepoTest(
		SampleRepoConfig{
			"django-psql",
			"https://raw.githubusercontent.com/openshift/django-ex/master/openshift/templates/django-postgresql.json",
			"django-psql-example",
			"django-psql-example",
			"django-psql-example",
			"Page views: 1",
			"",
			"postgresql",
			"postgresql",
		},
	))

	g.Describe("[image_ecosystem][nodejs] test nodejs images with nodejs-ex db repo", NewSampleRepoTest(
		SampleRepoConfig{
			"nodejs-mongodb",
			"https://raw.githubusercontent.com/openshift/nodejs-ex/master/openshift/templates/nodejs-mongodb.json",
			"nodejs-mongodb-example",
			"nodejs-mongodb-example",
			"nodejs-mongodb-example",
			"<span class=\"code\" id=\"count-value\">1</span>",
			"",
			"mongodb",
			"mongodb",
		},
	))

	var _ = g.Describe("[image_ecosystem][php] test php images with cakephp-ex db repo", NewSampleRepoTest(
		SampleRepoConfig{
			"cakephp-mysql",
			"https://raw.githubusercontent.com/openshift/cakephp-ex/master/openshift/templates/cakephp-mysql.json",
			"cakephp-mysql-example",
			"cakephp-mysql-example",
			"cakephp-mysql-example",
			"<span class=\"code\" id=\"count-value\">1</span>",
			"",
			"mysql",
			"mysql",
		},
	))

	var _ = g.Describe("[image_ecosystem][perl] test perl images with dancer-ex db repo", NewSampleRepoTest(
		SampleRepoConfig{
			"dancer-mysql",
			"https://raw.githubusercontent.com/openshift/dancer-ex/master/openshift/templates/dancer-mysql.json",
			"dancer-mysql-example",
			"dancer-mysql-example",
			"dancer-mysql-example",
			"<span class=\"code\" id=\"count-value\">1</span>",
			"",
			"database",
			"database",
		},
	))

	// test the no-db templates too
	g.Describe("[image_ecosystem][python] test python images with django-ex repo", NewSampleRepoTest(
		SampleRepoConfig{
			"django",
			"https://raw.githubusercontent.com/openshift/django-ex/master/openshift/templates/django.json",
			"django-example",
			"django-example",
			"django-example",
			"Welcome",
			"",
			"",
			"",
		},
	))

	g.Describe("[image_ecosystem][nodejs] images with nodejs-ex repo", NewSampleRepoTest(
		SampleRepoConfig{
			"nodejs",
			"https://raw.githubusercontent.com/openshift/nodejs-ex/master/openshift/templates/nodejs.json",
			"nodejs-example",
			"nodejs-example",
			"nodejs-example",
			"Welcome",
			"",
			"",
			"",
		},
	))

	var _ = g.Describe("[image_ecosystem][php] test php images with cakephp-ex repo", NewSampleRepoTest(
		SampleRepoConfig{
			"cakephp",
			"https://raw.githubusercontent.com/openshift/cakephp-ex/master/openshift/templates/cakephp.json",
			"cakephp-example",
			"cakephp-example",
			"cakephp-example",
			"Welcome",
			"",
			"",
			"",
		},
	))

	var _ = g.Describe("[image_ecosystem][perl] test perl images with dancer-ex repo", NewSampleRepoTest(
		SampleRepoConfig{
			"dancer",
			"https://raw.githubusercontent.com/openshift/dancer-ex/master/openshift/templates/dancer.json",
			"dancer-example",
			"dancer-example",
			"dancer-example",
			"Welcome",
			"",
			"",
			"",
		},
	))

})

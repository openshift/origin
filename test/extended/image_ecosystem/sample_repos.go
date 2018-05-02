package image_ecosystem

import (
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/util/wait"

	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

var htmlCountValueNonZeroRegexp = `<span class="code" id="count-value">[^0][0-9]*</span>`

type sampleRepoConfig struct {
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
func NewSampleRepoTest(c sampleRepoConfig) func() {
	return func() {
		defer g.GinkgoRecover()
		var oc = exutil.NewCLI(c.repoName+"-repo-test", exutil.KubeConfigPath())

		g.Context("", func() {
			g.BeforeEach(func() {
				exutil.DumpDockerInfo()
			})

			g.JustBeforeEach(func() {
				g.By("Waiting for builder service account")
				err := exutil.WaitForBuilderAccount(oc.KubeClient().CoreV1().ServiceAccounts(oc.Namespace()))
				o.Expect(err).NotTo(o.HaveOccurred())
			})

			g.AfterEach(func() {
				if g.CurrentGinkgoTestDescription().Failed {
					exutil.DumpPodStates(oc)
					exutil.DumpPodLogsStartingWith("", oc)
				}
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
					err = exutil.WaitForABuild(oc.BuildClient().Build().Builds(oc.Namespace()), buildName, nil, nil, nil)
					if err != nil {
						exutil.DumpBuildLogs(c.buildConfigName, oc)
					}
					o.Expect(err).NotTo(o.HaveOccurred())

					g.By("expecting the app deployment to be complete")
					err = exutil.WaitForDeploymentConfig(oc.KubeClient(), oc.AppsClient().Apps(), oc.Namespace(), c.deploymentConfigName, 1, true, oc)
					o.Expect(err).NotTo(o.HaveOccurred())

					if len(c.dbDeploymentConfigName) > 0 {
						g.By("expecting the db deployment to be complete")
						err = exutil.WaitForDeploymentConfig(oc.KubeClient(), oc.AppsClient().Apps(), oc.Namespace(), c.dbDeploymentConfigName, 1, true, oc)
						o.Expect(err).NotTo(o.HaveOccurred())

						g.By("expecting the db service is available")
						serviceIP, err := oc.Run("get").Args("service", c.dbServiceName).Template("{{ .spec.clusterIP }}").Output()
						o.Expect(err).NotTo(o.HaveOccurred())
						o.Expect(serviceIP).ShouldNot(o.Equal(""))

						g.By("expecting a db endpoint is available")
						err = e2e.WaitForEndpoint(oc.KubeFramework().ClientSet, oc.Namespace(), c.dbServiceName)
						o.Expect(err).NotTo(o.HaveOccurred())
					}

					g.By("expecting the app service is available")
					serviceIP, err := oc.Run("get").Args("service", c.serviceName).Template("{{ .spec.clusterIP }}").Output()
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(serviceIP).ShouldNot(o.Equal(""))

					g.By("expecting an app endpoint is available")
					err = e2e.WaitForEndpoint(oc.KubeFramework().ClientSet, oc.Namespace(), c.serviceName)
					o.Expect(err).NotTo(o.HaveOccurred())

					g.By("verifying string from app request")
					var response string
					err = wait.Poll(1*time.Second, 2*time.Minute, func() (bool, error) {
						response, err = exutil.FetchURL("http://"+serviceIP+":8080"+c.appPath, time.Duration(1*time.Minute))
						if err != nil {
							o.Expect(err).NotTo(o.HaveOccurred())
						}
						if strings.Contains(response, c.expectedString) {
							return true, nil
						}
						e2e.Logf("url check got %s, expected it to contain %s", response, c.expectedString)
						return false, nil
					})
					o.Expect(response).Should(o.MatchRegexp(c.expectedString))
				})
			})
		})
	}
}

var _ = g.Describe("[image_ecosystem][Slow] openshift sample application repositories", func() {

	g.Describe("[image_ecosystem][ruby] test ruby images with rails-ex db repo", NewSampleRepoTest(
		sampleRepoConfig{
			repoName:               "rails-postgresql",
			templateURL:            "https://raw.githubusercontent.com/openshift/rails-ex/master/openshift/templates/rails-postgresql.json",
			buildConfigName:        "rails-postgresql-example",
			serviceName:            "rails-postgresql-example",
			deploymentConfigName:   "rails-postgresql-example",
			expectedString:         "Listing articles",
			appPath:                "/articles",
			dbDeploymentConfigName: "postgresql",
			dbServiceName:          "postgresql",
		},
	))

	g.Describe("[image_ecosystem][python] test python images with django-ex db repo", NewSampleRepoTest(
		sampleRepoConfig{
			repoName:               "django-psql",
			templateURL:            "https://raw.githubusercontent.com/openshift/django-ex/master/openshift/templates/django-postgresql.json",
			buildConfigName:        "django-psql-example",
			serviceName:            "django-psql-example",
			deploymentConfigName:   "django-psql-example",
			expectedString:         "Page views: 1",
			appPath:                "",
			dbDeploymentConfigName: "postgresql",
			dbServiceName:          "postgresql",
		},
	))

	g.Describe("[image_ecosystem][nodejs] test nodejs images with nodejs-ex db repo", NewSampleRepoTest(
		sampleRepoConfig{
			repoName:               "nodejs-mongodb",
			templateURL:            "https://raw.githubusercontent.com/openshift/nodejs-ex/master/openshift/templates/nodejs-mongodb.json",
			buildConfigName:        "nodejs-mongodb-example",
			serviceName:            "nodejs-mongodb-example",
			deploymentConfigName:   "nodejs-mongodb-example",
			expectedString:         htmlCountValueNonZeroRegexp,
			appPath:                "",
			dbDeploymentConfigName: "mongodb",
			dbServiceName:          "mongodb",
		},
	))

	var _ = g.Describe("[image_ecosystem][php] test php images with cakephp-ex db repo", NewSampleRepoTest(
		sampleRepoConfig{
			repoName:               "cakephp-mysql",
			templateURL:            "https://raw.githubusercontent.com/openshift/cakephp-ex/master/openshift/templates/cakephp-mysql.json",
			buildConfigName:        "cakephp-mysql-example",
			serviceName:            "cakephp-mysql-example",
			deploymentConfigName:   "cakephp-mysql-example",
			expectedString:         htmlCountValueNonZeroRegexp,
			appPath:                "",
			dbDeploymentConfigName: "mysql",
			dbServiceName:          "mysql",
		},
	))

	var _ = g.Describe("[image_ecosystem][perl] test perl images with dancer-ex db repo", NewSampleRepoTest(
		sampleRepoConfig{
			repoName:               "dancer-mysql",
			templateURL:            "https://raw.githubusercontent.com/openshift/dancer-ex/master/openshift/templates/dancer-mysql.json",
			buildConfigName:        "dancer-mysql-example",
			serviceName:            "dancer-mysql-example",
			deploymentConfigName:   "dancer-mysql-example",
			expectedString:         htmlCountValueNonZeroRegexp,
			appPath:                "",
			dbDeploymentConfigName: "database",
			dbServiceName:          "database",
		},
	))

	// test the no-db templates too
	g.Describe("[image_ecosystem][python] test python images with django-ex repo", NewSampleRepoTest(
		sampleRepoConfig{
			repoName:               "django",
			templateURL:            "https://raw.githubusercontent.com/openshift/django-ex/master/openshift/templates/django.json",
			buildConfigName:        "django-example",
			serviceName:            "django-example",
			deploymentConfigName:   "django-example",
			expectedString:         "Welcome",
			appPath:                "",
			dbDeploymentConfigName: "",
			dbServiceName:          "",
		},
	))

	g.Describe("[image_ecosystem][nodejs] images with nodejs-ex repo", NewSampleRepoTest(
		sampleRepoConfig{
			repoName:               "nodejs",
			templateURL:            "https://raw.githubusercontent.com/openshift/nodejs-ex/master/openshift/templates/nodejs.json",
			buildConfigName:        "nodejs-example",
			serviceName:            "nodejs-example",
			deploymentConfigName:   "nodejs-example",
			expectedString:         "Welcome",
			appPath:                "",
			dbDeploymentConfigName: "",
			dbServiceName:          "",
		},
	))

	var _ = g.Describe("[image_ecosystem][php] test php images with cakephp-ex repo", NewSampleRepoTest(
		sampleRepoConfig{
			repoName:               "cakephp",
			templateURL:            "https://raw.githubusercontent.com/openshift/cakephp-ex/master/openshift/templates/cakephp.json",
			buildConfigName:        "cakephp-example",
			serviceName:            "cakephp-example",
			deploymentConfigName:   "cakephp-example",
			expectedString:         "Welcome",
			appPath:                "",
			dbDeploymentConfigName: "",
			dbServiceName:          "",
		},
	))

	var _ = g.Describe("[image_ecosystem][perl] test perl images with dancer-ex repo", NewSampleRepoTest(
		sampleRepoConfig{
			repoName:               "dancer",
			templateURL:            "https://raw.githubusercontent.com/openshift/dancer-ex/master/openshift/templates/dancer.json",
			buildConfigName:        "dancer-example",
			serviceName:            "dancer-example",
			deploymentConfigName:   "dancer-example",
			expectedString:         "Welcome",
			appPath:                "",
			dbDeploymentConfigName: "",
			dbServiceName:          "",
		},
	))

})

package image_ecosystem

import (
	"fmt"
	"regexp"
	"time"

	g "github.com/onsi/ginkgo/v2"
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
	deploymentName         string
	expectedString         string
	appPath                string
	dbDeploymentConfigName string
	dbDeploymentName       string
	dbServiceName          string
	newAppParams           string
}

// NewSampleRepoTest creates a function for a new ginkgo test case that will instantiate a template
// from a url, kick off the buildconfig defined in that template, wait for the build/deploy,
// and then confirm the application is serving an expected string value.
func NewSampleRepoTest(c sampleRepoConfig) func() {
	return func() {
		defer g.GinkgoRecover()
		oc := exutil.NewCLI(c.repoName + "-repo-test")

		g.Context("", func() {
			g.BeforeEach(func() {
				exutil.PreTestDump()
			})

			g.AfterEach(func() {
				if g.CurrentSpecReport().Failed() {
					exutil.DumpPodStates(oc)
					exutil.DumpPodLogsStartingWith("", oc)
				}
			})

			g.Describe("Building "+c.repoName+" app from new-app", func() {
				g.It(fmt.Sprintf("should build a %s image and run it in a pod [apigroup:build.openshift.io]", c.repoName), func() {
					err := exutil.WaitForOpenShiftNamespaceImageStreams(oc)
					o.Expect(err).NotTo(o.HaveOccurred())
					g.By(fmt.Sprintf("calling oc new-app with the %s example template", c.repoName))
					newAppArgs := []string{c.templateURL}
					if len(c.newAppParams) > 0 {
						newAppArgs = append(newAppArgs, "-p")
						c.newAppParams = fmt.Sprintf(c.newAppParams, oc.Namespace())
						newAppArgs = append(newAppArgs, c.newAppParams)
					}
					err = oc.Run("new-app").Args(newAppArgs...).Execute()
					o.Expect(err).NotTo(o.HaveOccurred())

					// all the templates automatically start a build.
					buildName := c.buildConfigName + "-1"

					g.By("expecting the build is in the Complete phase")
					err = exutil.WaitForABuildWithTimeout(oc.BuildClient().BuildV1().Builds(oc.Namespace()), buildName, 5*time.Minute, 15*time.Minute, nil, nil, nil)
					if err != nil {
						exutil.DumpBuildLogs(c.buildConfigName, oc)
					}
					o.Expect(err).NotTo(o.HaveOccurred())

					g.By("expecting the app deployment to be complete")

					if c.deploymentConfigName != "" {
						err = exutil.WaitForDeploymentConfig(oc.KubeClient(), oc.AppsClient().AppsV1(), oc.Namespace(), c.deploymentConfigName, 1, true, oc)
					} else if c.deploymentName != "" {
						err = exutil.WaitForDeploymentReadyWithTimeout(oc, c.deploymentName, oc.Namespace(), -1, 15*time.Minute)
					} else {
						g.Fail("invalid test configuration: neither deploymentConfigName nor deploymentName is set")
					}
					o.Expect(err).NotTo(o.HaveOccurred())

					if len(c.dbDeploymentConfigName) > 0 {
						g.By("expecting the db deployment to be complete")
						if c.dbDeploymentConfigName != "" {
							err = exutil.WaitForDeploymentConfig(oc.KubeClient(), oc.AppsClient().AppsV1(), oc.Namespace(), c.dbDeploymentConfigName, 1, true, oc)
						} else if c.dbDeploymentName != "" {
							err = exutil.WaitForDeploymentReadyWithTimeout(oc, c.dbDeploymentName, oc.Namespace(), -1, 15*time.Minute)
						} else {
							g.Fail("invalid test configuration: neither dbDeploymentConfigName nor dbDeploymentName is set")
						}
						o.Expect(err).NotTo(o.HaveOccurred())

						g.By("expecting the db service is available")
						serviceIP, err := oc.Run("get").Args("service", c.dbServiceName, "--output=template", "--template={{ .spec.clusterIP }}").Output()
						o.Expect(err).NotTo(o.HaveOccurred())
						o.Expect(serviceIP).ShouldNot(o.Equal(""))

						g.By("expecting a db endpoint is available")
						err = exutil.WaitForEndpoint(oc.KubeFramework().ClientSet, oc.Namespace(), c.dbServiceName)
						o.Expect(err).NotTo(o.HaveOccurred())
					}

					g.By("expecting the app service is available")
					serviceIP, err := oc.Run("get").Args("service", c.serviceName, "--output=template", "--template={{ .spec.clusterIP }}").Output()
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(serviceIP).ShouldNot(o.Equal(""))

					g.By("expecting an app endpoint is available")
					err = exutil.WaitForEndpoint(oc.KubeFramework().ClientSet, oc.Namespace(), c.serviceName)
					o.Expect(err).NotTo(o.HaveOccurred())

					g.By("verifying string from app request")
					var response string
					err = wait.Poll(1*time.Second, 2*time.Minute, func() (bool, error) {
						response, err = exutil.FetchURL(oc, "http://"+serviceIP+":8080"+c.appPath, time.Duration(1*time.Minute))
						if err != nil {
							o.Expect(err).NotTo(o.HaveOccurred())
						}
						if match, _ := regexp.MatchString(c.expectedString, response); match {
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

var _ = g.Describe("[sig-devex][Feature:ImageEcosystem][Slow] openshift sample application repositories", func() {
	g.Describe("[sig-devex][Feature:ImageEcosystem][ruby] test ruby images with rails-ex db repo", NewSampleRepoTest(
		sampleRepoConfig{
			repoName:         "rails-postgresql",
			templateURL:      "rails-postgresql-example",
			buildConfigName:  "rails-postgresql-example",
			serviceName:      "rails-postgresql-example",
			deploymentName:   "rails-postgresql-example",
			expectedString:   "Listing articles",
			appPath:          "/articles",
			dbDeploymentName: "postgresql",
			dbServiceName:    "postgresql",
			newAppParams:     "APPLICATION_DOMAIN=rails-%s.ocp.io",
		},
	))

	g.Describe("[sig-devex][Feature:ImageEcosystem][python] test python images with django-ex db repo", NewSampleRepoTest(
		sampleRepoConfig{
			repoName:               "django-psql",
			templateURL:            "django-psql-example",
			buildConfigName:        "django-psql-example",
			serviceName:            "django-psql-example",
			deploymentConfigName:   "django-psql-example",
			expectedString:         "Page views: 1",
			appPath:                "",
			dbDeploymentConfigName: "postgresql",
			dbServiceName:          "postgresql",
			newAppParams:           "APPLICATION_DOMAIN=django-%s.ocp.io",
		},
	))

	g.Describe("[sig-devex][Feature:ImageEcosystem][nodejs] test nodejs images with nodejs-rest-http-crud db repo", NewSampleRepoTest(
		sampleRepoConfig{
			repoName:               "nodejs-postgresql",
			templateURL:            "nodejs-postgresql-example",
			buildConfigName:        "nodejs-postgresql-example",
			serviceName:            "nodejs-postgresql-example",
			deploymentConfigName:   "nodejs-postgresql-example",
			expectedString:         "Fruit List",
			appPath:                "",
			dbDeploymentConfigName: "postgresql",
			dbServiceName:          "postgresql",
			newAppParams:           "APPLICATION_DOMAIN=nodejs-%s.ocp.io",
		},
	))

	_ = g.Describe("[sig-devex][Feature:ImageEcosystem][php] test php images with cakephp-ex db repo", NewSampleRepoTest(
		sampleRepoConfig{
			repoName:               "cakephp-mysql",
			templateURL:            "cakephp-mysql-example",
			buildConfigName:        "cakephp-mysql-example",
			serviceName:            "cakephp-mysql-example",
			deploymentConfigName:   "cakephp-mysql-example",
			expectedString:         htmlCountValueNonZeroRegexp,
			appPath:                "",
			dbDeploymentConfigName: "mysql",
			dbServiceName:          "mysql",
			newAppParams:           "APPLICATION_DOMAIN=cakephp-%s.ocp.io",
		},
	))

	// dependency download is intermittently slow enough to blow away the e2e timeouts
	/*var _ = g.Describe("[sig-devex][Feature:ImageEcosystem][perl] test perl images with dancer-ex db repo", NewSampleRepoTest(
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
	))*/

	// dependency download is intermittently slow enough to blow away the e2e timeouts
	/*var _ = g.Describe("[sig-devex][Feature:ImageEcosystem][perl] test perl images with dancer-ex repo", NewSampleRepoTest(
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
	))*/
})

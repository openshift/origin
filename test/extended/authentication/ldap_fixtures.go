package authentication

import (
	"fmt"
	"regexp"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("authentication: OpenLDAP build and deployment", func() {
	defer g.GinkgoRecover()
	var (
		imageStreamFixture       = exutil.FixturePath("fixtures", "ldap", "ldapserver-imagestream.json")
		imageStreamTargetFixture = exutil.FixturePath("fixtures", "ldap", "ldapserver-imagestream-testenv.json")
		buildConfigFixture       = exutil.FixturePath("fixtures", "ldap", "ldapserver-buildconfig.json")
		deploymentConfigFixture  = exutil.FixturePath("fixtures", "ldap", "ldapserver-deploymentconfig.json")
		serviceConfigFixture     = exutil.FixturePath("fixtures", "ldap", "ldapserver-service.json")
		oc                       = exutil.NewCLI("openldap", exutil.KubeConfigPath())
	)

	g.Describe("Building and deploying an OpenLDAP server", func() {
		g.It(fmt.Sprintf("should create a image from %s template and run it in a pod", buildConfigFixture), func() {
			nameRegex := regexp.MustCompile(`"[A-Za-z0-9\-]+"`)
			oc.SetOutputDir(exutil.TestContext.OutputDir)

			g.By(fmt.Sprintf("calling oc create -f %s", imageStreamFixture))
			imageStreamMessage, err := oc.Run("create").Args("-f", imageStreamFixture).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			imageStreamName := strings.Trim(nameRegex.FindString(imageStreamMessage), `"`)
			g.By("expecting the imagestream to fetch and tag the latest image")
			err = exutil.WaitForAnImageStream(oc.REST().ImageStreams(oc.Namespace()), imageStreamName,
				exutil.CheckImageStreamLatestTagPopulatedFunc, exutil.CheckImageStreamTagNotFoundFunc)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("calling oc create -f %s", imageStreamTargetFixture))
			err = oc.Run("create").Args("-f", imageStreamTargetFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("calling oc create -f %s", buildConfigFixture))
			buildConfigMessage, err := oc.Run("create").Args("-f", buildConfigFixture).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			buildConfigName := strings.Trim(nameRegex.FindString(buildConfigMessage), `"`)
			g.By(fmt.Sprintf("calling oc start-build %s", buildConfigName))
			buildName, err := oc.Run("start-build").Args(buildConfigName).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("expecting the build to be in Complete phase")
			err = exutil.WaitForABuild(oc.REST().Builds(oc.Namespace()), buildName,
				exutil.CheckBuildSuccessFunc, exutil.CheckBuildFailedFunc)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("calling oc create -f %s", deploymentConfigFixture))
			deploymentConfigMessage, err := oc.Run("create").Args("-f", deploymentConfigFixture).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			deploymentConfigName := strings.Trim(nameRegex.FindString(deploymentConfigMessage), `"`)
			g.By(fmt.Sprintf("calling oc deploy %s", deploymentConfigName))
			err = oc.Run("deploy").Args(deploymentConfigName).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("expecting the deployment to be in Complete phase")
			err = exutil.WaitForADeployment(oc.KubeREST().ReplicationControllers(oc.Namespace()), deploymentConfigName,
				exutil.CheckDeploymentCompletedFunc, exutil.CheckDeploymentFailedFunc)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("calling oc create -f %s", serviceConfigFixture))
			err = oc.Run("create").Args("-f", serviceConfigFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})
})

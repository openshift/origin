package builds

import (
	"fmt"
	"net"
	"net/url"
	"path/filepath"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
	testutil "github.com/openshift/origin/test/util"
)

// hostname returns the hostname from a hostport specification
func hostname(hostport string) (string, error) {
	host, _, err := net.SplitHostPort(hostport)
	return host, err
}

var _ = g.Describe("builds: parallel: gitauth: Check build for private source repository access", func() {
	defer g.GinkgoRecover()

	const (
		gitServerDeploymentConfigName = "gitserver"
		sourceSecretName              = "sourcesecret"
		hostNameSuffix                = "xip.io"
		gitUserName                   = "gituser"
		gitPassword                   = "gituserpassword"
		buildConfigName               = "gitauthtest"
		sourceURLTemplate             = "https://gitserver.%s/ruby-hello-world"
	)

	var (
		gitServerFixture = exutil.FixturePath("fixtures", "test-gitserver.yaml")
		testBuildFixture = exutil.FixturePath("fixtures", "test-auth-build.yaml")
		oc               = exutil.NewCLI("build-sti-env", exutil.KubeConfigPath())
		caCertPath       = filepath.Join(filepath.Dir(exutil.KubeConfigPath()), "ca.crt")
	)

	g.JustBeforeEach(func() {
		g.By("waiting for builder service account")
		err := exutil.WaitForBuilderAccount(oc.KubeREST().ServiceAccounts(oc.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.Describe("Build using a username, password, and CA certificate", func() {
		g.It("should create a new build using the internal gitserver", func() {
			oc.SetOutputDir(exutil.TestContext.OutputDir)

			g.By("obtaining the configured API server host from config")
			adminClientConfig, err := testutil.GetClusterAdminClientConfig(exutil.KubeConfigPath())
			o.Expect(err).NotTo(o.HaveOccurred())
			hostURL, err := url.Parse(adminClientConfig.Host)
			o.Expect(err).NotTo(o.HaveOccurred())
			host, err := hostname(hostURL.Host)
			o.Expect(err).NotTo(o.HaveOccurred())
			routeSuffix := fmt.Sprintf("%s.%s", host, hostNameSuffix)

			g.By(fmt.Sprintf("calling oc new-app -f %q -p ROUTE_SUFFIX=%s", gitServerFixture, routeSuffix))
			err = oc.Run("new-app").Args("-f", gitServerFixture, "-p", fmt.Sprintf("ROUTE_SUFFIX=%s", routeSuffix)).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("expecting the deployment of the gitserver to be in the Complete phase")
			err = exutil.WaitForADeployment(oc.KubeREST().ReplicationControllers(oc.Namespace()), gitServerDeploymentConfigName,
				exutil.CheckDeploymentCompletedFunc, exutil.CheckDeploymentFailedFunc)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("creating a new secret for the gitserver by calling oc secrets new-basicauth %s --username=%s --password=%s --cacert=%s",
				sourceSecretName, gitUserName, gitPassword, caCertPath))
			err = oc.Run("secrets").
				Args("new-basicauth", sourceSecretName,
				fmt.Sprintf("--username=%s", gitUserName),
				fmt.Sprintf("--password=%s", gitPassword),
				fmt.Sprintf("--ca-cert=%s", caCertPath)).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			sourceURL := fmt.Sprintf(sourceURLTemplate, routeSuffix)
			g.By(fmt.Sprintf("creating a new BuildConfig by calling oc new-app -f %q -p SOURCE_SECRET=%s,SOURCE_URL=%s",
				testBuildFixture, sourceSecretName, sourceURL))
			err = oc.Run("new-app").Args("-f", testBuildFixture, "-p", fmt.Sprintf("SOURCE_SECRET=%s,SOURCE_URL=%s",
				sourceSecretName, sourceURL)).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting a test build")
			buildName, err := oc.Run("start-build").Args(buildConfigName).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("expecting build %s to complete successfully", buildName))
			err = exutil.WaitForABuild(oc.REST().Builds(oc.Namespace()), buildName, exutil.CheckBuildSuccessFunc, exutil.CheckBuildFailedFunc)
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})
})

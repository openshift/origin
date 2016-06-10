package builds

// these tests are diabled because the xip.io dns hook was proving way too unreliable;
// we will reenable once an agreeable alternative is derived to get name resolution for the routes

/*import (
	"net"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

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

var _ = g.Describe("[builds][Slow] can use private repositories as build input", func() {
	defer g.GinkgoRecover()

	const (
		gitServerDeploymentConfigName = "gitserver"
		sourceSecretName              = "sourcesecret"
		hostNameSuffix                = "xip.io"
		gitUserName                   = "gituser"
		gitPassword                   = "gituserpassword"
		buildConfigName               = "gitauthtest"
		sourceURLTemplate             = "https://gitserver.%s/ruby-hello-world"
		sourceURLTemplateTokenAuth    = "https://gitserver-tokenauth.%s/ruby-hello-world"
	)

	var (
		gitServerFixture          = exutil.FixturePath("testdata", "test-gitserver.yaml")
		gitServerTokenAuthFixture = exutil.FixturePath("testdata", "test-gitserver-tokenauth.yaml")
		testBuildFixture          = exutil.FixturePath("testdata", "test-auth-build.yaml")
		oc                        = exutil.NewCLI("build-sti-private-repo", exutil.KubeConfigPath())
		caCertPath                = filepath.Join(filepath.Dir(exutil.KubeConfigPath()), "ca.crt")
	)

	g.JustBeforeEach(func() {
		g.By("waiting for builder service account")
		err := exutil.WaitForBuilderAccount(oc.KubeREST().ServiceAccounts(oc.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	testGitAuth := func(gitServerYaml, urlTemplate string, secretFunc func() string) {
		oc.SetOutputDir(exutil.TestContext.OutputDir)

		g.By("obtaining the configured API server host from config")
		adminClientConfig, err := testutil.GetClusterAdminClientConfig(exutil.KubeConfigPath())
		o.Expect(err).NotTo(o.HaveOccurred())
		hostURL, err := url.Parse(adminClientConfig.Host)
		o.Expect(err).NotTo(o.HaveOccurred())
		host, err := hostname(hostURL.Host)
		o.Expect(err).NotTo(o.HaveOccurred())
		routeSuffix := fmt.Sprintf("%s.%s", host, hostNameSuffix)

		g.By(fmt.Sprintf("calling oc new-app -f %q -p ROUTE_SUFFIX=%s", gitServerYaml, routeSuffix))
		err = oc.Run("new-app").Args("-f", gitServerYaml, "-p", fmt.Sprintf("ROUTE_SUFFIX=%s", routeSuffix)).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("expecting the deployment of the gitserver to be in the Complete phase")
		err = exutil.WaitForADeploymentToComplete(oc.KubeREST().ReplicationControllers(oc.Namespace()), gitServerDeploymentConfigName)
		o.Expect(err).NotTo(o.HaveOccurred())

		sourceSecretName := secretFunc()

		sourceURL := fmt.Sprintf(urlTemplate, routeSuffix)
		g.By(fmt.Sprintf("creating a new BuildConfig by calling oc new-app -f %q -p SOURCE_SECRET=%s,SOURCE_URL=%s",
			testBuildFixture, sourceSecretName, sourceURL))
		err = oc.Run("new-app").Args("-f", testBuildFixture, "-p", fmt.Sprintf("SOURCE_SECRET=%s,SOURCE_URL=%s",
			sourceSecretName, sourceURL)).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("starting a test build")
		buildName, err := oc.Run("start-build").Args(buildConfigName).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By(fmt.Sprintf("expecting build %s to complete successfully", buildName))
		err = exutil.WaitForABuild(oc.REST().Builds(oc.Namespace()), buildName, exutil.CheckBuildSuccessFn, exutil.CheckBuildFailedFn)
		if err != nil {
			exutil.DumpBuildLogs(buildConfigName, oc)
		}
		o.Expect(err).NotTo(o.HaveOccurred())
	}

	g.Describe("Build using a username, password, and CA certificate", func() {
		g.It("should create a new build using the internal gitserver", func() {
			testGitAuth(gitServerFixture, sourceURLTemplate, func() string {
				g.By(fmt.Sprintf("creating a new secret for the gitserver by calling oc secrets new-basicauth %s --username=%s --password=%s --cacert=%s",
					sourceSecretName, gitUserName, gitPassword, caCertPath))
				err := oc.Run("secrets").Args(
					"new-basicauth", sourceSecretName,
					fmt.Sprintf("--username=%s", gitUserName),
					fmt.Sprintf("--password=%s", gitPassword),
					fmt.Sprintf("--ca-cert=%s", caCertPath),
				).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				return sourceSecretName
			})
		})
	})

	g.Describe("Build using a service account token and CA certificate", func() {
		g.It("should create a new build using the internal gitserver", func() {
			testGitAuth(gitServerTokenAuthFixture, sourceURLTemplateTokenAuth, func() string {
				g.By("assigning the edit role to the builder service account")
				err := oc.Run("policy").Args("add-role-to-user", "edit", "--serviceaccount=builder").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("getting the token secret name for the builder service account")
				sa, err := oc.KubeREST().ServiceAccounts(oc.Namespace()).Get("builder")
				o.Expect(err).NotTo(o.HaveOccurred())
				for _, s := range sa.Secrets {
					if strings.Contains(s.Name, "token") {
						return s.Name
					}
				}
				return ""
			})
		})
	})
})*/

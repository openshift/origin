package builds

import (
	"fmt"
	"net"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	exutil "github.com/openshift/origin/test/extended/util"
)

// hostname returns the hostname from a hostport specification
func hostname(hostport string) (string, error) {
	host, _, err := net.SplitHostPort(hostport)
	return host, err
}

var _ = g.Describe("[Feature:Builds][Slow] can use private repositories as build input", func() {
	defer g.GinkgoRecover()

	const (
		gitServerDeploymentConfigName = "gitserver"
		sourceSecretName              = "sourcesecret"
		gitUserName                   = "gituser"
		gitPassword                   = "gituserpassword"
		buildConfigName               = "gitauthtest"
		sourceURLTemplate             = "https://%s/ruby-hello-world"
	)

	var (
		gitServerFixture          = exutil.FixturePath("testdata", "test-gitserver.yaml")
		gitServerTokenAuthFixture = exutil.FixturePath("testdata", "test-gitserver-tokenauth.yaml")
		testBuildFixture          = exutil.FixturePath("testdata", "builds", "test-auth-build.yaml")
		oc                        = exutil.NewCLI("build-sti-private-repo", exutil.KubeConfigPath())
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

		g.JustBeforeEach(func() {
			g.By("waiting for default service account")
			err := exutil.WaitForServiceAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()), "default")
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By("waiting for builder service account")
			err = exutil.WaitForServiceAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()), "builder")
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		testGitAuth := func(routeName, gitServerYaml, urlTemplate string, secretFunc func() string) {

			err := oc.Run("new-app").Args("-f", gitServerYaml).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("expecting the deployment of the gitserver to be in the Complete phase")
			err = exutil.WaitForDeploymentConfig(oc.KubeClient(), oc.AppsClient().AppsV1(), oc.Namespace(), gitServerDeploymentConfigName, 1, true, oc)
			o.Expect(err).NotTo(o.HaveOccurred())

			sourceSecretName := secretFunc()

			route, err := oc.AdminRouteClient().Route().Routes(oc.Namespace()).Get(routeName, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			sourceURL := fmt.Sprintf(urlTemplate, route.Spec.Host)
			g.By(fmt.Sprintf("creating a new BuildConfig by calling oc new-app -f %q -p SOURCE_SECRET=%s -p SOURCE_URL=%s",
				testBuildFixture, sourceSecretName, sourceURL))
			err = oc.Run("new-app").Args("-f", testBuildFixture, "-p", fmt.Sprintf("SOURCE_SECRET=%s", sourceSecretName), "-p", fmt.Sprintf("SOURCE_URL=%s", sourceURL)).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting a test build")
			br, _ := exutil.StartBuildAndWait(oc, buildConfigName)
			if !br.BuildSuccess {
				exutil.DumpApplicationPodLogs(gitServerDeploymentConfigName, oc)
			}
			br.AssertSuccess()
		}

		g.Describe("Build using a username and password", func() {
			g.It("should create a new build using the internal gitserver", func() {
				testGitAuth("gitserver", gitServerFixture, sourceURLTemplate, func() string {
					g.By(fmt.Sprintf("creating a new secret for the gitserver by calling oc secrets new-basicauth %s --username=%s --password=%s",
						sourceSecretName, gitUserName, gitPassword))
					sa, err := oc.KubeClient().Core().ServiceAccounts(oc.Namespace()).Get("builder", metav1.GetOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
					for _, s := range sa.Secrets {
						if strings.Contains(s.Name, "token") {
							secret, err := oc.KubeClient().Core().Secrets(oc.Namespace()).Get(s.Name, metav1.GetOptions{})
							o.Expect(err).NotTo(o.HaveOccurred())
							err = oc.Run("create").Args(
								"secret",
								"generic",
								sourceSecretName,
								"--type", "kubernetes.io/basic-auth",
								"--from-literal", fmt.Sprintf("username=%s", gitUserName),
								"--from-literal", fmt.Sprintf("password=%s", gitPassword),
								"--from-literal", fmt.Sprintf("ca.crt=%s", string(secret.Data["ca.crt"])),
							).Execute()
							o.Expect(err).NotTo(o.HaveOccurred())
							return sourceSecretName
						}
					}
					return ""
				})
			})
		})

		g.Describe("Build using a service account token", func() {
			g.It("should create a new build using the internal gitserver", func() {
				testGitAuth("gitserver-tokenauth", gitServerTokenAuthFixture, sourceURLTemplate, func() string {
					g.By("assigning the edit role to the builder service account")
					err := oc.Run("policy").Args("add-role-to-user", "edit", "--serviceaccount=builder").Execute()
					o.Expect(err).NotTo(o.HaveOccurred())

					g.By("getting the token secret name for the builder service account")
					sa, err := oc.KubeClient().Core().ServiceAccounts(oc.Namespace()).Get("builder", metav1.GetOptions{})
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
	})
})

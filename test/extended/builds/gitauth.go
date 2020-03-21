package builds

import (
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-builds][Feature:Builds][Slow] can use private repositories as build input", func() {
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
		gitServerFixture = exutil.FixturePath("testdata", "test-gitserver.yaml")
		testBuildFixture = exutil.FixturePath("testdata", "builds", "test-auth-build.yaml")
		oc               = exutil.NewCLI("build-sti-private-repo")
	)

	g.Context("", func() {

		g.BeforeEach(func() {
			exutil.PreTestDump()
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpConfigMapStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		testGitAuth := func(routeName, gitServerYaml, urlTemplate string, secretFunc func() string) {

			err := oc.Run("new-app").Args("-f", gitServerYaml).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("expecting the deployment of the gitserver to be in the Complete phase")
			err = exutil.WaitForDeploymentConfig(oc.KubeClient(), oc.AppsClient().AppsV1(), oc.Namespace(), gitServerDeploymentConfigName, 1, true, oc)
			o.Expect(err).NotTo(o.HaveOccurred())

			sourceSecretName := secretFunc()

			route, err := oc.AdminRouteClient().RouteV1().Routes(oc.Namespace()).Get(routeName, metav1.GetOptions{})
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
				g.Skip("Need to fetch router CA via https://github.com/openshift/cluster-ingress-operator/pull/111")
				testGitAuth("gitserver", gitServerFixture, sourceURLTemplate, func() string {
					g.By(fmt.Sprintf("creating a new secret for the gitserver by calling oc secrets new-basicauth %s --username=%s --password=%s",
						sourceSecretName, gitUserName, gitPassword))
					sa, err := oc.KubeClient().CoreV1().ServiceAccounts(oc.Namespace()).Get("builder", metav1.GetOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
					for _, s := range sa.Secrets {
						if strings.Contains(s.Name, "token") {
							secret, err := oc.KubeClient().CoreV1().Secrets(oc.Namespace()).Get(s.Name, metav1.GetOptions{})
							o.Expect(err).NotTo(o.HaveOccurred())
							err = oc.Run("create").Args(
								"secret",
								"generic",
								sourceSecretName,
								"--type", "kubernetes.io/basic-auth",
								"--from-literal", fmt.Sprintf("username=%s", gitUserName),
								"--from-literal", fmt.Sprintf("password=%s", gitPassword),
								// TODO this needs to come from https://github.com/openshift/cluster-ingress-operator/pull/111 instead
								"--from-literal", fmt.Sprintf("ca.crt=%s", string(secret.Data["service-ca.crt"])),
							).Execute()
							o.Expect(err).NotTo(o.HaveOccurred())
							return sourceSecretName
						}
					}
					return ""
				})
			})
		})
	})
})

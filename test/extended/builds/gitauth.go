package builds

import (
	"fmt"
	"os"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
)

// Execute a series of builds using different authentication methods against a Git repository. The
// service employed is GitHub directly, therefore we can simulate what the final users will be doing,
// and save testing effort of managing a Git server ourselves.
var _ = g.Describe("[sig-builds][Feature:Builds][Slow] can use private repositories as build input", func() {
	defer g.GinkgoRecover()

	const (
		buildConfigName = "gitauthtest"
		// TODO: replace it with a shared testing organization repo with "nodejs-ex" inside.
		httpSourceURL = "https://github.com/otaviof/typescript-ex"
		sshSourceURL  = "git@github.com:otaviof/typescript-ex.git"
	)

	var (
		oc               = exutil.NewCLI("build-sti-github")
		testBuildFixture = exutil.FixturePath("testdata", "builds", "test-auth-build.yaml")
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

		// createBasicAuthSecretFn creates a new "basic-auth" secret with informed credentials. It
		// returns the secret name.
		createBasicAuthSecretFn := func(username, password string) string {
			const sourceSecretName = "github-basic-auth"
			err := oc.Run("create").Args(
				"secret", "generic", sourceSecretName,
				"--type", "kubernetes.io/basic-auth",
				"--from-literal", fmt.Sprintf("username=%s", username),
				"--from-literal", fmt.Sprintf("password=%s", password),
			).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			return sourceSecretName
		}

		// createSSHAuthSecretFn creates a new "ssh-auth" secret with informed private-key. It
		// returns the secret name.
		createSSHAuthSecretFn := func(sshPrivateKey string) string {
			const sourceSecretName = "github-ssh-auth"
			err := oc.Run("create").Args(
				"secret", "generic", sourceSecretName,
				"--type", "kubernetes.io/ssh-auth",
				"--from-literal", fmt.Sprintf("ssh-privatekey=%s\n", sshPrivateKey),
			).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			return sourceSecretName
		}

		// buildWithSecretFn execute a complete build of the informed source URL, using informed
		// secret for authentication. Both parameters are passed to the fixture employed.
		buildWithSecretFn := func(sourceURL, sourceSecretName string) {
			err := oc.Run("new-app").Args(
				"-f", testBuildFixture,
				"-p", fmt.Sprintf("SOURCE_SECRET=%s", sourceSecretName),
				"-p", fmt.Sprintf("SOURCE_URL=%s", sourceURL),
			).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting a test build")
			br, _ := exutil.StartBuildAndWait(oc, buildConfigName)
			br.AssertSuccess()
		}

		g.Describe("Should create a new build", func() {
			// in order to simulate a user using a token on a private github repository, we need to
			// employ basic authentication, using a hardcoded user ("machineuser"), and the actual
			// token as password.
			g.It("using basic-auth with a token", func() {
				// TODO: pass the token via a regular OpenShift-CI method.
				token := os.Getenv("GITHUB_TOKEN")
				buildWithSecretFn(httpSourceURL, createBasicAuthSecretFn("machineuser", token))
			})

			// to simulate a user with SSH based authentication (or "deploy keys") on a private
			// github repository, we need to employ specific authentication method ("ssh-auth"), and
			// make sure the repository URL is adequate for SSH.
			g.It("using ssh-auth with private-key", func() {
				// TODO: pass the SSH private-key via a regular OpenShift-CI method.
				sshPrivateKey := os.Getenv("SSH_AUTH_PRIV")
				buildWithSecretFn(sshSourceURL, createSSHAuthSecretFn(sshPrivateKey))
			})
		})
	})
})

package builds

import (
	"context"
	"fmt"

	"github.com/openshift/origin/test/extended/util"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-builds][Feature:Builds][Slow] can use private repositories as build input", func() {
	defer g.GinkgoRecover()

	const (
		buildConfigName = "gitauth"
	)

	var (
		testBuildFixture = exutil.FixturePath("testdata", "builds", "test-auth-build.yaml")
		oc               = exutil.NewCLI("build-s2i-private-repo")
	)

	g.Context("", func() {

		g.BeforeEach(func() {
			exutil.PreTestDump()
		})

		g.AfterEach(func() {
			if g.CurrentSpecReport().Failed() {
				exutil.DumpPodStates(oc)
				exutil.DumpConfigMapStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		testGitAuth := func(sourceURL string, sourceSecretName string) {
			g.By(fmt.Sprintf("creating a new BuildConfig to clone source via %s", sourceURL))
			err := oc.Run("new-app").Args("-f", testBuildFixture, "-p", fmt.Sprintf("SOURCE_SECRET=%s", sourceSecretName), "-p", fmt.Sprintf("SOURCE_URL=%s", sourceURL)).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting a test build and waiting for success")
			br, _ := exutil.StartBuildAndWait(oc, buildConfigName)
			if !br.BuildSuccess {
				exutil.DumpBuildLogs(buildConfigName, oc)
			}
			br.AssertSuccess()
		}

		g.Describe("build using an HTTP token", func() {

			g.BeforeEach(func() {
				ctx := context.Background()
				httpToken, err := oc.AsAdmin().KubeClient().CoreV1().Secrets("build-e2e-github-secrets").Get(ctx, "github-http-token", metav1.GetOptions{})
				if err != nil && kerrors.IsNotFound(err) {
					g.Skip("required secret build-e2e-github-secrets/github-http-token is missing")
				}
				o.Expect(err).NotTo(o.HaveOccurred())
				copiedHTTPToken := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "github-http-token",
					},
					Data: httpToken.Data,
					Type: httpToken.Type,
				}
				_, err = oc.KubeClient().CoreV1().Secrets(oc.Namespace()).Create(ctx, copiedHTTPToken, metav1.CreateOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
			})

			g.It("should be able to clone source code via an HTTP token [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
				testGitAuth("https://github.com/openshift-github-testing/nodejs-ex-private.git", "github-http-token")
			})

		})

		g.Describe("build using an ssh private key", func() {

			g.BeforeEach(func() {
				// Skip this test when running in FIPS mode
				// FIPS requires ssh-based clone to have a known_hosts file provided, and GitHub's
				// known hosts can be dynamic
				isFIPS, err := util.IsFIPS(oc.AdminKubeClient().CoreV1())
				o.Expect(err).NotTo(o.HaveOccurred())
				if isFIPS {
					g.Skip("skipping ssh git clone test on FIPS cluster")
				}

				ctx := context.Background()
				sshKey, err := oc.AsAdmin().KubeClient().CoreV1().Secrets("build-e2e-github-secrets").Get(ctx, "github-ssh-privatekey", metav1.GetOptions{})
				if err != nil && kerrors.IsNotFound(err) {
					g.Skip("required secret build-e2e-github-secrets/github-ssh-privatekey is missing")
				}
				o.Expect(err).NotTo(o.HaveOccurred())
				copiedSSHKey := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "github-ssh-privatekey",
					},
					Data: sshKey.Data,
					Type: sshKey.Type,
				}
				_, err = oc.KubeClient().CoreV1().Secrets(oc.Namespace()).Create(ctx, copiedSSHKey, metav1.CreateOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
			})

			g.It("should be able to clone source code via ssh [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
				testGitAuth("ssh://git@github.com/openshift-github-testing/nodejs-ex-private.git", "github-ssh-privatekey")
			})

			g.It("should be able to clone source code via ssh using SCP-style URIs [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
				testGitAuth("git@github.com:openshift-github-testing/nodejs-ex-private.git", "github-ssh-privatekey")
			})
		})
	})
})

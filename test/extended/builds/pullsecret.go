package builds

import (
	"context"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-builds][Feature:Builds][Slow] using pull secrets in a build", func() {
	defer g.GinkgoRecover()
	var (
		exampleBuild    = exutil.FixturePath("testdata", "builds", "test-docker-app")
		linkedBuild     = exutil.FixturePath("testdata", "builds", "pullsecret", "linked-nodejs-bc.yaml")
		pullSecretBuild = exutil.FixturePath("testdata", "builds", "pullsecret", "pullsecret-nodejs-bc.yaml")
		oc              = exutil.NewCLI("cli-pullsecret-build")
	)

	g.Context("", func() {
		g.BeforeEach(func() {
			exutil.PreTestDump()
		})

		g.Context("start-build test context", func() {
			g.AfterEach(func() {
				if g.CurrentSpecReport().Failed() {
					exutil.DumpPodStates(oc)
					exutil.DumpPodLogsStartingWith("", oc)
				}
			})

			g.Describe("binary builds", func() {
				g.It("should be able to run a build that is implicitly pulling from the internal registry [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
					g.By("creating a build")
					err := oc.Run("new-build").Args("--binary", "--strategy=docker", "--name=docker").Execute()
					o.Expect(err).NotTo(o.HaveOccurred())
					br, err := exutil.StartBuildAndWait(oc, "docker", fmt.Sprintf("--from-dir=%s", exampleBuild))
					br.AssertSuccess()
				})
			})

			// Test pulling from registry.redhat.io - an authenticated registry
			// CI clusters should have a pull secret to this registry in order for the samples operator to work
			// Note that this registry is known to be flaky
			g.Describe("pulling from an external authenticated registry", func() {
				g.BeforeEach(func() {
					g.By("copying the cluster pull secret to the namespace")
					ps, err := oc.AsAdmin().AdminKubeClient().CoreV1().Secrets("openshift-config").Get(context.Background(), "pull-secret", metav1.GetOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
					localPullSecret := &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name: "local-ps",
						},
						Data: ps.Data,
						Type: ps.Type,
					}
					_, err = oc.KubeClient().CoreV1().Secrets(oc.Namespace()).Create(context.Background(), localPullSecret, metav1.CreateOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
				})

				g.AfterEach(func() {
					g.By("unlinking the cluster pull secret in the namespace")
					oc.Run("secrets").Args("unlink", "builder", "local-ps").Execute()
					g.By("deleting the cluster pull secret in the namespace")
					oc.Run("delete").Args("secret", "local-ps").Execute()
				})

				g.It("should be able to use a pull secret in a build [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
					g.By("creating build config")
					err := oc.Run("create").Args("-f", pullSecretBuild).Execute()
					o.Expect(err).NotTo(o.HaveOccurred())
					g.By("starting build with a pull secret")
					br, err := exutil.StartBuildAndWait(oc, "pullsecret-nodejs", "--build-loglevel=6")
					o.Expect(err).NotTo(o.HaveOccurred())
					br.AssertSuccess()
				})

				g.It("should be able to use a pull secret linked to the builder service account [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
					g.By("linking pull secret with the builder service account")
					err := oc.Run("secrets").Args("link", "builder", "local-ps").Execute()
					o.Expect(err).NotTo(o.HaveOccurred())
					g.By("creating build config")
					err = oc.Run("create").Args("-f", linkedBuild).Execute()
					o.Expect(err).NotTo(o.HaveOccurred())
					g.By("starting a build")
					br, err := exutil.StartBuildAndWait(oc, "linked-nodejs", "--build-loglevel=6")
					o.Expect(err).NotTo(o.HaveOccurred())
					br.AssertSuccess()
				})
			})
		})
	})
})

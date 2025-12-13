package builds

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	buildv1 "github.com/openshift/api/build/v1"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-builds][Feature:Builds][Slow] builds should support proxies", func() {
	defer g.GinkgoRecover()
	var (
		buildFixture = exutil.FixturePath("testdata", "builds", "test-build-proxy.yaml")
		oc           = exutil.NewCLI("build-proxy")
	)

	g.Context("", func() {

		g.BeforeEach(func() {
			exutil.PreTestDump()
		})

		g.JustBeforeEach(func() {
			oc.Run("create").Args("-f", buildFixture).Execute()
		})

		g.AfterEach(func() {
			if g.CurrentSpecReport().Failed() {
				exutil.DumpPodStates(oc)
				exutil.DumpConfigMapStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.Describe("start build with broken proxy", func() {
			g.It("should start a build and wait for the build to fail [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
				g.By("starting the build")

				br, _ := exutil.StartBuildAndWait(oc, "sample-build", "--build-loglevel=5")
				br.AssertFailure()

				g.By("verifying the build sample-build-1 output")
				// The git ls-remote check should exit the build when the remote
				// repository is not accessible. It should never get to the clone.
				buildLog, err := br.Logs()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(buildLog).NotTo(o.ContainSubstring("git clone"))
				o.Expect(buildLog).To(o.MatchRegexp(`unable to access '%s': Failed( to)? connect to`, "https://github.com/openshift/ruby-hello-world.git/"))

				g.By("verifying the build sample-build-1 status")
				o.Expect(br.Build.Status.Phase).Should(o.BeEquivalentTo(buildv1.BuildPhaseFailed))
			})
		})

		g.Describe("start build with broken proxy and a no_proxy override", func() {
			g.It("should start an s2i build and wait for the build to succeed [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
				g.By("starting the build")
				br, _ := exutil.StartBuildAndWait(oc, "sample-s2i-build-noproxy", "--build-loglevel=5")
				br.AssertSuccess()
				buildLog, err := br.Logs()
				o.Expect(err).NotTo(o.HaveOccurred())
				// envuser:password will appear in the log because the full/unstripped env HTTP_PROXY variable is injected
				// into the dockerfile and displayed by docker build.
				o.Expect(buildLog).NotTo(o.ContainSubstring("gituser:password"), "build log should not include proxy credentials")
				o.Expect(buildLog).To(o.ContainSubstring("proxy1"), "build log should include proxy host")
				o.Expect(buildLog).To(o.ContainSubstring("proxy2"), "build log should include proxy host")
				o.Expect(buildLog).To(o.ContainSubstring("proxy3"), "build log should include proxy host")
				o.Expect(buildLog).To(o.ContainSubstring("proxy4"), "build log should include proxy host")
			})
			g.It("should start a docker build and wait for the build to succeed [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
				g.By("starting the build")
				br, _ := exutil.StartBuildAndWait(oc, "sample-docker-build-noproxy", "--build-loglevel=5")
				br.AssertSuccess()
				buildLog, err := br.Logs()
				o.Expect(err).NotTo(o.HaveOccurred())
				// envuser:password will appear in the log because the full/unstripped env HTTP_PROXY variable is injected
				// into the dockerfile and displayed by docker build.
				o.Expect(buildLog).NotTo(o.ContainSubstring("gituser:password"), "build log should not include proxy credentials")
				o.Expect(buildLog).To(o.ContainSubstring("proxy1"), "build log should include proxy host")
				o.Expect(buildLog).To(o.ContainSubstring("proxy2"), "build log should include proxy host")
				o.Expect(buildLog).To(o.ContainSubstring("proxy3"), "build log should include proxy host")
				o.Expect(buildLog).To(o.ContainSubstring("proxy4"), "build log should include proxy host")
			})
		})

		g.Describe("start build with cluster-wide custom PKI", func() {

			g.It("should mount the custom PKI into the build if specified [apigroup:config.openshift.io][apigroup:build.openshift.io]", g.Label("Size:L"), func() {
				ctx := context.TODO()
				proxy, err := oc.AdminConfigClient().ConfigV1().Proxies().Get(ctx, "cluster", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				if len(proxy.Spec.TrustedCA.Name) == 0 {
					g.Skip("cluster custom PKI is not configured")
				}
				caData, err := oc.AsAdmin().KubeClient().CoreV1().ConfigMaps("openshift-config").Get(ctx, proxy.Spec.TrustedCA.Name, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				caBundle, present := caData.Data["ca-bundle.crt"]
				if !present {
					g.Skip("cluster custom PKI is missing key ca-bundle.crt")
				}
				br, _ := exutil.StartBuildAndWait(oc, "sample-docker-build-proxy-ca")
				br.AssertSuccess()
				buildLog, err := br.Logs()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(buildLog).To(o.ContainSubstring(caBundle))
			})

		})
	})
})

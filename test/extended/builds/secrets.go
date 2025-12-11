package builds

import (
	"context"
	"path/filepath"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	e2eoutput "k8s.io/kubernetes/test/e2e/framework/pod/output"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-builds][Feature:Builds][Slow] can use build secrets", func() {
	defer g.GinkgoRecover()
	var (
		buildSecretBaseDir     = exutil.FixturePath("testdata", "builds", "build-secrets")
		secretsFixture         = filepath.Join(buildSecretBaseDir, "test-secret.json")
		secondSecretsFixture   = filepath.Join(buildSecretBaseDir, "test-secret-2.json")
		configMapFixture       = filepath.Join(buildSecretBaseDir, "test-configmap.json")
		secondConfigMapFixture = filepath.Join(buildSecretBaseDir, "test-configmap-2.json")
		isFixture              = filepath.Join(buildSecretBaseDir, "test-is.json")
		dockerBuildFixture     = filepath.Join(buildSecretBaseDir, "test-docker-build.json")
		dockerBuildDockerfile  = filepath.Join(buildSecretBaseDir, "Dockerfile")
		sourceBuildFixture     = filepath.Join(buildSecretBaseDir, "test-s2i-build.json")
		sourceBuildBinDir      = filepath.Join(buildSecretBaseDir, "s2i-binary-dir")
		oc                     = exutil.NewCLIWithPodSecurityLevel("build-secrets", admissionapi.LevelBaseline)
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

		g.Describe("build with secrets and configMaps", func() {

			g.BeforeEach(func() {
				g.By("creating secret and configMap fixtures")
				err := oc.Run("create").Args("-f", secretsFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = oc.Run("create").Args("-f", secondSecretsFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = oc.Run("create").Args("-f", configMapFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = oc.Run("create").Args("-f", secondConfigMapFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("creating test image stream")
				err = oc.Run("create").Args("-f", isFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
			})

			g.It("should contain secrets during the source strategy build [apigroup:build.openshift.io][apigroup:image.openshift.io]", g.Label("Size:L"), func() {
				g.By("creating test build config")
				err := oc.Run("create").Args("-f", sourceBuildFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("starting the test source build")
				br, _ := exutil.StartBuildAndWait(oc, "test", "--from-dir", sourceBuildBinDir)
				br.AssertSuccess()

				g.By("getting the image name")
				image, err := exutil.GetDockerImageReference(oc.ImageClient().ImageV1().ImageStreams(oc.Namespace()), "test", "latest")
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("verifying the build sources were available during build and secrets were not present in the output image")
				pod := exutil.GetPodForContainer(corev1.Container{Name: "test", Image: image})
				e2eoutput.TestContainerOutput(context.TODO(), oc.KubeFramework(), "test-build-secret-source", pod, 0, []string{
					"testsecret/secret1=secret1",
					"testsecret/secret2=secret2",
					"testsecret/secret3=secret3",
					"testsecret2/secret1=secret1",
					"testsecret2/secret2=secret2",
					"testsecret2/secret3=secret3",
					"testconfig/foo=bar",
					"testconfig/red=hat",
					"testconfig/this=that",
					"testconfig2/foo=bar",
					"testconfig2/red=hat",
					"testconfig2/this=that",
				})
			})

			g.It("should contain secrets during the docker strategy build", g.Label("Size:L"), func() {
				g.By("creating test build config")
				err := oc.Run("create").Args("-f", dockerBuildFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("starting the test docker build")
				br, _ := exutil.StartBuildAndWait(oc, "test", "--from-file", dockerBuildDockerfile)
				br.AssertSuccess()

				g.By("getting the image name")
				image, err := exutil.GetDockerImageReference(oc.ImageClient().ImageV1().ImageStreams(oc.Namespace()), "test", "latest")
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("verifying the build sources are present in container output")
				pod := exutil.GetPodForContainer(corev1.Container{Name: "test", Image: image})
				e2eoutput.TestContainerOutput(context.TODO(), oc.KubeFramework(), "test-build-secret-docker", pod, 0, []string{
					"secret1=secret1",
					"relative-secret2=secret2",
					"foo=bar",
					"relative-this=that",
				})
			})
		})
	})
})

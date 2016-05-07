package builds

import (
	"path/filepath"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
	kapi "k8s.io/kubernetes/pkg/api"
)

var _ = g.Describe("[builds][Slow] can use build secrets", func() {
	defer g.GinkgoRecover()
	var (
		buildSecretBaseDir   = exutil.FixturePath("fixtures", "build-secrets")
		secretsFixture       = filepath.Join(buildSecretBaseDir, "test-secret.json")
		secondSecretsFixture = filepath.Join(buildSecretBaseDir, "test-secret-2.json")
		isFixture            = filepath.Join(buildSecretBaseDir, "test-is.json")
		dockerBuildFixture   = filepath.Join(buildSecretBaseDir, "test-docker-build.json")
		sourceBuildFixture   = filepath.Join(buildSecretBaseDir, "test-s2i-build.json")
		oc                   = exutil.NewCLI("build-secrets", exutil.KubeConfigPath())
	)

	g.Describe("build with secrets", func() {
		oc.SetOutputDir(exutil.TestContext.OutputDir)

		g.It("should print the secrets during the source strategy build", func() {
			g.By("creating the sample secret files")
			err := oc.Run("create").Args("-f", secretsFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			err = oc.Run("create").Args("-f", secondSecretsFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("creating the sample source build config and image stream")
			err = oc.Run("create").Args("-f", isFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			err = oc.Run("create").Args("-f", sourceBuildFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting the sample source build")
			out, err := oc.Run("start-build").Args("test", "--follow", "--wait").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("secret1=secret1"))
			o.Expect(out).To(o.ContainSubstring("secret3=secret3"))
			o.Expect(out).To(o.ContainSubstring("relative-secret1=secret1"))
			o.Expect(out).To(o.ContainSubstring("relative-secret3=secret3"))

			g.By("checking the status of the build")
			err = exutil.WaitForABuild(oc.REST().Builds(oc.Namespace()), "test-1", exutil.CheckBuildSuccessFn, exutil.CheckBuildFailedFn)
			if err != nil {
				exutil.DumpBuildLogs("test", oc)
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("getting the image name")
			image, err := exutil.GetDockerImageReference(oc.REST().ImageStreams(oc.Namespace()), "test", "latest")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying the build secrets are not present in the output image")
			pod := exutil.GetPodForContainer(kapi.Container{Name: "test", Image: image})
			oc.KubeFramework().TestContainerOutput("test-build-secret-source", pod, 0, []string{
				"relative-secret1=empty",
				"secret3=empty",
			})
		})

		g.It("should print the secrets during the docker strategy build", func() {
			g.By("creating the sample secret files")
			err := oc.Run("create").Args("-f", secretsFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			err = oc.Run("create").Args("-f", secondSecretsFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("creating the sample source build config and image stream")
			err = oc.Run("create").Args("-f", isFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			err = oc.Run("create").Args("-f", dockerBuildFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting the sample source build")
			out, err := oc.Run("start-build").Args("test", "--follow", "--wait").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("secret1=secret1"))
			o.Expect(out).To(o.ContainSubstring("relative-secret2=secret2"))

			g.By("checking the status of the build")
			err = exutil.WaitForABuild(oc.REST().Builds(oc.Namespace()), "test-1", exutil.CheckBuildSuccessFn, exutil.CheckBuildFailedFn)
			if err != nil {
				exutil.DumpBuildLogs("test", oc)
			}
			o.Expect(err).NotTo(o.HaveOccurred())
		})

	})
})

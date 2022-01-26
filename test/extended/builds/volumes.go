package builds

import (
	"path/filepath"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	deploymentutil "github.com/openshift/origin/test/extended/deployments"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-builds][Feature:Builds][volumes] build volumes", func() {
	var (
		oc                     = exutil.NewCLI("build-volumes")
		baseDir                = exutil.FixturePath("testdata", "builds", "volumes")
		secret                 = filepath.Join(baseDir, "secret.yaml")
		configmap              = filepath.Join(baseDir, "configmap.yaml")
		s2iImageStream         = filepath.Join(baseDir, "s2i-imagestream.yaml")
		s2iBuildConfig         = filepath.Join(baseDir, "s2i-buildconfig.yaml")
		s2iDeploymentConfig    = filepath.Join(baseDir, "s2i-deploymentconfig.yaml")
		dockerImageStream      = filepath.Join(baseDir, "docker-imagestream.yaml")
		dockerBuildConfig      = filepath.Join(baseDir, "docker-buildconfig.yaml")
		dockerDeploymentConfig = filepath.Join(baseDir, "docker-deploymentconfig.yaml")
	)

	g.Context("", func() {

		g.BeforeEach(func() {
			exutil.PreTestDump()
		})

		g.JustBeforeEach(func() {
			g.By("creating a secret")
			err := oc.Run("create").Args("-f", secret).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("creating a configmap")
			err = oc.Run("create").Args("-f", configmap).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpConfigMapStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.It("should mount given secrets and configmaps into the build pod for source strategy builds", func() {
			g.By("creating an imagestream")
			err := oc.Run("create").Args("-f", s2iImageStream).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("creating a build config")
			err = oc.Run("create").Args("-f", s2iBuildConfig).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting a build and waiting for it to complete")
			br, _ := exutil.StartBuildAndWait(oc, "mys2itest")
			br.AssertSuccess()

			g.By("ensuring that the build pod logs contain the provided secret and configmap values")
			buildPodLogs, err := br.Logs()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(buildPodLogs).To(o.ContainSubstring("my-secret-value"))
			o.Expect(buildPodLogs).To(o.ContainSubstring("my-configmap-value"))

			g.By("creating a deployment config")
			err = oc.Run("create").Args("-f", s2iDeploymentConfig).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for the deployment to complete")
			_, err = deploymentutil.WaitForDeployerToComplete(oc, "mys2itest-1", 5*time.Minute)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("ensuring that the secret does not exist in the build image")
			out, err := oc.Run("rsh").Args("dc/mys2itest", "cat", "/var/run/secrets/some-secret/key").Output()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("cat: /var/run/secrets/some-secret/key: No such file or directory"))

			g.By("ensuring that the configmap does not exist in the build image")
			out, err = oc.Run("rsh").Args("dc/mys2itest", "cat", "/var/run/configmaps/some-configmap/key").Output()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("cat: /var/run/configmaps/some-configmap/key: No such file or directory"))
		})

		g.It("should mount given secrets and configmaps into the build pod for docker strategy builds", func() {
			g.By("creating an imagestream")
			err := oc.Run("create").Args("-f", dockerImageStream).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("creating a build config")
			err = oc.Run("create").Args("-f", dockerBuildConfig).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting a build and waiting for it to complete")
			br, _ := exutil.StartBuildAndWait(oc, "mydockertest")
			br.AssertSuccess()

			g.By("ensuring that the build pod logs contain the provided secret and configmap values")
			buildPodLogs, err := br.Logs()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(buildPodLogs).To(o.ContainSubstring("my-secret-value"))
			o.Expect(buildPodLogs).To(o.ContainSubstring("my-configmap-value"))

			g.By("creating a deployment config")
			err = oc.Run("create").Args("-f", dockerDeploymentConfig).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for the deployment to complete")
			_, err = deploymentutil.WaitForDeployerToComplete(oc, "mydockertest-1", 5*time.Minute)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("ensuring that the secret does not exist in the build image")
			out, err := oc.Run("rsh").Args("dc/mydockertest", "cat", "/var/run/secrets/some-secret/key").Output()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("cat: /var/run/secrets/some-secret/key: No such file or directory"))

			g.By("ensuring that the configmap does not exist in the build image")
			out, err = oc.Run("rsh").Args("dc/mydockertest", "cat", "/var/run/configmaps/some-configmap/key").Output()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("cat: /var/run/configmaps/some-configmap/key: No such file or directory"))
		})
	})
})

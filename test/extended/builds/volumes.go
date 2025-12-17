package builds

import (
	"path/filepath"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-builds][Feature:Builds][volumes] build volumes", func() {
	var (
		oc                = exutil.NewCLIWithPodSecurityLevel("build-volumes", admissionapi.LevelBaseline)
		baseDir           = exutil.FixturePath("testdata", "builds", "volumes")
		secret            = filepath.Join(baseDir, "secret.yaml")
		configmap         = filepath.Join(baseDir, "configmap.yaml")
		s2iImageStream    = filepath.Join(baseDir, "s2i-imagestream.yaml")
		s2iBuildConfig    = filepath.Join(baseDir, "s2i-buildconfig.yaml")
		s2iDeployment     = filepath.Join(baseDir, "s2i-deployment.yaml")
		dockerImageStream = filepath.Join(baseDir, "docker-imagestream.yaml")
		dockerBuildConfig = filepath.Join(baseDir, "docker-buildconfig.yaml")
		dockerDeployment  = filepath.Join(baseDir, "docker-deployment.yaml")
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
			if g.CurrentSpecReport().Failed() {
				exutil.DumpPodStates(oc)
				exutil.DumpConfigMapStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.It("should mount given secrets and configmaps into the build pod for source strategy builds [apigroup:image.openshift.io][apigroup:build.openshift.io]", g.Label("Size:L"), func() {
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

			g.By("creating a deployment")
			err = oc.Run("create").Args("-f", s2iDeployment).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			projectName, err := oc.Run("project").Args("-q").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for the deployment to complete")
			err = exutil.WaitForDeploymentReady(oc, "mys2itest", projectName, -1)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("ensuring that the secret does not exist in the build image")
			out, err := oc.Run("rsh").Args("deployment/mys2itest", "cat", "/var/run/secrets/some-secret/key").Output()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("cat: /var/run/secrets/some-secret/key: No such file or directory"))

			g.By("ensuring that the configmap does not exist in the build image")
			out, err = oc.Run("rsh").Args("deployment/mys2itest", "cat", "/var/run/configmaps/some-configmap/key").Output()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("cat: /var/run/configmaps/some-configmap/key: No such file or directory"))
		})

		g.It("should mount given secrets and configmaps into the build pod for docker strategy builds [apigroup:image.openshift.io][apigroup:build.openshift.io]", g.Label("Size:L"), func() {
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

			g.By("creating a deployment")
			err = oc.Run("create").Args("-f", dockerDeployment).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			projectName, err := oc.Run("project").Args("-q").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for the deployment to complete")
			err = exutil.WaitForDeploymentReady(oc, "mydockertest", projectName, -1)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("ensuring that the secret does not exist in the build image")
			out, err := oc.Run("rsh").Args("deployment/mydockertest", "cat", "/var/run/secrets/some-secret/key").Output()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("cat: /var/run/secrets/some-secret/key: No such file or directory"))

			g.By("ensuring that the configmap does not exist in the build image")
			out, err = oc.Run("rsh").Args("deployment/mydockertest", "cat", "/var/run/configmaps/some-configmap/key").Output()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("cat: /var/run/configmaps/some-configmap/key: No such file or directory"))
		})
	})
})

package builds

import (
	"fmt"
	"path/filepath"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	admissionapi "k8s.io/pod-security-admission/api"

	deploymentutil "github.com/openshift/origin/test/extended/deployments"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-builds][Feature:Builds][volumes] build volumes", func() {
	var (
		oc                     = exutil.NewCLIWithPodSecurityLevel("build-volumes", admissionapi.LevelBaseline)
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
			if g.CurrentSpecReport().Failed() {
				exutil.DumpPodStates(oc)
				exutil.DumpConfigMapStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.It("should mount given secrets and configmaps into the build pod for source strategy builds [apigroup:image.openshift.io][apigroup:build.openshift.io][apigroup:apps.openshift.io]", func() {
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

		g.It("should mount given secrets and configmaps into the build pod for docker strategy builds [apigroup:image.openshift.io][apigroup:build.openshift.io][apigroup:apps.openshift.io]", func() {
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

var _ = g.Describe("[sig-builds][Feature:Builds][volumes] csi build volumes within Tech Preview enabled cluster", func() {
	defer g.GinkgoRecover()
	var (
		oc                     = exutil.NewCLIWithPodSecurityLevel("build-volumes-csi", admissionapi.LevelBaseline)
		baseDir                = exutil.FixturePath("testdata", "builds", "volumes")
		secret                 = filepath.Join(baseDir, "secret.yaml")
		s2iDeploymentConfig    = filepath.Join(baseDir, "s2i-deploymentconfig.yaml")
		s2iImageStream         = filepath.Join(baseDir, "s2i-imagestream.yaml")
		dockerDeploymentConfig = filepath.Join(baseDir, "docker-deploymentconfig.yaml")
		dockerImageStream      = filepath.Join(baseDir, "docker-imagestream.yaml")
		// csi enabled volume specifics
		csiSharedSecret                            = filepath.Join(baseDir, "csi-sharedsecret.yaml")
		csiSharedRole                              = filepath.Join(baseDir, "csi-sharedresourcerole.yaml")
		csiSharedRoleBinding                       = filepath.Join(baseDir, "csi-sharedresourcerolebinding.yaml")
		csiS2iBuildConfig                          = filepath.Join(baseDir, "csi-s2i-buildconfig.yaml")
		csiDockerBuildConfig                       = filepath.Join(baseDir, "csi-docker-buildconfig.yaml")
		csiWihthoutResourceRefreshS2iBuildConfig   = filepath.Join(baseDir, "csi-without-rr-s2i-buildconfig.yaml")
		csiWithoutResourceRefreshDockerBuildConfig = filepath.Join(baseDir, "csi-without-rr-docker-buildconfig.yaml")
	)

	g.Context("[apigroup:config.openshift.io]", func() {
		g.BeforeEach(func() {
			exutil.PreTestDump()
		})

		g.JustBeforeEach(func() {
			//TODO remove this check once https://github.com/openshift/cluster-storage-operator/pull/335 and https://github.com/openshift/openshift-controller-manager/pull/250 have merged
			if !exutil.IsTechPreviewNoUpgrade(oc) {
				g.Skip("the test is not expected to work within Tech Preview disabled clusters")
			}
			// create the secret to share in a new namespace
			g.By("creating a secret")
			err := oc.AsAdmin().Run("--namespace=default", "apply").Args("-f", secret).Execute()
			if err != nil && !apierrors.IsAlreadyExists(err) {
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			// create the csi shared secret object
			g.By("creating a csi shared secret resource")
			err = oc.AsAdmin().Run("apply").Args("-f", csiSharedSecret).Execute()
			if err != nil && !apierrors.IsAlreadyExists(err) {
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			// process the role to grant use of the share
			g.By("creating a csi shared role resource")
			err = oc.AsAdmin().Run("create").Args("-f", csiSharedRole).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			// process the rolebinding to grant use of the share
			g.By("creating a csi shared role binding resource")
			rolebinding, _, err := oc.AsAdmin().Run("process").Args("-f", csiSharedRoleBinding, "-p", fmt.Sprintf("NAMESPACE=%s", oc.Namespace())).Outputs()
			o.Expect(err).NotTo(o.HaveOccurred())
			err = oc.AsAdmin().Run("create").Args("-f", "-").InputString(rolebinding).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.AfterEach(func() {
			if g.CurrentSpecReport().Failed() {
				exutil.DumpPodStates(oc)
				exutil.DumpConfigMapStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.It("should mount given csi shared resource secret into the build pod for source strategy builds [apigroup:image.openshift.io][apigroup:build.openshift.io][apigroup:apps.openshift.io]", func() {
			g.By("creating an imagestream")
			err := oc.Run("create").Args("-f", s2iImageStream).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("creating a build config")
			err = oc.Run("create").Args("-f", csiS2iBuildConfig).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting a build and waiting for it to complete")
			br, _ := exutil.StartBuildAndWait(oc, "mys2itest")
			br.AssertSuccess()

			g.By("ensuring that the build pod logs contain the provided shared secret")
			buildPodLogs, err := br.Logs()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(buildPodLogs).To(o.ContainSubstring("my-secret-value"))

			g.By("creating a deployment config")
			err = oc.Run("create").Args("-f", s2iDeploymentConfig).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for the deployment to complete")
			_, err = deploymentutil.WaitForDeployerToComplete(oc, "mys2itest-1", 5*time.Minute)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("ensuring that the shared secret does not exist in the build image")
			out, err := oc.Run("rsh").Args("dc/mys2itest", "cat", "/var/run/secrets/some-secret/key").Output()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("cat: /var/run/secrets/some-secret/key: No such file or directory"))
		})

		g.It("should mount given csi shared resource secret without resource refresh into the build pod for source strategy builds [apigroup:image.openshift.io][apigroup:build.openshift.io][apigroup:apps.openshift.io]", func() {
			g.By("creating an imagestream")
			err := oc.Run("create").Args("-f", s2iImageStream).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("creating a build config")
			err = oc.Run("create").Args("-f", csiWihthoutResourceRefreshS2iBuildConfig).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting a build and waiting for it to complete")
			br, _ := exutil.StartBuildAndWait(oc, "mys2itest")
			br.AssertSuccess()

			g.By("ensuring that the build pod logs contain the provided shared secret")
			buildPodLogs, err := br.Logs()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(buildPodLogs).To(o.ContainSubstring("my-secret-value"))

			g.By("creating a deployment config")
			err = oc.Run("create").Args("-f", s2iDeploymentConfig).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for the deployment to complete")
			_, err = deploymentutil.WaitForDeployerToComplete(oc, "mys2itest-1", 5*time.Minute)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("ensuring that the shared secret does not exist in the build image")
			out, err := oc.Run("rsh").Args("dc/mys2itest", "cat", "/var/run/secrets/some-secret/key").Output()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("cat: /var/run/secrets/some-secret/key: No such file or directory"))
		})

		g.It("should mount given csi shared resource secret into the build pod for docker strategy builds [apigroup:image.openshift.io][apigroup:build.openshift.io][apigroup:apps.openshift.io]", func() {
			g.By("creating an imagestream")
			err := oc.Run("create").Args("-f", dockerImageStream).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("creating a build config")
			err = oc.Run("create").Args("-f", csiDockerBuildConfig).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting a build and waiting for it to complete")
			br, _ := exutil.StartBuildAndWait(oc, "mydockertest")
			br.AssertSuccess()

			g.By("ensuring that the build pod logs contain the provided shared")
			buildPodLogs, err := br.Logs()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(buildPodLogs).To(o.ContainSubstring("my-secret-value"))

			g.By("creating a deployment config")
			err = oc.Run("create").Args("-f", dockerDeploymentConfig).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for the deployment to complete")
			_, err = deploymentutil.WaitForDeployerToComplete(oc, "mydockertest-1", 5*time.Minute)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("ensuring that the shared secret does not exist in the build image")
			out, err := oc.Run("rsh").Args("dc/mydockertest", "cat", "/var/run/secrets/some-secret/key").Output()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("cat: /var/run/secrets/some-secret/key: No such file or directory"))
		})

		g.It("should mount given csi shared resource secret without resource refresh into the build pod for docker strategy builds [apigroup:image.openshift.io][apigroup:build.openshift.io][apigroup:apps.openshift.io]", func() {
			g.By("creating an imagestream")
			err := oc.Run("create").Args("-f", dockerImageStream).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("creating a build config")
			err = oc.Run("create").Args("-f", csiWithoutResourceRefreshDockerBuildConfig).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting a build and waiting for it to complete")
			br, _ := exutil.StartBuildAndWait(oc, "mydockertest")
			br.AssertSuccess()

			g.By("ensuring that the build pod logs contain the provided shared")
			buildPodLogs, err := br.Logs()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(buildPodLogs).To(o.ContainSubstring("my-secret-value"))

			g.By("creating a deployment config")
			err = oc.Run("create").Args("-f", dockerDeploymentConfig).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for the deployment to complete")
			_, err = deploymentutil.WaitForDeployerToComplete(oc, "mydockertest-1", 5*time.Minute)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("ensuring that the shared secret does not exist in the build image")
			out, err := oc.Run("rsh").Args("dc/mydockertest", "cat", "/var/run/secrets/some-secret/key").Output()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("cat: /var/run/secrets/some-secret/key: No such file or directory"))
		})
	})
})

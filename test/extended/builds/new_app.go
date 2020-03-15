package builds

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	deployutil "github.com/openshift/origin/test/extended/deployments"
	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	a58 = "a234567890123456789012345678901234567890123456789012345678"
	a59 = "a2345678901234567890123456789012345678901234567890123456789"
)

var _ = g.Describe("[sig-builds][Feature:Builds] oc new-app", func() {
	// Previously, the maximum length of app names creatable by new-app has
	// inadvertently been decreased, e.g. by creating an annotation somewhere
	// whose name itself includes the app name.  Ensure we can create and fully
	// deploy an app with a 58 character name [63 maximum - len('-9999' suffix)].

	oc := exutil.NewCLI("new-app", exutil.KubeConfigPath())

	g.Context("", func() {

		g.BeforeEach(func() {
			exutil.PreTestDump()
		})

		g.JustBeforeEach(func() {
			g.By("waiting on the local namespace builder/default SAs")
			err := exutil.WaitForServiceAccount(oc.KubeClient().CoreV1().ServiceAccounts(oc.Namespace()), "builder")
			o.Expect(err).NotTo(o.HaveOccurred())
			err = exutil.WaitForServiceAccount(oc.KubeClient().CoreV1().ServiceAccounts(oc.Namespace()), "default")
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By("waiting for openshift namespace imagestreams")
			err = exutil.WaitForOpenShiftNamespaceImageStreams(oc)
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpConfigMapStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
			deployutil.DeploymentConfigFailureTrap(oc, a58, g.CurrentGinkgoTestDescription().Failed)
			deployutil.DeploymentConfigFailureTrap(oc, a59, g.CurrentGinkgoTestDescription().Failed)
		})

		g.It("should succeed with a --name of 58 characters", func() {
			g.By("calling oc new-app")
			err := oc.Run("new-app").Args("https://github.com/sclorg/nodejs-ex", "--name", a58, "--build-env=BUILD_LOGLEVEL=5").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for the build to complete")
			err = exutil.WaitForABuild(oc.BuildClient().BuildV1().Builds(oc.Namespace()), a58+"-1", nil, nil, nil)
			if err != nil {
				exutil.DumpBuildLogs(a58, oc)
				exutil.DumpPodStates(oc)
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for the deployment to complete")
			err = exutil.WaitForDeploymentConfig(oc.KubeClient(), oc.AppsClient().AppsV1(), oc.Namespace(), a58, 1, true, oc)
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.It("should fail with a --name longer than 58 characters", func() {
			g.By("calling oc new-app")
			out, err := oc.Run("new-app").Args("https://github.com/sclorg/nodejs-ex", "--name", a59).Output()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("error: invalid name: "))
		})

		g.It("should succeed with an imagestream", func() {
			// Bug 1767163 - oc new-app with --image-stream produced invalid labels
			g.By("calling oc new-app with imagestream")
			out, err := oc.Run("new-app").Args("https://github.com/sclorg/nodejs-ex", "--image-stream=nodejs:latest").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).NotTo(o.ContainSubstring("error:"))
		})
	})
})

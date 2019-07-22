package builds

import (
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

// e2e tests of the build controller configuration.
// These are tagged [Serial] because each test modifies the cluster-wide build controller config.
var _ = g.Describe("[Feature:Builds][Serial][Slow][Disruptive] alter builds via cluster configuration", func() {
	defer g.GinkgoRecover()
	var (
		buildFixture           = exutil.FixturePath("testdata", "builds", "test-build.yaml")
		defaultConfigFixture   = exutil.FixturePath("testdata", "builds", "cluster-config.yaml")
		blacklistConfigFixture = exutil.FixturePath("testdata", "builds", "cluster-config", "registry-blacklist.yaml")
		whitelistConfigFixture = exutil.FixturePath("testdata", "builds", "cluster-config", "registry-whitelist.yaml")
		oc                     = exutil.NewCLI("build-cluster-config", exutil.KubeConfigPath())
	)

	g.Context("", func() {

		g.BeforeEach(func() {
			exutil.PreTestDump()
		})

		g.JustBeforeEach(func() {
			g.By("waiting for default service account")
			err := exutil.WaitForServiceAccount(oc.KubeClient().CoreV1().ServiceAccounts(oc.Namespace()), "default")
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By("waiting for builder service account")
			err = exutil.WaitForServiceAccount(oc.KubeClient().CoreV1().ServiceAccounts(oc.Namespace()), "builder")
			o.Expect(err).NotTo(o.HaveOccurred())
			oc.Run("create").Args("-f", buildFixture).Execute()
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
				exutil.DumpConfigMapStates(oc)
			}
			oc.AsAdmin().Run("apply").Args("-f", defaultConfigFixture).Execute()
		})

		g.Context("registries config context", func() {

			g.It("should default registry search to docker.io for image pulls", func() {
				g.Skip("TODO: disabled due to https://bugzilla.redhat.com/show_bug.cgi?id=1685185")
				g.By("apply default cluster configuration")
				err := oc.AsAdmin().Run("apply").Args("-f", defaultConfigFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				g.By("waiting 1s for build controller configuration to propagate")
				time.Sleep(1 * time.Second)
				g.By("starting build sample-build and waiting for success")
				// Image used by sample-build (centos/ruby-25-centos7) is only available on docker.io
				br, err := exutil.StartBuildAndWait(oc, "sample-build")
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertSuccess()
				g.By("expecting the build logs to indicate docker.io was the default registry")
				buildLog, err := br.LogsNoTimestamp()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(buildLog).To(o.ContainSubstring("defaulting registry to docker.io"))
			})

			g.It("should allow registries to be blacklisted", func() {
				g.Skip("TODO: disabled due to https://bugzilla.redhat.com/show_bug.cgi?id=1685185")
				g.By("apply blacklist cluster configuration")
				err := oc.AsAdmin().Run("apply").Args("-f", blacklistConfigFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				g.By("waiting 1s for build controller configuration to propagate")
				time.Sleep(1 * time.Second)
				g.By("starting build sample-build-docker-args-preset and waiting for failure")
				br, err := exutil.StartBuildAndWait(oc, "sample-build-docker-args-preset")
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertFailure()
				g.By("expecting the build logs to indicate the image was rejected")
				buildLog, err := br.LogsNoTimestamp()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(buildLog).To(o.ContainSubstring("Source image rejected"))
			})

			g.It("should allow registries to be whitelisted", func() {
				g.Skip("TODO: disabled due to https://bugzilla.redhat.com/show_bug.cgi?id=1685185")
				g.By("apply whitelist cluster configuration")
				err := oc.AsAdmin().Run("apply").Args("-f", whitelistConfigFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				g.By("waiting 1s for build controller configuration to propagate")
				time.Sleep(1 * time.Second)
				g.By("starting build sample-build-docker-args-preset and waiting for failure")
				br, err := exutil.StartBuildAndWait(oc, "sample-build-docker-args-preset")
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertFailure()
				g.By("expecting the build logs to indicate the image was rejected")
				buildLog, err := br.LogsNoTimestamp()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(buildLog).To(o.ContainSubstring("Source image rejected"))
			})

		})
	})
})

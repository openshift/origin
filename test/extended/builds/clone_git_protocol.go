package builds

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-builds][Feature:Builds] clone repository using git:// protocol", func() {
	var (
		oc = exutil.NewCLI("build-clone-git-protocol")
	)

	g.Context("", func() {

		g.BeforeEach(func() {
			exutil.PreTestDump()
		})

		g.JustBeforeEach(func() {
			g.By("waiting for openshift namespace imagestreams")
			err := exutil.WaitForOpenShiftNamespaceImageStreams(oc)
			o.Expect(err).NotTo(o.HaveOccurred())

		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpConfigMapStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.It("should clone using git:// if no proxy is configured", func() {

			if true {
				// TODO:
				g.Skip("test disabled due to https://bugzilla.redhat.com/show_bug.cgi?id=2019433 and https://github.blog/2021-09-01-improving-git-protocol-security-github/#git-protocol-troubleshooting: 'The unauthenticated git protocol on port 9418 is no longer supported'")
				return
			}

			proxyConfigured, err := exutil.IsClusterProxyEnabled(oc)
			o.Expect(err).NotTo(o.HaveOccurred())
			if proxyConfigured {
				g.Skip("Skipping test because proxy is configured")
			}

			g.By("creating a new application using the git:// protocol")
			err = oc.Run("new-app").Args("git://github.com/sclorg/ruby-ex.git").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})
})

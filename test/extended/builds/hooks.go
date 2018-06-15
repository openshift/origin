package builds

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:Builds][Slow] testing build configuration hooks", func() {
	defer g.GinkgoRecover()
	var (
		buildFixture = exutil.FixturePath("testdata", "builds", "test-build-postcommit.json")
		oc           = exutil.NewCLI("cli-test-hooks", exutil.KubeConfigPath())
	)

	g.Context("", func() {

		g.BeforeEach(func() {
			exutil.DumpDockerInfo()
		})

		g.JustBeforeEach(func() {
			g.By("waiting for builder service account")
			err := exutil.WaitForBuilderAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()))
			o.Expect(err).NotTo(o.HaveOccurred())
			oc.Run("create").Args("-f", buildFixture).Execute()

			g.By("waiting for istag to initialize")
			exutil.WaitForAnImageStreamTag(oc, oc.Namespace(), "busybox", "1")
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.Describe("testing postCommit hook", func() {

			g.It("successful postCommit script with args", func() {
				err := oc.Run("patch").Args("bc/busybox", "-p", `{"spec":{"postCommit":{"script":"echo hello $1","args":["world"],"command":null}}}`).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				br, _ := exutil.StartBuildAndWait(oc, "busybox")
				br.AssertSuccess()
				o.Expect(br.Logs()).To(o.ContainSubstring("hello world"))
			})

			g.It("successful postCommit explicit command", func() {
				err := oc.Run("patch").Args("bc/busybox", "-p", `{"spec":{"postCommit":{"command":["sh","-c"],"args":["echo explicit command"],"script":""}}}`).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				br, _ := exutil.StartBuildAndWait(oc, "busybox")
				br.AssertSuccess()
				o.Expect(br.Logs()).To(o.ContainSubstring("explicit command"))
			})

			g.It("successful postCommit default entrypoint", func() {
				err := oc.Run("patch").Args("bc/busybox", "-p", `{"spec":{"postCommit":{"args":["echo","default entrypoint"],"command":null,"script":""}}}`).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				br, _ := exutil.StartBuildAndWait(oc, "busybox")
				br.AssertSuccess()
				o.Expect(br.Logs()).To(o.ContainSubstring("default entrypoint"))
			})

			g.It("failing postCommit script", func() {
				err := oc.Run("patch").Args("bc/busybox", "-p", `{"spec":{"postCommit":{"script":"echo about to fail && false","command":null}}}`).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				br, _ := exutil.StartBuildAndWait(oc, "busybox")
				br.AssertFailure()
				o.Expect(br.Logs()).To(o.ContainSubstring("about to fail"))
			})

			g.It("failing postCommit explicit command", func() {
				err := oc.Run("patch").Args("bc/busybox", "-p", `{"spec":{"postCommit":{"command":["sh","-c"],"args":["echo about to fail && false"],"script":""}}}`).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				br, _ := exutil.StartBuildAndWait(oc, "busybox")
				br.AssertFailure()
				o.Expect(br.Logs()).To(o.ContainSubstring("about to fail"))
			})

			g.It("failing postCommit default entrypoint", func() {
				err := oc.Run("patch").Args("bc/busybox", "-p", `{"spec":{"postCommit":{"args":["sh","-c","echo about to fail && false"],"command":null,"script":""}}}`).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				br, _ := exutil.StartBuildAndWait(oc, "busybox")
				br.AssertFailure()
				o.Expect(br.Logs()).To(o.ContainSubstring("about to fail"))
			})

		})
	})
})

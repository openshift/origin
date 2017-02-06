package cli

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[cli] oc debug", func() {
	oc := exutil.NewCLI("oc-debug", exutil.KubeConfigPath())
	templatePath := exutil.FixturePath("testdata", "test-cli-debug.yaml")

	g.JustBeforeEach(func() {
		g.By("waiting for builder service account")
		err := exutil.WaitForBuilderAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("calling oc create -f " + templatePath)
		err = oc.Run("create").Args("-f", templatePath).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		exutil.WaitForAnImageStreamTag(oc, oc.Namespace(), "busybox", "latest")
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("should print the container entrypoint/command", func() {
		out, err := oc.Run("debug").Args("dc/busybox1").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("Debugging with pod/busybox1-debug, original command: sh\n"))
	})

	g.It("should print the overridden container entrypoint/command", func() {
		out, err := oc.Run("debug").Args("dc/busybox2").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("Debugging with pod/busybox2-debug, original command: foo bar baz qux\n"))
	})
})

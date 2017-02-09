package cli

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[cli][Slow] oc debug", func() {
	oc := exutil.NewCLI("oc-debug", exutil.KubeConfigPath())
	templatePath := exutil.FixturePath("testdata", "test-cli-debug.yaml")

	g.JustBeforeEach(func() {
		g.By("waiting for builder service account")
		err := exutil.WaitForBuilderAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("calling oc create -f " + templatePath)
		err = oc.Run("create").Args("-f", templatePath).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		exutil.WaitForAnImageStreamTag(oc, oc.Namespace(), "local-busybox", "latest")
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("should print the imagestream-based container entrypoint/command", func() {
		out, err := oc.Run("debug").Args("dc/local-busybox1").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("Debugging with pod/local-busybox1-debug, original command: sh\n"))
	})

	g.It("should print the overridden imagestream-based container entrypoint/command", func() {
		out, err := oc.Run("debug").Args("dc/local-busybox2").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("Debugging with pod/local-busybox2-debug, original command: foo bar baz qux\n"))
	})

	g.It("should print the docker image-based container entrypoint/command", func() {
		out, err := oc.Run("debug").Args("dc/busybox1").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("Debugging with pod/busybox1-debug, original command: sh\n"))
	})

	g.It("should print the overridden docker image-based container entrypoint/command", func() {
		out, err := oc.Run("debug").Args("dc/busybox2").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("Debugging with pod/busybox2-debug, original command: foo bar baz qux\n"))
	})
})

package cli

import (
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-cli] oc completion", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("oc-completion")

	g.It("returns expected help messages", g.Label("Size:S"), func() {
		out, err := oc.Run("completion").Args("-h").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("interactive completion of oc commands"))

		out, err = oc.Run("completion").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("Shell not specified."))

		_, err = oc.Run("completion").Args("bash").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = oc.Run("completion").Args("zsh").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		out, err = oc.Run("completion").Args("test_shell").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("Unsupported shell type \"test_shell\""))
	})
})

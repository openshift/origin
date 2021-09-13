package cli

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-cli] oc project", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("oc-project").AsAdmin()

	g.It("--show-labels works for projects", func() {
		out, err := oc.Run("label").Args("namespace", oc.Namespace(), "foo=bar").Output()
		o.Expect(out).To(o.ContainSubstring("labeled"))
		o.Expect(err).NotTo(o.HaveOccurred())

		out, err = oc.Run("get").Args("project", oc.Namespace(), "--show-labels").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("foo=bar"))
	})
})

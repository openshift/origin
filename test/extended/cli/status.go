package cli

import (
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-cli] oc status", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("oc-status")

	g.It("returns expected help messages [apigroup:project.openshift.io][apigroup:build.openshift.io][apigroup:image.openshift.io][apigroup:apps.openshift.io][apigroup:route.openshift.io]", func() {
		out, err := oc.Run("status").Args("-h").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("oc describe buildconfig"))

		out, err = oc.Run("status").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("oc new-app"))
	})
})

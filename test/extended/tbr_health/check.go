package tbr_health

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-devex] check registry.redhat.io is available and samples operator can import sample imagestreams", func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLI("samples-health-check")
	)
	g.It("run sample related validations", func() {
		err := exutil.WaitForOpenShiftNamespaceImageStreams(oc)
		if err != nil {
			// so the error string shows up in the top level ginkgo message
			o.Expect(err.Error()).To(o.BeEmpty())
		}
	})
})

package networking

import (
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-network] ServiceCIDR", func() {
	oc := exutil.NewCLIWithoutNamespace("servicecidr")

	g.BeforeEach(func() {
		// The VAP is created by CNO, which doesn't run on MicroShift
		isMicroshift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroshift {
			g.Skip("Feature is not currently blocked on Microshift")
		}
	})

	g.It("should be blocked", g.Label("Size:S"), func() {
		g.By("Trying to create a new ServiceCIDR")
		yaml := exutil.FixturePath("testdata", "servicecidr.yaml")
		err := oc.AsAdmin().Run("create").Args("-f", yaml).Execute()
		if err == nil {
			// This shouldn't have worked! We'll fail below, but delete the
			// ServiceCIDR first because otherwise it may cause spurious
			// failures throughout the rest of the test run.
			_ = oc.AsAdmin().Run("delete").Args("newcidr1").Execute()
		}
		o.Expect(err).To(o.HaveOccurred(), "Creating a ServiceCIDR should have been blocked by ValidatingAdmissionPolicy")

		g.By("Trying to modify an existing ServiceCIDR")
		err = oc.AsAdmin().Run("annotate").Args("servicecidr", "kubernetes", "e2etest=success").Execute()
		o.Expect(err).To(o.HaveOccurred(), "Modifying existing ServiceCIDR should have been blocked by ValidatingAdmissionPolicy")
	})
})

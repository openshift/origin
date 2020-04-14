package marketplace

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-operator][Feature:Marketplace] Marketplace manage the default opsrc", func() {

	defer g.GinkgoRecover()

	var (
		oc            = exutil.NewCLI("marketplace")
		marketplaceNs = "openshift-marketplace"
		timeWait      = 100
	)
	g.AfterEach(func() {
	})
	// Test the marketplace manage the default opsrc
	g.It("manage the default opsrc [Serial]", func() {

		// Get the default opsrc of cluster
		output, err := oc.AsAdmin().Run("get").Args("operatorsource", "-o", "name", "-n", marketplaceNs).Output()
		// Output get redhat-operators, community-operators, certified-operators
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).Should(o.ContainSubstring("redhat-operators"))
		o.Expect(output).Should(o.ContainSubstring("community-operators"))
		o.Expect(output).Should(o.ContainSubstring("certified-operators"))

		// Delete default opsrc "redhat-operators"
		_, err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("operatorsource", "redhat-operators", "-n", marketplaceNs).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Edit default opsrc "community-operators" to be invalid one
		_, err = oc.AsAdmin().WithoutNamespace().Run("patch").Args("operatorsource/community-operators", "-p", `{"spec":{"registryNamespace": "noExistRegistoryforTest"}}`, "--type=merge", "-n", marketplaceNs).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Check the status of opsrc "redhat-operators"	"community-operators"
		err = waitForSource(oc, "operatorsource", "redhat-operators", timeWait, marketplaceNs)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = waitForSource(oc, "operatorsource", "community-operators", timeWait, marketplaceNs)
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})

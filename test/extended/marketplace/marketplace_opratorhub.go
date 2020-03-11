package marketplace

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:Marketplace][Serial] Marketplace operatorhub test", func() {

	defer g.GinkgoRecover()

	var (
		oc            = exutil.NewCLI("marketplace", exutil.KubeConfigPath())
		marketplaceNs = "openshift-marketplace"
	)

	//OCP-24588 test the operatorhub config impact the default opsrc"
	g.It("[OCP-24588] operatorhub config test", func() {

		// Get the default opsrc of cluster
		output, err := oc.AsAdmin().Run("get").Args("operatorsource", "-o", "name", "-n", marketplaceNs).Output()
		// Output get redhat-operators, community-operators, certified-operators
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(output).Should(o.ContainSubstring("redhat-operators"))
		o.Expect(output).Should(o.ContainSubstring("community-operators"))
		o.Expect(output).Should(o.ContainSubstring("certified-operators"))

		// Set the disableAll to be true, and the redhat-operators' disable to be true, community-operators' disable to be false
		_, err = oc.AsAdmin().Run("patch").Args("operatorhub/cluster", "-p", `{"spec":{"disableAllDefaultSources": true}}`, "--type=merge").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = oc.AsAdmin().Run("patch").Args("operatorhub/cluster", "-p", `{"spec":{"sources":[{"disabled": true,"name": "redhat-operators"},{"disabled": false,"name": "community-operators"}]}}`, "--type=merge").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		output, err = oc.AsAdmin().Run("get").Args("operatorsource", "-o", "name", "-n", marketplaceNs).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		// Output doesn't have redhat-operators, certified-operators
		o.Expect(output).ShouldNot(o.ContainSubstring("redhat-operators"))
		o.Expect(output).ShouldNot(o.ContainSubstring("certified-operators"))

		// Set the disableAll to be false
		_, err = oc.AsAdmin().Run("patch").Args("operatorhub/cluster", "-p", `{"spec":{"disableAllDefaultSources": false}}`, "--type=merge").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		output, err = oc.AsAdmin().Run("get").Args("operatorsource", "-o", "name", "-n", marketplaceNs).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		// Output doesn't have redhat-operators
		o.Expect(output).ShouldNot(o.ContainSubstring("redhat-operators"))

		// Set the redhat-operators enable
		_, err = oc.AsAdmin().Run("patch").Args("operatorhub/cluster", "-p", `{"spec":{"sources":[{"disabled": false,"name": "redhat-operators"}]}}`, "--type=merge").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})
